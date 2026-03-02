import { Hono } from "hono";
import type { Variables } from "../types";
import { requireAuth } from "../middleware/auth";
import { sessionMiddleware } from "../middleware/session";
import { renderPartial } from "../lib/render";
import { apiClient } from "../lib/api";

export const onboardingRoutes = new Hono<{ Variables: Variables }>();

onboardingRoutes.use("*", sessionMiddleware, requireAuth);

// Step 1 — basic info
onboardingRoutes.get("/step1", (c) => renderPartial(c, "pages/onboarding/step1-basic", {}));

onboardingRoutes.post("/step1", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const body = await c.req.parseBody();
  const api = apiClient(session.accessToken);

  try {
    await api.patch("/api/v1/profiles/me", {
      display_name: body.display_name,
      tagline: body.tagline || undefined,
    });
  } catch (e: any) {
    return c.html(`<div id="step-error" class="alert alert-danger mt-3">${e.message ?? "Failed to save. Please try again."}</div>`);
  }

  c.header("HX-Redirect", "/onboarding/step2");
  return c.body(null, 204);
});

// Step 2 — Ikigai
onboardingRoutes.get("/step2", (c) => renderPartial(c, "pages/onboarding/step2-ikigai", {}));

onboardingRoutes.post("/step2", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const body = await c.req.parseBody();
  const api = apiClient(session.accessToken);

  try {
    await api.post("/api/v1/onboarding/ikigai", {
      what_you_love:            body.what_you_love,
      what_youre_good_at:       body.what_youre_good_at,
      what_world_needs:         body.what_world_needs,
      what_you_can_be_paid_for: body.what_you_can_be_paid_for,
    });
  } catch (e: any) {
    return c.html(`<div id="step-error" class="alert alert-danger mt-3">${e.message ?? "Failed to save. Please try again."}</div>`);
  }

  c.header("HX-Redirect", "/onboarding/step3");
  return c.body(null, 204);
});

// Step 3 — skills & interests
onboardingRoutes.get("/step3", (c) => renderPartial(c, "pages/onboarding/step3-skills", {}));

onboardingRoutes.post("/step3", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const body = await c.req.parseBody({ all: true });
  const api = apiClient(session.accessToken);

  const skills = [body["skills[]"]].flat().filter(Boolean) as string[];
  const interests = [body["interests[]"]].flat().filter(Boolean) as string[];

  try {
    await api.patch("/api/v1/profiles/me", { skills, interests });
  } catch (e: any) {
    return c.html(`<div id="step-error" class="alert alert-danger mt-3">${e.message ?? "Failed to save. Please try again."}</div>`);
  }

  c.header("HX-Redirect", "/onboarding/step4");
  return c.body(null, 204);
});

// Step 4 — intent & availability
onboardingRoutes.get("/step4", (c) => renderPartial(c, "pages/onboarding/step4-intent", {}));

onboardingRoutes.post("/step4", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const body = await c.req.parseBody({ all: true });
  const api = apiClient(session.accessToken);

  const intent = [body["intent[]"]].flat().filter(Boolean) as string[];

  try {
    await api.patch("/api/v1/profiles/me", {
      intent,
      availability:  body.availability,
      working_style: body.working_style,
    });
  } catch (e: any) {
    return c.html(`<div id="step-error" class="alert alert-danger mt-3">${e.message ?? "Failed to save. Please try again."}</div>`);
  }

  c.header("HX-Redirect", "/onboarding/step5");
  return c.body(null, 204);
});

// CV import — calls Go API, returns structured CVData JSON for HTMX pre-fill
onboardingRoutes.post("/import-cv", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const body = await c.req.json<{ url: string }>();
  const api = apiClient(session.accessToken);

  try {
    const cv = await api.post<{
      display_name?: string;
      bio?: string;
      skills?: string[];
      interests?: string[];
      linkedin_url?: string;
      github_url?: string;
      portfolio_url?: string;
    }>("/api/v1/onboarding/import-cv", { url: body.url });

    return c.json(cv);
  } catch (e: any) {
    return c.json({ error: e.message ?? "CV import failed" }, 400);
  }
});

// Step 5 — links (final step)
onboardingRoutes.get("/step5", (c) => renderPartial(c, "pages/onboarding/step5-links", {}));

onboardingRoutes.post("/step5", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const body = await c.req.parseBody();
  const api = apiClient(session.accessToken);

  try {
    await api.patch("/api/v1/profiles/me", {
      linkedin_url:  body.linkedin_url  || undefined,
      github_url:    body.github_url    || undefined,
      portfolio_url: body.portfolio_url || undefined,
      twitter_url:   body.twitter_url   || undefined,
    });
    await api.post("/api/v1/onboarding/complete", {});
  } catch (e: any) {
    return c.html(`<div id="step-error" class="alert alert-danger mt-3">${e.message ?? "Failed to save. Please try again."}</div>`);
  }

  c.header("HX-Redirect", "/dashboard");
  return c.body(null, 204);
});
