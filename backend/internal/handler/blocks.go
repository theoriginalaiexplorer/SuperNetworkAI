package handler

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"supernetwork/backend/internal/middleware"
	"supernetwork/backend/internal/model"
)

// BlockHandler handles /api/v1/blocks and /api/v1/account routes.
type BlockHandler struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewBlockHandler creates a BlockHandler.
func NewBlockHandler(pool *pgxpool.Pool, logger *slog.Logger) *BlockHandler {
	return &BlockHandler{pool: pool, logger: logger}
}

// BlockUser handles POST /api/v1/blocks.
//
// @Summary     Block a user
// @Tags        blocks
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body object true "{ \"blocked_id\": \"uuid\" }"
// @Success     201 {object} map[string]interface{}
// @Failure     409 {object} model.AppError
// @Router      /api/v1/blocks [post]
func (h *BlockHandler) BlockUser(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)

	var body struct {
		BlockedID string `json:"blocked_id"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return model.NewAppError(model.ErrValidation, "invalid request body")
	}
	blockedID, err := uuid.Parse(body.BlockedID)
	if err != nil {
		return model.NewAppError(model.ErrValidation, "invalid blocked_id")
	}
	if blockedID == userID {
		return model.NewAppError(model.ErrValidation, "cannot block yourself")
	}

	// Idempotency check
	var exists bool
	_ = h.pool.QueryRow(c.Context(),
		`SELECT EXISTS(SELECT 1 FROM blocks WHERE blocker_id=$1 AND blocked_id=$2)`,
		userID, blockedID,
	).Scan(&exists)
	if exists {
		return model.NewAppError(model.ErrConflict, "already blocked")
	}

	if _, err = h.pool.Exec(c.Context(),
		`INSERT INTO blocks (blocker_id, blocked_id) VALUES ($1, $2)`,
		userID, blockedID,
	); err != nil {
		h.logger.Error("block user", "error", err)
		return model.NewAppError(model.ErrInternal, "failed to block user")
	}

	// Remove any connection between the two users
	_, _ = h.pool.Exec(c.Context(),
		`DELETE FROM connections
		 WHERE (requester_id=$1 AND recipient_id=$2)
		    OR (requester_id=$2 AND recipient_id=$1)`,
		userID, blockedID,
	)

	// Remove match_cache entries in both directions
	_, _ = h.pool.Exec(c.Context(),
		`DELETE FROM match_cache
		 WHERE (user_id=$1 AND matched_user_id=$2)
		    OR (user_id=$2 AND matched_user_id=$1)`,
		userID, blockedID,
	)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "blocked"})
}

// UnblockUser handles DELETE /api/v1/blocks/:userId.
//
// @Summary     Unblock a user
// @Tags        blocks
// @Produce     json
// @Security    BearerAuth
// @Param       userId path string true "Blocked user UUID"
// @Success     200 {object} map[string]interface{}
// @Failure     404 {object} model.AppError
// @Router      /api/v1/blocks/{userId} [delete]
func (h *BlockHandler) UnblockUser(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)
	blockedID, err := uuid.Parse(c.Params("userId"))
	if err != nil {
		return model.NewAppError(model.ErrValidation, "invalid userId")
	}

	result, err := h.pool.Exec(c.Context(),
		`DELETE FROM blocks WHERE blocker_id=$1 AND blocked_id=$2`,
		userID, blockedID,
	)
	if err != nil {
		return model.NewAppError(model.ErrInternal, "failed to unblock user")
	}
	if result.RowsAffected() == 0 {
		return model.NewAppError(model.ErrNotFound, "block not found")
	}

	return c.JSON(fiber.Map{"status": "unblocked"})
}

// DeleteAccount handles DELETE /api/v1/account.
// All child rows (profiles, connections, messages, match_cache, blocks, etc.)
// cascade-delete via FK ON DELETE CASCADE defined on users.id.
//
// @Summary     Delete own account (irreversible)
// @Tags        account
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} map[string]interface{}
// @Failure     404 {object} model.AppError
// @Router      /api/v1/account [delete]
func (h *BlockHandler) DeleteAccount(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)

	result, err := h.pool.Exec(c.Context(),
		`DELETE FROM users WHERE id = $1`, userID,
	)
	if err != nil {
		h.logger.Error("delete account", "error", err, "user_id", userID)
		return model.NewAppError(model.ErrInternal, "failed to delete account")
	}
	if result.RowsAffected() == 0 {
		return model.NewAppError(model.ErrNotFound, "user not found")
	}

	return c.JSON(fiber.Map{"status": "deleted"})
}
