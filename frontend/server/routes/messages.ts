import { Hono } from "hono";
import type { Variables } from "../types";
import { requireAuth } from "../middleware/auth";
import { sessionMiddleware } from "../middleware/session";
import { renderPartial } from "../lib/render";
import { apiClient } from "../lib/api";

const GO_API_URL = process.env.GO_API_URL ?? "http://localhost:3001";

// Derive WebSocket URL from the Go API HTTP URL
function wsUrl(): string {
  return GO_API_URL.replace(/^http:/, "ws:").replace(/^https:/, "wss:") + "/ws";
}

type ConversationSummary = {
  id: string;
  other_user_id: string;
  display_name: string;
  avatar_url: string;
  last_message: string;
  last_message_at: string;
  unread_count: number;
};

type Message = {
  id: string;
  conversation_id: string;
  sender_id: string;
  content: string;
  read_at?: string;
  created_at: string;
};

export const messageRoutes = new Hono<{ Variables: Variables }>();

messageRoutes.use("*", sessionMiddleware, requireAuth);

// ---------------------------------------------------------------------------
// GET /messages — conversation list (no chat open)
// ---------------------------------------------------------------------------
messageRoutes.get("/", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);

  let conversations: ConversationSummary[] = [];
  try {
    const result = await api.get<{ conversations: ConversationSummary[] }>(
      "/api/v1/conversations"
    );
    conversations = result.conversations;
  } catch {
    // empty state — show empty list
  }

  return renderPartial(c, "pages/messages", {
    conversations,
    convId: null,
    messages: [],
    wsUrl: wsUrl(),
    wsToken: null,
    currentUserId: null,
  });
});

// ---------------------------------------------------------------------------
// GET /messages/new?userId=<uuid> — create/get conversation, redirect to chat
// ---------------------------------------------------------------------------
messageRoutes.get("/new", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const userId = c.req.query("userId");
  if (!userId) return c.redirect("/messages");

  try {
    const api = apiClient(session.accessToken);
    const { conversation_id } = await api.post<{ conversation_id: string }>(
      "/api/v1/conversations",
      { user_id: userId }
    );
    return c.redirect(`/messages/${conversation_id}`);
  } catch {
    return c.redirect("/messages");
  }
});

// ---------------------------------------------------------------------------
// GET /messages/:convId — full chat page
// ---------------------------------------------------------------------------
messageRoutes.get("/:convId", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);
  const convId = c.req.param("convId");

  try {
    const [convResult, msgResult, tokenResult, meResult] = await Promise.all([
      api.get<{ conversations: ConversationSummary[] }>("/api/v1/conversations"),
      api.get<{ messages: Message[] }>(`/api/v1/conversations/${convId}/messages`),
      api.post<{ token: string; expires_at: string }>("/api/v1/auth/ws-token", {}),
      api.get<{ user: { id: string } }>("/api/v1/users/me"),
    ]);

    // Mark as read (best-effort)
    api.patch(`/api/v1/conversations/${convId}/read`, {}).catch(() => {});

    // Messages arrive newest-first from the API; reverse for display (oldest first)
    const messages = [...msgResult.messages].reverse();

    return renderPartial(c, "pages/messages", {
      conversations: convResult.conversations,
      convId,
      messages,
      wsUrl: wsUrl(),
      wsToken: tokenResult.token,
      currentUserId: meResult.user.id,
    });
  } catch (e: any) {
    if (e.status === 403) return c.text("Access denied", 403);
    if (e.status === 404) return c.text("Conversation not found", 404);
    throw e;
  }
});

// ---------------------------------------------------------------------------
// GET /partials/ws-token — fresh WS token for Alpine reconnect
// ---------------------------------------------------------------------------
messageRoutes.get("/partials/ws-token", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);
  try {
    const result = await api.post<{ token: string; expires_at: string }>(
      "/api/v1/auth/ws-token",
      {}
    );
    return c.json(result);
  } catch {
    return c.json({ token: "", expires_at: "" }, 500);
  }
});
