package handler

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"

	"supernetwork/backend/internal/service"
)

// mockMatchService satisfies service.MatchService for handler tests.
type mockMatchService struct {
	getMatchesErr    error
	getMatches       []service.Match
	dismissErr       error
	explanationText  string
	explanationErr   error
}

func (m *mockMatchService) GetMatches(_ context.Context, _ uuid.UUID, _ service.MatchFilter) ([]service.Match, error) {
	return m.getMatches, m.getMatchesErr
}
func (m *mockMatchService) DismissMatch(_ context.Context, _, _ uuid.UUID) error {
	return m.dismissErr
}
func (m *mockMatchService) GetExplanation(_ context.Context, _, _ uuid.UUID) (string, error) {
	return m.explanationText, m.explanationErr
}
func (m *mockMatchService) RefreshCacheForUser(_ context.Context, _ uuid.UUID) error {
	return nil
}

func TestGetMatches_DefaultLimitAndOffset(t *testing.T) {
	mock := &mockMatchService{getMatches: []service.Match{}}
	h := NewMatchHandler(mock, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Get("/matches", h.GetMatches)

	req, _ := http.NewRequest("GET", "/matches", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, body)
	}
}

func TestGetMatches_ValidLimitAndOffset(t *testing.T) {
	mock := &mockMatchService{getMatches: []service.Match{}}
	h := NewMatchHandler(mock, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Get("/matches", h.GetMatches)

	req, _ := http.NewRequest("GET", "/matches?limit=50&offset=10", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetMatches_InvalidLimit_UseDefault(t *testing.T) {
	mock := &mockMatchService{getMatches: []service.Match{}}
	h := NewMatchHandler(mock, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Get("/matches", h.GetMatches)

	// Non-numeric limit should fall back to default (20), not error
	req, _ := http.NewRequest("GET", "/matches?limit=abc", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with default limit fallback, got %d", resp.StatusCode)
	}
}

func TestGetMatches_LimitOver100_UseDefault(t *testing.T) {
	mock := &mockMatchService{getMatches: []service.Match{}}
	h := NewMatchHandler(mock, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Get("/matches", h.GetMatches)

	// limit > 100 should clamp to default (20), not error
	req, _ := http.NewRequest("GET", "/matches?limit=500", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with clamped limit, got %d", resp.StatusCode)
	}
}

func TestGetMatches_NegativeOffset_UseDefault(t *testing.T) {
	mock := &mockMatchService{getMatches: []service.Match{}}
	h := NewMatchHandler(mock, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Get("/matches", h.GetMatches)

	req, _ := http.NewRequest("GET", "/matches?offset=-5", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	// Negative offset ignored, default (0) used
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with default offset for negative input, got %d", resp.StatusCode)
	}
}

func TestDismissMatch_InvalidUUID(t *testing.T) {
	mock := &mockMatchService{}
	h := NewMatchHandler(mock, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Post("/matches/:matchedUserId/dismiss", h.DismissMatch)

	req, _ := http.NewRequest("POST", "/matches/not-a-uuid/dismiss", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
}

func TestGetExplanation_InvalidUUID(t *testing.T) {
	mock := &mockMatchService{}
	h := NewMatchHandler(mock, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Get("/matches/:matchedUserId/explanation", h.GetExplanation)

	req, _ := http.NewRequest("GET", "/matches/not-a-uuid/explanation", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
}

func TestGetExplanation_ValidUUID(t *testing.T) {
	mock := &mockMatchService{explanationText: "You both love Go and AI."}
	h := NewMatchHandler(mock, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Get("/matches/:matchedUserId/explanation", h.GetExplanation)

	otherID := uuid.New()
	req, _ := http.NewRequest("GET", "/matches/"+otherID.String()+"/explanation", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}
