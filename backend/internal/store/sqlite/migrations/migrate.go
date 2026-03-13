package migrations

import (
	"context"
	"database/sql"
	"embed"

	"github.com/pressly/goose/v3"
)

//go:embed *.sql
var fs embed.FS

// Run executes all pending migrations against the given database.
// Uses the goose Provider API to avoid mutating package-level globals,
// making it safe for concurrent use in tests.
func Run(ctx context.Context, db *sql.DB) error {
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, fs)
	if err != nil {
		return err
	}
	_, err = provider.Up(ctx)
	return err
}
