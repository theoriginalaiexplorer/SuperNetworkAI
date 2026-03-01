package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"supernetwork/backend/internal/service"
)

// CVStructurer calls Groq to extract structured profile data from raw CV text.
type CVStructurer struct {
	client *openai.Client
}

// NewCVStructurer creates a CVStructurer wired to the Groq API.
func NewCVStructurer(groqAPIKey string) *CVStructurer {
	cfg := openai.DefaultConfig(groqAPIKey)
	cfg.BaseURL = "https://api.groq.com/openai/v1"
	return &CVStructurer{client: openai.NewClientWithConfig(cfg)}
}

// StructureCV calls llama-3.3-70b-versatile (JSON mode) to extract structured
// profile fields from raw CV/resume text.
func (s *CVStructurer) StructureCV(ctx context.Context, text string) (*service.CVData, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Truncate input to ~12k chars to stay within context limits
	if len(text) > 12000 {
		text = text[:12000]
	}

	prompt := fmt.Sprintf(
		`You are extracting structured data from a CV/resume for a professional networking platform.
Return ONLY valid JSON with exactly this structure (omit fields if not found):
{
  "display_name": "Full Name",
  "bio": "2-3 sentence professional summary",
  "skills": ["skill1", "skill2"],
  "interests": ["interest1"],
  "linkedin_url": "https://...",
  "github_url": "https://...",
  "portfolio_url": "https://..."
}

CV text:
%s`, text)

	resp, err := s.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "llama-3.3-70b-versatile",
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

	var out service.CVData
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &out); err != nil {
		return nil, fmt.Errorf("parse groq response: %w", err)
	}
	return &out, nil
}
