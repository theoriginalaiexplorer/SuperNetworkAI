package handler

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"supernetwork/backend/internal/middleware"
	"supernetwork/backend/internal/model"
)

// UserHandler handles /api/v1/users/* routes.
type UserHandler struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewUserHandler creates a UserHandler.
func NewUserHandler(pool *pgxpool.Pool, logger *slog.Logger) *UserHandler {
	return &UserHandler{pool: pool, logger: logger}
}

// GetMe returns the authenticated user's own data: user, profile, and ikigai.
//
// @Summary     Get own user data
// @Tags        users
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} model.AppError
// @Failure     404 {object} model.AppError
// @Router      /api/v1/users/me [get]
func (h *UserHandler) GetMe(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)

	row := h.pool.QueryRow(c.Context(),
		`SELECT u.id, u.email, u.created_at,
		        p.id AS profile_id, p.display_name, p.tagline, p.bio,
		        p.avatar_url, p.portfolio_url, p.linkedin_url, p.github_url,
		        p.twitter_url, p.location, p.timezone, p.skills, p.interests,
		        p.intent, p.availability, p.working_style, p.visibility,
		        p.embedding_status, p.onboarding_complete,
		        ik.what_you_love, ik.what_youre_good_at, ik.what_world_needs,
		        ik.what_you_can_be_paid_for, ik.ai_summary
		 FROM users u
		 LEFT JOIN profiles p ON p.user_id = u.id
		 LEFT JOIN ikigai_profiles ik ON ik.user_id = u.id
		 WHERE u.id = $1`, userID)

	var (
		uID, pID                                        uuid.UUID
		email, displayName, tagline, bio                string
		avatarURL, portfolioURL, linkedinURL            *string
		githubURL, twitterURL, location, timezone       *string
		skills, interests, intent                       []string
		availability, workingStyle, visibility          string
		embStatus                                       string
		onboardingComplete                              bool
		uCreatedAt                                      interface{}
		love, goodAt, worldNeeds, paidFor, aiSummary    *string
	)

	err := row.Scan(
		&uID, &email, &uCreatedAt,
		&pID, &displayName, &tagline, &bio,
		&avatarURL, &portfolioURL, &linkedinURL, &githubURL,
		&twitterURL, &location, &timezone, &skills, &interests,
		&intent, &availability, &workingStyle, &visibility,
		&embStatus, &onboardingComplete,
		&love, &goodAt, &worldNeeds, &paidFor, &aiSummary,
	)
	if err != nil {
		return model.NewAppError(model.ErrNotFound, "user not found")
	}

	return c.JSON(fiber.Map{
		"user": fiber.Map{
			"id":         uID,
			"email":      email,
			"created_at": uCreatedAt,
		},
		"profile": fiber.Map{
			"id":                  pID,
			"display_name":        displayName,
			"tagline":             tagline,
			"bio":                 bio,
			"avatar_url":          avatarURL,
			"portfolio_url":       portfolioURL,
			"linkedin_url":        linkedinURL,
			"github_url":          githubURL,
			"twitter_url":         twitterURL,
			"location":            location,
			"timezone":            timezone,
			"skills":              skills,
			"interests":           interests,
			"intent":              intent,
			"availability":        availability,
			"working_style":       workingStyle,
			"visibility":          visibility,
			"embedding_status":    embStatus,
			"onboarding_complete": onboardingComplete,
		},
		"ikigai": fiber.Map{
			"what_you_love":             love,
			"what_youre_good_at":        goodAt,
			"what_world_needs":          worldNeeds,
			"what_you_can_be_paid_for":  paidFor,
			"ai_summary":                aiSummary,
		},
	})
}

// GetByID returns another user's public profile.
//
// @Summary     Get user profile by ID
// @Tags        users
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "User UUID"
// @Success     200 {object} map[string]interface{}
// @Failure     403 {object} model.AppError
// @Failure     404 {object} model.AppError
// @Router      /api/v1/users/{id} [get]
func (h *UserHandler) GetByID(c fiber.Ctx) error {
	currentUserID := middleware.UserFromCtx(c)

	targetID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return model.NewAppError(model.ErrValidation, "invalid user id")
	}
	if targetID == currentUserID {
		return c.Redirect().To("/api/v1/users/me")
	}

	// Block check — both directions
	var blocked bool
	_ = h.pool.QueryRow(c.Context(),
		`SELECT EXISTS(
		   SELECT 1 FROM blocks
		   WHERE (blocker_id = $1 AND blocked_id = $2)
		      OR (blocker_id = $2 AND blocked_id = $1)
		 )`, currentUserID, targetID).Scan(&blocked)
	if blocked {
		return model.NewAppError(model.ErrForbidden, "access denied")
	}

	// Fetch profile — visibility enforced
	row := h.pool.QueryRow(c.Context(),
		`SELECT p.user_id, p.display_name, p.tagline, p.bio, p.avatar_url,
		        p.skills, p.interests, p.intent, p.availability, p.working_style,
		        p.visibility
		 FROM profiles p
		 WHERE p.user_id = $1`, targetID)

	var (
		pUserID                       uuid.UUID
		displayName, tagline, bio     string
		avatarURL                     *string
		skills, interests, intent     []string
		availability, workingStyle    string
		visibility                    string
	)
	if err := row.Scan(&pUserID, &displayName, &tagline, &bio, &avatarURL,
		&skills, &interests, &intent, &availability, &workingStyle, &visibility); err != nil {
		return model.NewAppError(model.ErrNotFound, "user not found")
	}

	// Private visibility: only accessible to accepted connections
	if visibility == "private" {
		var connected bool
		_ = h.pool.QueryRow(c.Context(),
			`SELECT EXISTS(
			   SELECT 1 FROM connections
			   WHERE status = 'accepted'
			     AND ((requester_id = $1 AND recipient_id = $2)
			       OR (requester_id = $2 AND recipient_id = $1))
			 )`, currentUserID, targetID).Scan(&connected)
		if !connected {
			return model.NewAppError(model.ErrForbidden, "this profile is private")
		}
	}

	return c.JSON(fiber.Map{
		"user_id":       pUserID,
		"display_name":  displayName,
		"tagline":       tagline,
		"bio":           bio,
		"avatar_url":    avatarURL,
		"skills":        skills,
		"interests":     interests,
		"intent":        intent,
		"availability":  availability,
		"working_style": workingStyle,
	})
}
