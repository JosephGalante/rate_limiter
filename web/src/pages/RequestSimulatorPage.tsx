import { FormEvent, useEffect, useMemo, useRef, useState } from "react";

import { CreatedAPIKey, RouteID, createAPIKey, sendProtectedRequest } from "../api";
import { SelectableKey } from "../appTypes";

type RequestLogItem = {
  id: string;
  at: string;
  routeID: RouteID;
  status: number;
  remaining: number | null;
};

type Props = {
  adminToken: string;
  apiBaseURL: string;
  publicDemoMode: boolean;
  onCreatedKey: (created: CreatedAPIKey) => Promise<void> | void;
  onImportedKey: (apiKeyID: string, rawKey: string) => Promise<void> | void;
  onRefreshKeys: () => Promise<void>;
  selectableKeys: SelectableKey[];
};

export default function RequestSimulatorPage(props: Props) {
  const { adminToken, apiBaseURL, publicDemoMode, onCreatedKey, onImportedKey, onRefreshKeys, selectableKeys } = props;
  const [selectedKeyID, setSelectedKeyID] = useState("");
  const [routeID, setRouteID] = useState<RouteID>("ping");
  const [requestCount, setRequestCount] = useState(10);
  const [requestsPerSecond, setRequestsPerSecond] = useState(2);
  const [createKeyName, setCreateKeyName] = useState("simulator");
  const [importRawKey, setImportRawKey] = useState("");
  const [isRunning, setIsRunning] = useState(false);
  const [isCreatingKey, setIsCreatingKey] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  const [totals, setTotals] = useState({ sent: 0, allowed: 0, blocked: 0 });
  const [remainingTokens, setRemainingTokens] = useState<number | null>(null);
  const [recentLog, setRecentLog] = useState<RequestLogItem[]>([]);
  const runControl = useRef({ cancelled: false });

  const selectableCount = useMemo(() => selectableKeys.filter((item) => item.rawKey).length, [selectableKeys]);
  const selectedKey = selectableKeys.find((item) => item.id === selectedKeyID) ?? null;

  useEffect(() => {
    if (!selectedKeyID && selectableKeys.length > 0) {
      const firstKnown = selectableKeys.find((item) => item.rawKey) ?? selectableKeys[0];
      setSelectedKeyID(firstKnown.id);
    }
  }, [selectableKeys, selectedKeyID]);

  useEffect(() => {
    if (!isRunning) {
      return undefined;
    }

    const selected = selectableKeys.find((item) => item.id === selectedKeyID && item.rawKey);
    if (!selected?.rawKey) {
      setErrorMessage("Choose an API key that was created in this browser session so the raw key is available.");
      setIsRunning(false);
      return undefined;
    }

    const selectedRawKey = selected.rawKey;
    const control = { cancelled: false };
    runControl.current = control;
    setTotals({ sent: 0, allowed: 0, blocked: 0 });
    setRecentLog([]);
    setErrorMessage("");

    const intervalMs = Math.max(1000 / Math.max(requestsPerSecond, 1), 100);

    void (async () => {
      let sent = 0;
      let allowed = 0;
      let blocked = 0;

      while (!control.cancelled && sent < requestCount) {
        try {
          const result = await sendProtectedRequest(apiBaseURL, selectedRawKey, routeID);
          sent += 1;
          if (result.status === 429) {
            blocked += 1;
          } else if (result.status >= 200 && result.status < 300) {
            allowed += 1;
          } else {
            setErrorMessage(`Protected request failed with status ${result.status}.`);
            control.cancelled = true;
          }

          setTotals({ sent, allowed, blocked });
          setRemainingTokens(result.remaining);
          setRecentLog((current) => [
            {
              id: `${Date.now()}-${sent}`,
              at: new Date().toLocaleTimeString(),
              routeID,
              status: result.status,
              remaining: result.remaining,
            },
            ...current,
          ].slice(0, 12));
        } catch (error) {
          setErrorMessage(error instanceof Error ? error.message : "Protected request failed.");
          control.cancelled = true;
        }

        if (!control.cancelled && sent < requestCount) {
          await wait(intervalMs);
        }
      }

      setIsRunning(false);
    })();

    return () => {
      control.cancelled = true;
    };
  }, [apiBaseURL, isRunning, requestCount, requestsPerSecond, routeID, selectableKeys, selectedKeyID]);

  async function handleCreateKey(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsCreatingKey(true);
    setErrorMessage("");

    try {
      const created = await createAPIKey(apiBaseURL, adminToken, createKeyName.trim() || "simulator");
      await onCreatedKey(created);
      setSelectedKeyID(created.api_key.id);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "Failed to create API key.");
    } finally {
      setIsCreatingKey(false);
    }
  }

  async function handleImportRawKey(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setErrorMessage("");

    if (!selectedKeyID) {
      setErrorMessage("Choose an API key before attaching a raw key.");
      return;
    }

    if (!importRawKey.trim()) {
      setErrorMessage("Paste a raw API key to attach it to this browser session.");
      return;
    }

    await onImportedKey(selectedKeyID, importRawKey);
    setImportRawKey("");
  }

  return (
    <div className="page-grid">
      <section className="panel grid-two">
        <div>
          <h2>{publicDemoMode ? "Demo Key" : "Session Key"}</h2>
          {publicDemoMode ? (
            <>
              <p className="hint">
                This deployment runs in public demo mode. Use the preloaded demo key below to drive the shared Redis
                buckets without exposing policy mutation routes to the internet.
              </p>
              <div className="actions">
                <button className="button secondary" type="button" onClick={() => void onRefreshKeys()}>
                  Refresh demo key
                </button>
              </div>
            </>
          ) : (
            <>
              <p className="hint">
                The backend only returns raw API keys once. This page stores keys you create in local browser storage so
                they remain usable for simulation.
              </p>
              <form onSubmit={(event) => void handleCreateKey(event)}>
                <label className="field">
                  <span>New key name</span>
                  <input value={createKeyName} onChange={(event) => setCreateKeyName(event.target.value)} />
                </label>
                <div className="actions">
                  <button className="button" type="submit" disabled={isCreatingKey}>
                    {isCreatingKey ? "Creating..." : "Create session API key"}
                  </button>
                  <button className="button secondary" type="button" onClick={() => void onRefreshKeys()}>
                    Refresh API keys
                  </button>
                </div>
              </form>
            </>
          )}
        </div>

        <div>
          <h2>Traffic Run</h2>
          <label className="field">
            <span>API key</span>
            <select value={selectedKeyID} onChange={(event) => setSelectedKeyID(event.target.value)}>
              <option value="">Select a key</option>
              {selectableKeys.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.name} · {item.key_prefix} {item.rawKey ? "" : "(metadata only)"}
                </option>
              ))}
            </select>
          </label>
          <label className="field">
            <span>Route</span>
            <select value={routeID} onChange={(event) => setRouteID(event.target.value as RouteID)}>
              <option value="ping">GET /api/protected/ping</option>
              <option value="orders">POST /api/protected/orders</option>
              <option value="report">GET /api/protected/report</option>
            </select>
          </label>
          <div className="field-row">
            <label className="field">
              <span>Request count</span>
              <input
                type="number"
                min={1}
                value={requestCount}
                onChange={(event) => setRequestCount(Number(event.target.value))}
              />
            </label>
            <label className="field">
              <span>Requests / second</span>
              <input
                type="number"
                min={1}
                max={20}
                value={requestsPerSecond}
                onChange={(event) => setRequestsPerSecond(Number(event.target.value))}
              />
            </label>
          </div>
          <div className="actions">
            <button className="button" onClick={() => setIsRunning(true)} disabled={isRunning || !selectedKey?.rawKey}>
              Start simulation
            </button>
            <button
              className="button secondary"
              onClick={() => {
                runControl.current.cancelled = true;
                setIsRunning(false);
              }}
              disabled={!isRunning}
            >
              Stop
            </button>
          </div>
          <p className="hint">
            Selectable keys with raw material in this browser: <strong>{selectableCount}</strong>
          </p>
          {!selectedKey?.rawKey && selectedKey ? (
            <form onSubmit={(event) => void handleImportRawKey(event)}>
              <label className="field">
                <span>Paste raw key for selected API key</span>
                <input
                  value={importRawKey}
                  onChange={(event) => setImportRawKey(event.target.value)}
                  placeholder="Paste a raw key from make demo-bootstrap or API key creation"
                />
              </label>
              <div className="actions">
                <button className="button secondary" type="submit">
                  Remember raw key
                </button>
              </div>
            </form>
          ) : null}
        </div>
      </section>

      <section className="panel grid-two">
        <div>
          <h2>Live Totals</h2>
          <div className="stats">
            <article className="stat-card">
              <span>Total sent</span>
              <strong>{totals.sent}</strong>
            </article>
            <article className="stat-card">
              <span>Allowed</span>
              <strong>{totals.allowed}</strong>
            </article>
            <article className="stat-card">
              <span>Blocked</span>
              <strong>{totals.blocked}</strong>
            </article>
            <article className="stat-card">
              <span>Remaining tokens</span>
              <strong>{remainingTokens ?? "—"}</strong>
            </article>
          </div>
        </div>

        <div>
          <h2>Current Selection</h2>
          {selectedKey ? (
            <div className="selection-card">
              <span>Selected key</span>
              <strong>{selectedKey.name}</strong>
              <small>{selectedKey.key_prefix}</small>
              <small>{selectedKey.rawKey ? "raw key available" : "metadata only"}</small>
            </div>
          ) : (
            <p className="hint">Choose an API key to begin.</p>
          )}
        </div>
      </section>

      <section className="panel">
        <h2>Recent Request Log</h2>
        {errorMessage ? <p className="error-banner">{errorMessage}</p> : null}
        {recentLog.length === 0 ? (
          <p className="hint">Run a simulation to populate the log.</p>
        ) : (
          <div className="log-list">
            {recentLog.map((item) => (
              <article key={item.id} className="log-row">
                <div>
                  <strong>{item.routeID}</strong>
                  <span>{item.at}</span>
                </div>
                <div>
                  <span className={item.status === 429 ? "pill blocked" : "pill allowed"}>{item.status}</span>
                  <span>remaining {item.remaining ?? "—"}</span>
                </div>
              </article>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}
