import { Hono } from "hono";
import type { Variables } from "./types";
import { authRoutes } from "./routes/auth";
import { pageRoutes } from "./routes/pages";
import { errorRoutes } from "./routes/errors";

const app = new Hono<{ Variables: Variables }>();

// Liveness probe — always 200 if process is alive.
app.get("/healthz", (c) => c.json({ status: "ok" }));

// Auth routes (login, logout, signup, confirm)
app.route("/auth", authRoutes);

// Page routes (full pages + authenticated shells)
app.route("/", pageRoutes);

// Error pages (404, 500, etc.)
app.route("/error", errorRoutes);

export default {
  port: Number(process.env.PORT ?? 3000),
  fetch: app.fetch,
};
