export type RouteID = "ping" | "orders" | "report";
export type ScopeType = "global" | "api_key" | "route" | "api_key_route";

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

export interface PolicyRecord {
  id: string;
  scope_type: ScopeType;
  scope_identifier?: string | null;
  route_pattern?: RouteID | null;
  capacity: number;
  refill_tokens: number;
  refill_interval_seconds: number;
  is_active: boolean;
  created_at: string;
  updated_at: string;
}

export interface PolicyWriteInput {
  scope_type: ScopeType;
  scope_identifier?: string | null;
  route_pattern?: RouteID | null;
  capacity: number;
  refill_tokens: number;
  refill_interval_seconds: number;
}

export interface ProtectedRequestResult {
  status: number;
  limit: number | null;
  remaining: number | null;
  reset: number | null;
  retryAfter: number | null;
  body: Record<string, unknown> | null;
}

export interface EffectivePolicyInspectionResponse {
  found: boolean;
  route_id: RouteID;
  api_key_id?: string;
  matched_scope_type?: ScopeType;
  matched_scope_identifier?: string | null;
  matched_route_pattern?: RouteID | null;
  policy: PolicyRecord | null;
}

export interface BucketInspectionResponse extends EffectivePolicyInspectionResponse {
  bucket_key?: string;
  bucket_found?: boolean;
  bucket?: {
    key: string;
    tokens_remaining: number;
    last_refill_unix_ms: number;
    last_refill_at: string;
  } | null;
}

export interface MetricsSummary {
  allowed_requests: number;
  blocked_requests: number;
}

const protectedRoutes: Record<RouteID, { method: string; path: string }> = {
  ping: { method: "GET", path: "/api/protected/ping" },
  orders: { method: "POST", path: "/api/protected/orders" },
  report: { method: "GET", path: "/api/protected/report" },
};

export async function listAPIKeys(baseURL: string, adminToken: string): Promise<APIKeyRecord[]> {
  const payload = await adminRequest<{ api_keys: APIKeyRecord[] }>(baseURL, adminToken, "/api/admin/api-keys");
  return payload.api_keys ?? [];
}

export async function createAPIKey(baseURL: string, adminToken: string, name: string): Promise<CreatedAPIKey> {
  return adminRequest<CreatedAPIKey>(baseURL, adminToken, "/api/admin/api-keys", {
    method: "POST",
    body: JSON.stringify({ name }),
  });
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

export async function listPolicies(baseURL: string, adminToken: string): Promise<PolicyRecord[]> {
  const payload = await adminRequest<{ policies: PolicyRecord[] }>(baseURL, adminToken, "/api/admin/policies");
  return payload.policies ?? [];
}

export async function createPolicy(baseURL: string, adminToken: string, input: PolicyWriteInput): Promise<PolicyRecord> {
  const payload = await adminRequest<{ policy: PolicyRecord }>(baseURL, adminToken, "/api/admin/policies", {
    method: "POST",
    body: JSON.stringify(input),
  });

  return payload.policy;
}

export async function updatePolicy(baseURL: string, adminToken: string, policyID: string, input: PolicyWriteInput): Promise<PolicyRecord> {
  const payload = await adminRequest<{ policy: PolicyRecord }>(baseURL, adminToken, `/api/admin/policies/${policyID}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });

  return payload.policy;
}

export async function deactivatePolicy(baseURL: string, adminToken: string, policyID: string): Promise<PolicyRecord> {
  const payload = await adminRequest<{ policy: PolicyRecord }>(baseURL, adminToken, `/api/admin/policies/${policyID}/deactivate`, {
    method: "POST",
  });

  return payload.policy;
}

export async function inspectEffectivePolicy(
  baseURL: string,
  adminToken: string,
  routeID: RouteID,
  apiKeyID: string | null,
): Promise<EffectivePolicyInspectionResponse> {
  const search = new URLSearchParams({ route_id: routeID });
  if (apiKeyID) {
    search.set("api_key_id", apiKeyID);
  }

  return adminRequest<EffectivePolicyInspectionResponse>(
    baseURL,
    adminToken,
    `/api/admin/inspect/effective-policy?${search.toString()}`,
  );
}

export async function inspectBucket(
  baseURL: string,
  adminToken: string,
  routeID: RouteID,
  apiKeyID: string | null,
): Promise<BucketInspectionResponse> {
  const search = new URLSearchParams({ route_id: routeID });
  if (apiKeyID) {
    search.set("api_key_id", apiKeyID);
  }

  return adminRequest<BucketInspectionResponse>(
    baseURL,
    adminToken,
    `/api/admin/inspect/bucket?${search.toString()}`,
  );
}

export async function getMetricsSummary(baseURL: string, adminToken: string): Promise<MetricsSummary> {
  const payload = await adminRequest<{ metrics: MetricsSummary }>(baseURL, adminToken, "/api/admin/metrics/summary");
  return payload.metrics;
}

function joinURL(baseURL: string, path: string): string {
  return `${baseURL.replace(/\/+$/, "")}${path}`;
}

async function adminRequest<T>(baseURL: string, adminToken: string, path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(joinURL(baseURL, path), {
    ...init,
    headers: {
      Authorization: `Bearer ${adminToken}`,
      "Content-Type": "application/json",
      ...(init.headers ?? {}),
    },
  });

  if (!response.ok) {
    throw new Error(`Request failed (${response.status})`);
  }

  return (await response.json()) as T;
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
