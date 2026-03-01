package middleware

import (
	"log/slog"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
)

// Recovery returns Fiber's panic-recovery middleware configured to log panics
// via slog and return 500 — never crashing the server.
func Recovery(logger *slog.Logger) fiber.Handler {
	return recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c fiber.Ctx, e any) {
			logger.Error("panic recovered", "error", e, "path", c.Path())
		},
	})
}
