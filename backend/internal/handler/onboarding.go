package handler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"supernetwork/backend/internal/middleware"
	"supernetwork/backend/internal/model"
	"supernetwork/backend/internal/service"
	"supernetwork/backend/internal/service/embedding"
)

// OnboardingHandler handles /api/v1/onboarding/* routes.
type OnboardingHandler struct {
	pool      *pgxpool.Pool
	embedder  service.EmbeddingProvider
	summariser service.IkigaiSummariser
	wg        *sync.WaitGroup
	logger    *slog.Logger
}

// NewOnboardingHandler creates an OnboardingHandler.
func NewOnboardingHandler(
	pool *pgxpool.Pool,
	embedder service.EmbeddingProvider,
	summariser service.IkigaiSummariser,
	wg *sync.WaitGroup,
	logger *slog.Logger,
) *OnboardingHandler {
	return &OnboardingHandler{pool: pool, embedder: embedder, summariser: summariser, wg: wg, logger: logger}
}

// SaveIkigai handles POST /api/v1/onboarding/ikigai.
//
// @Summary     Save Ikigai answers
// @Tags        onboarding
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} map[string]interface{}
// @Failure     422 {object} model.AppError
// @Router      /api/v1/onboarding/ikigai [post]
func (h *OnboardingHandler) SaveIkigai(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)

	var body struct {
		WhatYouLove           string `json:"what_you_love"`
		WhatYoureGoodAt       string `json:"what_youre_good_at"`
		WhatWorldNeeds        string `json:"what_world_needs"`
		WhatYouCanBePaidFor   string `json:"what_you_can_be_paid_for"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return model.NewAppError(model.ErrValidation, "invalid request body")
	}
	if body.WhatYouLove == "" || body.WhatYoureGoodAt == "" ||
		body.WhatWorldNeeds == "" || body.WhatYouCanBePaidFor == "" {
		return model.NewAppError(model.ErrValidation, "all four Ikigai fields are required")
	}

	// Upsert ikigai
	_, err := h.pool.Exec(c.Context(),
		`INSERT INTO ikigai_profiles
		   (user_id, what_you_love, what_youre_good_at, what_world_needs, what_you_can_be_paid_for)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (user_id) DO UPDATE SET
		   what_you_love            = EXCLUDED.what_you_love,
		   what_youre_good_at       = EXCLUDED.what_youre_good_at,
		   what_world_needs         = EXCLUDED.what_world_needs,
		   what_you_can_be_paid_for = EXCLUDED.what_you_can_be_paid_for,
		   updated_at               = NOW()`,
		userID,
		body.WhatYouLove, body.WhatYoureGoodAt,
		body.WhatWorldNeeds, body.WhatYouCanBePaidFor,
	)
	if err != nil {
		h.logger.Error("upsert ikigai", "error", err)
		return model.NewAppError(model.ErrInternal, "failed to save Ikigai")
	}

	// Async: AI summary + embedding (tracked by WaitGroup)
	answers := service.IkigaiAnswers{
		WhatYouLove:         body.WhatYouLove,
		WhatYoureGoodAt:     body.WhatYoureGoodAt,
		WhatWorldNeeds:      body.WhatWorldNeeds,
		WhatYouCanBePaidFor: body.WhatYouCanBePaidFor,
	}
	h.wg.Add(1)
	go h.asyncIkigaiPost(userID.String(), answers)

	return c.JSON(fiber.Map{"status": "saved"})
}

func (h *OnboardingHandler) asyncIkigaiPost(userIDStr string, answers service.IkigaiAnswers) {
	defer h.wg.Done()
	defer func() {
		if r := recover(); r != nil {
			h.logger.Error("ikigai goroutine panic", "error", r)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// AI summary
	if h.summariser != nil {
		summary, err := h.summariser.SummariseIkigai(ctx, answers)
		if err != nil {
			h.logger.Error("ikigai summary failed", "error", err)
		} else {
			_, _ = h.pool.Exec(ctx,
				`UPDATE ikigai_profiles SET ai_summary=$2, updated_at=NOW() WHERE user_id=$1`,
				userIDStr, summary)
		}
	}

	// Trigger re-embedding
	var bio string
	var skills, interests, intent []string
	_ = h.pool.QueryRow(ctx,
		`SELECT bio, skills, interests, intent FROM profiles WHERE user_id=$1`, userIDStr,
	).Scan(&bio, &skills, &interests, &intent)

	text := embedding.BuildEmbeddingText(
		embedding.ProfileInput{Bio: bio, Skills: skills, Interests: interests, Intent: intent},
		embedding.IkigaiInput{
			WhatYouLove:         answers.WhatYouLove,
			WhatYoureGoodAt:     answers.WhatYoureGoodAt,
			WhatWorldNeeds:      answers.WhatWorldNeeds,
			WhatYouCanBePaidFor: answers.WhatYouCanBePaidFor,
		},
	)
	if text == "" {
		return
	}

	vec, err := h.embedder.Embed(ctx, text)
	if err != nil {
		h.logger.Error("ikigai embedding failed", "error", err)
		_, _ = h.pool.Exec(ctx,
			`UPDATE profiles SET embedding_status='failed', updated_at=NOW() WHERE user_id=$1`,
			userIDStr)
		return
	}

	vecStr := formatVector(vec)
	_, _ = h.pool.Exec(ctx,
		`UPDATE profiles SET embedding=$2::vector, embedding_status='current',
		  embedding_updated_at=NOW(), updated_at=NOW() WHERE user_id=$1`,
		userIDStr, vecStr)
}

// CompleteOnboarding handles POST /api/v1/onboarding/complete — marks onboarding done.
//
// @Summary     Mark onboarding complete
// @Tags        onboarding
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} map[string]interface{}
// @Router      /api/v1/onboarding/complete [post]
func (h *OnboardingHandler) CompleteOnboarding(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)
	_, err := h.pool.Exec(c.Context(),
		`UPDATE profiles SET onboarding_complete=TRUE, updated_at=NOW() WHERE user_id=$1`, userID)
	if err != nil {
		return model.NewAppError(model.ErrInternal, "failed to complete onboarding")
	}
	return c.JSON(fiber.Map{"status": "complete"})
}
