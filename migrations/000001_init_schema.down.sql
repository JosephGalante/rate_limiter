DROP TABLE IF EXISTS request_audit_logs;

DROP TRIGGER IF EXISTS trg_rate_limit_policies_updated_at ON rate_limit_policies;
DROP FUNCTION IF EXISTS set_updated_at();

DROP TABLE IF EXISTS rate_limit_policies;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS users;
