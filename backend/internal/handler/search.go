package handler

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"supernetwork/backend/internal/middleware"
	"supernetwork/backend/internal/model"
	"supernetwork/backend/internal/service"
)

// SearchHandler handles POST /api/v1/search.
type SearchHandler struct {
	pool     *pgxpool.Pool
	parser   service.NLSearchParser
	embedder service.EmbeddingProvider
	logger   *slog.Logger
}

// NewSearchHandler creates a SearchHandler.
func NewSearchHandler(
	pool *pgxpool.Pool,
	parser service.NLSearchParser,
	embedder service.EmbeddingProvider,
	logger *slog.Logger,
) *SearchHandler {
	return &SearchHandler{pool: pool, parser: parser, embedder: embedder, logger: logger}
}

// searchResult is the per-profile row returned by /api/v1/search.
type searchResult struct {
	UserID      string   `json:"user_id"`
	DisplayName string   `json:"display_name"`
	Tagline     string   `json:"tagline"`
	Skills      []string `json:"skills"`
	Intent      []string `json:"intent"`
	AvatarURL   string   `json:"avatar_url"`
	Score       float64  `json:"score"`
}

// Search handles POST /api/v1/search.
//
// @Summary     Natural-language profile search
// @Tags        search
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body object true "{ \"query\": \"...\" }"
// @Success     200 {object} map[string]interface{}
// @Failure     400 {object} model.AppError
// @Failure     503 {object} model.AppError
// @Router      /api/v1/search [post]
func (h *SearchHandler) Search(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)

	var body struct {
		Query string `json:"query"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return model.NewAppError(model.ErrValidation, "invalid request body")
	}
	if body.Query == "" {
		return model.NewAppError(model.ErrValidation, "query is required")
	}
	if len(body.Query) > 500 {
		return model.NewAppError(model.ErrValidation, "query too long (max 500 chars)")
	}

	// 1. Parse NL query via Groq
	params, err := h.parser.ParseSearchQuery(c.Context(), body.Query)
	if err != nil {
		h.logger.Error("NL parse failed, falling back to raw query", "error", err)
		params = &service.SearchParams{EmbeddingText: body.Query}
	}

	// 2. Embed the parsed text
	vec, err := h.embedder.Embed(c.Context(), params.EmbeddingText)
	if err != nil {
		h.logger.Error("embed search query", "error", err)
		return model.NewAppError(model.ErrInternal, "failed to embed search query")
	}
	vecStr := formatVector(vec)

	// 3. pgvector search with visibility + block guards
	var intentArg *string
	if params.IntentFilter != "" {
		intentArg = &params.IntentFilter
	}
	var availArg *string
	if params.AvailabilityFilter != "" {
		availArg = &params.AvailabilityFilter
	}

	rows, err := h.pool.Query(c.Context(), `
		SELECT
			p.user_id,
			p.display_name,
			COALESCE(p.tagline,  ''),
			COALESCE(p.skills,   ARRAY[]::text[]),
			COALESCE(p.intent,   ARRAY[]::text[]),
			COALESCE(p.avatar_url, ''),
			1 - (p.embedding <=> $1::vector) AS score
		FROM profiles p
		WHERE p.user_id          != $2
		  AND p.visibility        = 'public'
		  AND p.embedding_status  = 'current'
		  AND p.onboarding_complete = true
		  AND ($3::text IS NULL OR $3 = ANY(p.intent))
		  AND ($4::text IS NULL OR p.availability = $4)
		  AND NOT EXISTS (
		    SELECT 1 FROM blocks
		    WHERE (blocker_id = $2 AND blocked_id = p.user_id)
		       OR (blocker_id = p.user_id AND blocked_id = $2)
		  )
		ORDER BY p.embedding <=> $1::vector
		LIMIT 20
	`, vecStr, userID, intentArg, availArg)
	if err != nil {
		h.logger.Error("search query", "error", err)
		return model.NewAppError(model.ErrInternal, "search failed")
	}
	defer rows.Close()

	var results []searchResult
	for rows.Next() {
		var r searchResult
		var uid uuid.UUID
		if err := rows.Scan(
			&uid, &r.DisplayName, &r.Tagline,
			&r.Skills, &r.Intent, &r.AvatarURL, &r.Score,
		); err != nil {
			return model.NewAppError(model.ErrInternal, "scan error")
		}
		r.UserID = uid.String()
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		return model.NewAppError(model.ErrInternal, "query error")
	}
	if results == nil {
		results = []searchResult{}
	}

	return c.JSON(fiber.Map{"results": results, "count": len(results)})
}
