import { useEffect, useMemo, useState } from "react";
import { BrowserRouter, NavLink, Navigate, Route, Routes } from "react-router-dom";

import { APIKeyRecord, CreatedAPIKey, DemoAPIKeyRecord, PublicConfig, getPublicConfig, listAPIKeys } from "./api";
import { StoredKey, mergeSelectableKeys } from "./appTypes";
import BucketInspectorPage from "./pages/BucketInspectorPage";
import PolicyAdminPage from "./pages/PolicyAdminPage";
import RequestSimulatorPage from "./pages/RequestSimulatorPage";

const storageKeys = {
  apiBaseURL: "rate-limiter-web:api-base-url",
  adminToken: "rate-limiter-web:admin-token",
  sessionKeys: "rate-limiter-web:session-keys",
};

const defaultAPIBaseURL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";
const defaultAdminToken = "dev-admin-token";
const githubRepoURL =
  import.meta.env.VITE_GITHUB_REPO_URL ?? "https://github.com/JosephGalante/rate_limiter";

export default function App() {
  const [apiBaseURL, setAPIBaseURL] = useState(() => loadString(storageKeys.apiBaseURL, defaultAPIBaseURL));
  const [adminToken, setAdminToken] = useState(() => loadString(storageKeys.adminToken, defaultAdminToken));
  const [publicConfig, setPublicConfig] = useState<PublicConfig | null>(null);
  const [apiKeys, setAPIKeys] = useState<APIKeyRecord[]>([]);
  const [storedKeys, setStoredKeys] = useState<StoredKey[]>(() => loadStoredKeys());
  const [errorMessage, setErrorMessage] = useState("");
  const [isLoadingKeys, setIsLoadingKeys] = useState(false);

  const publicDemoMode = publicConfig?.public_demo_mode ?? false;
  const selectableKeys = useMemo(() => mergeSelectableKeys(apiKeys, storedKeys), [apiKeys, storedKeys]);

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
    void refreshPublicConfig();
  }, [apiBaseURL]);

  useEffect(() => {
    if (publicConfig == null) {
      return;
    }

    if (publicConfig.public_demo_mode) {
      const demoKey = publicConfig.demo_api_key;
      setAPIKeys(demoKey ? [toAPIKeyRecord(demoKey)] : []);
      if (demoKey) {
        setStoredKeys((current) => [
          toStoredKey(demoKey),
          ...current.filter((item) => item.id !== demoKey.id),
        ]);
      }
      return;
    }

    void refreshKeys();
  }, [adminToken, apiBaseURL, publicConfig]);

  async function refreshPublicConfig() {
    try {
      const resolved = await getPublicConfig(apiBaseURL);
      setPublicConfig(resolved);
      setErrorMessage("");
    } catch (error) {
      setPublicConfig({
        public_demo_mode: false,
        admin_mutations_enabled: true,
        demo_api_key: null,
      });
      setErrorMessage(error instanceof Error ? error.message : "Failed to load app configuration.");
    }
  }

  async function refreshKeys() {
    if (publicDemoMode) {
      return;
    }
    setIsLoadingKeys(true);
    setErrorMessage("");
    try {
      setAPIKeys(await listAPIKeys(apiBaseURL, adminToken));
    } catch (error) {
      setErrorMessage(error instanceof Error ? error.message : "Failed to load API keys.");
    } finally {
      setIsLoadingKeys(false);
    }
  }

  async function rememberCreatedKey(created: CreatedAPIKey) {
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
  }

  function rememberImportedKey(apiKeyID: string, rawKey: string) {
    const normalizedRawKey = rawKey.trim();
    if (!normalizedRawKey) {
      return;
    }

    const selected = selectableKeys.find((item) => item.id === apiKeyID);
    if (!selected) {
      return;
    }

    setStoredKeys((current) => [
      {
        id: selected.id,
        name: selected.name,
        keyPrefix: selected.key_prefix,
        rawKey: normalizedRawKey,
      },
      ...current.filter((item) => item.id !== selected.id),
    ]);
  }

  return (
    <BrowserRouter>
      <a
        aria-label="View the Distributed Rate Limiter GitHub repository"
        className="github-link"
        href={githubRepoURL}
        rel="noreferrer"
        target="_blank"
      >
        <svg aria-hidden="true" className="github-link__icon" viewBox="0 0 24 24">
          <path
            d="M12 1.5C6.2 1.5 1.5 6.29 1.5 12.2c0 4.72 3.05 8.72 7.28 10.14.53.1.72-.23.72-.52 0-.26-.01-.94-.01-1.84-2.96.66-3.58-1.45-3.58-1.45-.48-1.25-1.18-1.58-1.18-1.58-.97-.67.07-.66.07-.66 1.07.08 1.64 1.13 1.64 1.13.95 1.67 2.49 1.19 3.09.91.1-.71.37-1.19.67-1.46-2.36-.28-4.84-1.21-4.84-5.41 0-1.2.42-2.18 1.1-2.95-.11-.28-.48-1.41.11-2.94 0 0 .9-.29 2.97 1.13a10.18 10.18 0 0 1 5.4 0c2.06-1.42 2.96-1.13 2.96-1.13.6 1.53.23 2.66.11 2.94.69.77 1.1 1.75 1.1 2.95 0 4.21-2.49 5.13-4.87 5.4.39.34.72 1 .72 2.03 0 1.46-.01 2.64-.01 3 0 .29.19.63.73.52 4.22-1.42 7.27-5.42 7.27-10.14C22.5 6.29 17.8 1.5 12 1.5Z"
            fill="currentColor"
          />
        </svg>
        <span className="github-link__label">GitHub</span>
      </a>
      <main className="app-shell">
        <header className="hero">
          <p className="eyebrow">Distributed Rate Limiting Service</p>
          <h1>Thin UI, backend-heavy story.</h1>
          <p className="lede">
            Use the simulator to generate pressure and inspect the resolved Redis bucket state.
            {publicDemoMode
              ? " This deployment is running in public demo mode, so policy management is read-only."
              : " Policy management stays available here for the full backend walkthrough."}
          </p>
        </header>

        <section className="panel connection-panel">
          <div className="panel-header">
            <h2>Connection</h2>
            <button
              className="button secondary"
              onClick={() => void (publicDemoMode ? refreshPublicConfig() : refreshKeys())}
              disabled={isLoadingKeys}
            >
              {isLoadingKeys ? "Refreshing..." : publicDemoMode ? "Refresh demo config" : "Refresh API keys"}
            </button>
          </div>
          <div className="field-row field-row--triple">
            <label className="field">
              <span>API base URL</span>
              <input value={apiBaseURL} onChange={(event) => setAPIBaseURL(event.target.value)} />
            </label>
            {publicDemoMode ? (
              <div className="selection-card selection-card--compact">
                <span>Mode</span>
                <strong>Public demo</strong>
                <small>admin mutations disabled</small>
              </div>
            ) : (
              <label className="field">
                <span>Admin token</span>
                <input value={adminToken} onChange={(event) => setAdminToken(event.target.value)} />
              </label>
            )}
            <div className="selection-card selection-card--compact">
              <span>Active API keys</span>
              <strong>{apiKeys.filter((item) => item.is_active).length}</strong>
              <small>session raw keys {storedKeys.length}</small>
            </div>
          </div>
          {errorMessage ? <p className="error-banner">{errorMessage}</p> : null}
        </section>

        <nav className="tab-nav">
          <NavLink end to="/" className={({ isActive }) => navClassName(isActive)}>
            Request Simulator
          </NavLink>
          <NavLink to="/policies" className={({ isActive }) => navClassName(isActive)}>
            {publicDemoMode ? "Policies" : "Policy Admin"}
          </NavLink>
          <NavLink to="/inspector" className={({ isActive }) => navClassName(isActive)}>
            Bucket Inspector
          </NavLink>
        </nav>

        <Routes>
          <Route
            path="/"
            element={
              <RequestSimulatorPage
                adminToken={adminToken}
                apiBaseURL={apiBaseURL}
                publicDemoMode={publicDemoMode}
                onCreatedKey={rememberCreatedKey}
                onImportedKey={rememberImportedKey}
                onRefreshKeys={publicDemoMode ? refreshPublicConfig : refreshKeys}
                selectableKeys={selectableKeys}
              />
            }
          />
          <Route
            path="/policies"
            element={<PolicyAdminPage adminToken={adminToken} apiBaseURL={apiBaseURL} apiKeys={apiKeys} publicDemoMode={publicDemoMode} />}
          />
          <Route
            path="/inspector"
            element={<BucketInspectorPage adminToken={adminToken} apiBaseURL={apiBaseURL} apiKeys={apiKeys} publicDemoMode={publicDemoMode} />}
          />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
    </BrowserRouter>
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

function navClassName(isActive: boolean): string {
  return isActive ? "tab-link tab-link--active" : "tab-link";
}

function toAPIKeyRecord(item: DemoAPIKeyRecord): APIKeyRecord {
  return {
    id: item.id,
    name: item.name,
    key_prefix: item.key_prefix,
    is_active: item.is_active,
    created_at: item.created_at,
  };
}

function toStoredKey(item: DemoAPIKeyRecord): StoredKey {
  return {
    id: item.id,
    name: item.name,
    keyPrefix: item.key_prefix,
    rawKey: item.raw_key,
  };
}
