package ws

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valyala/fasthttp"

	"supernetwork/backend/internal/model"
)

// ---------------------------------------------------------------------------
// Wire types — all WS messages share this envelope
// ---------------------------------------------------------------------------

type wsMsg struct {
	Type           string `json:"type"`
	Token          string `json:"token,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	Content        string `json:"content,omitempty"`
	// outbound-only fields
	ID        string `json:"id,omitempty"`
	SenderID  string `json:"sender_id,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	UserID    string `json:"user_id,omitempty"`
	Message   string `json:"message,omitempty"`
}

// ---------------------------------------------------------------------------
// client — single WS connection with a per-connection write lock
// ---------------------------------------------------------------------------

type client struct {
	conn   *websocket.Conn
	mu     sync.Mutex
	userID uuid.UUID
}

func (cl *client) send(v any) {
	cl.mu.Lock()
	defer cl.mu.Unlock()
	_ = cl.conn.WriteJSON(v)
}

// ---------------------------------------------------------------------------
// Hub
// ---------------------------------------------------------------------------

// Hub manages WebSocket rooms (one room per conversation).
type Hub struct {
	pool          *pgxpool.Pool
	validateToken func(string) (uuid.UUID, error)
	logger        *slog.Logger

	mu    sync.RWMutex
	rooms map[string]map[*client]bool // conversation_id → set of clients

	done chan struct{}
	wg   sync.WaitGroup
}

// NewHub creates a Hub. validateToken is typically authHandler.ValidateWSToken.
func NewHub(
	pool *pgxpool.Pool,
	validateToken func(string) (uuid.UUID, error),
	logger *slog.Logger,
) *Hub {
	return &Hub{
		pool:          pool,
		validateToken: validateToken,
		logger:        logger,
		rooms:         make(map[string]map[*client]bool),
		done:          make(chan struct{}),
	}
}

// Stop signals the hub to close and waits for all connection goroutines to exit.
func (h *Hub) Stop() {
	close(h.done)
	h.wg.Wait()
}

// ---------------------------------------------------------------------------
// Room helpers
// ---------------------------------------------------------------------------

func (h *Hub) addToRoom(roomID string, cl *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*client]bool)
	}
	h.rooms[roomID][cl] = true
}

func (h *Hub) removeFromRoom(roomID string, cl *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.rooms[roomID], cl)
	if len(h.rooms[roomID]) == 0 {
		delete(h.rooms, roomID)
	}
}

func (h *Hub) removeFromAllRooms(cl *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for roomID, clients := range h.rooms {
		delete(clients, cl)
		if len(clients) == 0 {
			delete(h.rooms, roomID)
		}
	}
}

func (h *Hub) broadcast(roomID string, msg any) {
	h.mu.RLock()
	clients := make([]*client, 0, len(h.rooms[roomID]))
	for cl := range h.rooms[roomID] {
		clients = append(clients, cl)
	}
	h.mu.RUnlock()
	for _, cl := range clients {
		go cl.send(msg)
	}
}

// ---------------------------------------------------------------------------
// FastHTTP upgrader — works directly with Fiber v3's c.Context()
// ---------------------------------------------------------------------------

var upgrader = websocket.FastHTTPUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(_ *fasthttp.RequestCtx) bool { return true }, // auth via first message
}

