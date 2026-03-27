CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    email TEXT NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    name TEXT NOT NULL,
    key_prefix TEXT NOT NULL,
    key_hash TEXT NOT NULL UNIQUE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at TIMESTAMPTZ
);

CREATE INDEX idx_api_keys_user_id ON api_keys (user_id);
CREATE INDEX idx_api_keys_active_created_at ON api_keys (is_active, created_at DESC);

CREATE TABLE rate_limit_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scope_type TEXT NOT NULL,
    scope_identifier UUID REFERENCES api_keys(id) ON DELETE RESTRICT,
    route_pattern TEXT,
    capacity INTEGER NOT NULL CHECK (capacity > 0),
    refill_tokens INTEGER NOT NULL CHECK (refill_tokens > 0),
    refill_interval_seconds INTEGER NOT NULL CHECK (refill_interval_seconds > 0),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_rate_limit_scope_type
        CHECK (scope_type IN ('global', 'api_key', 'route', 'api_key_route')),
    CONSTRAINT chk_rate_limit_route_pattern
        CHECK (route_pattern IS NULL OR route_pattern IN ('ping', 'orders', 'report')),
    CONSTRAINT chk_rate_limit_scope_shape
        CHECK (
            (scope_type = 'global' AND scope_identifier IS NULL AND route_pattern IS NULL) OR
            (scope_type = 'api_key' AND scope_identifier IS NOT NULL AND route_pattern IS NULL) OR
            (scope_type = 'route' AND scope_identifier IS NULL AND route_pattern IS NOT NULL) OR
            (scope_type = 'api_key_route' AND scope_identifier IS NOT NULL AND route_pattern IS NOT NULL)
        )
);

CREATE INDEX idx_rate_limit_policies_active_lookup
    ON rate_limit_policies (scope_type, scope_identifier, route_pattern)
    WHERE is_active;

CREATE UNIQUE INDEX uniq_active_global_policy
    ON rate_limit_policies (scope_type)
    WHERE is_active AND scope_type = 'global';

CREATE UNIQUE INDEX uniq_active_api_key_policy
    ON rate_limit_policies (scope_identifier)
    WHERE is_active AND scope_type = 'api_key';

CREATE UNIQUE INDEX uniq_active_route_policy
    ON rate_limit_policies (route_pattern)
    WHERE is_active AND scope_type = 'route';

CREATE UNIQUE INDEX uniq_active_api_key_route_policy
    ON rate_limit_policies (scope_identifier, route_pattern)
    WHERE is_active AND scope_type = 'api_key_route';

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_rate_limit_policies_updated_at
    BEFORE UPDATE ON rate_limit_policies
    FOR EACH ROW
    EXECUTE FUNCTION set_updated_at();

CREATE TABLE request_audit_logs (
    id BIGSERIAL PRIMARY KEY,
    api_key_id UUID REFERENCES api_keys(id) ON DELETE SET NULL,
    route_id TEXT NOT NULL,
    policy_id UUID REFERENCES rate_limit_policies(id) ON DELETE SET NULL,
    outcome TEXT NOT NULL,
    request_cost INTEGER NOT NULL CHECK (request_cost > 0),
    tokens_remaining INTEGER NOT NULL CHECK (tokens_remaining >= 0),
    blocked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_request_audit_route
        CHECK (route_id IN ('ping', 'orders', 'report')),
    CONSTRAINT chk_request_audit_outcome
        CHECK (outcome IN ('blocked'))
);

CREATE INDEX idx_request_audit_logs_blocked_at
    ON request_audit_logs (blocked_at DESC);

CREATE INDEX idx_request_audit_logs_api_key_route
    ON request_audit_logs (api_key_id, route_id, blocked_at DESC);
