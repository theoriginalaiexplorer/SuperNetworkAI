import { Hono } from "hono";
import type { Variables } from "../types";
import { requireAuth } from "../middleware/auth";
import { sessionMiddleware } from "../middleware/session";
import { renderPartial } from "../lib/render";
import { apiClient } from "../lib/api";

type Connection = {
  id: string;
  status: string;
  direction: "sent" | "received";
  message: string;
  other_user_id: string;
  display_name: string;
  tagline: string;
  avatar_url: string;
  created_at: string;
};

export const connectionRoutes = new Hono<{ Variables: Variables }>();

connectionRoutes.use("*", sessionMiddleware, requireAuth);

// GET /connections — full connections page (defaults to accepted tab)
connectionRoutes.get("/", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);
  const tab = (c.req.query("tab") ?? "accepted") as "accepted" | "pending";

  let connections: Connection[] = [];
  try {
    const result = await api.get<{ connections: Connection[]; count: number }>(
      `/api/v1/connections?status=${tab}`
    );
    connections = result.connections;
  } catch {
    // empty state
  }

  return renderPartial(c, "pages/connections", { connections, tab });
});

// GET /connections/list?tab=accepted|pending — HTMX partial for tab switch
connectionRoutes.get("/list", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);
  const tab = (c.req.query("tab") ?? "accepted") as "accepted" | "pending";

  let connections: Connection[] = [];
  try {
    const result = await api.get<{ connections: Connection[]; count: number }>(
      `/api/v1/connections?status=${tab}`
    );
    connections = result.connections;
  } catch {
    // empty state
  }

  return renderPartial(c, "partials/connection-list", { connections, tab });
});

// POST /connections/request/:userId — send connection request; returns button HTML for HTMX swap
connectionRoutes.post("/request/:userId", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);
  const userId = c.req.param("userId");

  try {
    await api.post("/api/v1/connections", { recipient_id: userId });
    return c.html(
      `<button class="btn btn-outline-secondary btn-sm" disabled>Pending&hellip;</button>`
    );
  } catch (e: any) {
    if (e.status === 409) {
      return c.html(
        `<button class="btn btn-outline-secondary btn-sm" disabled>Already connected</button>`
      );
    }
    return c.html(
      `<button class="btn btn-danger btn-sm" disabled>Error</button>`
    );
  }
});

// POST /connections/:connId/accept — accept pending request; returns empty (card replaced by HTMX)
connectionRoutes.post("/:connId/accept", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);
  const connId = c.req.param("connId");

  try {
    await api.patch(`/api/v1/connections/${connId}`, { status: "accepted" });
  } catch {
    // best-effort
  }

  // Return an "accepted" badge to replace the action buttons
  return c.html(
    `<span class="badge bg-success">Connected</span>`
  );
});

// POST /connections/:connId/reject — reject; returns empty to remove the card
connectionRoutes.post("/:connId/reject", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);
  const connId = c.req.param("connId");

  try {
    await api.patch(`/api/v1/connections/${connId}`, { status: "rejected" });
  } catch {
    // best-effort
  }

  return c.html(""); // HTMX outerHTML swap removes the card
});
