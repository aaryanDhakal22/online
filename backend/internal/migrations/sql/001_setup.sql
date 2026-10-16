-- Drop the legacy orders table on every startup. Old online/ boxes wrote raw
-- order payloads (incl. customer PII) to this table; we now treat order bodies
-- as opaque and forward them to SQS without persisting. IF EXISTS keeps this
-- idempotent for fresh installs.
DROP TABLE IF EXISTS orders;

CREATE TABLE IF NOT EXISTS api_keys (
    id VARCHAR(255) NOT NULL,
    key VARCHAR(255) NOT NULL,
    status VARCHAR(255) NOT NULL,
    PRIMARY KEY (id)
);
