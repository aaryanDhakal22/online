-- Vendor-key guard (applied at every startup, so every statement here is
-- idempotent). The columns `created_at` / `last_used_at` are added by the Go
-- runner (migrations.Apply) before this file runs: SQLite has no
-- ADD COLUMN IF NOT EXISTS.
--
-- Invariant: the api_keys table always keeps at least one ACTIVE key once it
-- has one. Deleting the only active key, deactivating it, or overwriting its
-- plaintext is refused with RAISE(ABORT, 'last_active_key') — whatever the
-- caller (this service, an older build, or someone with the sqlite3 CLI).
-- A multi-row UPDATE that would deactivate every active key aborts as a whole
-- (the trigger fires per row; the last row sees a count of 1).

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
