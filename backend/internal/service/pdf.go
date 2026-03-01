package service

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// ExtractPDFText extracts plain text from a PDF byte slice.
// Returns an error for image-only (scanned) PDFs where no text is found.
func ExtractPDFText(data []byte) (string, error) {
	r := bytes.NewReader(data)

	pr, err := pdf.NewReader(r, int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}

	var sb strings.Builder
	for i := 1; i <= pr.NumPage(); i++ {
		p := pr.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			continue // skip pages with errors; try remaining pages
		}
		sb.WriteString(text)
		sb.WriteString("\n")
	}

	result := strings.TrimSpace(sb.String())
	if result == "" {
		return "", fmt.Errorf("no text found in PDF — may be image-only (scanned)")
	}
	return result, nil
}
