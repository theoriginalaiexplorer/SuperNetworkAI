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
	"supernetwork/backend/internal/service/embedding"
	"supernetwork/backend/internal/service/llm"
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

	// --- JWKS URL for auth middleware ---
	jwksURL := fmt.Sprintf("%s/auth/v1/.well-known/jwks.json", cfg.SupabaseURL)

	// --- Handlers ---
	authH := handler.NewAuthHandler(cfg.WSTokenSecret)
	userH := handler.NewUserHandler(pool, logger)
	profileH := handler.NewProfileHandler(pool, embedProvider, &wg, logger)
	onboardingH := handler.NewOnboardingHandler(pool, embedProvider, ikigaiSummariser, &wg, logger)

	// --- API v1 routes (all require JWT) ---
	api := app.Group("/api/v1", middleware.RequireAuth(jwksURL))

	api.Post("/auth/ws-token", authH.IssueWSToken)

	api.Get("/users/me", userH.GetMe)
	api.Get("/users/:id", userH.GetByID)

	api.Patch("/profiles/me", profileH.UpdateProfile)
	api.Patch("/profiles/me/visibility", profileH.SetVisibility)

	api.Post("/onboarding/ikigai", onboardingH.SaveIkigai)
	api.Post("/onboarding/complete", onboardingH.CompleteOnboarding)

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

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	logger.Info("server stopped")
}
