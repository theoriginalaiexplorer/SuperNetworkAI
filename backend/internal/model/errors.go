package model

import (
	"github.com/gofiber/fiber/v3"
)

// Error codes — exhaustive registry per §7 API Contract.
const (
	ErrUnauthorized       = "UNAUTHORIZED"
	ErrForbidden          = "FORBIDDEN"
	ErrNotFound           = "NOT_FOUND"
	ErrConflict           = "CONFLICT"
	ErrValidation         = "VALIDATION_ERROR"
	ErrRateLimited        = "RATE_LIMITED"
	ErrInternal           = "INTERNAL_ERROR"
	ErrServiceUnavailable = "SERVICE_UNAVAILABLE"
)

// AppError is the standard error envelope returned by all API endpoints.
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *AppError) Error() string { return e.Message }

// New returns an AppError.
func NewAppError(code, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

// ErrorHandler is the Fiber global error handler. Register as:
//
//	app.Use(ErrorHandler)
func ErrorHandler(c fiber.Ctx, err error) error {
	if ae, ok := err.(*AppError); ok {
		status := appErrorStatus(ae.Code)
		return c.Status(status).JSON(ae)
	}

	// Fiber's own errors (e.g., 404 from router)
	if fe, ok := err.(*fiber.Error); ok {
		return c.Status(fe.Code).JSON(&AppError{
			Code:    "INTERNAL_ERROR",
			Message: fe.Message,
		})
	}

	// Unknown error → 500
	return c.Status(fiber.StatusInternalServerError).JSON(&AppError{
		Code:    ErrInternal,
		Message: "an unexpected error occurred",
	})
}

func appErrorStatus(code string) int {
	switch code {
	case ErrUnauthorized:
		return fiber.StatusUnauthorized
	case ErrForbidden:
		return fiber.StatusForbidden
	case ErrNotFound:
		return fiber.StatusNotFound
	case ErrConflict:
		return fiber.StatusConflict
	case ErrValidation:
		return fiber.StatusUnprocessableEntity
	case ErrRateLimited:
		return fiber.StatusTooManyRequests
	case ErrServiceUnavailable:
		return fiber.StatusServiceUnavailable
	default:
		return fiber.StatusInternalServerError
	}
}
