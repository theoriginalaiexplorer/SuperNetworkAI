package embedding

import (
	"fmt"
	"strings"
)

// ProfileInput holds the fields used to build an embedding text.
type ProfileInput struct {
	DisplayName string
	Bio         string
	Skills      []string
	Interests   []string
	Intent      []string
}

// IkigaiInput holds the Ikigai answers used to build an embedding text.
type IkigaiInput struct {
	WhatYouLove           string
	WhatYoureGoodAt       string
	WhatWorldNeeds        string
	WhatYouCanBePaidFor   string
}

// BuildEmbeddingText is the SINGLE source of truth for constructing the text
// fed into the embedding model. Changing this function affects all embeddings —
// update embedding_status to 'stale' for all profiles if the format changes.
func BuildEmbeddingText(p ProfileInput, ik IkigaiInput) string {
	var sb strings.Builder

	if p.Bio != "" {
		fmt.Fprintf(&sb, "Bio: %s\n", p.Bio)
	}
	if len(p.Skills) > 0 {
		fmt.Fprintf(&sb, "Skills: %s\n", strings.Join(p.Skills, ", "))
	}
	if len(p.Interests) > 0 {
		fmt.Fprintf(&sb, "Interests: %s\n", strings.Join(p.Interests, ", "))
	}
	if len(p.Intent) > 0 {
		fmt.Fprintf(&sb, "Looking for: %s\n", strings.Join(p.Intent, ", "))
	}
	if ik.WhatYouLove != "" {
		fmt.Fprintf(&sb, "What I love: %s\n", ik.WhatYouLove)
	}
	if ik.WhatYoureGoodAt != "" {
		fmt.Fprintf(&sb, "What I'm good at: %s\n", ik.WhatYoureGoodAt)
	}
	if ik.WhatWorldNeeds != "" {
		fmt.Fprintf(&sb, "What the world needs: %s\n", ik.WhatWorldNeeds)
	}
	if ik.WhatYouCanBePaidFor != "" {
		fmt.Fprintf(&sb, "What I can be paid for: %s\n", ik.WhatYouCanBePaidFor)
	}

	return strings.TrimSpace(sb.String())
}
