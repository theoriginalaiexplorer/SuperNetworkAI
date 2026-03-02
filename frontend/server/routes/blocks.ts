import { Hono } from "hono";
import type { Variables } from "../types";
import { requireAuth } from "../middleware/auth";
import { sessionMiddleware, clearSessionCookies } from "../middleware/session";
import { apiClient } from "../lib/api";

export const blockRoutes = new Hono<{ Variables: Variables }>();

blockRoutes.use("*", sessionMiddleware, requireAuth);

// POST /blocks/:userId — block a user; returns an unblock button (HTMX swap)
blockRoutes.post("/:userId", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);
  const userId = c.req.param("userId");

  try {
    await api.post("/api/v1/blocks", { blocked_id: userId });
  } catch (e: any) {
    if (e.status !== 409) {
      return c.html(`<span class="text-danger small">Could not block. Try again.</span>`);
    }
    // 409 = already blocked — fall through to show unblock button
  }

  return c.html(
    `<div id="profile-block-btn">
       <button class="btn btn-outline-danger btn-sm"
               hx-delete="/blocks/${userId}"
               hx-target="#profile-block-btn"
               hx-swap="outerHTML"
               hx-confirm="Unblock this person?">
         Unblock
       </button>
     </div>`
  );
});

// DELETE /blocks/:userId — unblock; returns a block button (HTMX swap)
blockRoutes.delete("/:userId", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);
  const userId = c.req.param("userId");

  try {
    await api.delete(`/api/v1/blocks/${userId}`);
  } catch {
    // Best-effort — idempotent
  }

  return c.html(
    `<div id="profile-block-btn">
       <button class="btn btn-outline-secondary btn-sm"
               hx-post="/blocks/${userId}"
               hx-target="#profile-block-btn"
               hx-swap="outerHTML"
               hx-confirm="Block this person? They will no longer appear in your matches.">
         Block
       </button>
     </div>`
  );
});

// POST /account/delete — delete account then redirect to login
blockRoutes.post("/account/delete", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);

  try {
    await api.delete("/api/v1/account");
  } catch {
    return c.html(
      `<span class="text-danger">Account deletion failed. Please try again.</span>`
    );
  }

  clearSessionCookies(c);
  c.header("HX-Redirect", "/login");
  return c.html("");
});
