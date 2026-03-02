package handler

import (
	"log/slog"
	"net/http"
	"os"
	"strings"
	"testing"
)

func newConnectionsApp() *http.Client {
	return nil // unused — we use app.Test directly
}

func TestCreateConnection_InvalidRecipientID(t *testing.T) {
	h := NewConnectionHandler(nil, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Post("/connections", h.CreateConnection)

	req, _ := http.NewRequest("POST", "/connections",
		strings.NewReader(`{"recipient_id":"not-a-uuid","message":""}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
	if code := parseErrorCode(t, resp); code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %s", code)
	}
}

func TestCreateConnection_SelfConnect(t *testing.T) {
	h := NewConnectionHandler(nil, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Post("/connections", h.CreateConnection)

	// recipient_id is the same as testUserID
	body := `{"recipient_id":"` + testUserID.String() + `"}`
	req, _ := http.NewRequest("POST", "/connections", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for self-connection, got %d", resp.StatusCode)
	}
	if code := parseErrorCode(t, resp); code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %s", code)
	}
}

func TestCreateConnection_EmptyBody(t *testing.T) {
	h := NewConnectionHandler(nil, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Post("/connections", h.CreateConnection)

	req, _ := http.NewRequest("POST", "/connections", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for empty recipient_id, got %d", resp.StatusCode)
	}
}

func TestUpdateConnection_InvalidStatus(t *testing.T) {
	h := NewConnectionHandler(nil, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Patch("/connections/:id", h.UpdateConnection)

	connID := "22222222-2222-2222-2222-222222222222"
	req, _ := http.NewRequest("PATCH", "/connections/"+connID,
		strings.NewReader(`{"status":"blocked"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for invalid status 'blocked', got %d", resp.StatusCode)
	}
	if code := parseErrorCode(t, resp); code != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %s", code)
	}
}

func TestUpdateConnection_InvalidConnID(t *testing.T) {
	h := NewConnectionHandler(nil, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Patch("/connections/:id", h.UpdateConnection)

	req, _ := http.NewRequest("PATCH", "/connections/not-a-uuid",
		strings.NewReader(`{"status":"accepted"}`))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for non-UUID conn ID, got %d", resp.StatusCode)
	}
}

func TestListConnections_InvalidStatusParam(t *testing.T) {
	h := NewConnectionHandler(nil, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Get("/connections", h.ListConnections)

	req, _ := http.NewRequest("GET", "/connections?status=unknown", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for invalid status param, got %d", resp.StatusCode)
	}
}

func TestGetStatus_InvalidUserID(t *testing.T) {
	h := NewConnectionHandler(nil, slog.New(slog.NewTextHandler(os.Stderr, nil)))
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Get("/connections/status/:userId", h.GetStatus)

	req, _ := http.NewRequest("GET", "/connections/status/not-a-uuid", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for invalid userId param, got %d", resp.StatusCode)
	}
}
