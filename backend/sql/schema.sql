CREATE TABLE IF NOT EXISTS api_keys (
    id VARCHAR(255) NOT NULL,
    key VARCHAR(255) NOT NULL,
    status VARCHAR(255) NOT NULL,
    PRIMARY KEY (id)
);

CREATE TABLE IF NOT EXISTS orders (
    id VARCHAR(255) PRIMARY KEY,
    payload TEXT NOT NULL,
    date_created TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d', 'now')),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%d %H:%M:%S', 'now'))
);
