import { Hono } from "hono";
import { requireAuth } from "../middleware/auth";
import { sessionMiddleware } from "../middleware/session";
import { renderPartial } from "../lib/render";
import { apiClient } from "../lib/api";

export const pageRoutes = new Hono();

// Apply session middleware globally for page routes
pageRoutes.use("*", sessionMiddleware);

// --- Public pages ---
pageRoutes.get("/", (c) => renderPartial(c, "pages/landing", {}));
pageRoutes.get("/login", (c) => renderPartial(c, "pages/login", { error: c.req.query("error") }));
pageRoutes.get("/signup", (c) => renderPartial(c, "pages/signup", {}));

// --- Authenticated pages (onboarding guard applied) ---
const authenticated = new Hono();
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

// Onboarding steps (no onboarding guard — that would cause redirect loop)
authenticated.get("/onboarding/step1", (c) => renderPartial(c, "pages/onboarding/step1-basic", {}));

pageRoutes.route("/", authenticated);
