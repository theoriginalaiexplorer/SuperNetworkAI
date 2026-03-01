import type { Context, Next } from "hono";
import type { Variables } from "../types";
import { SignJWT, decodeJwt } from "jose";

export interface Session {
  accessToken: string;
  refreshToken: string;
}

// Mutex guard: Firebase refresh tokens are single-use.
// Only one refresh request may proceed at a time.
let refreshing: Promise<Session | null> | null = null;

// getSession reads the sn_access cookie and returns the current session,
// or null if no cookie is present.
export function getSession(c: Context): Session | null {
  const access = getCookie(c, "sn_access");
  if (!access) return null;
  // sn_refresh has Path=/auth so it's only present on /auth/* requests
  const refresh = getCookie(c, "sn_refresh") ?? "";
  return { accessToken: access, refreshToken: refresh };
}

// refreshSession calls the Firebase token refresh REST endpoint to get a new
// refresh token, then re-signs a fresh BFF HS256 JWT (preserving the UUID sub
// from the existing access cookie).
export async function refreshSession(
  c: Context,
  refreshToken: string
): Promise<Session | null> {
  if (refreshing) return refreshing;

  refreshing = (async () => {
    try {
      const res = await fetch(
        `https://securetoken.googleapis.com/v1/token?key=${process.env.FIREBASE_API_KEY}`,
        {
          method: "POST",
          headers: { "Content-Type": "application/x-www-form-urlencoded" },
          body: `grant_type=refresh_token&refresh_token=${encodeURIComponent(refreshToken)}`,
        }
      );
      if (!res.ok) return null;
      const data = await res.json() as { refresh_token?: string };
      if (!data.refresh_token) return null;

      // Extract UUID sub from existing access cookie — decodeJwt skips verification
      // so it works even on an expired token (which is why we're refreshing).
      const existingJwt = getCookie(c, "sn_access");
      if (!existingJwt) return null;
      let sub: string;
      let email: string = "";
      try {
        const claims = decodeJwt(existingJwt);
        if (!claims.sub) return null;
        sub = claims.sub;
        email = typeof claims.email === "string" ? claims.email : "";
      } catch {
        return null;
      }

      const BFF_SECRET = new TextEncoder().encode(process.env.BFF_JWT_SECRET!);
      const newBffJwt = await new SignJWT({ sub, email })
        .setProtectedHeader({ alg: "HS256" })
        .setIssuedAt()
        .setExpirationTime("1h")
        .sign(BFF_SECRET);

      const session: Session = { accessToken: newBffJwt, refreshToken: data.refresh_token };
      setSessionCookies(c, session);
      return session;
    } finally {
      refreshing = null;
    }
  })();

  return refreshing;
}

// setSessionCookies writes the sn_access and sn_refresh HttpOnly cookies.
// Both use { append: true } so Hono emits two separate Set-Cookie headers
// instead of the second overwriting the first.
export function setSessionCookies(c: Context, session: Session): void {
  const secure = process.env.NODE_ENV === "production" ? "; Secure" : "";
  c.header(
    "Set-Cookie",
    `sn_access=${session.accessToken}; HttpOnly${secure}; SameSite=Lax; Path=/; Max-Age=3600`,
    { append: true }
  );
  c.header(
    "Set-Cookie",
    `sn_refresh=${session.refreshToken}; HttpOnly${secure}; SameSite=Lax; Path=/auth; Max-Age=604800`,
    { append: true }
  );
}

// clearSessionCookies removes both session cookies.
export function clearSessionCookies(c: Context): void {
  c.header("Set-Cookie", "sn_access=; HttpOnly; SameSite=Lax; Path=/; Max-Age=0");
  c.header("Set-Cookie", "sn_refresh=; HttpOnly; SameSite=Lax; Path=/auth; Max-Age=0");
}

// sessionMiddleware attaches session to c.var and handles transparent token refresh.
export async function sessionMiddleware(c: Context<{ Variables: Variables }>, next: Next): Promise<Response | void> {
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
