package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestServer spins up a local HTTP server and returns its URL and a cleanup func.
func newTestServer(handler http.Handler) (*httptest.Server, string) {
	srv := httptest.NewServer(handler)
	return srv, srv.URL
}

func TestDownloadPDF_AllowedHosts(t *testing.T) {
	// Serve a minimal valid PDF-like response from localhost
	srv, base := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		fmt.Fprint(w, "%PDF-1.4 fake content")
	}))
	defer srv.Close()

	_, err := DownloadPDF(context.Background(), base+"/cv.pdf")
	if err != nil {
		t.Errorf("expected localhost to be allowed, got error: %v", err)
	}
}

func TestDownloadPDF_BlockedHost(t *testing.T) {
	_, err := DownloadPDF(context.Background(), "http://evil.com/steal.pdf")
	if err == nil {
		t.Fatal("expected error for blocked host, got nil")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("expected 'not allowed' error, got: %v", err)
	}
}

func TestDownloadPDF_BlockedHost_Internal(t *testing.T) {
	_, err := DownloadPDF(context.Background(), "http://169.254.169.254/latest/meta-data/")
	if err == nil {
		t.Fatal("expected error for metadata endpoint, got nil")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("expected 'not allowed' error, got: %v", err)
	}
}

func TestDownloadPDF_AllowedSubdomain(t *testing.T) {
	// sub.utfs.io should be allowed
	// We can't actually make a network call, but we can verify the allowlist logic
	// by providing a mocked URL that won't connect. What we want is: parsing/allowlist
	// check passes, and the error (if any) is a connection error, not an allowlist error.
	// Since we can't intercept DNS, just verify it doesn't reject the host itself.
	// We test this indirectly: the error must NOT be "URL host not allowed".
	_, err := DownloadPDF(context.Background(), "http://cdn.utfs.io/file.pdf")
	if err != nil && strings.Contains(err.Error(), "not allowed") {
		t.Errorf("expected cdn.utfs.io to pass allowlist, got: %v", err)
	}
}

func TestDownloadPDF_InvalidScheme(t *testing.T) {
	_, err := DownloadPDF(context.Background(), "ftp://localhost/file.pdf")
	if err == nil {
		t.Fatal("expected error for ftp scheme, got nil")
	}
	if !strings.Contains(err.Error(), "scheme") {
		t.Errorf("expected scheme error, got: %v", err)
	}
}

func TestDownloadPDF_InvalidURL(t *testing.T) {
	_, err := DownloadPDF(context.Background(), "://bad-url")
	if err == nil {
		t.Fatal("expected error for invalid URL, got nil")
	}
}

func TestDownloadPDF_NonPDFContentType(t *testing.T) {
	srv, base := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html>not a pdf</html>")
	}))
	defer srv.Close()

	_, err := DownloadPDF(context.Background(), base+"/page.html")
	if err == nil {
		t.Fatal("expected error for non-PDF content type, got nil")
	}
	if !strings.Contains(err.Error(), "Content-Type") {
		t.Errorf("expected Content-Type error, got: %v", err)
	}
}

func TestDownloadPDF_OctetStreamAllowed(t *testing.T) {
	srv, base := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		fmt.Fprint(w, "%PDF-1.4 binary content")
	}))
	defer srv.Close()

	_, err := DownloadPDF(context.Background(), base+"/file.bin")
	if err != nil {
		t.Errorf("expected octet-stream to be accepted, got: %v", err)
	}
}

func TestDownloadPDF_404Status(t *testing.T) {
	srv, base := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := DownloadPDF(context.Background(), base+"/missing.pdf")
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
	if !strings.Contains(err.Error(), "status 404") {
		t.Errorf("expected status 404 error, got: %v", err)
	}
}

func TestDownloadPDF_SizeLimit(t *testing.T) {
	// Serve more than 5MB
	const overLimit = maxPDFBytes + 100
	srv, base := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		big := make([]byte, overLimit)
		w.Write(big)
	}))
	defer srv.Close()

	_, err := DownloadPDF(context.Background(), base+"/big.pdf")
	if err == nil {
		t.Fatal("expected error for oversized PDF, got nil")
	}
	if !strings.Contains(err.Error(), "5MB") {
		t.Errorf("expected size limit error, got: %v", err)
	}
}

func TestDownloadPDF_ExactSizeLimit_Passes(t *testing.T) {
	// Serve exactly maxPDFBytes — should succeed
	srv, base := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Write(make([]byte, maxPDFBytes))
	}))
	defer srv.Close()

	data, err := DownloadPDF(context.Background(), base+"/exact.pdf")
	if err != nil {
		t.Errorf("expected success at exact size limit, got: %v", err)
	}
	if len(data) != maxPDFBytes {
		t.Errorf("expected %d bytes, got %d", maxPDFBytes, len(data))
	}
}
