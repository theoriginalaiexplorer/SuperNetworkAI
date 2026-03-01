package middleware

import (
	"crypto/subtle"

	"github.com/gofiber/fiber/v3"
	"supernetwork/backend/internal/model"
)

// RequireInternal guards /internal/* routes with a shared secret header.
// Used by Cloud Scheduler and cron jobs.
func RequireInternal(secret string) fiber.Handler {
	expected := []byte(secret)
	return func(c fiber.Ctx) error {
		got := []byte(c.Get("X-Internal-Secret"))
		if subtle.ConstantTimeCompare(got, expected) != 1 {
			return model.NewAppError(model.ErrUnauthorized, "invalid internal secret")
		}
		return c.Next()
	}
}
