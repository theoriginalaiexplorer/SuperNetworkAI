package health

import (
	"context"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler holds the DB pool and optional embedding provider URL for readiness checks.
type Handler struct {
	pool             *pgxpool.Pool
	ollamaBaseURL    string // non-empty only when EMBEDDING_PROVIDER=ollama
}

// New creates a health Handler.
func New(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
}

// NewWithOllama creates a health Handler that also pings Ollama during readiness.
func NewWithOllama(pool *pgxpool.Pool, ollamaBaseURL string) *Handler {
	return &Handler{pool: pool, ollamaBaseURL: ollamaBaseURL}
}

// Liveness responds 200 {"status":"ok"} — always, if the process is alive.
//
// @Summary     Liveness probe
// @Tags        health
// @Produce     json
// @Success     200 {object} map[string]string
// @Router      /healthz [get]
func (h *Handler) Liveness(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

// Readiness pings the DB (and Ollama if configured). Returns 503 on failure so
// Cloud Run stops routing traffic to the instance.
//
// @Summary     Readiness probe
// @Tags        health
// @Produce     json
// @Success     200 {object} map[string]string
// @Failure     503 {object} map[string]string
// @Router      /readyz [get]
func (h *Handler) Readiness(c fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()

	if h.pool == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "db_unavailable"})
	}
	if err := h.pool.Ping(ctx); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "db_unavailable"})
	}

	if h.ollamaBaseURL != "" {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, h.ollamaBaseURL+"/api/tags", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil || resp.StatusCode >= 500 {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "embedding_unavailable"})
		}
		resp.Body.Close()
	}

	return c.JSON(fiber.Map{"status": "ready"})
}
