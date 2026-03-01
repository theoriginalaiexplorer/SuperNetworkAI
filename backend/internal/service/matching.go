package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const matchThreshold = 0.6

type matchService struct {
	pool      *pgxpool.Pool
	explainer MatchExplainer
	logger    *slog.Logger
}

// NewMatchService creates a MatchService backed by pgvector cosine similarity.
// explainer may be nil — GetExplanation returns "" when none is configured.
func NewMatchService(pool *pgxpool.Pool, explainer MatchExplainer, logger *slog.Logger) MatchService {
	return &matchService{pool: pool, explainer: explainer, logger: logger}
}

// GetMatches returns ranked, non-dismissed matches for userID from the cache.
func (s *matchService) GetMatches(ctx context.Context, userID uuid.UUID, f MatchFilter) ([]Match, error) {
	limit := f.Limit
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	// NULL-safe category filter: pass nil to skip filtering
	var catArg *string
	if f.Category != "" {
		catArg = &f.Category
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			mc.matched_user_id,
			mc.score,
			mc.categories,
			COALESCE(mc.explanation, ''),
			p.display_name,
			COALESCE(p.tagline, ''),
			COALESCE(p.skills,  ARRAY[]::text[]),
			COALESCE(p.intent,  ARRAY[]::text[]),
			COALESCE(p.avatar_url, '')
		FROM match_cache mc
		JOIN profiles p ON p.user_id = mc.matched_user_id
		WHERE mc.user_id    = $1
		  AND mc.dismissed  = false
		  AND p.visibility  = 'public'
		  AND ($2::text IS NULL OR $2 = ANY(mc.categories))
		  AND NOT EXISTS (
			SELECT 1 FROM blocks
			WHERE (blocker_id = $1 AND blocked_id = mc.matched_user_id)
			   OR (blocker_id = mc.matched_user_id AND blocked_id = $1)
		  )
		ORDER BY mc.score DESC
		LIMIT $3 OFFSET $4
	`, userID, catArg, limit, f.Offset)
	if err != nil {
		return nil, fmt.Errorf("query matches: %w", err)
	}
	defer rows.Close()

	var matches []Match
	for rows.Next() {
		var m Match
		if err := rows.Scan(
			&m.MatchedUserID, &m.Score, &m.Categories, &m.Explanation,
			&m.DisplayName, &m.Tagline, &m.Skills, &m.Intent, &m.AvatarURL,
		); err != nil {
			return nil, fmt.Errorf("scan match: %w", err)
		}
		matches = append(matches, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if matches == nil {
		matches = []Match{}
	}
	return matches, nil
}

// DismissMatch marks a match as dismissed so it no longer appears in results.
func (s *matchService) DismissMatch(ctx context.Context, userID, matchedUserID uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE match_cache SET dismissed = true, computed_at = NOW()
		 WHERE user_id = $1 AND matched_user_id = $2`,
		userID, matchedUserID,
	)
	if err != nil {
		return fmt.Errorf("dismiss match: %w", err)
	}
	return nil
}

// GetExplanation returns a cached explanation for the (currentUser, matchedUser) pair.
// On first call it generates one via Groq, saves it, and returns it.
// Subsequent calls return the cached value with no Groq call.
func (s *matchService) GetExplanation(ctx context.Context, matchedUserID, currentUserID uuid.UUID) (string, error) {
	// Check cache first
	var explanation *string
	err := s.pool.QueryRow(ctx,
		`SELECT explanation FROM match_cache WHERE user_id = $1 AND matched_user_id = $2`,
		currentUserID, matchedUserID,
	).Scan(&explanation)
	if err != nil {
		return "", fmt.Errorf("get explanation: %w", err)
	}
	if explanation != nil && *explanation != "" {
		return *explanation, nil
	}

	if s.explainer == nil {
		return "", nil
	}

	// Fetch both profiles
	fetchProfile := func(uid uuid.UUID) (Profile, error) {
		var p Profile
		p.UserID = uid
		var bio *string
		err := s.pool.QueryRow(ctx,
			`SELECT display_name, COALESCE(bio,''),
			        COALESCE(skills,  ARRAY[]::text[]),
			        COALESCE(interests, ARRAY[]::text[]),
			        COALESCE(intent,  ARRAY[]::text[])
			 FROM profiles WHERE user_id = $1`, uid,
		).Scan(&p.DisplayName, &bio, &p.Skills, &p.Interests, &p.Intent)
		if bio != nil {
			p.Bio = *bio
		}
		return p, err
	}

	profileA, err := fetchProfile(currentUserID)
	if err != nil {
		return "", fmt.Errorf("fetch current user profile: %w", err)
	}
	profileB, err := fetchProfile(matchedUserID)
	if err != nil {
		return "", fmt.Errorf("fetch matched user profile: %w", err)
	}

	text, err := s.explainer.ExplainMatch(ctx, profileA, profileB)
	if err != nil {
		return "", fmt.Errorf("explain match: %w", err)
	}

	// Persist so subsequent calls are free
	_, _ = s.pool.Exec(ctx,
		`UPDATE match_cache SET explanation = $3, computed_at = NOW()
		 WHERE user_id = $1 AND matched_user_id = $2`,
		currentUserID, matchedUserID, text)

	return text, nil
}

