package service

import (
	"context"

	"github.com/google/uuid"
)

// --- Embedding ---

// EmbeddingProvider generates a fixed 768-dim embedding vector for a given text.
type EmbeddingProvider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// --- LLM (Interface Segregation: one interface per consumer) ---

// NLSearchParser parses a natural-language query into structured search params.
type NLSearchParser interface {
	ParseSearchQuery(ctx context.Context, query string) (*SearchParams, error)
}

// CVStructurer extracts structured profile data from raw CV text.
type CVStructurer interface {
	StructureCV(ctx context.Context, text string) (*CVData, error)
}

// MatchExplainer generates a human-readable explanation for a pair of matched profiles.
type MatchExplainer interface {
	ExplainMatch(ctx context.Context, a, b Profile) (string, error)
}

// IkigaiSummariser generates a concise AI summary of a user's Ikigai answers.
type IkigaiSummariser interface {
	SummariseIkigai(ctx context.Context, answers IkigaiAnswers) (string, error)
}

// --- Match Service ---

// MatchService manages match cache computation and retrieval.
type MatchService interface {
	GetMatches(ctx context.Context, userID uuid.UUID, f MatchFilter) ([]Match, error)
	GetExplanation(ctx context.Context, matchedUserID uuid.UUID, currentUserID uuid.UUID) (string, error)
	RefreshCacheForUser(ctx context.Context, userID uuid.UUID) error
}

// --- Shared value types used across interfaces ---

// Profile is a lightweight struct passed between service interfaces.
// Full DB types live in db/generated.
type Profile struct {
	UserID      uuid.UUID
	DisplayName string
	Skills      []string
	Interests   []string
	Intent      []string
	Bio         string
}

// IkigaiAnswers holds the four Ikigai question responses.
type IkigaiAnswers struct {
	WhatYouLove           string
	WhatYoureGoodAt       string
	WhatWorldNeeds        string
	WhatYouCanBePaidFor   string
}

// SearchParams is the structured output from NLSearchParser.
type SearchParams struct {
	EmbeddingText      string
	IntentFilter       string
	AvailabilityFilter string
}

// CVData is the structured output from CVStructurer.
type CVData struct {
	DisplayName  string
	Bio          string
	Skills       []string
	Interests    []string
	LinkedInURL  string
	GitHubURL    string
	PortfolioURL string
}

// Match represents a cached match entry.
type Match struct {
	MatchedUserID uuid.UUID
	Score         float64
	Categories    []string
	Explanation   string
	Dismissed     bool
}

// MatchFilter controls which matches to return.
type MatchFilter struct {
	Category string
	Limit    int
	Offset   int
}
