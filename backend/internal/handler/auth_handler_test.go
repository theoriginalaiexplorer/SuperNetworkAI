package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

const testWSSecret = "test-ws-hmac-secret-32-bytes-ok!"

func newTestAuthHandler() *AuthHandler {
	return NewAuthHandler(testWSSecret)
}

func TestSignAndValidateWSToken_RoundTrip(t *testing.T) {
	h := newTestAuthHandler()
	userID := uuid.New()

	expiry := time.Now().Add(wsTokenTTL)
	token := h.signToken(userID, expiry)

	got, err := h.ValidateWSToken(token)
	if err != nil {
		t.Fatalf("expected valid token to pass, got: %v", err)
	}
	if got != userID {
		t.Errorf("expected userID %s, got %s", userID, got)
	}
}

func TestValidateWSToken_ExpiredToken(t *testing.T) {
	h := newTestAuthHandler()
	userID := uuid.New()

	// Sign with expiry in the past
	expiry := time.Now().Add(-2 * time.Second)
	token := h.signToken(userID, expiry)

	_, err := h.ValidateWSToken(token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
}

func TestValidateWSToken_TamperedSignature(t *testing.T) {
	h := newTestAuthHandler()
	userID := uuid.New()

	token := h.signToken(userID, time.Now().Add(wsTokenTTL))
	// Tamper: flip the last character
	tampered := token[:len(token)-1] + "x"

	_, err := h.ValidateWSToken(tampered)
	if err == nil {
		t.Fatal("expected error for tampered token, got nil")
	}
}

func TestValidateWSToken_SingleUse(t *testing.T) {
	h := newTestAuthHandler()
	// Clear the global map for this test
	usedTokens.Range(func(k, _ any) bool { usedTokens.Delete(k); return true })

	userID := uuid.New()
	token := h.signToken(userID, time.Now().Add(wsTokenTTL))

	// First use — should succeed
	if _, err := h.ValidateWSToken(token); err != nil {
		t.Fatalf("first use failed: %v", err)
	}

	// Second use — should be rejected
	if _, err := h.ValidateWSToken(token); err == nil {
		t.Fatal("expected replay rejection on second use, got nil")
	}
}

func TestValidateWSToken_MalformedToken(t *testing.T) {
	h := newTestAuthHandler()
	cases := []string{
		"",
		"notavalidtoken",
		"::::",
		"only-one-part",
	}
	for _, tc := range cases {
		_, err := h.ValidateWSToken(tc)
		if err == nil {
			t.Errorf("expected error for malformed token %q, got nil", tc)
		}
	}
}

func TestValidateWSToken_WrongSecret(t *testing.T) {
	// Token signed by a different secret should not validate
	other := NewAuthHandler("completely-different-secret-32b!")
	h := newTestAuthHandler()

	userID := uuid.New()
	token := other.signToken(userID, time.Now().Add(wsTokenTTL))

	_, err := h.ValidateWSToken(token)
	if err == nil {
		t.Fatal("expected error for token signed by wrong secret, got nil")
	}
}

func TestIssueWSToken_HTTPResponse(t *testing.T) {
	h := newTestAuthHandler()
	app := newApp(testUserID)
	app.Use(injectUser(testUserID))
	app.Post("/auth/ws-token", h.IssueWSToken)

	req, _ := http.NewRequest("POST", "/auth/ws-token", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if result.Token == "" {
		t.Error("expected non-empty token in response")
	}
	if result.ExpiresAt == "" {
		t.Error("expected non-empty expires_at in response")
	}

	// Token from response should validate immediately
	gotUID, err := h.ValidateWSToken(result.Token)
	if err != nil {
		t.Errorf("token from response did not validate: %v", err)
	}
	if gotUID != testUserID {
		t.Errorf("token encodes wrong userID: got %s, want %s", gotUID, testUserID)
	}
}
