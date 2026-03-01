package middleware

import (
	"github.com/gofiber/fiber/v3"
	"supernetwork/backend/internal/model"
)

// RequireInternal guards /internal/* routes with a shared secret header.
// Used by Cloud Scheduler and cron jobs.
func RequireInternal(secret string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Get("X-Internal-Secret") != secret {
			return model.NewAppError(model.ErrUnauthorized, "invalid internal secret")
		}
		return c.Next()
	}
}
