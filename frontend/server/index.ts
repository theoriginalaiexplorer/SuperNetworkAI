import { Hono } from "hono";

const app = new Hono();

// Liveness probe — always 200 if the process is alive.
app.get("/healthz", (c) => c.json({ status: "ok" }));

// Additional routes registered in later phases:
// import { authRoutes } from "./routes/auth"
// import { pageRoutes }  from "./routes/pages"
// app.route("/auth", authRoutes)
// app.route("/",    pageRoutes)

const PORT = Number(process.env.PORT ?? 3000);

export default {
  port: PORT,
  fetch: app.fetch,
};
