import type { Session } from "./middleware/session";

// Hono context variable types — used by all route handlers via c.get("session")
export type Variables = {
  session: Session;
};
