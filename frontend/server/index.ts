import { Hono } from "hono";
import type { Variables } from "./types";
import { authRoutes } from "./routes/auth";
import { pageRoutes } from "./routes/pages";

const app = new Hono<{ Variables: Variables }>();

// Liveness probe — always 200 if the process is alive.
app.get("/healthz", (c) => c.json({ status: "ok" }));

// Auth routes (login, logout, signup, confirm)
app.route("/auth", authRoutes);

// Page routes (full pages + authenticated shells)
app.route("/", pageRoutes);

const PORT = Number(process.env.PORT ?? 3000);

export default {
  port: PORT,
  fetch: app.fetch,
};
