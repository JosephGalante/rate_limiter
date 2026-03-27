import { useEffect, useState } from "react";

import { APIKeyRecord, BucketInspectionResponse, EffectivePolicyInspectionResponse, MetricsSummary, RouteID, getMetricsSummary, inspectBucket, inspectEffectivePolicy } from "../api";

type Props = {
  adminToken: string;
  apiBaseURL: string;
  apiKeys: APIKeyRecord[];
  publicDemoMode: boolean;
};

export default function BucketInspectorPage(props: Props) {
  const { adminToken, apiBaseURL, apiKeys, publicDemoMode } = props;
  const [routeID, setRouteID] = useState<RouteID>("ping");
  const [apiKeyID, setAPIKeyID] = useState("");
  const [effectivePolicy, setEffectivePolicy] = useState<EffectivePolicyInspectionResponse | null>(null);
  const [bucketInspection, setBucketInspection] = useState<BucketInspectionResponse | null>(null);
  const [metrics, setMetrics] = useState<MetricsSummary | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");

  useEffect(() => {
    void refreshMetrics();
  }, [adminToken, apiBaseURL]);

  async function handleInspect() {
    setIsLoading(true);
    setErrorMessage("");

    try {
      const [policy, bucket] = await Promise.all([
        inspectEffectivePolicy(apiBaseURL, adminToken, routeID, apiKeyID || null, publicDemoMode),
        inspectBucket(apiBaseURL, adminToken, routeID, apiKeyID || null, publicDemoMode),
      ]);

      setEffectivePolicy(policy);
      setBucketInspection(bucket);
      await refreshMetrics();
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "Failed to inspect bucket.");
    } finally {
      setIsLoading(false);
    }
  }

  async function refreshMetrics() {
    try {
      setMetrics(await getMetricsSummary(apiBaseURL, adminToken, publicDemoMode));
    } catch {
      setMetrics(null);
    }
  }

  return (
    <div className="page-grid">
      <section className="panel grid-two">
        <div>
          <h2>Lookup</h2>
          <label className="field">
            <span>Route</span>
            <select value={routeID} onChange={(event) => setRouteID(event.target.value as RouteID)}>
              <option value="ping">ping</option>
              <option value="orders">orders</option>
              <option value="report">report</option>
            </select>
          </label>
          <label className="field">
            <span>API key</span>
            <select value={apiKeyID} onChange={(event) => setAPIKeyID(event.target.value)}>
              <option value="">None / global-route lookup</option>
              {apiKeys.filter((item) => item.is_active).map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name} · {item.key_prefix}
                </option>
              ))}
            </select>
          </label>
          <div className="actions">
            <button className="button" onClick={() => void handleInspect()} disabled={isLoading}>
              {isLoading ? "Inspecting..." : "Inspect"}
            </button>
          </div>
        </div>

        <div>
          <h2>Summary Metrics</h2>
          <div className="stats">
            <article className="stat-card">
              <span>Allowed requests</span>
              <strong>{metrics?.allowed_requests ?? "—"}</strong>
            </article>
            <article className="stat-card">
              <span>Blocked requests</span>
              <strong>{metrics?.blocked_requests ?? "—"}</strong>
            </article>
          </div>
        </div>
      </section>

      {errorMessage ? <p className="error-banner">{errorMessage}</p> : null}

      <section className="panel grid-two">
        <div>
          <h2>Effective Policy</h2>
          {!effectivePolicy ? (
            <p className="hint">Run an inspection to see the resolved policy.</p>
          ) : effectivePolicy.policy ? (
            <div className="detail-card">
              <span>Matched scope</span>
              <strong>{effectivePolicy.matched_scope_type}</strong>
              <small>route {effectivePolicy.route_id}</small>
              <small>capacity {effectivePolicy.policy.capacity}</small>
              <small>
                refill {effectivePolicy.policy.refill_tokens} / {effectivePolicy.policy.refill_interval_seconds}s
              </small>
            </div>
          ) : (
            <p className="hint">No active policy matched this lookup.</p>
          )}
        </div>

        <div>
          <h2>Bucket State</h2>
          {!bucketInspection ? (
            <p className="hint">Run an inspection to see the bucket.</p>
          ) : bucketInspection.bucket ? (
            <div className="detail-card">
              <span>Bucket key</span>
              <strong>{bucketInspection.bucket.key}</strong>
              <small>tokens remaining {bucketInspection.bucket.tokens_remaining}</small>
              <small>last refill {bucketInspection.bucket.last_refill_at}</small>
            </div>
          ) : (
            <p className="hint">No Redis bucket exists yet for this resolution. It will appear after traffic hits the route.</p>
          )}
        </div>
      </section>
    </div>
  );
}
