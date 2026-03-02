package middleware

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

var testSecret = []byte("super-secret-test-key-32bytes-ok")

// signTestJWT creates a signed HS256 JWT for tests.
func signTestJWT(t *testing.T, sub string, expiry time.Time) string {
	t.Helper()
	tok, err := jwt.NewBuilder().
		Subject(sub).
		Expiration(expiry).
		IssuedAt(time.Now()).
		Build()
	if err != nil {
		t.Fatalf("build jwt: %v", err)
	}
	signed, err := jwt.Sign(tok, jwt.WithKey(jwa.HS256, testSecret))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}
	return string(signed)
}

// newAuthTestApp returns a Fiber app with RequireAuth and a simple handler
// that echoes the user UUID.
func newAuthTestApp() *fiber.App {
	app := fiber.New(fiber.Config{
		ErrorHandler: func(c fiber.Ctx, err error) error {
			if ae, ok := err.(interface{ Error() string }); ok {
				return c.Status(fiber.StatusUnauthorized).SendString(ae.Error())
			}
			return c.Status(fiber.StatusInternalServerError).SendString(err.Error())
		},
	})
	app.Use(RequireAuth(testSecret))
	app.Get("/me", func(c fiber.Ctx) error {
		uid := UserFromCtx(c)
		return c.SendString(uid.String())
	})
	return app
}

func TestRequireAuth_ValidToken(t *testing.T) {
	app := newAuthTestApp()
	userID := uuid.New()
	token := signTestJWT(t, userID.String(), time.Now().Add(time.Hour))

	req, _ := http.NewRequest("GET", "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != userID.String() {
		t.Errorf("expected userID %s, got %s", userID.String(), string(body))
	}
}

func TestRequireAuth_MissingHeader(t *testing.T) {
	app := newAuthTestApp()

	req, _ := http.NewRequest("GET", "/me", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRequireAuth_BearerPrefixRequired(t *testing.T) {
	app := newAuthTestApp()
	userID := uuid.New()
	token := signTestJWT(t, userID.String(), time.Now().Add(time.Hour))

	// Send without "Bearer " prefix
	req, _ := http.NewRequest("GET", "/me", nil)
	req.Header.Set("Authorization", token)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing Bearer prefix, got %d", resp.StatusCode)
	}
}

func TestRequireAuth_ExpiredToken(t *testing.T) {
	app := newAuthTestApp()
	userID := uuid.New()
	// Token expired 1 second ago
	token := signTestJWT(t, userID.String(), time.Now().Add(-time.Second))

	req, _ := http.NewRequest("GET", "/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired token, got %d", resp.StatusCode)
	}
}

func TestRequireAuth_WrongSecret(t *testing.T) {
	app := newAuthTestApp()
	userID := uuid.New()

	// Sign with a different secret
	wrongSecret := []byte("wrong-secret-key-32-bytes-padding")
	tok, _ := jwt.NewBuilder().
		Subject(userID.String()).
		Expiration(time.Now().Add(time.Hour)).
		Build()
	signed, _ := jwt.Sign(tok, jwt.WithKey(jwa.HS256, wrongSecret))

	req, _ := http.NewRequest("GET", "/me", nil)
	req.Header.Set("Authorization", "Bearer "+string(signed))

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong secret, got %d", resp.StatusCode)
	}
}

func TestRequireAuth_GarbageToken(t *testing.T) {
	app := newAuthTestApp()

	req, _ := http.NewRequest("GET", "/me", nil)
	req.Header.Set("Authorization", "Bearer not.a.jwt")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for garbage token, got %d", resp.StatusCode)
	}
}

func TestRequireAuth_NonUUIDSubject(t *testing.T) {
	app := newAuthTestApp()

	tok, _ := jwt.NewBuilder().
		Subject("not-a-uuid").
		Expiration(time.Now().Add(time.Hour)).
		Build()
	signed, _ := jwt.Sign(tok, jwt.WithKey(jwa.HS256, testSecret))

	req, _ := http.NewRequest("GET", "/me", nil)
	req.Header.Set("Authorization", "Bearer "+string(signed))

	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for non-UUID subject, got %d", resp.StatusCode)
	}
}
