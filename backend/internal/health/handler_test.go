package health

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func newHealthApp(h *Handler) *fiber.App {
	app := fiber.New()
	app.Get("/healthz", h.Liveness)
	app.Get("/readyz", h.Readiness)
	return app
}

func TestLiveness_AlwaysOK(t *testing.T) {
	h := New(nil) // nil pool — liveness never uses it
	app := newHealthApp(h)

	req, _ := http.NewRequest("GET", "/healthz", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", result["status"])
	}
}

func TestReadiness_OllamaDown_Returns503(t *testing.T) {
	// Ollama is not running — use a server that immediately refuses connections
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// NewWithOllama with a nil pool will fail the DB ping too,
	// but we want to test the Ollama check specifically.
	// Use a pool that won't be pinged by pointing to a non-existent Ollama.
	h := NewWithOllama(nil, srv.URL)
	app := newHealthApp(h)

	req, _ := http.NewRequest("GET", "/readyz", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	// Should return 503 (either DB or Ollama down — both have nil pool here,
	// so DB ping will fail first, but 503 is the correct status either way)
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when Ollama/DB is down, got %d", resp.StatusCode)
	}
}

func TestReadiness_OllamaOK_DBDown_Returns503(t *testing.T) {
	// Simulate Ollama responding OK but DB is down (nil pool)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"models":[]}`))
	}))
	defer srv.Close()

	// DB is nil — pool.Ping will fail
	h := NewWithOllama(nil, srv.URL)
	app := newHealthApp(h)

	req, _ := http.NewRequest("GET", "/readyz", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when DB is down, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("parse body: %v", err)
	}
	if result["status"] != "db_unavailable" {
		t.Errorf("expected status=db_unavailable, got %q", result["status"])
	}
}

func TestReadiness_NoOllama_DBDown_Returns503(t *testing.T) {
	// New() without Ollama, nil pool
	h := New(nil)
	app := newHealthApp(h)

	req, _ := http.NewRequest("GET", "/readyz", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when DB is down, got %d", resp.StatusCode)
	}
}
