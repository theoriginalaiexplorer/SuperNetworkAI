package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"supernetwork/backend/internal/service"
)

// IkigaiSummariser calls Groq to generate a concise AI summary of Ikigai answers.
type IkigaiSummariser struct {
	client *openai.Client
}

// NewIkigaiSummariser creates an IkigaiSummariser wired to the Groq API.
func NewIkigaiSummariser(groqAPIKey string) *IkigaiSummariser {
	cfg := openai.DefaultConfig(groqAPIKey)
	cfg.BaseURL = "https://api.groq.com/openai/v1"
	return &IkigaiSummariser{client: openai.NewClientWithConfig(cfg)}
}

type ikigaiSummaryResponse struct {
	Summary string `json:"summary"`
}

// SummariseIkigai calls llama-3.1-8b-instant (JSON mode) to generate a 2-3 sentence
// Ikigai summary used as part of the user's match explanation.
func (s *IkigaiSummariser) SummariseIkigai(ctx context.Context, answers service.IkigaiAnswers) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	prompt := fmt.Sprintf(
		`You are summarising a person's Ikigai for a professional networking platform.
Write a 2-3 sentence summary in third person, focusing on their unique intersection of passion, skills, purpose and livelihood.
Return ONLY valid JSON: {"summary": "..."}

Ikigai answers:
- What they love: %s
- What they are good at: %s
- What the world needs: %s
- What they can be paid for: %s`,
		answers.WhatYouLove, answers.WhatYoureGoodAt,
		answers.WhatWorldNeeds, answers.WhatYouCanBePaidFor,
	)

	resp, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "llama-3.1-8b-instant",
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: prompt},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{Type: "json_object"},
	})
	if err != nil {
		return "", fmt.Errorf("groq request: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("groq returned no choices")
	}

	var out ikigaiSummaryResponse
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &out); err != nil {
		return "", fmt.Errorf("parse groq response: %w", err)
	}
	return out.Summary, nil
}
