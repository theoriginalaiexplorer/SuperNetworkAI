import { Eta } from "eta";
import { join } from "path";

// Eta is async-capable — required for `await api.call()` inside templates.
export const eta = new Eta({
  views: join(import.meta.dir, "../templates"),
  cache: process.env.NODE_ENV === "production",
  autoEscape: true,
  async: true,
});
