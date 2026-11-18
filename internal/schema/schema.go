package schema

import (
	"context"
	"database/sql"
	_ "embed"
)

//go:embed schema.sql
var schemaSQL string

// Apply creates all tables idempotently.
func Apply(db *sql.DB) error {
	_, err := db.Exec(schemaSQL)
	return err
}

// ApplyContext creates all tables idempotently.
func ApplyContext(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, schemaSQL)
	return err
}
