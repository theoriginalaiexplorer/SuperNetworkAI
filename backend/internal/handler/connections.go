package handler

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"supernetwork/backend/internal/middleware"
	"supernetwork/backend/internal/model"
)

// ConnectionHandler handles /api/v1/connections/* routes.
type ConnectionHandler struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewConnectionHandler creates a ConnectionHandler.
func NewConnectionHandler(pool *pgxpool.Pool, logger *slog.Logger) *ConnectionHandler {
	return &ConnectionHandler{pool: pool, logger: logger}
}

// CreateConnection handles POST /api/v1/connections.
//
// @Summary     Send a connection request
// @Tags        connections
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       body body object true "{ \"recipient_id\": \"uuid\", \"message\": \"...\" }"
// @Success     201 {object} map[string]interface{}
// @Failure     409 {object} model.AppError
// @Router      /api/v1/connections [post]
func (h *ConnectionHandler) CreateConnection(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)

	var body struct {
		RecipientID string `json:"recipient_id"`
		Message     string `json:"message"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return model.NewAppError(model.ErrValidation, "invalid request body")
	}
	recipientID, err := uuid.Parse(body.RecipientID)
	if err != nil {
		return model.NewAppError(model.ErrValidation, "invalid recipient_id")
	}
	if recipientID == userID {
		return model.NewAppError(model.ErrValidation, "cannot connect with yourself")
	}

	// Check for existing connection in either direction
	var exists bool
	_ = h.pool.QueryRow(c.Context(),
		`SELECT EXISTS (
		   SELECT 1 FROM connections
		   WHERE (requester_id=$1 AND recipient_id=$2)
		      OR (requester_id=$2 AND recipient_id=$1)
		 )`, userID, recipientID,
	).Scan(&exists)
	if exists {
		return model.NewAppError(model.ErrConflict, "connection already exists")
	}

	var msgArg *string
	if body.Message != "" {
		msgArg = &body.Message
	}

	var connID uuid.UUID
	err = h.pool.QueryRow(c.Context(),
		`INSERT INTO connections (requester_id, recipient_id, message)
		 VALUES ($1, $2, $3) RETURNING id`,
		userID, recipientID, msgArg,
	).Scan(&connID)
	if err != nil {
		h.logger.Error("create connection", "error", err)
		return model.NewAppError(model.ErrInternal, "failed to create connection")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"connection_id": connID,
		"status":        "pending",
	})
}

// connection is the response shape for list and status endpoints.
type connection struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"`
	Direction   string    `json:"direction"` // "sent" | "received"
	Message     string    `json:"message"`
	OtherUserID string    `json:"other_user_id"`
	DisplayName string    `json:"display_name"`
	Tagline     string    `json:"tagline"`
	AvatarURL   string    `json:"avatar_url"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListConnections handles GET /api/v1/connections?status=pending|accepted.
//
// @Summary     List connections
// @Tags        connections
// @Produce     json
// @Security    BearerAuth
// @Param       status query string false "pending | accepted (default: accepted)"
// @Success     200 {object} map[string]interface{}
// @Router      /api/v1/connections [get]
func (h *ConnectionHandler) ListConnections(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)
	status := c.Query("status", "accepted")
	if status != "pending" && status != "accepted" {
		return model.NewAppError(model.ErrValidation, "status must be 'pending' or 'accepted'")
	}

	rows, err := h.pool.Query(c.Context(), `
		SELECT
			c.id,
			c.requester_id,
			c.recipient_id,
			c.status,
			COALESCE(c.message, ''),
			c.created_at,
			CASE WHEN c.requester_id=$1 THEN c.recipient_id ELSE c.requester_id END AS other_id,
			p.display_name,
			COALESCE(p.tagline,   ''),
			COALESCE(p.avatar_url,'')
		FROM connections c
		JOIN profiles p ON p.user_id =
			CASE WHEN c.requester_id=$1 THEN c.recipient_id ELSE c.requester_id END
		WHERE (c.requester_id=$1 OR c.recipient_id=$1)
		  AND c.status=$2
		ORDER BY c.updated_at DESC
	`, userID, status)
	if err != nil {
		h.logger.Error("list connections", "error", err)
		return model.NewAppError(model.ErrInternal, "failed to list connections")
	}
	defer rows.Close()

	var list []connection
	for rows.Next() {
		var conn connection
		var connID, requesterID, recipientID, otherID uuid.UUID
		if err := rows.Scan(
			&connID, &requesterID, &recipientID, &conn.Status,
			&conn.Message, &conn.CreatedAt, &otherID,
			&conn.DisplayName, &conn.Tagline, &conn.AvatarURL,
		); err != nil {
			return model.NewAppError(model.ErrInternal, "scan error")
		}
		conn.ID = connID.String()
		conn.OtherUserID = otherID.String()
		if requesterID == userID {
			conn.Direction = "sent"
		} else {
			conn.Direction = "received"
		}
		list = append(list, conn)
	}
	if err := rows.Err(); err != nil {
		return model.NewAppError(model.ErrInternal, "query error")
	}
	if list == nil {
		list = []connection{}
	}

	return c.JSON(fiber.Map{"connections": list, "count": len(list)})
}

// UpdateConnection handles PATCH /api/v1/connections/:id.
// Only the recipient may accept or reject a pending request.
//
// @Summary     Accept or reject a connection request
// @Tags        connections
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Param       id   path   string true "Connection UUID"
// @Param       body body   object true "{ \"status\": \"accepted\" | \"rejected\" }"
// @Success     200 {object} map[string]interface{}
// @Failure     403 {object} model.AppError
// @Router      /api/v1/connections/{id} [patch]
func (h *ConnectionHandler) UpdateConnection(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)
	connID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return model.NewAppError(model.ErrValidation, "invalid connection id")
	}

	var body struct {
		Status string `json:"status"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return model.NewAppError(model.ErrValidation, "invalid request body")
	}
	if body.Status != "accepted" && body.Status != "rejected" {
		return model.NewAppError(model.ErrValidation, "status must be 'accepted' or 'rejected'")
	}

	result, err := h.pool.Exec(c.Context(),
		`UPDATE connections SET status=$3, updated_at=NOW()
		 WHERE id=$1 AND recipient_id=$2 AND status='pending'`,
		connID, userID, body.Status,
	)
	if err != nil {
		h.logger.Error("update connection", "error", err)
		return model.NewAppError(model.ErrInternal, "failed to update connection")
	}
	if result.RowsAffected() == 0 {
		return model.NewAppError(model.ErrForbidden, "connection not found or not authorised")
	}

	return c.JSON(fiber.Map{"status": body.Status})
}

// GetStatus handles GET /api/v1/connections/status/:userId.
//
// @Summary     Get connection status with a user
// @Tags        connections
// @Produce     json
// @Security    BearerAuth
// @Param       userId path string true "Target user UUID"
// @Success     200 {object} map[string]interface{}
// @Router      /api/v1/connections/status/{userId} [get]
func (h *ConnectionHandler) GetStatus(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)
	otherID, err := uuid.Parse(c.Params("userId"))
	if err != nil {
		return model.NewAppError(model.ErrValidation, "invalid userId")
	}

	var connID uuid.UUID
	var status, direction string
	err = h.pool.QueryRow(c.Context(), `
		SELECT id, status,
		       CASE WHEN requester_id=$1 THEN 'sent' ELSE 'received' END
		FROM connections
		WHERE (requester_id=$1 AND recipient_id=$2)
		   OR (requester_id=$2 AND recipient_id=$1)
	`, userID, otherID).Scan(&connID, &status, &direction)
	if err != nil {
		// No row → no connection
		return c.JSON(fiber.Map{"status": "none"})
	}

	return c.JSON(fiber.Map{
		"status":        status,
		"connection_id": connID,
		"direction":     direction,
	})
}
