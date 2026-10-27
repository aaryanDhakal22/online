// Package migrations holds the embedded schema and the startup runner. Every
// step is idempotent and runs on every boot (there is no version table): the
// SQL files in order, with the column additions SQLite can't express
// idempotently done in Go between 001 and 002.
package migrations

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"strings"
)

//go:embed sql/*.sql
var FS embed.FS

// addedColumns are the api_keys columns added after 001 (see 002_key_guard.sql).
var addedColumns = []struct{ name, decl string }{
	{"created_at", "TEXT"},
	{"last_used_at", "TEXT"},
}

// Apply brings db up to the current schema. Safe to call on every startup and
// from two processes in turn (blue-green slots share one SQLite file).
func Apply(ctx context.Context, db *sql.DB) error {
	if err := execFile(ctx, db, "sql/001_setup.sql"); err != nil {
		return err
	}
	for _, c := range addedColumns {
		if err := ensureColumn(ctx, db, "api_keys", c.name, c.decl); err != nil {
			return err
		}
	}
	return execFile(ctx, db, "sql/002_key_guard.sql")
}

func execFile(ctx context.Context, db *sql.DB, name string) error {
	body, err := FS.ReadFile(name)
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}
	if _, err := db.ExecContext(ctx, string(body)); err != nil {
		return fmt.Errorf("apply %s: %w", name, err)
	}
	return nil
}

func ensureColumn(ctx context.Context, db *sql.DB, table, column, decl string) error {
	rows, err := db.QueryContext(ctx, "SELECT name FROM pragma_table_info(?)", table)
	if err != nil {
		return fmt.Errorf("table_info %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	if _, err := db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, decl)); err != nil {
		// Another process (the other blue-green slot) added it in between.
		if strings.Contains(err.Error(), "duplicate column name") {
			return nil
		}
		return fmt.Errorf("add column %s.%s: %w", table, column, err)
	}
	return nil
}
