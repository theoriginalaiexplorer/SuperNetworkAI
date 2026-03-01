// apiClient is the SINGLE source for all BFF → Go API calls.
// It adds the Bearer JWT automatically so no handler has to do it manually.

const GO_API_URL = process.env.GO_API_URL ?? "http://localhost:3001";

type Method = "GET" | "POST" | "PATCH" | "DELETE";

async function request<T>(
  method: Method,
  path: string,
  jwt: string,
  body?: unknown
): Promise<T> {
  const res = await fetch(`${GO_API_URL}${path}`, {
    method,
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${jwt}`,
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (!res.ok) {
    const err = await res.json().catch(() => ({ message: res.statusText }));
    throw Object.assign(new Error(err.message ?? "API error"), {
      status: res.status,
      code: err.code,
    });
  }

  return res.json() as Promise<T>;
}

export function apiClient(jwt: string) {
  return {
    get: <T>(path: string) => request<T>("GET", path, jwt),
    post: <T>(path: string, body: unknown) => request<T>("POST", path, jwt, body),
    patch: <T>(path: string, body: unknown) => request<T>("PATCH", path, jwt, body),
    delete: <T>(path: string) => request<T>("DELETE", path, jwt),
  };
}
