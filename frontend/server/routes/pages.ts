import { Hono } from "hono";
import type { Variables } from "../types";
import { requireAuth } from "../middleware/auth";
import { sessionMiddleware } from "../middleware/session";
import { renderPartial } from "../lib/render";
import { apiClient } from "../lib/api";
import { onboardingRoutes } from "./onboarding";
import { discoverRoutes } from "./discover";
import { connectionRoutes } from "./connections";
import { messageRoutes } from "./messages";
import { blockRoutes } from "./blocks";

export const pageRoutes = new Hono<{ Variables: Variables }>();

// Apply session middleware globally for page routes
pageRoutes.use("*", sessionMiddleware);

// --- Public pages ---
pageRoutes.get("/", (c) => renderPartial(c, "pages/landing", {}));
pageRoutes.get("/login", (c) => renderPartial(c, "pages/login", { error: c.req.query("error") }));
pageRoutes.get("/signup", (c) => renderPartial(c, "pages/signup", {}));

// --- Authenticated pages (onboarding guard applied) ---
const authenticated = new Hono<{ Variables: Variables }>();
authenticated.use("*", requireAuth);

// Onboarding guard: redirect to /onboarding/step1 if profile incomplete
async function onboardingGuard(
  c: Parameters<typeof requireAuth>[0],
  next: Parameters<typeof requireAuth>[1]
): Promise<Response | void> {
  const session = c.get("session") as { accessToken: string } | undefined;
  if (!session) return next();

  try {
    const api = apiClient(session.accessToken);
    const me = await api.get<{ profile: { onboarding_complete: boolean } }>("/api/v1/users/me");
    if (!me.profile.onboarding_complete) {
      return c.redirect("/onboarding/step1");
    }
  } catch {
    // Go API unreachable — allow through; individual handlers can retry
  }
  return next();
}

// Dashboard and other guarded pages
authenticated.use("/dashboard", onboardingGuard);
authenticated.get("/dashboard", (c) => renderPartial(c, "pages/dashboard", {}));

// Profile view
authenticated.get("/profile/me", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);
  const me = await api.get<{ user: object; profile: object; ikigai: object }>("/api/v1/users/me");
  return renderPartial(c, "pages/profile", { profile: me.profile, ikigai: me.ikigai, isOwn: true });
});

authenticated.get("/profile/:id", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);
  const id = c.req.param("id");
  try {
    type ConnStatus = { status: string; connection_id?: string; direction?: string };
    const [profileData, connStatus] = await Promise.all([
      api.get<object>(`/api/v1/users/${id}`),
      api
        .get<ConnStatus>(`/api/v1/connections/status/${id}`)
        .catch((): ConnStatus => ({ status: "none" })),
    ]);
    return renderPartial(c, "pages/profile", {
      profile: profileData,
      isOwn: false,
      connectionStatus: connStatus.status,
      connectionId: connStatus.connection_id ?? null,
      connectionDirection: connStatus.direction ?? null,
    });
  } catch (e: any) {
    if (e.status === 403) return c.text("Access denied", 403);
    if (e.status === 404) return c.text("Not found", 404);
    throw e;
  }
});

// POST /profile/visibility — HTMX toggle; returns updated badge HTML
authenticated.post("/profile/visibility", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);
  const body = await c.req.parseBody();
  const visibility = body.visibility as string;
  if (visibility !== "public" && visibility !== "private") {
    return c.html(`<span class="text-danger small">Invalid visibility value</span>`);
  }
  try {
    await api.patch("/api/v1/profiles/me/visibility", { visibility });
    const next = visibility === "public" ? "private" : "public";
    const label = visibility === "public" ? "Public" : "Private";
    const cls = visibility === "public" ? "btn-outline-success" : "btn-outline-warning";
    return c.html(
      `<div id="visibility-toggle">
         <form hx-post="/profile/visibility" hx-target="#visibility-toggle" hx-swap="outerHTML">
           <input type="hidden" name="visibility" value="${next}" />
           <button type="submit" class="btn ${cls} btn-sm w-100">${label} (click to toggle)</button>
         </form>
       </div>`
    );
  } catch {
    return c.html(`<span class="text-danger small">Failed to update visibility</span>`);
  }
});

// Onboarding steps (no onboarding guard — that would cause redirect loop)
pageRoutes.route("/onboarding", onboardingRoutes);

// Discover (match browsing) — auth handled inside discoverRoutes
pageRoutes.route("/discover", discoverRoutes);

// Connections — auth handled inside connectionRoutes
pageRoutes.route("/connections", connectionRoutes);

// Messages + WebSocket token — auth handled inside messageRoutes
pageRoutes.route("/messages", messageRoutes);

// Block / unblock / account delete — auth handled inside blockRoutes
pageRoutes.route("/blocks", blockRoutes);

pageRoutes.route("/", authenticated);
