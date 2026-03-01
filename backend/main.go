// @title           SuperNetworkAI API
// @version         1.0
// @description     Ikigai-based AI networking platform — Go REST + WebSocket backend
// @host            localhost:3001
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"

	_ "supernetwork/backend/docs"
	"supernetwork/backend/internal/config"
	"supernetwork/backend/internal/db"
	"supernetwork/backend/internal/handler"
	"supernetwork/backend/internal/health"
	"supernetwork/backend/internal/middleware"
	"supernetwork/backend/internal/model"
	"supernetwork/backend/internal/service"
	"supernetwork/backend/internal/service/embedding"
	"supernetwork/backend/internal/service/llm"
	"supernetwork/backend/internal/ws"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg := config.Load()

	// --- Database ---
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// --- Embedding provider (swapped via EMBEDDING_PROVIDER env var) ---
	var embedProvider interface {
		Embed(ctx context.Context, text string) ([]float32, error)
	}
	if cfg.EmbeddingProvider == "nomic" {
		embedProvider = embedding.NewNomicProvider(cfg.NomicAPIKey)
	} else {
		embedProvider = embedding.NewOllamaProvider(cfg.OllamaBaseURL)
	}

	// --- LLM services ---
	ikigaiSummariser := llm.NewIkigaiSummariser(cfg.GroqAPIKey)
	cvStructurer := llm.NewCVStructurer(cfg.GroqAPIKey)
	nlSearchParser := llm.NewNLSearchParser(cfg.GroqAPIKey)
	matchExplainer := llm.NewMatchExplainer(cfg.GroqAPIKey)

	// --- WaitGroup for goroutine tracking (graceful shutdown) ---
	var wg sync.WaitGroup

	// --- Background services ---
	handler.StartTokenPurger()

	// --- Fiber app ---
	app := fiber.New(fiber.Config{
		ErrorHandler: model.ErrorHandler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	})

	// --- Global middleware ---
	bffOrigin := os.Getenv("BFF_ORIGIN")
	if bffOrigin == "" {
		bffOrigin = "http://localhost:3000"
	}
	app.Use(middleware.Recovery(logger))
	app.Use(middleware.CORS(bffOrigin))
	app.Use(middleware.Logger(logger))

	// --- Health routes ---
	h := health.New(pool)
	app.Get("/healthz", h.Liveness)
	app.Get("/readyz", h.Readiness)

	// --- Swagger UI (CDN-hosted UI + spec from /swagger/doc.json) ---
	app.Get("/swagger/doc.json", handler.SwaggerJSON)
	app.Get("/swagger", handler.SwaggerUI)

	// --- BFF JWT secret for auth middleware ---
	bffSecret := []byte(cfg.BffJWTSecret)

	// --- Services ---
	matchSvc := service.NewMatchService(pool, matchExplainer, logger)

	// --- Handlers ---
	authH := handler.NewAuthHandler(cfg.WSTokenSecret)
	userH := handler.NewUserHandler(pool, logger)
	profileH := handler.NewProfileHandler(pool, embedProvider, matchSvc, &wg, logger)
	onboardingH := handler.NewOnboardingHandler(pool, embedProvider, ikigaiSummariser, cvStructurer, matchSvc, &wg, logger)
	matchH := handler.NewMatchHandler(matchSvc, logger)
	searchH := handler.NewSearchHandler(pool, nlSearchParser, embedProvider, logger)
	connH := handler.NewConnectionHandler(pool, logger)
	internalH := handler.NewInternalHandler(pool, matchSvc, &wg, logger)
	convH := handler.NewConversationHandler(pool, logger)

	// --- WebSocket hub ---
	authHRef := authH // capture for closure
	hub := ws.NewHub(pool, authHRef.ValidateWSToken, logger)

	// --- API v1 routes (all require JWT) ---
	api := app.Group("/api/v1", middleware.RequireAuth(bffSecret))

	api.Post("/auth/ws-token", authH.IssueWSToken)

	api.Get("/users/me", userH.GetMe)
	api.Get("/users/:id", userH.GetByID)

	api.Patch("/profiles/me", profileH.UpdateProfile)
	api.Patch("/profiles/me/visibility", profileH.SetVisibility)

	api.Post("/onboarding/ikigai", onboardingH.SaveIkigai)
	api.Post("/onboarding/complete", onboardingH.CompleteOnboarding)
	api.Post("/onboarding/import-cv", onboardingH.ImportCV)

	api.Get("/matches", matchH.GetMatches)
	api.Post("/matches/:matchedUserId/dismiss", matchH.DismissMatch)
	api.Get("/matches/:matchedUserId/explanation", matchH.GetExplanation)

	api.Post("/search", searchH.Search)

	api.Post("/connections", connH.CreateConnection)
	api.Get("/connections", connH.ListConnections)
	api.Get("/connections/status/:userId", connH.GetStatus)
	api.Patch("/connections/:id", connH.UpdateConnection)

	api.Post("/conversations", convH.CreateConversation)
	api.Get("/conversations", convH.ListConversations)
	api.Get("/conversations/:id/messages", convH.GetMessages)
	api.Patch("/conversations/:id/read", convH.MarkRead)

	// --- WebSocket (auth via first-message token, not JWT) ---
	app.Get("/ws", func(c fiber.Ctx) error {
		return hub.Upgrade(c.RequestCtx())
	})

	// --- Internal routes (Cloud Scheduler / cron) ---
	internal := app.Group("/internal", middleware.RequireInternal(cfg.InternalAPISecret))
	internal.Post("/matches/refresh", internalH.RefreshMatches)

	// --- Graceful shutdown ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		addr := fmt.Sprintf(":%s", cfg.Port)
		logger.Info("starting server", "addr", addr)
		if err := app.Listen(addr); err != nil {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-quit
	logger.Info("shutting down server — waiting for goroutines")

	// Wait for embedding + LLM goroutines to finish
	wg.Wait()

	// Close WebSocket hub (sends close frames, waits for handler goroutines)
	hub.Stop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	logger.Info("server stopped")
}
