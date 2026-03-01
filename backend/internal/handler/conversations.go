package handler

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"supernetwork/backend/internal/middleware"
	"supernetwork/backend/internal/model"
)

// ConversationHandler handles REST routes for conversations and messages.
type ConversationHandler struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

// NewConversationHandler creates a ConversationHandler.
func NewConversationHandler(pool *pgxpool.Pool, logger *slog.Logger) *ConversationHandler {
	return &ConversationHandler{pool: pool, logger: logger}
}

// ---------------------------------------------------------------------------
// POST /api/v1/conversations
// ---------------------------------------------------------------------------

// CreateConversation creates a conversation with the target user, or returns
// the existing one. Requires an accepted connection between the two users.
//
// @Summary     Create or get conversation
// @Tags        conversations
// @Accept      json
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} map[string]interface{}
// @Failure     400 {object} model.AppError
// @Failure     403 {object} model.AppError
// @Router      /api/v1/conversations [post]
func (h *ConversationHandler) CreateConversation(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)

	var body struct {
		UserID string `json:"user_id"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return model.NewAppError(model.ErrValidation, "invalid request body")
	}
	targetID, err := uuid.Parse(body.UserID)
	if err != nil {
		return model.NewAppError(model.ErrValidation, "invalid user_id")
	}
	if targetID == userID {
		return model.NewAppError(model.ErrValidation, "cannot start a conversation with yourself")
	}

	ctx := c.Context()

	// Return existing conversation if one already exists
	var existingID uuid.UUID
	err = h.pool.QueryRow(ctx,
		`SELECT cm1.conversation_id
         FROM conversation_members cm1
         JOIN conversation_members cm2
           ON cm1.conversation_id = cm2.conversation_id
         WHERE cm1.user_id = $1 AND cm2.user_id = $2
         LIMIT 1`,
		userID, targetID,
	).Scan(&existingID)
	if err == nil {
		return c.JSON(fiber.Map{"conversation_id": existingID.String()})
	}

	// Require an accepted connection
	var connected bool
	if err := h.pool.QueryRow(ctx,
		`SELECT EXISTS (
           SELECT 1 FROM connections
           WHERE status = 'accepted'
             AND ((requester_id = $1 AND recipient_id = $2)
               OR (requester_id = $2 AND recipient_id = $1))
         )`,
		userID, targetID,
	).Scan(&connected); err != nil || !connected {
		return model.NewAppError(model.ErrForbidden, "accepted connection required to start a conversation")
	}

	// Create conversation + add both members atomically
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		h.logger.Error("begin tx", "error", err)
		return model.NewAppError(model.ErrInternal, "could not create conversation")
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var convID uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO conversations DEFAULT VALUES RETURNING id`,
	).Scan(&convID); err != nil {
		return model.NewAppError(model.ErrInternal, "could not create conversation")
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO conversation_members (conversation_id, user_id)
         VALUES ($1, $2), ($1, $3)`,
		convID, userID, targetID,
	); err != nil {
		return model.NewAppError(model.ErrInternal, "could not create conversation")
	}

	if err := tx.Commit(ctx); err != nil {
		return model.NewAppError(model.ErrInternal, "could not create conversation")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"conversation_id": convID.String()})
}

// ---------------------------------------------------------------------------
// GET /api/v1/conversations
// ---------------------------------------------------------------------------

type conversationSummary struct {
	ID            string `json:"id"`
	OtherUserID   string `json:"other_user_id"`
	DisplayName   string `json:"display_name"`
	AvatarURL     string `json:"avatar_url"`
	LastMessage   string `json:"last_message"`
	LastMessageAt string `json:"last_message_at,omitempty"`
	UnreadCount   int    `json:"unread_count"`
}

// ListConversations returns all conversations for the authenticated user with
// last-message preview and unread count.
//
// @Summary     List conversations
// @Tags        conversations
// @Produce     json
// @Security    BearerAuth
// @Success     200 {object} map[string]interface{}
// @Router      /api/v1/conversations [get]
func (h *ConversationHandler) ListConversations(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)

	rows, err := h.pool.Query(c.Context(),
		`SELECT
           c.id,
           p.user_id AS other_user_id,
           COALESCE(p.display_name, '') AS display_name,
           COALESCE(p.avatar_url, '') AS avatar_url,
           COALESCE(
             (SELECT content FROM messages
              WHERE conversation_id = c.id
              ORDER BY created_at DESC LIMIT 1),
             '') AS last_message,
           (SELECT created_at FROM messages
            WHERE conversation_id = c.id
            ORDER BY created_at DESC LIMIT 1) AS last_message_at,
           (SELECT COUNT(*) FROM messages
            WHERE conversation_id = c.id
              AND sender_id != $1
              AND read_at IS NULL)::int AS unread_count
         FROM conversations c
         JOIN conversation_members cm  ON cm.conversation_id = c.id AND cm.user_id = $1
         JOIN conversation_members cm2 ON cm2.conversation_id = c.id AND cm2.user_id != $1
         JOIN profiles p ON p.user_id = cm2.user_id
         ORDER BY last_message_at DESC NULLS LAST`,
		userID,
	)
	if err != nil {
		h.logger.Error("list conversations", "error", err)
		return model.NewAppError(model.ErrInternal, "could not list conversations")
	}
	defer rows.Close()

	convs := make([]conversationSummary, 0)
	for rows.Next() {
		var s conversationSummary
		var lastAt *time.Time
		if err := rows.Scan(
			&s.ID, &s.OtherUserID, &s.DisplayName, &s.AvatarURL,
			&s.LastMessage, &lastAt, &s.UnreadCount,
		); err != nil {
			continue
		}
		if lastAt != nil {
			s.LastMessageAt = lastAt.UTC().Format(time.RFC3339)
		}
		convs = append(convs, s)
	}

	return c.JSON(fiber.Map{"conversations": convs})
}

// ---------------------------------------------------------------------------
// GET /api/v1/conversations/:id/messages
// ---------------------------------------------------------------------------

type messageRow struct {
	ID             string `json:"id"`
	ConversationID string `json:"conversation_id"`
	SenderID       string `json:"sender_id"`
	Content        string `json:"content"`
	ReadAt         string `json:"read_at,omitempty"`
	CreatedAt      string `json:"created_at"`
}

// GetMessages returns paginated message history for a conversation.
//   - ?after=<RFC3339>             — catch-up: messages newer than timestamp (ASC)
//   - ?before=<RFC3339>&before_id=<uuid> — cursor: 50 messages before cursor (DESC)
//   - (no params)                  — most recent 50 messages (DESC)
//
// @Summary     Get conversation messages
// @Tags        conversations
// @Produce     json
// @Security    BearerAuth
// @Param       id path string true "Conversation ID"
// @Param       before query string false "Cursor timestamp (RFC3339)"
// @Param       before_id query string false "Cursor message ID"
// @Param       after query string false "Catch-up: messages newer than this RFC3339 timestamp"
// @Success     200 {object} map[string]interface{}
// @Failure     403 {object} model.AppError
// @Router      /api/v1/conversations/{id}/messages [get]
func (h *ConversationHandler) GetMessages(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)

	convID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return model.NewAppError(model.ErrValidation, "invalid conversation id")
	}

	ctx := c.Context()

	// Verify membership
	var isMember bool
	if err := h.pool.QueryRow(ctx,
		`SELECT EXISTS (
           SELECT 1 FROM conversation_members
           WHERE conversation_id = $1 AND user_id = $2
         )`,
		convID, userID,
	).Scan(&isMember); err != nil || !isMember {
		return model.NewAppError(model.ErrForbidden, "not a member of this conversation")
	}

	const limit = 50

	// Catch-up after reconnect: messages newer than a given timestamp
	if afterStr := c.Query("after"); afterStr != "" {
		afterTime, err := time.Parse(time.RFC3339, afterStr)
		if err != nil {
			return model.NewAppError(model.ErrValidation, "invalid after timestamp (use RFC3339)")
		}
		rows, err := h.pool.Query(ctx,
			`SELECT id, conversation_id, sender_id, content, read_at, created_at
             FROM messages
             WHERE conversation_id = $1 AND created_at > $2
             ORDER BY created_at ASC, id ASC
             LIMIT $3`,
			convID, afterTime, limit,
		)
		if err != nil {
			return model.NewAppError(model.ErrInternal, "could not load messages")
		}
		defer rows.Close()
		return c.JSON(fiber.Map{"messages": scanMsgRows(rows)})
	}

	// Cursor pagination: messages before the given (created_at, id) cursor
	if beforeStr := c.Query("before"); beforeStr != "" {
		beforeTime, err := time.Parse(time.RFC3339, beforeStr)
		if err != nil {
			return model.NewAppError(model.ErrValidation, "invalid before timestamp (use RFC3339)")
		}
		beforeID, _ := uuid.Parse(c.Query("before_id")) // zero UUID if missing — still valid cursor

		rows, err := h.pool.Query(ctx,
			`SELECT id, conversation_id, sender_id, content, read_at, created_at
             FROM messages
             WHERE conversation_id = $1
               AND (created_at, id) < ($2, $3)
             ORDER BY created_at DESC, id DESC
             LIMIT $4`,
			convID, beforeTime, beforeID, limit,
		)
		if err != nil {
			return model.NewAppError(model.ErrInternal, "could not load messages")
		}
		defer rows.Close()
		return c.JSON(fiber.Map{"messages": scanMsgRows(rows)})
	}

	// Default: most recent 50
	rows, err := h.pool.Query(ctx,
		`SELECT id, conversation_id, sender_id, content, read_at, created_at
         FROM messages
         WHERE conversation_id = $1
         ORDER BY created_at DESC, id DESC
         LIMIT $2`,
		convID, limit,
	)
	if err != nil {
		return model.NewAppError(model.ErrInternal, "could not load messages")
	}
	defer rows.Close()
	return c.JSON(fiber.Map{"messages": scanMsgRows(rows)})
}

// ---------------------------------------------------------------------------
// PATCH /api/v1/conversations/:id/read
// ---------------------------------------------------------------------------

// MarkRead marks all unread messages sent by the other user as read.
//
// @Summary     Mark conversation as read
// @Tags        conversations
// @Security    BearerAuth
// @Param       id path string true "Conversation ID"
// @Success     204
// @Failure     403 {object} model.AppError
// @Router      /api/v1/conversations/{id}/read [patch]
func (h *ConversationHandler) MarkRead(c fiber.Ctx) error {
	userID := middleware.UserFromCtx(c)

	convID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return model.NewAppError(model.ErrValidation, "invalid conversation id")
	}

	ctx := c.Context()

	var isMember bool
	if err := h.pool.QueryRow(ctx,
		`SELECT EXISTS (
           SELECT 1 FROM conversation_members
           WHERE conversation_id = $1 AND user_id = $2
         )`,
		convID, userID,
	).Scan(&isMember); err != nil || !isMember {
		return model.NewAppError(model.ErrForbidden, "not a member of this conversation")
	}

	_, err = h.pool.Exec(ctx,
		`UPDATE messages
         SET read_at = NOW()
         WHERE conversation_id = $1
           AND sender_id != $2
           AND read_at IS NULL`,
		convID, userID,
	)
	if err != nil {
		h.logger.Error("mark read", "error", err)
		return model.NewAppError(model.ErrInternal, "could not mark messages as read")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func scanMsgRows(rows pgx.Rows) []messageRow {
	msgs := make([]messageRow, 0)
	for rows.Next() {
		var m messageRow
		var readAt *time.Time
		var createdAt time.Time
		if err := rows.Scan(
			&m.ID, &m.ConversationID, &m.SenderID, &m.Content, &readAt, &createdAt,
		); err != nil {
			continue
		}
		m.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		if readAt != nil {
			m.ReadAt = readAt.UTC().Format(time.RFC3339)
		}
		msgs = append(msgs, m)
	}
	return msgs
}
