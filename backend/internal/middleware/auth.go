package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwa"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"supernetwork/backend/internal/model"
)

const userIDKey = "userID"

// RequireAuth validates the BFF-signed HS256 JWT from the Authorization header.
// bffSecret is the raw bytes of BFF_JWT_SECRET (shared with the BFF).
// On success, stores the user UUID in c.Locals(userIDKey).
func RequireAuth(bffSecret []byte) fiber.Handler {
	return func(c fiber.Ctx) error {
		header := c.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			return model.NewAppError(model.ErrUnauthorized, "missing or malformed authorization header")
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")

		token, err := jwt.ParseString(tokenStr,
			jwt.WithKey(jwa.HS256, bffSecret),
			jwt.WithValidate(true),
		)
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
