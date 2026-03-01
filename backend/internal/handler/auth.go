package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"supernetwork/backend/internal/middleware"
	"supernetwork/backend/internal/model"
)

const wsTokenTTL = 60 * time.Second

// usedTokens prevents WS token replay. Entries are purged by a background
// goroutine started via StartTokenPurger.
var (
	usedTokens sync.Map // key: token string → value: expiry time.Time
)

// StartTokenPurger runs a background goroutine that evicts expired WS tokens
// from the replay-protection map every 60 seconds.
func StartTokenPurger() {
	go func() {
		ticker := time.NewTicker(wsTokenTTL)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			usedTokens.Range(func(k, v any) bool {
				if exp, ok := v.(time.Time); ok && now.After(exp) {
					usedTokens.Delete(k)
				}
				return true
			})
		}
	}()
}

// AuthHandler handles authentication-related API routes.
type AuthHandler struct {
	wsSecret []byte
}

// NewAuthHandler creates an AuthHandler.
func NewAuthHandler(wsTokenSecret string) *AuthHandler {
	return &AuthHandler{wsSecret: []byte(wsTokenSecret)}
}

// IssueWSToken issues a 60-second HMAC-signed WebSocket authentication token.
//
// @Summary     Issue WebSocket token
// @Tags        auth
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} map[string]interface{}
// @Failure     401 {object} model.AppError
// @Router      /api/v1/auth/ws-token [post]
func (h *AuthHandler) IssueWSToken(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)
	expiry := time.Now().Add(wsTokenTTL)

	token := h.signToken(userID, expiry)

	return c.JSON(fiber.Map{
		"token":      token,
		"expires_at": expiry.UTC().Format(time.RFC3339),
	})
}

// ValidateWSToken verifies a WS token and marks it used (single-use).
// Returns the user UUID and expiry, or an error if invalid/expired/replayed.
func (h *AuthHandler) ValidateWSToken(token string) (uuid.UUID, error) {
	// token format: <userID>:<expiry_unix>:<hmac>
	var rawUserID, expiryStr, sig string
	if n, _ := fmt.Sscanf(token, "%36s:%s", &rawUserID, &expiryStr); n < 2 {
		return uuid.Nil, fmt.Errorf("malformed token")
	}

	// Check replay before anything else
	if _, used := usedTokens.Load(token); used {
		return uuid.Nil, fmt.Errorf("token already used")
	}

	// Parse and verify the full token
	_ = sig // sig is embedded in expiryStr+hmac combined — re-sign to verify
	userID, err := uuid.Parse(rawUserID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid user id in token")
	}

	var expiryUnix int64
	if _, err := fmt.Sscanf(expiryStr, "%d:", &expiryUnix); err != nil {
		// Try direct parse if no colon
		if _, err := fmt.Sscanf(expiryStr, "%d", &expiryUnix); err != nil {
			return uuid.Nil, fmt.Errorf("invalid expiry in token")
		}
	}
	expiry := time.Unix(expiryUnix, 0)

	if time.Now().After(expiry) {
		return uuid.Nil, fmt.Errorf("token expired")
	}

	expected := h.signToken(userID, expiry)
	if !hmac.Equal([]byte(token), []byte(expected)) {
		return uuid.Nil, fmt.Errorf("invalid token signature")
	}

	// Mark used (single-use)
	usedTokens.Store(token, expiry)

	return userID, nil
}

// signToken produces the canonical signed token string.
func (h *AuthHandler) signToken(userID uuid.UUID, expiry time.Time) string {
	payload := fmt.Sprintf("%s:%d", userID.String(), expiry.Unix())
	mac := hmac.New(sha256.New, h.wsSecret)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s:%s", payload, sig)
}

// Ensure AppError is imported (used in swagger annotation).
var _ = model.AppError{}
