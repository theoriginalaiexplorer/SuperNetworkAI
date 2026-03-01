package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// NomicProvider generates embeddings using the Nomic Embed API.
// Model: nomic-embed-text-v1.5 (768-dim, same as local Ollama model).
type NomicProvider struct {
	apiKey string
	client *http.Client
}

// NewNomicProvider creates a NomicProvider.
func NewNomicProvider(apiKey string) *NomicProvider {
	return &NomicProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

type nomicRequest struct {
	Model      string   `json:"model"`
	Texts      []string `json:"texts"`
	TaskType   string   `json:"task_type"`
}

type nomicResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// Embed calls the Nomic Embed API and returns a 768-dim vector.
func (p *NomicProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(nomicRequest{
		Model:    "nomic-embed-text-v1.5",
		Texts:    []string{text},
		TaskType: "search_document",
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api-atlas.nomic.ai/v1/embedding/text", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nomic request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nomic returned status %d", resp.StatusCode)
	}

	var out nomicResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(out.Embeddings) == 0 || len(out.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("nomic returned empty embedding")
	}
	return out.Embeddings[0], nil
}
