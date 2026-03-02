package middleware

import (
	"io"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"

	"supernetwork/backend/internal/model"
)

func newInternalTestApp(secret string) *fiber.App {
	app := fiber.New(fiber.Config{ErrorHandler: model.ErrorHandler})
	app.Use(RequireInternal(secret))
	app.Post("/internal/action", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})
	return app
}

func TestRequireInternal_CorrectSecret(t *testing.T) {
	app := newInternalTestApp("my-secret")

	req, _ := http.NewRequest("POST", "/internal/action", nil)
	req.Header.Set("X-Internal-Secret", "my-secret")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

func TestRequireInternal_WrongSecret(t *testing.T) {
	app := newInternalTestApp("my-secret")

	req, _ := http.NewRequest("POST", "/internal/action", nil)
	req.Header.Set("X-Internal-Secret", "wrong-secret")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRequireInternal_MissingHeader(t *testing.T) {
	app := newInternalTestApp("my-secret")

	req, _ := http.NewRequest("POST", "/internal/action", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing header, got %d", resp.StatusCode)
	}
}

func TestRequireInternal_EmptySecret_RequiresEmptyHeader(t *testing.T) {
	// When configured with empty secret, only empty header matches
	app := newInternalTestApp("")

	req, _ := http.NewRequest("POST", "/internal/action", nil)
	req.Header.Set("X-Internal-Secret", "anything")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 when secret is empty and header has value, got %d", resp.StatusCode)
	}
}

func TestRequireInternal_DoesNotLeakSecret(t *testing.T) {
	// Ensure the response body never contains the expected secret
	app := newInternalTestApp("super-secret-value")

	req, _ := http.NewRequest("POST", "/internal/action", nil)
	req.Header.Set("X-Internal-Secret", "wrong")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	if contains := func(s, substr string) bool {
		return len(s) >= len(substr) && (s == substr || len(s) > 0 &&
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())
	}(string(body), "super-secret-value"); contains {
		t.Errorf("response body leaks expected secret: %s", string(body))
	}
}
