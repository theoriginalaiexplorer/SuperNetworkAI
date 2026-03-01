import type { Context, Next } from "hono";
import { createClient } from "@supabase/supabase-js";

const SUPABASE_URL = process.env.SUPABASE_URL!;
const SUPABASE_ANON_KEY = process.env.SUPABASE_ANON_KEY!;

export interface Session {
  accessToken: string;
  refreshToken: string;
}

// Mutex guard: Supabase refresh tokens are single-use.
// Only one refresh request may proceed at a time.
let refreshing: Promise<Session | null> | null = null;

// getSession reads the sn_access cookie and returns the current session,
// or null if no cookie is present.
export function getSession(c: Context): Session | null {
  const access = getCookie(c, "sn_access");
  const refresh = getCookie(c, "sn_refresh");
  if (!access || !refresh) return null;
  return { accessToken: access, refreshToken: refresh };
}

// refreshSession exchanges the refresh token for a new access token.
// Protected by a mutex so only one refresh happens at a time across
// concurrent HTMX requests.
export async function refreshSession(
  c: Context,
  refreshToken: string
): Promise<Session | null> {
  if (refreshing) return refreshing;

  refreshing = (async () => {
    try {
      const supabase = createClient(SUPABASE_URL, SUPABASE_ANON_KEY);
      const { data, error } = await supabase.auth.refreshSession({ refresh_token: refreshToken });
      if (error || !data.session) return null;

      const session: Session = {
        accessToken: data.session.access_token,
        refreshToken: data.session.refresh_token,
      };
      setSessionCookies(c, session);
      return session;
    } finally {
      refreshing = null;
    }
  })();

  return refreshing;
}

// setSessionCookies writes the sn_access and sn_refresh HttpOnly cookies.
export function setSessionCookies(c: Context, session: Session): void {
  const secure = process.env.NODE_ENV === "production" ? "; Secure" : "";
  c.header(
    "Set-Cookie",
    `sn_access=${session.accessToken}; HttpOnly${secure}; SameSite=Lax; Path=/; Max-Age=3600`
  );
  c.header(
    "Set-Cookie",
    `sn_refresh=${session.refreshToken}; HttpOnly${secure}; SameSite=Lax; Path=/auth; Max-Age=604800`
  );
}

// clearSessionCookies removes both session cookies.
export function clearSessionCookies(c: Context): void {
  c.header("Set-Cookie", "sn_access=; HttpOnly; SameSite=Lax; Path=/; Max-Age=0");
  c.header("Set-Cookie", "sn_refresh=; HttpOnly; SameSite=Lax; Path=/auth; Max-Age=0");
}

// sessionMiddleware attaches session to c.var and handles transparent token refresh.
// If the access token is missing but refresh token exists, attempts a refresh.
export async function sessionMiddleware(c: Context, next: Next): Promise<Response | void> {
  const session = getSession(c);

  if (!session) {
    // No cookies at all — continue unauthenticated
    return next();
  }

  // Token exists — attach to context for downstream handlers
  c.set("session", session);
  return next();
}

function getCookie(c: Context, name: string): string | undefined {
  const header = c.req.header("cookie") ?? "";
  for (const part of header.split(";")) {
    const [k, v] = part.trim().split("=");
    if (k === name) return v;
  }
  return undefined;
}
