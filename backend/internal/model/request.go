package model

import "github.com/gofiber/fiber/v3"

// PaginationQuery is the standard pagination struct used across all list endpoints.
// Default limit: 20. Max: 100.
type PaginationQuery struct {
	Limit  int `query:"limit"`
	Offset int `query:"offset"`
}

// Validate clamps limit to [1, 100] and offset to >= 0.
func (p *PaginationQuery) Validate(c fiber.Ctx) error {
	if err := c.Bind().Query(p); err != nil {
		return NewAppError(ErrValidation, "invalid pagination parameters")
	}
	if p.Limit <= 0 {
		p.Limit = 20
	}
	if p.Limit > 100 {
		p.Limit = 100
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return nil
}
