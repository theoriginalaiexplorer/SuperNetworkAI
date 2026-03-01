package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// OllamaProvider generates embeddings using a local Ollama instance.
// Model: nomic-embed-text (768-dim fixed).
type OllamaProvider struct {
	baseURL string
	client  *http.Client
}

// NewOllamaProvider creates an OllamaProvider.
func NewOllamaProvider(baseURL string) *OllamaProvider {
	return &OllamaProvider{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 5 * time.Second},
	}
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaResponse struct {
	Embedding []float32 `json:"embedding"`
}

// Embed calls the Ollama /api/embeddings endpoint and returns a 768-dim vector.
func (p *OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	body, err := json.Marshal(ollamaRequest{Model: "nomic-embed-text", Prompt: text})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var out ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(out.Embedding) == 0 {
		return nil, fmt.Errorf("ollama returned empty embedding")
	}
	return out.Embedding, nil
}
