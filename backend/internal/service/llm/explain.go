package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"supernetwork/backend/internal/service"
)

// MatchExplainer calls Groq to generate a human-readable explanation for a matched pair.
type MatchExplainer struct {
	client *openai.Client
}

// NewMatchExplainer creates a MatchExplainer wired to the Groq API.
func NewMatchExplainer(groqAPIKey string) *MatchExplainer {
	cfg := openai.DefaultConfig(groqAPIKey)
	cfg.BaseURL = "https://api.groq.com/openai/v1"
	return &MatchExplainer{client: openai.NewClientWithConfig(cfg)}
}

type explainResponse struct {
	Explanation string `json:"explanation"`
}

// ExplainMatch calls llama-3.3-70b-versatile (JSON mode, 10s timeout) to generate
// a 2-3 sentence explanation of why two profiles are a strong match.
func (e *MatchExplainer) ExplainMatch(ctx context.Context, a, b service.Profile) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	prompt := fmt.Sprintf(
		`You are explaining why two people on a professional networking platform are a strong match.
Write 2-3 sentences that highlight their complementary skills, shared goals, or aligned purpose.
Be specific. Avoid generic phrases like "they would work well together" or "they have a lot in common".
Return ONLY valid JSON: {"explanation":"..."}

Person A:
- Name: %s
- Bio: %s
- Skills: %s
- Interests: %s
- Looking for: %s

Person B:
- Name: %s
- Bio: %s
- Skills: %s
- Interests: %s
- Looking for: %s`,
		a.DisplayName, a.Bio,
		strings.Join(a.Skills, ", "), strings.Join(a.Interests, ", "), strings.Join(a.Intent, ", "),
		b.DisplayName, b.Bio,
		strings.Join(b.Skills, ", "), strings.Join(b.Interests, ", "), strings.Join(b.Intent, ", "),
	)

	resp, err := e.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: "llama-3.3-70b-versatile",
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

	var out explainResponse
	if err := json.Unmarshal([]byte(resp.Choices[0].Message.Content), &out); err != nil {
		return "", fmt.Errorf("parse groq response: %w", err)
	}
	return out.Explanation, nil
}
