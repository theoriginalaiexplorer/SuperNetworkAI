import { Hono } from "hono";
import type { Variables } from "../types";
import { requireAuth } from "../middleware/auth";
import { sessionMiddleware } from "../middleware/session";
import { renderPartial } from "../lib/render";
import { apiClient } from "../lib/api";

type Match = {
  matched_user_id: string;
  score: number;
  categories: string[];
  explanation: string;
  display_name: string;
  tagline: string;
  skills: string[];
  intent: string[];
  avatar_url: string;
};

type SearchResult = {
  user_id: string;
  display_name: string;
  tagline: string;
  skills: string[];
  intent: string[];
  avatar_url: string;
  score: number;
};

export const discoverRoutes = new Hono<{ Variables: Variables }>();

discoverRoutes.use("*", sessionMiddleware, requireAuth);

const DEFAULT_LIMIT = 20;

// GET /discover — full discover page with initial match list
discoverRoutes.get("/", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);
  const category = c.req.query("category") ?? "";

  let matches: Match[] = [];
  try {
    const params = new URLSearchParams({ limit: String(DEFAULT_LIMIT) });
    if (category) params.set("category", category);
    const result = await api.get<{ matches: Match[]; count: number }>(
      `/api/v1/matches?${params}`
    );
    matches = result.matches;
  } catch {
    // Go API unreachable — render empty state
  }

  return renderPartial(c, "pages/discover", {
    matches,
    category,
    limit: DEFAULT_LIMIT,
  });
});

// GET /discover/matches — HTMX partial for category filter & load-more
discoverRoutes.get("/matches", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);
  const category = c.req.query("category") ?? "";
  const offset = parseInt(c.req.query("offset") ?? "0", 10);

  let matches: Match[] = [];
  try {
    const params = new URLSearchParams({
      limit: String(DEFAULT_LIMIT),
      offset: String(offset),
    });
    if (category) params.set("category", category);
    const result = await api.get<{ matches: Match[]; count: number }>(
      `/api/v1/matches?${params}`
    );
    matches = result.matches;
  } catch {
    // empty state
  }

  return renderPartial(c, "partials/match-list", {
    matches,
    category,
    offset,
    limit: DEFAULT_LIMIT,
  });
});

// POST /discover/matches/:id/dismiss — HTMX dismiss; returns empty to remove card
discoverRoutes.post("/matches/:id/dismiss", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);
  const id = c.req.param("id");

  try {
    await api.post(`/api/v1/matches/${id}/dismiss`, {});
  } catch {
    // Best-effort — card is still removed from UI
  }

  return c.html("");
});

// GET /discover/matches/:id/explanation — HTMX lazy-load explanation text for a match card
discoverRoutes.get("/matches/:id/explanation", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);
  const id = c.req.param("id");

  try {
    const result = await api.get<{ explanation: string }>(
      `/api/v1/matches/${id}/explanation`
    );
    return c.html(result.explanation || "No explanation available.");
  } catch {
    return c.html("Could not load explanation.");
  }
});

// POST /discover/search — HTMX NL search; returns search-results partial
discoverRoutes.post("/search", async (c) => {
  const session = c.get("session") as { accessToken: string };
  const api = apiClient(session.accessToken);

  const body = await c.req.parseBody();
  const query = (body.query as string) ?? "";

  if (!query.trim()) {
    // Empty query: restore the match list
    let matches: Match[] = [];
    try {
      const result = await api.get<{ matches: Match[]; count: number }>(
        `/api/v1/matches?limit=${DEFAULT_LIMIT}`
      );
      matches = result.matches;
    } catch {
      // empty state
    }
    return renderPartial(c, "partials/match-list", {
      matches,
      category: "",
      offset: 0,
      limit: DEFAULT_LIMIT,
    });
  }

  let results: SearchResult[] = [];
  try {
    const result = await api.post<{ results: SearchResult[]; count: number }>(
      "/api/v1/search",
      { query }
    );
    results = result.results;
  } catch {
    // empty state
  }

  return renderPartial(c, "partials/search-results", { results });
});
