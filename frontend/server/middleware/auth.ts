import type { Context, Next } from "hono";
import type { Variables } from "../types";
import { getSession, refreshSession, clearSessionCookies } from "./session";

// requireAuth redirects unauthenticated requests to /login.
// Must be used after sessionMiddleware on protected routes.
export async function requireAuth(c: Context<{ Variables: Variables }>, next: Next): Promise<Response | void> {
  let session = c.get("session") as { accessToken: string; refreshToken: string } | undefined;

  if (!session) {
    // Check if there is a refresh token to attempt silent refresh
    const cookieHeader = c.req.header("cookie") ?? "";
    const refreshToken = cookieHeader
      .split(";")
      .map((p) => p.trim().split("="))
      .find(([k]) => k === "sn_refresh")?.[1];

    if (refreshToken) {
      const refreshed = await refreshSession(c, refreshToken);
      if (refreshed) {
        c.set("session", refreshed);
        return next();
      }
    }

    clearSessionCookies(c);
    return c.redirect("/login");
  }

  return next();
}
