// @title           SuperNetworkAI API
// @version         1.0
// @description     Ikigai-based AI networking platform — Go REST + WebSocket backend
// @host            localhost:3001
// @BasePath        /

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"

	"supernetwork/backend/internal/config"
	"supernetwork/backend/internal/db"
	"supernetwork/backend/internal/health"
	"supernetwork/backend/internal/middleware"
	"supernetwork/backend/internal/model"
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
	logger.Info("shutting down server")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Error("shutdown error", "error", err)
	}

	logger.Info("server stopped")
}
