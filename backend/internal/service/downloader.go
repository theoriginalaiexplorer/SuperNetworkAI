package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	maxPDFBytes    = 5 * 1024 * 1024 // 5 MB
	downloadTimeout = 10 * time.Second
)

// allowedHosts is the allowlist for PDF download URLs (SSRF protection).
// Only Uploadthing CDN is allowed in production.
var allowedHosts = []string{
	"uploadthing.com",
	"utfs.io",
	"ufs.sh",
	// Local dev: allow localhost for testing
	"localhost",
	"127.0.0.1",
}

// DownloadPDF fetches a PDF from a URL with SSRF protection, a 10s timeout,
// and a 5MB cap. Returns the raw PDF bytes.
func DownloadPDF(ctx context.Context, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, fmt.Errorf("URL scheme must be http or https")
	}

	// SSRF allowlist check
	host := strings.ToLower(parsed.Hostname())
	allowed := false
	for _, h := range allowedHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, fmt.Errorf("URL host not allowed")
	}

	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: downloadTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", resp.StatusCode)
	}

	// MIME check
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "application/pdf") && !strings.Contains(ct, "octet-stream") {
		return nil, fmt.Errorf("URL did not return a PDF (Content-Type: %s)", ct)
	}

	// Size cap: read at most maxPDFBytes+1 bytes
	limited := io.LimitReader(resp.Body, int64(maxPDFBytes+1))
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(data) > maxPDFBytes {
		return nil, fmt.Errorf("PDF exceeds 5MB limit")
	}
	return data, nil
}
