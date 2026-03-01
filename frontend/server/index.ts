import { Hono } from "hono";
import { serveStatic } from "hono/bun";
import type { Variables } from "./types";
import { authRoutes } from "./routes/auth";
import { pageRoutes } from "./routes/pages";
import { errorRoutes } from "./routes/errors";

const app = new Hono<{ Variables: Variables }>();

// Serve static assets from dist/public
app.use("/public/*", serveStatic({ root: "./dist" }));

// Liveness probe — always 200 if process is alive.
app.get("/healthz", (c) => c.json({ status: "ok" }));

// Error pages (404, 500, etc.) - must come before page routes
app.route("/error", errorRoutes);

// Auth routes (login, logout, signup, confirm)
app.route("/auth", authRoutes);

// Page routes (full pages + authenticated shells)
app.route("/", pageRoutes);

export default {
  port: Number(process.env.PORT ?? 3000),
  fetch: app.fetch,
};
