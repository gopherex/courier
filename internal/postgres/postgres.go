// Package postgres owns generated SQL and embedded schema migrations.
package postgres

import (
	"context"
	"embed"
	"fmt"

	"github.com/gopherex/sqld/pkg/migrate"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrations embed.FS

// Migrate applies the generated schema, including the pinned outbox schema, under a database lock.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	if err := migrate.Migrate(ctx, pool, migrations); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}

	return nil
}
