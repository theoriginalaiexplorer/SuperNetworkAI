package handler

import (
	"github.com/gofiber/fiber/v3"
	"supernetwork/backend/internal/model"
)

// FilesHandler handles /api/v1/files/* routes.
type FilesHandler struct{}

// NewFilesHandler creates a FilesHandler.
func NewFilesHandler() *FilesHandler { return &FilesHandler{} }

// Presign handles POST /api/v1/files/presign — returns an Uploadthing presigned URL.
// Full Uploadthing integration implemented in Phase 3+ with the uploadthing SDK.
//
// @Summary     Get presigned upload URL
// @Tags        files
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} map[string]interface{}
// @Failure     503 {object} model.AppError
// @Router      /api/v1/files/presign [post]
func (h *FilesHandler) Presign(c fiber.Ctx) error {
	var body struct {
		Type     string `json:"type"`     // "avatar" | "cv"
		Filename string `json:"filename"`
		Size     int64  `json:"size"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return model.NewAppError(model.ErrValidation, "invalid request body")
	}
	if body.Type != "avatar" && body.Type != "cv" {
		return model.NewAppError(model.ErrValidation, "type must be 'avatar' or 'cv'")
	}
	if body.Size > 5*1024*1024 {
		return model.NewAppError(model.ErrValidation, "file size exceeds 5MB limit")
	}

	// TODO: integrate Uploadthing SDK — returns presigned URL for direct browser upload
	// For now return a placeholder to unblock Phase 3 testing
	return model.NewAppError(model.ErrServiceUnavailable, "file uploads not yet configured — set UPLOADTHING_SECRET")
}