// Upgrade upgrades the HTTP connection to WebSocket and handles the session.
// Call this from a Fiber handler: return h.hub.Upgrade(c.Context())
func (h *Hub) Upgrade(fctx *fasthttp.RequestCtx) error {
	return upgrader.Upgrade(fctx, func(conn *websocket.Conn) {
		h.wg.Add(1)
		defer h.wg.Done()

		cl := &client{conn: conn}
		defer h.removeFromAllRooms(cl)

		// ---------------------------------------------------------------
		// Step 1: First-message auth (10-second deadline)
		// ---------------------------------------------------------------
		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			return
		}

		var authMsg wsMsg
		if err := conn.ReadJSON(&authMsg); err != nil || authMsg.Type != "auth" {
			cl.send(wsMsg{Type: "error", Message: "expected {type:'auth',token:'...'}"})
			return
		}

		userID, err := h.validateToken(authMsg.Token)
		if err != nil {
			cl.send(wsMsg{Type: "error", Message: "invalid token: " + err.Error()})
			return
		}
		cl.userID = userID
		cl.send(wsMsg{Type: "auth_ok", UserID: userID.String()})

		// Clear deadline for normal operation
		_ = conn.SetReadDeadline(time.Time{})

		// ---------------------------------------------------------------
		// Step 2: Main message loop
		// ---------------------------------------------------------------
		for {
			select {
			case <-h.done:
				return
			default:
			}

			var msg wsMsg
			if err := conn.ReadJSON(&msg); err != nil {
				break
			}

			switch msg.Type {
			case "join_room":
				h.handleJoinRoom(cl, msg.ConversationID)
			case "leave_room":
				h.handleLeaveRoom(cl, msg.ConversationID)
			case "message":
				h.handleMessage(cl, msg)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// Per-message handlers
// ---------------------------------------------------------------------------

func (h *Hub) handleJoinRoom(cl *client, convIDStr string) {
	convID, err := uuid.Parse(convIDStr)
	if err != nil {
		cl.send(wsMsg{Type: "error", Message: "invalid conversation_id"})
		return
	}

	ctx := context.Background()

	var isMember bool
	if err := h.pool.QueryRow(ctx,
		`SELECT EXISTS (
           SELECT 1 FROM conversation_members
           WHERE conversation_id = $1 AND user_id = $2
         )`,
		convID, cl.userID,
	).Scan(&isMember); err != nil || !isMember {
		cl.send(wsMsg{Type: "error", Message: model.ErrForbidden})
		return
	}

	h.addToRoom(convIDStr, cl)
	cl.send(wsMsg{Type: "joined", ConversationID: convIDStr})
}

func (h *Hub) handleLeaveRoom(cl *client, convIDStr string) {
	h.removeFromRoom(convIDStr, cl)
}

func (h *Hub) handleMessage(cl *client, msg wsMsg) {
	if len(msg.Content) == 0 || len(msg.Content) > 4000 {
		cl.send(wsMsg{Type: "error", Message: "content must be 1–4000 characters"})
		return
	}

	convID, err := uuid.Parse(msg.ConversationID)
	if err != nil {
		cl.send(wsMsg{Type: "error", Message: "invalid conversation_id"})
		return
	}

	ctx := context.Background()

	// 1. Verify sender is a member of this conversation
	var isMember bool
	if err := h.pool.QueryRow(ctx,
		`SELECT EXISTS (
           SELECT 1 FROM conversation_members
           WHERE conversation_id = $1 AND user_id = $2
         )`,
		convID, cl.userID,
	).Scan(&isMember); err != nil || !isMember {
		cl.send(wsMsg{Type: "error", Message: model.ErrForbidden})
		return
	}

	// 2. Find the other member
	var otherUserID uuid.UUID
	if err := h.pool.QueryRow(ctx,
		`SELECT user_id FROM conversation_members
         WHERE conversation_id = $1 AND user_id != $2`,
		convID, cl.userID,
	).Scan(&otherUserID); err != nil {
		cl.send(wsMsg{Type: "error", Message: "conversation not found"})
		return
	}

	// 3. Verify accepted connection
	var connected bool
	if err := h.pool.QueryRow(ctx,
		`SELECT EXISTS (
           SELECT 1 FROM connections
           WHERE status = 'accepted'
             AND ((requester_id = $1 AND recipient_id = $2)
               OR (requester_id = $2 AND recipient_id = $1))
         )`,
		cl.userID, otherUserID,
	).Scan(&connected); err != nil || !connected {
		cl.send(wsMsg{Type: "error", Message: model.ErrForbidden})
		return
	}

	// 4. Verify no blocks
	var blocked bool
	if err := h.pool.QueryRow(ctx,
		`SELECT EXISTS (
           SELECT 1 FROM blocks
           WHERE (blocker_id = $1 AND blocked_id = $2)
              OR (blocker_id = $2 AND blocked_id = $1)
         )`,
		cl.userID, otherUserID,
	).Scan(&blocked); err == nil && blocked {
		cl.send(wsMsg{Type: "error", Message: model.ErrForbidden})
		return
	}

	// 5. Persist message
	var msgID uuid.UUID
	var createdAt time.Time
	if err := h.pool.QueryRow(ctx,
		`INSERT INTO messages (conversation_id, sender_id, content)
         VALUES ($1, $2, $3)
         RETURNING id, created_at`,
		convID, cl.userID, msg.Content,
	).Scan(&msgID, &createdAt); err != nil {
		h.logger.Error("ws insert message", "error", err)
		cl.send(wsMsg{Type: "error", Message: "failed to send message"})
		return
	}

	// 6. Broadcast to all room members
	h.broadcast(msg.ConversationID, wsMsg{
		Type:           "new_message",
		ID:             msgID.String(),
		ConversationID: msg.ConversationID,
		SenderID:       cl.userID.String(),
		Content:        msg.Content,
		CreatedAt:      createdAt.UTC().Format(time.RFC3339),
	})
}
