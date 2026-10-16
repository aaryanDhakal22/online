-- The orders table that lived here previously has been removed.
-- online/ is now a stateless relay — the only persisted state is the api_keys
-- table. The startup migration also drops any pre-existing orders table on
-- boot so prod boxes upgrading in place don't carry stale order PII.

CREATE TABLE IF NOT EXISTS api_keys (
    id VARCHAR(255) NOT NULL,
    key VARCHAR(255) NOT NULL,
    status VARCHAR(255) NOT NULL,
    PRIMARY KEY (id)
);
