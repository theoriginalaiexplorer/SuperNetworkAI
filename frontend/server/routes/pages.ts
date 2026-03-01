import { Hono } from "hono";
import type { Variables } from "../types";
import { requireAuth } from "../middleware/auth";
import { sessionMiddleware } from "../middleware/session";
import { renderPartial } from "../lib/render";
import { apiClient } from "../lib/api";
import { onboardingRoutes } from "./onboarding";

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
  try {
    const profile = await api.get<object>(`/api/v1/users/${c.req.param("id")}`);
    return renderPartial(c, "pages/profile", { profile, isOwn: false });
  } catch (e: any) {
    if (e.status === 403) return c.text("Access denied", 403);
    if (e.status === 404) return c.text("Not found", 404);
    throw e;
  }
});

// Onboarding steps (no onboarding guard — that would cause redirect loop)
pageRoutes.route("/onboarding", onboardingRoutes);

pageRoutes.route("/", authenticated);
