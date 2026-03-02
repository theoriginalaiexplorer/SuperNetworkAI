package embedding

import (
	"strings"
	"testing"
)

func TestBuildEmbeddingText(t *testing.T) {
	t.Run("full profile and ikigai", func(t *testing.T) {
		p := ProfileInput{
			Bio:       "Experienced engineer",
			Skills:    []string{"Go", "TypeScript"},
			Interests: []string{"AI", "distributed systems"},
			Intent:    []string{"cofounder", "teammate"},
		}
		ik := IkigaiInput{
			WhatYouLove:         "building products",
			WhatYoureGoodAt:     "backend engineering",
			WhatWorldNeeds:      "reliable software",
			WhatYouCanBePaidFor: "consulting and coding",
		}

		got := BuildEmbeddingText(p, ik)

		for _, want := range []string{
			"Bio: Experienced engineer",
			"Skills: Go, TypeScript",
			"Interests: AI, distributed systems",
			"Looking for: cofounder, teammate",
			"What I love: building products",
			"What I'm good at: backend engineering",
			"What the world needs: reliable software",
			"What I can be paid for: consulting and coding",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("expected output to contain %q, got:\n%s", want, got)
			}
		}
	})

	t.Run("empty profile and ikigai returns empty string", func(t *testing.T) {
		got := BuildEmbeddingText(ProfileInput{}, IkigaiInput{})
		if got != "" {
			t.Errorf("expected empty string, got %q", got)
		}
	})

	t.Run("profile only — no ikigai", func(t *testing.T) {
		p := ProfileInput{
			Bio:    "Solo founder",
			Skills: []string{"React"},
		}
		got := BuildEmbeddingText(p, IkigaiInput{})
		if !strings.Contains(got, "Bio: Solo founder") {
			t.Errorf("expected Bio line, got %q", got)
		}
		if strings.Contains(got, "What I love") {
			t.Errorf("expected no ikigai lines, got %q", got)
		}
	})

	t.Run("ikigai only — no profile", func(t *testing.T) {
		ik := IkigaiInput{
			WhatYouLove:         "design",
			WhatYoureGoodAt:     "UX",
			WhatWorldNeeds:      "better interfaces",
			WhatYouCanBePaidFor: "product design",
		}
		got := BuildEmbeddingText(ProfileInput{}, ik)
		if strings.Contains(got, "Bio:") || strings.Contains(got, "Skills:") {
			t.Errorf("expected no profile lines, got %q", got)
		}
		if !strings.Contains(got, "What I love: design") {
			t.Errorf("expected ikigai lines, got %q", got)
		}
	})

	t.Run("result is trimmed — no leading or trailing whitespace", func(t *testing.T) {
		p := ProfileInput{Bio: "test"}
		got := BuildEmbeddingText(p, IkigaiInput{})
		if got != strings.TrimSpace(got) {
			t.Errorf("result has leading/trailing whitespace: %q", got)
		}
	})

	t.Run("skills and interests joined with comma-space", func(t *testing.T) {
		p := ProfileInput{
			Skills:    []string{"A", "B", "C"},
			Interests: []string{"X", "Y"},
		}
		got := BuildEmbeddingText(p, IkigaiInput{})
		if !strings.Contains(got, "Skills: A, B, C") {
			t.Errorf("skills not joined correctly, got %q", got)
		}
		if !strings.Contains(got, "Interests: X, Y") {
			t.Errorf("interests not joined correctly, got %q", got)
		}
	})

	t.Run("empty skills slice omitted", func(t *testing.T) {
		p := ProfileInput{Bio: "hello", Skills: []string{}}
		got := BuildEmbeddingText(p, IkigaiInput{})
		if strings.Contains(got, "Skills:") {
			t.Errorf("expected Skills line to be omitted for empty slice, got %q", got)
		}
	})
}
