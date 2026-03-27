import { FormEvent, useEffect, useMemo, useRef, useState } from "react";

import { APIKeyRecord, RouteID, createAPIKey, listAPIKeys, sendProtectedRequest } from "./api";

type StoredKey = {
  id: string;
  name: string;
  keyPrefix: string;
  rawKey: string;
};

type SelectableKey = APIKeyRecord & {
  rawKey: string | null;
};

type RequestLogItem = {
  id: string;
  at: string;
  routeID: RouteID;
  status: number;
  remaining: number | null;
};

const storageKeys = {
  apiBaseURL: "rate-limiter-web:api-base-url",
  adminToken: "rate-limiter-web:admin-token",
  sessionKeys: "rate-limiter-web:session-keys",
};

const defaultAPIBaseURL = "http://localhost:8080";
const defaultAdminToken = "dev-admin-token";

export default function App() {
  const [apiBaseURL, setAPIBaseURL] = useState(() => loadString(storageKeys.apiBaseURL, defaultAPIBaseURL));
  const [adminToken, setAdminToken] = useState(() => loadString(storageKeys.adminToken, defaultAdminToken));
  const [apiKeys, setAPIKeys] = useState<APIKeyRecord[]>([]);
  const [storedKeys, setStoredKeys] = useState<StoredKey[]>(() => loadStoredKeys());
  const [selectedKeyID, setSelectedKeyID] = useState("");
  const [routeID, setRouteID] = useState<RouteID>("ping");
  const [requestCount, setRequestCount] = useState(10);
  const [requestsPerSecond, setRequestsPerSecond] = useState(2);
  const [createKeyName, setCreateKeyName] = useState("simulator");
  const [isRunning, setIsRunning] = useState(false);
  const [isLoadingKeys, setIsLoadingKeys] = useState(false);
  const [isCreatingKey, setIsCreatingKey] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  const [totals, setTotals] = useState({ sent: 0, allowed: 0, blocked: 0 });
  const [remainingTokens, setRemainingTokens] = useState<number | null>(null);
  const [recentLog, setRecentLog] = useState<RequestLogItem[]>([]);
  const runControl = useRef({ cancelled: false });

  const selectableKeys = useMemo<SelectableKey[]>(() => {
    const rawKeysByID = new Map(storedKeys.map((item) => [item.id, item.rawKey]));

    return apiKeys
      .filter((item) => item.is_active)
      .map((item) => ({
        ...item,
        rawKey: rawKeysByID.get(item.id) ?? null,
      }));
  }, [apiKeys, storedKeys]);

  useEffect(() => {
    window.localStorage.setItem(storageKeys.apiBaseURL, apiBaseURL);
  }, [apiBaseURL]);

  useEffect(() => {
    window.localStorage.setItem(storageKeys.adminToken, adminToken);
  }, [adminToken]);

  useEffect(() => {
    window.localStorage.setItem(storageKeys.sessionKeys, JSON.stringify(storedKeys));
  }, [storedKeys]);

  useEffect(() => {
    void refreshKeys();
  }, []);

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

    const selectedKey = selectableKeys.find((item) => item.id === selectedKeyID && item.rawKey);
    if (!selectedKey?.rawKey) {
      setErrorMessage("Choose an API key that was created in this browser session so the raw key is available.");
      setIsRunning(false);
      return undefined;
    }
    const selectedRawKey = selectedKey.rawKey;

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

  async function refreshKeys() {
    setIsLoadingKeys(true);
    setErrorMessage("");
    try {
      const items = await listAPIKeys(apiBaseURL, adminToken);
      setAPIKeys(items);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "Failed to load API keys.");
    } finally {
      setIsLoadingKeys(false);
    }
  }

  async function handleCreateKey(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setIsCreatingKey(true);
    setErrorMessage("");

    try {
      const created = await createAPIKey(apiBaseURL, adminToken, createKeyName.trim() || "simulator");
      setStoredKeys((current) => [
        {
          id: created.api_key.id,
          name: created.api_key.name,
          keyPrefix: created.api_key.key_prefix,
          rawKey: created.raw_key,
        },
        ...current.filter((item) => item.id !== created.api_key.id),
      ]);
      await refreshKeys();
      setSelectedKeyID(created.api_key.id);
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "Failed to create API key.");
    } finally {
      setIsCreatingKey(false);
    }
  }

  const selectedKey = selectableKeys.find((item) => item.id === selectedKeyID) ?? null;
  const selectableCount = selectableKeys.filter((item) => item.rawKey).length;

  return (
    <main className="app-shell">
      <section className="hero">
        <p className="eyebrow">Distributed Rate Limiting Service</p>
        <h1>Request Simulator</h1>
        <p className="lede">
          Drive protected endpoints with a real API key, watch remaining tokens drop, and confirm when requests flip to
          <code>429</code>.
        </p>
      </section>

      <section className="panel grid-two">
        <div>
          <h2>Connection</h2>
          <label className="field">
            <span>API base URL</span>
            <input value={apiBaseURL} onChange={(event) => setAPIBaseURL(event.target.value)} />
          </label>
          <label className="field">
            <span>Admin token</span>
            <input value={adminToken} onChange={(event) => setAdminToken(event.target.value)} />
          </label>
          <button className="button secondary" onClick={() => void refreshKeys()} disabled={isLoadingKeys}>
            {isLoadingKeys ? "Refreshing..." : "Refresh API keys"}
          </button>
        </div>

        <form onSubmit={(event) => void handleCreateKey(event)}>
          <h2>Session Key</h2>
          <p className="hint">
            The backend only shows raw API keys once. This page keeps keys you create in local browser storage so they
            remain selectable for simulation.
          </p>
          <label className="field">
            <span>New key name</span>
            <input value={createKeyName} onChange={(event) => setCreateKeyName(event.target.value)} />
          </label>
          <button className="button" type="submit" disabled={isCreatingKey}>
            {isCreatingKey ? "Creating..." : "Create session API key"}
          </button>
        </form>
      </section>

      <section className="panel grid-two">
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
            <button
              className="button"
              onClick={() => setIsRunning(true)}
              disabled={isRunning || !selectedKey?.rawKey}
            >
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
        </div>

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
          {selectedKey ? (
            <div className="selection-card">
              <span>Selected key</span>
              <strong>{selectedKey.name}</strong>
              <small>{selectedKey.key_prefix}</small>
            </div>
          ) : null}
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
    </main>
  );
}

function loadString(key: string, fallback: string): string {
  const value = window.localStorage.getItem(key);
  return value ? value : fallback;
}

function loadStoredKeys(): StoredKey[] {
  const raw = window.localStorage.getItem(storageKeys.sessionKeys);
  if (!raw) {
    return [];
  }

  try {
    const parsed = JSON.parse(raw) as StoredKey[];
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}
