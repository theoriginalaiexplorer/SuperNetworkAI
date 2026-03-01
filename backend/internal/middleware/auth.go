package middleware

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"supernetwork/backend/internal/model"
)

const userIDKey = "userID"

// jwksCache caches the JWKS for 30 minutes.
type jwksCache struct {
	mu        sync.RWMutex
	set       jwk.Set
	fetchedAt time.Time
	url       string
}

var cache = &jwksCache{}

func getJWKS(ctx context.Context, url string) (jwk.Set, error) {
	cache.mu.RLock()
	if cache.set != nil && time.Since(cache.fetchedAt) < 30*time.Minute {
		set := cache.set
		cache.mu.RUnlock()
		return set, nil
	}
	cache.mu.RUnlock()

	cache.mu.Lock()
	defer cache.mu.Unlock()
	// Double-check after acquiring write lock
	if cache.set != nil && time.Since(cache.fetchedAt) < 30*time.Minute {
		return cache.set, nil
	}

	set, err := jwk.Fetch(ctx, url)
	if err != nil {
		return nil, err
	}
	cache.set = set
	cache.fetchedAt = time.Now()
	cache.url = url
	return set, nil
}

// RequireAuth verifies the Supabase JWT from the Authorization header.
// On success, stores the user UUID in c.Locals(userIDKey).
func RequireAuth(jwksURL string) fiber.Handler {
	return func(c fiber.Ctx) error {
		header := c.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			return model.NewAppError(model.ErrUnauthorized, "missing or malformed authorization header")
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")

		set, err := getJWKS(c.Context(), jwksURL)
		if err != nil {
			return model.NewAppError(model.ErrInternal, "unable to fetch auth keys")
		}

		token, err := jwt.ParseString(tokenStr, jwt.WithKeySet(set), jwt.WithValidate(true))
		if err != nil {
			return model.NewAppError(model.ErrUnauthorized, "invalid or expired token")
		}

		sub := token.Subject()
		uid, err := uuid.Parse(sub)
		if err != nil {
			return model.NewAppError(model.ErrUnauthorized, "invalid token subject")
		}

		c.Locals(userIDKey, uid)
		return c.Next()
	}
}

// UserFromCtx extracts the authenticated user UUID from the Fiber context.
// Must only be called inside a handler protected by RequireAuth.
func UserFromCtx(c fiber.Ctx) uuid.UUID {
	return c.Locals(userIDKey).(uuid.UUID)
}
