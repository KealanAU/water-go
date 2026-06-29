// Package store wraps the pgx connection pool and the sqlc-generated queries,
// and applies schema migrations.
package store

import (
	"context"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kealanclarke/water-go/internal/db"
)

// Store holds the connection pool and type-safe queries.
type Store struct {
	Pool    *pgxpool.Pool
	Queries *db.Queries
}

// New opens a pgx pool against databaseURL and returns a ready Store.
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

// Close releases the connection pool.
func (s *Store) Close() { s.Pool.Close() }

// Migrate applies every migrations/*.sql file (lexically ordered) from the given FS.
// Migrations are written to be idempotent (IF NOT EXISTS / if_not_exists).
func (s *Store) Migrate(ctx context.Context, migrations fs.FS) error {
	entries, err := fs.ReadDir(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("store: read migrations: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		sqlBytes, err := fs.ReadFile(migrations, "migrations/"+name)
		if err != nil {
			return fmt.Errorf("store: read %s: %w", name, err)
		}
		if _, err := s.Pool.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("store: apply %s: %w", name, err)
		}
	}
	return nil
}
