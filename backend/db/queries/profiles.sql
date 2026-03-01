-- name: GetProfileByUserID :one
SELECT *
FROM profiles
WHERE user_id = $1;

-- name: GetPublicProfileByUserID :one
-- Used for viewing other users' profiles. Returns only if public OR if requester is connected.
SELECT *
FROM profiles
WHERE user_id = $1
  AND visibility = 'public';

-- name: CreateProfile :one
INSERT INTO profiles (user_id)
VALUES ($1)
RETURNING *;

-- name: UpdateProfile :one
UPDATE profiles
SET
  display_name        = COALESCE($2, display_name),
  tagline             = COALESCE($3, tagline),
  bio                 = COALESCE($4, bio),
  avatar_url          = COALESCE($5, avatar_url),
  portfolio_url       = COALESCE($6, portfolio_url),
  linkedin_url        = COALESCE($7, linkedin_url),
  github_url          = COALESCE($8, github_url),
  twitter_url         = COALESCE($9, twitter_url),
  location            = COALESCE($10, location),
  timezone            = COALESCE($11, timezone),
  skills              = COALESCE($12, skills),
  interests           = COALESCE($13, interests),
  intent              = COALESCE($14, intent),
  availability        = COALESCE($15, availability),
  working_style       = COALESCE($16, working_style),
  updated_at          = NOW()
WHERE user_id = $1
RETURNING *;

-- name: SetEmbeddingStatus :exec
UPDATE profiles
SET embedding_status = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: UpdateEmbedding :exec
UPDATE profiles
SET
  embedding            = $2,
  embedding_status     = 'current',
  embedding_updated_at = NOW(),
  updated_at           = NOW()
WHERE user_id = $1;

-- name: SetOnboardingComplete :exec
UPDATE profiles
SET onboarding_complete = TRUE, updated_at = NOW()
WHERE user_id = $1;

-- name: SetVisibility :exec
UPDATE profiles
SET visibility = $2, updated_at = NOW()
WHERE user_id = $1;

-- name: GetIkigaiByUserID :one
SELECT *
FROM ikigai_profiles
WHERE user_id = $1;

-- name: UpsertIkigai :one
INSERT INTO ikigai_profiles (
  user_id,
  what_you_love,
  what_youre_good_at,
  what_world_needs,
  what_you_can_be_paid_for
) VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id) DO UPDATE SET
  what_you_love            = EXCLUDED.what_you_love,
  what_youre_good_at       = EXCLUDED.what_youre_good_at,
  what_world_needs         = EXCLUDED.what_world_needs,
  what_you_can_be_paid_for = EXCLUDED.what_you_can_be_paid_for,
  updated_at               = NOW()
RETURNING *;

-- name: UpdateIkigaiSummary :exec
UPDATE ikigai_profiles
SET ai_summary = $2, updated_at = NOW()
WHERE user_id = $1;
