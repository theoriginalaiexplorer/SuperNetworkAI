package handler

import (
	"log/slog"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"supernetwork/backend/internal/middleware"
	"supernetwork/backend/internal/model"
	"supernetwork/backend/internal/service"
)

// MatchHandler handles /api/v1/matches/* routes.
type MatchHandler struct {
	svc    service.MatchService
	logger *slog.Logger
}

// NewMatchHandler creates a MatchHandler.
func NewMatchHandler(svc service.MatchService, logger *slog.Logger) *MatchHandler {
	return &MatchHandler{svc: svc, logger: logger}
}

// GetMatches handles GET /api/v1/matches.
//
// @Summary     List ranked matches
// @Tags        matches
// @Produce     json
// @Security    BearerAuth
// @Param       category query string false "Filter: cofounder | teammate | client"
// @Param       limit    query int    false "Page size (default 20, max 100)"
// @Param       offset   query int    false "Page offset"
// @Success     200 {object} map[string]interface{}
// @Router      /api/v1/matches [get]
func (h *MatchHandler) GetMatches(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)

	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	matches, err := h.svc.GetMatches(c.Context(), userID, service.MatchFilter{
		Category: c.Query("category"),
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		h.logger.Error("get matches", "error", err)
		return model.NewAppError(model.ErrInternal, "failed to load matches")
	}

	return c.JSON(fiber.Map{"matches": matches, "count": len(matches)})
}

// DismissMatch handles POST /api/v1/matches/:matchedUserId/dismiss.
//
// @Summary     Dismiss a match
// @Tags        matches
// @Produce     json
// @Security    BearerAuth
// @Param       matchedUserId path string true "Matched user UUID"
// @Success     200 {object} map[string]interface{}
// @Failure     400 {object} model.AppError
// @Router      /api/v1/matches/{matchedUserId}/dismiss [post]
func (h *MatchHandler) DismissMatch(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)

	matchedUserID, err := uuid.Parse(c.Params("matchedUserId"))
	if err != nil {
		return model.NewAppError(model.ErrValidation, "invalid matchedUserId")
	}

	if err := h.svc.DismissMatch(c.Context(), userID, matchedUserID); err != nil {
		h.logger.Error("dismiss match", "error", err)
		return model.NewAppError(model.ErrInternal, "failed to dismiss match")
	}

	return c.JSON(fiber.Map{"status": "dismissed"})
}

// GetExplanation handles GET /api/v1/matches/:matchedUserId/explanation.
// Returns a cached AI explanation; generates and caches it on first call.
//
// @Summary     Get AI match explanation
// @Tags        matches
// @Produce     json
// @Security    BearerAuth
// @Param       matchedUserId path string true "Matched user UUID"
// @Success     200 {object} map[string]interface{}
// @Failure     400 {object} model.AppError
// @Router      /api/v1/matches/{matchedUserId}/explanation [get]
func (h *MatchHandler) GetExplanation(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)

	matchedUserID, err := uuid.Parse(c.Params("matchedUserId"))
	if err != nil {
		return model.NewAppError(model.ErrValidation, "invalid matchedUserId")
	}

	explanation, err := h.svc.GetExplanation(c.Context(), matchedUserID, userID)
	if err != nil {
		h.logger.Error("get explanation", "error", err)
		return model.NewAppError(model.ErrInternal, "failed to get explanation")
	}

	return c.JSON(fiber.Map{"explanation": explanation})
}
