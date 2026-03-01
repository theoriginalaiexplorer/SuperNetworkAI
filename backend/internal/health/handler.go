package health

import (
	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler holds the DB pool for readiness checks.
type Handler struct {
	pool *pgxpool.Pool
}

// New creates a health Handler.
func New(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
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

// Readiness pings the DB. Returns 200 {"status":"ready"} on success or
// 200 {"status":"db_unavailable"} on failure — never crashes.
//
// @Summary     Readiness probe
// @Tags        health
// @Produce     json
// @Success     200 {object} map[string]string
// @Router      /readyz [get]
func (h *Handler) Readiness(c fiber.Ctx) error {
	if err := h.pool.Ping(c.Context()); err != nil {
		return c.JSON(fiber.Map{"status": "db_unavailable"})
	}
	return c.JSON(fiber.Map{"status": "ready"})
}
