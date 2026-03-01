package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

// CORS returns a CORS middleware that allows requests from the BFF origin only.
// In production, BFF_ORIGIN should be the Cloud Run BFF URL.
func CORS(bffOrigin string) fiber.Handler {
	return cors.New(cors.Config{
		AllowOrigins:     []string{bffOrigin},
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Content-Type", "Authorization", "X-Internal-Secret"},
		AllowCredentials: false,
	})
}
