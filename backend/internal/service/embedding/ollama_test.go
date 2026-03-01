package embedding_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"

	"supernetwork/backend/internal/service/embedding"
)

// TestOllamaEmbedding verifies the Ollama nomic-embed-text provider
// returns a 768-dimensional vector for a sample profile text.
func TestOllamaEmbedding(t *testing.T) {
	_ = godotenv.Load("../../../.env")

	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	provider := embedding.NewOllamaProvider(baseURL)
	text := embedding.BuildEmbeddingText(
		embedding.ProfileInput{
			Bio:       "Software engineer specialising in AI-powered products",
			Skills:    []string{"Go", "TypeScript", "machine learning"},
			Interests: []string{"AI", "distributed systems"},
			Intent:    []string{"cofounder"},
		},
		embedding.IkigaiInput{
			WhatYouLove:         "building products that make people's lives easier",
			WhatYoureGoodAt:     "backend systems and LLM integration",
			WhatWorldNeeds:      "reliable, explainable AI",
			WhatYouCanBePaidFor: "SaaS products and consulting",
		},
	)

	t.Logf("Embedding input (%d chars): %s...", len(text), text[:min(120, len(text))])

	vec, err := provider.Embed(ctx, text)
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}

	fmt.Printf("\n=== Embedding Result ===\n")
	fmt.Printf("dimensions: %d (expected 768)\n", len(vec))
	fmt.Printf("first 4:    %v\n", vec[:4])
	fmt.Println("========================")

	if len(vec) != 768 {
		t.Errorf("expected 768-dim vector, got %d", len(vec))
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
