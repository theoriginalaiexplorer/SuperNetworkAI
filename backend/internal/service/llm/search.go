package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"supernetwork/backend/internal/service"
)

// NLSearchParser calls Groq to parse a natural-language query into structured search params.
type NLSearchParser struct {
	client *openai.Client
}

// NewNLSearchParser creates an NLSearchParser wired to the Groq API.
func NewNLSearchParser(groqAPIKey string) *NLSearchParser {
	cfg := openai.DefaultConfig(groqAPIKey)
	cfg.BaseURL = "https://api.groq.com/openai/v1"
	return &NLSearchParser{client: openai.NewClientWithConfig(cfg)}
}

type searchParseResponse struct {
	EmbeddingText      string `json:"embedding_text"`
	IntentFilter       string `json:"intent_filter"`
	AvailabilityFilter string `json:"availability_filter"`
}

// ParseSearchQuery calls llama-3.1-8b-instant (JSON mode, 5s timeout) to convert a
// free-text query into embedding text + optional intent/availability filters.
func (p *NLSearchParser) ParseSearchQuery(ctx context.Context, query string) (*service.SearchParams, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	prompt := fmt.Sprintf(
		`You are extracting search parameters for a professional networking platform.
The user entered a natural language search query. Extract these fields:
- embedding_text: a clean description of the kind of person they seek (used for semantic search). Must be non-empty.
- intent_filter: if the query clearly mentions looking for a "cofounder", "teammate", or "client", output exactly one of those words; otherwise output "".
- availability_filter: if the query mentions availability (e.g. "full-time", "part-time", "freelance", "open-to-equity"), output the exact value; otherwise output "".

Return ONLY valid JSON: {"embedding_text":"...","intent_filter":"...","availability_filter":"..."}

User query: %s`, query)

	resp, err := p.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "llama-3.1-8b-instant",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{Type: "json_object"},
	})
	if err != nil {
		return nil, fmt.Errorf("groq request: %w", err)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("groq returned no choices")
	}

	var out searchParseResponse
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &out); err != nil {
		return nil, fmt.Errorf("parse groq response: %w", err)
	}
	if out.EmbeddingText == "" {
		out.EmbeddingText = query // fallback: use raw query for embedding
	}

	return &service.SearchParams{
		EmbeddingText:      out.EmbeddingText,
		IntentFilter:       out.IntentFilter,
		AvailabilityFilter: out.AvailabilityFilter,
	}, nil
}
