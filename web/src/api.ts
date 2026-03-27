export type RouteID = "ping" | "orders" | "report";

export interface APIKeyRecord {
  id: string;
  name: string;
  key_prefix: string;
  is_active: boolean;
  created_at: string;
}

export interface CreatedAPIKey {
  api_key: APIKeyRecord;
  raw_key: string;
}

export interface ProtectedRequestResult {
  status: number;
  limit: number | null;
  remaining: number | null;
  reset: number | null;
  retryAfter: number | null;
  body: Record<string, unknown> | null;
}

const protectedRoutes: Record<RouteID, { method: string; path: string }> = {
  ping: { method: "GET", path: "/api/protected/ping" },
  orders: { method: "POST", path: "/api/protected/orders" },
  report: { method: "GET", path: "/api/protected/report" },
};

export async function listAPIKeys(baseURL: string, adminToken: string): Promise<APIKeyRecord[]> {
  const response = await fetch(joinURL(baseURL, "/api/admin/api-keys"), {
    headers: {
      Authorization: `Bearer ${adminToken}`,
    },
  });

  if (!response.ok) {
    throw new Error(`Failed to load API keys (${response.status})`);
  }

  const payload = (await response.json()) as { api_keys: APIKeyRecord[] };
  return payload.api_keys ?? [];
}

export async function createAPIKey(baseURL: string, adminToken: string, name: string): Promise<CreatedAPIKey> {
  const response = await fetch(joinURL(baseURL, "/api/admin/api-keys"), {
    method: "POST",
    headers: {
      Authorization: `Bearer ${adminToken}`,
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ name }),
  });

  if (!response.ok) {
    throw new Error(`Failed to create API key (${response.status})`);
  }

  return (await response.json()) as CreatedAPIKey;
}

export async function sendProtectedRequest(baseURL: string, rawKey: string, routeID: RouteID): Promise<ProtectedRequestResult> {
  const route = protectedRoutes[routeID];
  const response = await fetch(joinURL(baseURL, route.path), {
    method: route.method,
    headers: {
      "X-API-Key": rawKey,
      "Content-Type": "application/json",
    },
    body: route.method === "POST" ? JSON.stringify({ demo: true }) : undefined,
  });

  const body = await parseJSON(response);

  return {
    status: response.status,
    limit: parseNumberHeader(response.headers.get("X-RateLimit-Limit")),
    remaining: parseNumberHeader(response.headers.get("X-RateLimit-Remaining")),
    reset: parseNumberHeader(response.headers.get("X-RateLimit-Reset")),
    retryAfter: parseNumberHeader(response.headers.get("Retry-After")),
    body,
  };
}

function joinURL(baseURL: string, path: string): string {
  return `${baseURL.replace(/\/+$/, "")}${path}`;
}

async function parseJSON(response: Response): Promise<Record<string, unknown> | null> {
  const text = await response.text();
  if (!text) {
    return null;
  }

  try {
    return JSON.parse(text) as Record<string, unknown>;
  } catch {
    return { raw: text };
  }
}

function parseNumberHeader(value: string | null): number | null {
  if (!value) {
    return null;
  }

  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : null;
}
