import { Hono } from "hono";
import { createClient } from "@supabase/supabase-js";
import { setSessionCookies, clearSessionCookies } from "../middleware/session";

const SUPABASE_URL = process.env.SUPABASE_URL!;
const SUPABASE_ANON_KEY = process.env.SUPABASE_ANON_KEY!;

export const authRoutes = new Hono();

// POST /auth/login
authRoutes.post("/login", async (c) => {
  const body = await c.req.parseBody();
  const email = String(body.email ?? "");
  const password = String(body.password ?? "");

  const supabase = createClient(SUPABASE_URL, SUPABASE_ANON_KEY);
  const { data, error } = await supabase.auth.signInWithPassword({ email, password });

  if (error || !data.session) {
    // Re-render login with error (HTMX partial swap)
    return c.html(
      `<div id="auth-error" class="alert alert-danger">${error?.message ?? "Login failed"}</div>`
    );
  }

  setSessionCookies(c, {
    accessToken: data.session.access_token,
    refreshToken: data.session.refresh_token,
  });

  // HTMX redirect to dashboard (onboarding guard in pages.ts handles incomplete profiles)
  c.header("HX-Redirect", "/dashboard");
  return c.body(null, 204);
});

// POST /auth/signup
authRoutes.post("/signup", async (c) => {
  const body = await c.req.parseBody();
  const email = String(body.email ?? "");
  const password = String(body.password ?? "");

  const supabase = createClient(SUPABASE_URL, SUPABASE_ANON_KEY);
  const { data, error } = await supabase.auth.signUp({ email, password });

  if (error) {
    return c.html(
      `<div id="auth-error" class="alert alert-danger">${error.message}</div>`
    );
  }

  // Email verification disabled: session is returned immediately
  if (data.session) {
    setSessionCookies(c, {
      accessToken: data.session.access_token,
      refreshToken: data.session.refresh_token,
    });
    c.header("HX-Redirect", "/onboarding/step1");
    return c.body(null, 204);
  }

  // Email verification enabled: show confirmation message
  return c.html(
    `<div class="alert alert-info">Check your email to confirm your account.</div>`
  );
});

// GET /auth/logout
authRoutes.get("/logout", async (c) => {
  clearSessionCookies(c);
  return c.redirect("/");
});

// GET /auth/confirm — exchanges email verification token
authRoutes.get("/confirm", async (c) => {
  const token = c.req.query("token") ?? "";
  const type = c.req.query("type") ?? "signup";

  const supabase = createClient(SUPABASE_URL, SUPABASE_ANON_KEY);
  const { data, error } = await supabase.auth.verifyOtp({
    token_hash: token,
    type: type as "signup" | "recovery",
  });

  if (error || !data.session) {
    return c.redirect("/login?error=invalid_confirmation_link");
  }

  setSessionCookies(c, {
    accessToken: data.session.access_token,
    refreshToken: data.session.refresh_token,
  });
  return c.redirect("/onboarding/step1");
});