// RefreshCacheForUser recomputes match_cache for the given user using pgvector
// cosine similarity, the category algorithm from §9.3, and an upsert that
// preserves dismissed state and existing explanations.
func (s *matchService) RefreshCacheForUser(ctx context.Context, userID uuid.UUID) error {
	// Fetch this user's embedding and intent
	var embeddingStr string
	var userIntent []string
	err := s.pool.QueryRow(ctx,
		`SELECT embedding::text, COALESCE(intent, ARRAY[]::text[])
		 FROM profiles WHERE user_id = $1 AND embedding_status = 'current'`,
		userID,
	).Scan(&embeddingStr, &userIntent)
	if err != nil {
		return fmt.Errorf("fetch user embedding: %w", err)
	}
	if embeddingStr == "" {
		return nil // no embedding yet
	}

	// Find candidates via pgvector cosine similarity
	type candidate struct {
		userID uuid.UUID
		score  float64
		intent []string
	}

	rows, err := s.pool.Query(ctx, `
		SELECT
			p.user_id,
			1 - (p.embedding <=> $1::vector) AS score,
			COALESCE(p.intent, ARRAY[]::text[])
		FROM profiles p
		WHERE p.user_id          != $2
		  AND p.embedding        IS NOT NULL
		  AND p.embedding_status  = 'current'
		  AND p.visibility        = 'public'
		  AND p.onboarding_complete = true
		  AND 1 - (p.embedding <=> $1::vector) >= $3
		  AND NOT EXISTS (
			SELECT 1 FROM blocks
			WHERE (blocker_id = $2 AND blocked_id = p.user_id)
			   OR (blocker_id = p.user_id AND blocked_id = $2)
		  )
		ORDER BY score DESC
		LIMIT 200
	`, embeddingStr, userID, matchThreshold)
	if err != nil {
		return fmt.Errorf("vector search: %w", err)
	}
	defer rows.Close()

	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.userID, &c.score, &c.intent); err != nil {
			return fmt.Errorf("scan candidate: %w", err)
		}
		candidates = append(candidates, c)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	// Upsert each candidate — preserve dismissed + explanation
	var newIDs []string
	for _, c := range candidates {
		cats := computeCategories(userIntent, c.intent)
		if len(cats) == 0 {
			continue
		}
		newIDs = append(newIDs, "'"+c.userID.String()+"'")

		_, err := s.pool.Exec(ctx, `
			INSERT INTO match_cache (user_id, matched_user_id, score, categories)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id, matched_user_id) DO UPDATE SET
				score      = EXCLUDED.score,
				categories = EXCLUDED.categories,
				computed_at = NOW()
			-- dismissed and explanation are intentionally NOT overwritten
		`, userID, c.userID, c.score, cats)
		if err != nil {
			s.logger.Error("upsert match_cache", "user", userID, "matched", c.userID, "error", err)
		}
	}

	// Remove stale non-dismissed rows that fell below the threshold
	if len(newIDs) > 0 {
		_, _ = s.pool.Exec(ctx, fmt.Sprintf(`
			DELETE FROM match_cache
			WHERE user_id = $1
			  AND dismissed = false
			  AND matched_user_id NOT IN (%s)
		`, strings.Join(newIDs, ",")), userID)
	} else {
		// No candidates at all — remove all non-dismissed rows
		_, _ = s.pool.Exec(ctx,
			`DELETE FROM match_cache WHERE user_id = $1 AND dismissed = false`,
			userID)
	}

	return nil
}

// computeCategories applies the §9.3 algorithm to determine overlap categories.
func computeCategories(aIntent, bIntent []string) []string {
	has := func(intent []string, val string) bool {
		for _, v := range intent {
			if v == val {
				return true
			}
		}
		return false
	}

	var cats []string
	if has(aIntent, "cofounder") && has(bIntent, "cofounder") {
		cats = append(cats, "cofounder")
	}
	if has(aIntent, "teammate") && has(bIntent, "teammate") {
		cats = append(cats, "teammate")
	}
	if has(aIntent, "client") && (has(bIntent, "cofounder") || has(bIntent, "teammate")) {
		cats = append(cats, "client")
	}
	if has(bIntent, "client") && (has(aIntent, "cofounder") || has(aIntent, "teammate")) {
		if !contains(cats, "client") {
			cats = append(cats, "client")
		}
	}
	return cats
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
