package service_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"

	"supernetwork/backend/internal/service"
	"supernetwork/backend/internal/service/llm"
)

// TestCVPipeline exercises the full CV import pipeline:
// DownloadPDF → ExtractPDFText → CVStructurer.StructureCV
// Uses a PDF served on localhost (within the SSRF allowlist).
func TestCVPipeline(t *testing.T) {
	_ = godotenv.Load("../../.env")

	groqKey := os.Getenv("GROQ_API_KEY")
	if groqKey == "" {
		t.Skip("GROQ_API_KEY not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const pdfURL = "http://localhost:8888/test-cv.pdf"

	// Step 1: Download
	t.Log("Step 1: downloading PDF from", pdfURL)
	data, err := service.DownloadPDF(ctx, pdfURL)
	if err != nil {
		t.Fatalf("DownloadPDF: %v", err)
	}
	t.Logf("  → downloaded %d bytes", len(data))

	// Step 2: Extract text
	t.Log("Step 2: extracting text")
	text, err := service.ExtractPDFText(data)
	if err != nil {
		t.Fatalf("ExtractPDFText: %v", err)
	}
	t.Logf("  → extracted %d chars", len(text))
	if len(text) > 300 {
		t.Logf("  → preview: %s...", text[:300])
	} else {
		t.Logf("  → full text: %s", text)
	}

	// Step 3: LLM structuring
	t.Log("Step 3: calling Groq LLM to structure CV")
	structurer := llm.NewCVStructurer(groqKey)
	cv, err := structurer.StructureCV(ctx, text)
	if err != nil {
		t.Fatalf("StructureCV: %v", err)
	}

	fmt.Printf("\n=== Structured CV Data ===\n")
	fmt.Printf("display_name:  %q\n", cv.DisplayName)
	fmt.Printf("bio:           %q\n", cv.Bio)
	fmt.Printf("skills:        %v\n", cv.Skills)
	fmt.Printf("interests:     %v\n", cv.Interests)
	fmt.Printf("linkedin_url:  %q\n", cv.LinkedInURL)
	fmt.Printf("github_url:    %q\n", cv.GitHubURL)
	fmt.Printf("portfolio_url: %q\n", cv.PortfolioURL)
	fmt.Println("==========================")

	if cv.DisplayName == "" && len(cv.Skills) == 0 {
		t.Error("LLM returned empty CV data — check prompt or model response")
	}
}
