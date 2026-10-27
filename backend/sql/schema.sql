-- The orders table that lived here previously has been removed.
-- online/ is now a stateless relay — the only persisted state is the api_keys
-- table. The startup migration also drops any pre-existing orders table on
-- boot so prod boxes upgrading in place don't carry stale order PII.
--
-- This file is sqlc's view of the schema. The live schema is built at startup
-- by internal/migrations (001_setup.sql, then the created_at / last_used_at
-- columns, then 002_key_guard.sql's triggers).

CREATE TABLE IF NOT EXISTS api_keys (
    id VARCHAR(255) NOT NULL,
    key VARCHAR(255) NOT NULL,
    status VARCHAR(255) NOT NULL,
    -- UTC, fixed width "2006-01-02T15:04:05.000000Z"; NULL on rows from
    -- before the key guard (002).
    created_at TEXT,
    -- last successful vendor auth with this key (stamped by Verify).
    last_used_at TEXT,
    PRIMARY KEY (id)
);

-- Mirror of 002_key_guard.sql: the only active key can't be deleted,
-- deactivated or overwritten (RAISE(ABORT, 'last_active_key')).

CREATE TRIGGER IF NOT EXISTS api_keys_keep_last_active_delete
BEFORE DELETE ON api_keys
WHEN OLD.status = 'active'
 AND (SELECT COUNT(*) FROM api_keys WHERE status = 'active') <= 1
BEGIN
    SELECT RAISE(ABORT, 'last_active_key');
END;

CREATE TRIGGER IF NOT EXISTS api_keys_keep_last_active_status
BEFORE UPDATE OF status ON api_keys
WHEN OLD.status = 'active'
 AND NEW.status <> 'active'
 AND (SELECT COUNT(*) FROM api_keys WHERE status = 'active') <= 1
BEGIN
    SELECT RAISE(ABORT, 'last_active_key');
END;

CREATE TRIGGER IF NOT EXISTS api_keys_keep_last_active_key
BEFORE UPDATE OF key ON api_keys
WHEN OLD.status = 'active'
 AND NEW.key <> OLD.key
 AND (SELECT COUNT(*) FROM api_keys WHERE status = 'active') <= 1
BEGIN
    SELECT RAISE(ABORT, 'last_active_key');
END;
