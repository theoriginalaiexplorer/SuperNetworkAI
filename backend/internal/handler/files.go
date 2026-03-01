package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v3"
	"supernetwork/backend/internal/model"
)

// FilesHandler handles /api/v1/files/* routes.
type FilesHandler struct {
	apiKey string
	appId  string
}

// NewFilesHandler creates a FilesHandler with Uploadthing credentials.
func NewFilesHandler(apiKey, appId string) *FilesHandler {
	return &FilesHandler{apiKey: apiKey, appId: appId}
}

// presignRequest is the request body for Uploadthing presigned URL API.
type presignRequest struct {
	FileSlug string `json:"fileSlug"`
	FileName string `json:"fileName"`
	FileSize int64  `json:"fileSize"`
}

// presignResponse is the response from Uploadthing presigned URL API.
type presignResponse struct {
	PresignedURL string `json:"presignedUrl"`
	Key          string `json:"key"`
}

// Presign handles POST /api/v1/files/presign — returns an Uploadthing presigned URL.
//
// NOTE: This implementation uses Uploadthing REST API v6 directly.
// The official UploadThing Go SDK v5+ is recommended but not publicly available.
//
// To upgrade to official SDK: Contact UploadThing on Discord for access.
//
// Current implementation status:
// - Uses standard Go HTTP client
// - Handles authentication via API key
// - Generates presigned URLs for direct browser uploads
// - Validates file type, filename, and size
//
// API Documentation: https://uploadthing.com/docs
// Support: Discord (for SDK access)
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
		Type     string `json:"type"` // "avatar" | "cv"
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
	if body.Filename == "" {
		return model.NewAppError(model.ErrValidation, "filename is required")
	}

	fileSlug := fmt.Sprintf("%s-%d", body.Type, time.Now().Unix())

	reqBody := presignRequest{
		FileSlug: fileSlug,
		FileName: body.Filename,
		FileSize: body.Size,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return model.NewAppError(model.ErrInternal, "failed to marshal request")
	}

	req, err := http.NewRequest(
		"POST",
		fmt.Sprintf("https://api.uploadthing.com/v6/files/%s/presigned-url", fileSlug),
		bytes.NewReader(reqJSON),
	)
	if err != nil {
		return model.NewAppError(model.ErrInternal, "failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Uploadthing-Api-Key", h.apiKey)
	req.Header.Set("X-Uploadthing-Version", "v6")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return model.NewAppError(model.ErrServiceUnavailable, fmt.Sprintf("Uploadthing API error: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return model.NewAppError(model.ErrServiceUnavailable,
			fmt.Sprintf("Uploadthing API returned status %d", resp.StatusCode))
	}

	var presignResp presignResponse
	if err := json.NewDecoder(resp.Body).Decode(&presignResp); err != nil {
		return model.NewAppError(model.ErrInternal, "failed to parse Uploadthing response")
	}

	return c.JSON(fiber.Map{
		"presigned_url": presignResp.PresignedURL,
		"key":           presignResp.Key,
		"file_slug":     fileSlug,
	})
}
