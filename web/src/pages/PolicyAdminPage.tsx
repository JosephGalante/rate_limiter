import { FormEvent, useEffect, useState } from "react";

import { APIKeyRecord, PolicyRecord, PolicyWriteInput, RouteID, ScopeType, createPolicy, deactivatePolicy, listPolicies, updatePolicy } from "../api";

type Props = {
  adminToken: string;
  apiBaseURL: string;
  apiKeys: APIKeyRecord[];
  publicDemoMode: boolean;
};

type PolicyFormState = {
  scopeType: ScopeType;
  scopeIdentifier: string;
  routePattern: "" | RouteID;
  capacity: string;
  refillTokens: string;
  refillIntervalSeconds: string;
};

const defaultFormState: PolicyFormState = {
  scopeType: "global",
  scopeIdentifier: "",
  routePattern: "",
  capacity: "10",
  refillTokens: "1",
  refillIntervalSeconds: "60",
};

export default function PolicyAdminPage(props: Props) {
  const { adminToken, apiBaseURL, apiKeys, publicDemoMode } = props;
  const [policies, setPolicies] = useState<PolicyRecord[]>([]);
  const [form, setForm] = useState<PolicyFormState>(defaultFormState);
  const [editingPolicyID, setEditingPolicyID] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");

  useEffect(() => {
    void refreshPolicies();
  }, [adminToken, apiBaseURL]);

  async function refreshPolicies() {
    setIsLoading(true);
    setErrorMessage("");
    try {
      setPolicies(await listPolicies(apiBaseURL, adminToken, publicDemoMode));
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "Failed to load policies.");
    } finally {
      setIsLoading(false);
    }
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsSaving(true);
    setErrorMessage("");

    const payload = toPolicyWriteInput(form);
    try {
      if (editingPolicyID) {
        await updatePolicy(apiBaseURL, adminToken, editingPolicyID, payload);
      } else {
        await createPolicy(apiBaseURL, adminToken, payload);
      }
      setForm(defaultFormState);
      setEditingPolicyID(null);
      await refreshPolicies();
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "Failed to save policy.");
    } finally {
      setIsSaving(false);
    }
  }

  async function handleDeactivate(policyID: string) {
    setErrorMessage("");
    try {
      await deactivatePolicy(apiBaseURL, adminToken, policyID);
      await refreshPolicies();
      if (editingPolicyID === policyID) {
        setEditingPolicyID(null);
        setForm(defaultFormState);
      }
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "Failed to deactivate policy.");
    }
  }

  function beginEdit(policy: PolicyRecord) {
    setEditingPolicyID(policy.id);
    setForm({
      scopeType: policy.scope_type,
      scopeIdentifier: policy.scope_identifier ?? "",
      routePattern: (policy.route_pattern ?? "") as "" | RouteID,
      capacity: String(policy.capacity),
      refillTokens: String(policy.refill_tokens),
      refillIntervalSeconds: String(policy.refill_interval_seconds),
    });
  }

  return (
    <div className="page-grid">
      <section className="panel grid-two">
        {publicDemoMode ? (
          <div>
            <h2>Policy Catalog</h2>
            <p className="hint">
              Public demo mode keeps the policy model visible but read-only. Recruiters can still inspect precedence and
              route-specific behavior without being able to rewrite the backing config.
            </p>
          </div>
        ) : (
          <form onSubmit={(event) => void handleSubmit(event)}>
            <h2>{editingPolicyID ? "Edit Policy" : "Create Policy"}</h2>
            <label className="field">
              <span>Scope type</span>
              <select
                value={form.scopeType}
                onChange={(event) =>
                  setForm((current) => normalizePolicyForm({ ...current, scopeType: event.target.value as ScopeType }))
                }
              >
                <option value="global">global</option>
                <option value="api_key">api_key</option>
                <option value="route">route</option>
                <option value="api_key_route">api_key_route</option>
              </select>
            </label>

            {(form.scopeType === "api_key" || form.scopeType === "api_key_route") && (
              <label className="field">
                <span>API key scope</span>
                <select
                  value={form.scopeIdentifier}
                  onChange={(event) => setForm((current) => ({ ...current, scopeIdentifier: event.target.value }))}
                >
                  <option value="">Select an API key</option>
                  {apiKeys.filter((item) => item.is_active).map((item) => (
                    <option key={item.id} value={item.id}>
                      {item.name} · {item.key_prefix}
                    </option>
                  ))}
                </select>
              </label>
            )}

            {(form.scopeType === "route" || form.scopeType === "api_key_route") && (
              <label className="field">
                <span>Route</span>
                <select
                  value={form.routePattern}
                  onChange={(event) => setForm((current) => ({ ...current, routePattern: event.target.value as "" | RouteID }))}
                >
                  <option value="">Select a route</option>
                  <option value="ping">ping</option>
                  <option value="orders">orders</option>
                  <option value="report">report</option>
                </select>
              </label>
            )}

            <div className="field-row">
              <label className="field">
                <span>Capacity</span>
                <input
                  type="number"
                  min={1}
                  value={form.capacity}
                  onChange={(event) => setForm((current) => ({ ...current, capacity: event.target.value }))}
                />
              </label>
              <label className="field">
                <span>Refill tokens</span>
                <input
                  type="number"
                  min={1}
                  value={form.refillTokens}
                  onChange={(event) => setForm((current) => ({ ...current, refillTokens: event.target.value }))}
                />
              </label>
            </div>

            <label className="field">
              <span>Refill interval seconds</span>
              <input
                type="number"
                min={1}
                value={form.refillIntervalSeconds}
                onChange={(event) => setForm((current) => ({ ...current, refillIntervalSeconds: event.target.value }))}
              />
            </label>

            <div className="actions">
              <button className="button" type="submit" disabled={isSaving}>
                {isSaving ? "Saving..." : editingPolicyID ? "Save changes" : "Create policy"}
              </button>
              <button
                className="button secondary"
                type="button"
                onClick={() => {
                  setEditingPolicyID(null);
                  setForm(defaultFormState);
                }}
              >
                Reset
              </button>
            </div>
          </form>
        )}

        <div>
          <h2>Notes</h2>
          <p className="hint">This UI stays intentionally thin. The important interview story is in the backend precedence, projection, and bucket mutation logic.</p>
          <div className="selection-card">
            <span>Deterministic precedence</span>
            <strong>api_key_route → api_key → route → global</strong>
          </div>
        </div>
      </section>

      <section className="panel">
        <div className="panel-header">
          <h2>Policies</h2>
          <button className="button secondary" onClick={() => void refreshPolicies()} disabled={isLoading}>
            {isLoading ? "Refreshing..." : "Refresh"}
          </button>
        </div>
        {errorMessage ? <p className="error-banner">{errorMessage}</p> : null}
        {policies.length === 0 ? (
          <p className="hint">No policies yet.</p>
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Scope</th>
                  <th>API key</th>
                  <th>Route</th>
                  <th>Capacity</th>
                  <th>Refill</th>
                  <th>Status</th>
                  {!publicDemoMode ? <th /> : null}
                </tr>
              </thead>
              <tbody>
                {policies.map((policy) => (
                  <tr key={policy.id}>
                    <td>{policy.scope_type}</td>
                    <td>{policy.scope_identifier ?? "—"}</td>
                    <td>{policy.route_pattern ?? "ALL"}</td>
                    <td>{policy.capacity}</td>
                    <td>
                      {policy.refill_tokens} / {policy.refill_interval_seconds}s
                    </td>
                    <td>{policy.is_active ? "active" : "inactive"}</td>
                    {!publicDemoMode ? (
                      <td className="table-actions">
                        <button className="text-button" onClick={() => beginEdit(policy)}>
                          Edit
                        </button>
                        <button className="text-button danger" onClick={() => void handleDeactivate(policy.id)} disabled={!policy.is_active}>
                          Deactivate
                        </button>
                      </td>
                    ) : null}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}

function toPolicyWriteInput(form: PolicyFormState): PolicyWriteInput {
  return {
    scope_type: form.scopeType,
    scope_identifier: form.scopeIdentifier || null,
    route_pattern: form.routePattern || null,
    capacity: Number(form.capacity),
    refill_tokens: Number(form.refillTokens),
    refill_interval_seconds: Number(form.refillIntervalSeconds),
  };
}

function normalizePolicyForm(form: PolicyFormState): PolicyFormState {
  switch (form.scopeType) {
    case "global":
      return { ...form, scopeIdentifier: "", routePattern: "" };
    case "api_key":
      return { ...form, routePattern: "" };
    case "route":
      return { ...form, scopeIdentifier: "" };
    case "api_key_route":
    default:
      return form;
  }
}
