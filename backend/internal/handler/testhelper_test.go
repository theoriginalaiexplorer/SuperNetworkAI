package handler

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"supernetwork/backend/internal/middleware"
	"supernetwork/backend/internal/model"
)

// injectUser is a test middleware that sets a known UUID into Fiber's Locals,
// bypassing RequireAuth so handler tests don't need real JWTs.
func injectUser(userID uuid.UUID) fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Locals("userID", userID)
		return c.Next()
	}
}

// testUserID is a fixed UUID for use across handler tests.
var testUserID = uuid.MustParse("11111111-1111-1111-1111-111111111111")

// newApp creates a Fiber app with the model error handler and panic recovery.
// Recovery ensures that nil-pool panics in "passes validation" tests yield 500
// instead of crashing the test process.
func newApp(_ uuid.UUID) *fiber.App {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	app := fiber.New(fiber.Config{ErrorHandler: model.ErrorHandler})
	app.Use(middleware.Recovery(logger))
	return app
}

// parseErrorCode reads the JSON response body and returns the "code" field.
func parseErrorCode(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var e struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		t.Fatalf("parse error JSON %q: %v", string(body), err)
	}
	return e.Code
}
