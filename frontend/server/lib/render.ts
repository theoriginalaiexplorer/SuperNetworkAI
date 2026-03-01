import type { Context } from "hono";
import { eta } from "./eta";

// renderPartial renders an Eta template and returns it as an HTML response.
// Used for HTMX partial swaps.
export async function renderPartial(
  c: Context,
  template: string,
  data: Record<string, unknown> = {}
): Promise<Response> {
  const html = await eta.renderAsync(template, data);
  return c.html(html);
}
