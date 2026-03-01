package handler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"supernetwork/backend/internal/model"
	"supernetwork/backend/internal/service"
)

// InternalHandler handles /internal/* routes (Cloud Scheduler, cron jobs).
// All routes require the X-Internal-Secret header.
type InternalHandler struct {
	pool    *pgxpool.Pool
	matchSvc service.MatchService
	wg      *sync.WaitGroup
	logger  *slog.Logger
}

// NewInternalHandler creates an InternalHandler.
func NewInternalHandler(
	pool *pgxpool.Pool,
	matchSvc service.MatchService,
	wg *sync.WaitGroup,
	logger *slog.Logger,
) *InternalHandler {
	return &InternalHandler{pool: pool, matchSvc: matchSvc, wg: wg, logger: logger}
}

// RefreshMatches handles POST /internal/matches/refresh.
// Recomputes match_cache for all users whose embedding_status is 'current'
// but whose match cache hasn't been refreshed in the last 6 hours.
//
// @Summary     Refresh match cache (internal)
// @Tags        internal
// @Produce     json
// @Param       X-Internal-Secret header string true "Internal API secret"
// @Success     200 {object} map[string]interface{}
// @Router      /internal/matches/refresh [post]
func (h *InternalHandler) RefreshMatches(c fiber.Ctx) error {
	// Find users with current embeddings that need a cache refresh
	rows, err := h.pool.Query(c.Context(), `
		SELECT DISTINCT p.user_id
		FROM profiles p
		WHERE p.embedding_status = 'current'
		  AND p.onboarding_complete = true
		  AND p.visibility = 'public'
		  AND NOT EXISTS (
			SELECT 1 FROM match_cache mc
			WHERE mc.user_id = p.user_id
			  AND mc.computed_at > NOW() - INTERVAL '6 hours'
		  )
		LIMIT 100
	`)
	if err != nil {
		h.logger.Error("internal refresh: query users", "error", err)
		return model.NewAppError(model.ErrInternal, "failed to find users for refresh")
	}

	var userIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return model.NewAppError(model.ErrInternal, "scan error")
		}
		userIDs = append(userIDs, id)
	}
	rows.Close()

	// Refresh each user async, tracked by WaitGroup
	for _, uid := range userIDs {
		h.wg.Add(1)
		go func(id uuid.UUID) {
			defer h.wg.Done()
			defer func() {
				if r := recover(); r != nil {
					h.logger.Error("refresh goroutine panic", "user", id, "error", r)
				}
			}()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := h.matchSvc.RefreshCacheForUser(ctx, id); err != nil {
				h.logger.Error("refresh match cache", "user", id, "error", err)
			}
		}(uid)
	}

	return c.JSON(fiber.Map{"queued": len(userIDs)})
}
