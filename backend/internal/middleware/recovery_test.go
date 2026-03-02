package middleware

import (
	"log/slog"
	"net/http"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestRecovery_PanicReturns500(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	app := fiber.New()
	app.Use(Recovery(logger))
	app.Get("/panic", func(c fiber.Ctx) error {
		panic("test panic")
	})

	req, _ := http.NewRequest("GET", "/panic", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 after panic, got %d", resp.StatusCode)
	}
}

func TestRecovery_NoPanicPassesThrough(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	app := fiber.New()
	app.Use(Recovery(logger))
	app.Get("/ok", func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	req, _ := http.NewRequest("GET", "/ok", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for non-panicking handler, got %d", resp.StatusCode)
	}
}

func TestRecovery_MultiplePanicsHandled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	app := fiber.New()
	app.Use(Recovery(logger))
	app.Get("/panic", func(c fiber.Ctx) error {
		panic("boom")
	})

	// Panic recovery should work for multiple requests (not just first)
	for i := 0; i < 3; i++ {
		req, _ := http.NewRequest("GET", "/panic", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		if resp.StatusCode != http.StatusInternalServerError {
			t.Errorf("request %d: expected 500, got %d", i, resp.StatusCode)
		}
	}
}
