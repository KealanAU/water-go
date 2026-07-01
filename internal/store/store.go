// Package store wraps the pgx connection pool and the sqlc-generated queries,
// and applies schema migrations.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver ("pgx") for goose
	"github.com/pressly/goose/v3"

	"github.com/KealanAU/water-go/internal/db"
)

type Store struct {
	Pool    *pgxpool.Pool
	Queries *db.Queries
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &Store{Pool: pool, Queries: db.New(pool)}, nil
}

func (s *Store) Close() { s.Pool.Close() }

// Migrate applies the embedded SQL migrations using goose. goose tracks applied
// versions in a goose_db_version table and takes a session-level advisory lock,
// so it is safe to run on every boot and across concurrent instances. The
// migrations themselves remain idempotent (IF NOT EXISTS), so a database that
// was previously migrated by the old hand-rolled runner upgrades cleanly.
func (s *Store) Migrate(ctx context.Context, migrations fs.FS) error {
	goose.SetBaseFS(migrations)
	defer goose.SetBaseFS(nil)

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("store: goose dialect: %w", err)
	}

	// goose needs a database/sql handle; open one from the pool's DSN via the
	// pgx stdlib driver rather than sharing the pgxpool.
	sqlDB, err := sql.Open("pgx", s.Pool.Config().ConnString())
	if err != nil {
		return fmt.Errorf("store: open migration db: %w", err)
	}
	defer sqlDB.Close()

	if err := goose.UpContext(ctx, sqlDB, "migrations"); err != nil {
		return fmt.Errorf("store: apply migrations: %w", err)
	}
	return nil
}
