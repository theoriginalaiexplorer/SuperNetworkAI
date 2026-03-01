package handler

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/pgxpool"

	"supernetwork/backend/internal/middleware"
	"supernetwork/backend/internal/model"
	"supernetwork/backend/internal/service"
	"supernetwork/backend/internal/service/embedding"
)

// ProfileHandler handles /api/v1/profiles/* routes.
type ProfileHandler struct {
	pool      *pgxpool.Pool
	embedder  service.EmbeddingProvider
	logger    *slog.Logger
	wg        *sync.WaitGroup
}

// NewProfileHandler creates a ProfileHandler.
func NewProfileHandler(pool *pgxpool.Pool, embedder service.EmbeddingProvider, wg *sync.WaitGroup, logger *slog.Logger) *ProfileHandler {
	return &ProfileHandler{pool: pool, embedder: embedder, wg: wg, logger: logger}
}

// UpdateProfile handles PATCH /api/v1/profiles/me.
//
// @Summary     Update own profile (partial)
// @Tags        profiles
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} map[string]interface{}
// @Failure     422 {object} model.AppError
// @Router      /api/v1/profiles/me [patch]
func (h *ProfileHandler) UpdateProfile(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)

	var body struct {
		DisplayName  *string  `json:"display_name"`
		Tagline      *string  `json:"tagline"`
		Bio          *string  `json:"bio"`
		AvatarURL    *string  `json:"avatar_url"`
		PortfolioURL *string  `json:"portfolio_url"`
		LinkedInURL  *string  `json:"linkedin_url"`
		GitHubURL    *string  `json:"github_url"`
		TwitterURL   *string  `json:"twitter_url"`
		Location     *string  `json:"location"`
		Timezone     *string  `json:"timezone"`
		Skills       []string `json:"skills"`
		Interests    []string `json:"interests"`
		Intent       []string `json:"intent"`
		Availability *string  `json:"availability"`
		WorkingStyle *string  `json:"working_style"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return model.NewAppError(model.ErrValidation, "invalid request body")
	}

	_, err := h.pool.Exec(c.Context(),
		`UPDATE profiles SET
		   display_name  = COALESCE($2, display_name),
		   tagline       = COALESCE($3, tagline),
		   bio           = COALESCE($4, bio),
		   avatar_url    = COALESCE($5, avatar_url),
		   portfolio_url = COALESCE($6, portfolio_url),
		   linkedin_url  = COALESCE($7, linkedin_url),
		   github_url    = COALESCE($8, github_url),
		   twitter_url   = COALESCE($9, twitter_url),
		   location      = COALESCE($10, location),
		   timezone      = COALESCE($11, timezone),
		   skills        = COALESCE($12, skills),
		   interests     = COALESCE($13, interests),
		   intent        = COALESCE($14, intent),
		   availability  = COALESCE($15, availability),
		   working_style = COALESCE($16, working_style),
		   updated_at    = NOW()
		 WHERE user_id = $1`,
		userID,
		body.DisplayName, body.Tagline, body.Bio,
		body.AvatarURL, body.PortfolioURL, body.LinkedInURL, body.GitHubURL, body.TwitterURL,
		body.Location, body.Timezone,
		body.Skills, body.Interests, body.Intent,
		body.Availability, body.WorkingStyle,
	)
	if err != nil {
		h.logger.Error("update profile", "error", err, "user_id", userID)
		return model.NewAppError(model.ErrInternal, "failed to update profile")
	}

	// Embedding-triggering fields: skills, interests, bio, intent
	embeddingTriggered := body.Skills != nil || body.Interests != nil ||
		body.Bio != nil || body.Intent != nil

	if embeddingTriggered {
		h.triggerEmbedding(userID.String())
	}

	return c.JSON(fiber.Map{"status": "updated"})
}

// SetVisibility handles PATCH /api/v1/profiles/me/visibility.
//
// @Summary     Set profile visibility
// @Tags        profiles
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} map[string]interface{}
// @Router      /api/v1/profiles/me/visibility [patch]
func (h *ProfileHandler) SetVisibility(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)

	var body struct {
		Visibility string `json:"visibility"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return model.NewAppError(model.ErrValidation, "invalid request body")
	}
	if body.Visibility != "public" && body.Visibility != "private" {
		return model.NewAppError(model.ErrValidation, "visibility must be 'public' or 'private'")
	}

	_, err := h.pool.Exec(c.Context(),
		`UPDATE profiles SET visibility = $2, updated_at = NOW() WHERE user_id = $1`,
		userID, body.Visibility)
	if err != nil {
		return model.NewAppError(model.ErrInternal, "failed to update visibility")
	}
	return c.JSON(fiber.Map{"visibility": body.Visibility})
}

// triggerEmbedding sets embedding_status to stale and spawns an async goroutine
// to recompute and store the embedding vector.
func (h *ProfileHandler) triggerEmbedding(userIDStr string) {
	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		defer func() {
			if r := recover(); r != nil {
				h.logger.Error("embedding goroutine panic", "error", r)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Fetch profile + ikigai for embedding text
		var bio string
		var skills, interests, intent []string
		var love, goodAt, worldNeeds, paidFor string
		_ = h.pool.QueryRow(ctx,
			`SELECT p.bio, p.skills, p.interests, p.intent,
			        COALESCE(ik.what_you_love,''), COALESCE(ik.what_youre_good_at,''),
			        COALESCE(ik.what_world_needs,''), COALESCE(ik.what_you_can_be_paid_for,'')
			 FROM profiles p
			 LEFT JOIN ikigai_profiles ik ON ik.user_id = p.user_id
			 WHERE p.user_id = $1`, userIDStr,
		).Scan(&bio, &skills, &interests, &intent, &love, &goodAt, &worldNeeds, &paidFor)

		text := embedding.BuildEmbeddingText(
			embedding.ProfileInput{Bio: bio, Skills: skills, Interests: interests, Intent: intent},
			embedding.IkigaiInput{
				WhatYouLove: love, WhatYoureGoodAt: goodAt,
				WhatWorldNeeds: worldNeeds, WhatYouCanBePaidFor: paidFor,
			},
		)
		if text == "" {
			return
		}

		vec, err := h.embedder.Embed(ctx, text)
		if err != nil {
			h.logger.Error("embedding failed", "error", err, "user_id", userIDStr)
			_, _ = h.pool.Exec(ctx,
				`UPDATE profiles SET embedding_status='failed', updated_at=NOW() WHERE user_id=$1`,
				userIDStr)
			return
		}

		// Store using pgvector format
		vecStr := formatVector(vec)
		_, _ = h.pool.Exec(ctx,
			`UPDATE profiles SET embedding=$2::vector, embedding_status='current',
			  embedding_updated_at=NOW(), updated_at=NOW() WHERE user_id=$1`,
			userIDStr, vecStr)
	}()
}

// formatVector converts []float32 to a pgvector-compatible string "[x,y,z,...]".
func formatVector(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	buf := make([]byte, 0, len(v)*10+2)
	buf = append(buf, '[')
	for i, f := range v {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, []byte(formatFloat(f))...)
	}
	buf = append(buf, ']')
	return string(buf)
}

func formatFloat(f float32) string {
	return strconv.FormatFloat(float64(f), 'f', -1, 32)
}
