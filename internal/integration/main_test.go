//go:build integration

// Package integration holds cross-cutting integration tests that exercise the
// store, pipeline stages, and HTTP API against a real TimescaleDB instance
// started via testcontainers. They are gated behind the `integration` build tag
// so a plain `go test ./...` stays hermetic and Docker-free.
package integration

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	dbassets "github.com/KealanAU/water-go/db"
	"github.com/KealanAU/water-go/internal/store"
)

// testStore is the shared Store backed by one container reused across the whole
// package's tests (see TestMain). Tests must call resetDB before seeding.
var testStore *store.Store

// discardLogger keeps test output quiet.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"timescale/timescaledb:latest-pg16",
		postgres.WithDatabase("water"),
		postgres.WithUsername("water"),
		postgres.WithPassword("water"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start container: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = testcontainers.TerminateContainer(container) }()

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "connection string: %v\n", err)
		os.Exit(1)
	}

	// Give the store a moment; BasicWaitStrategies already waits for readiness.
	sctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	testStore, err = store.New(sctx, dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "store.New: %v\n", err)
		os.Exit(1)
	}
	defer testStore.Close()

	if err := testStore.Migrate(sctx, dbassets.Migrations); err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// resetDB truncates all data tables so each test starts clean. It runs before
// tests that seed rows.
func resetDB(t *testing.T) {
	t.Helper()
	_, err := testStore.Pool.Exec(context.Background(),
		"TRUNCATE anomalies, observations, stations RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("reset db: %v", err)
	}
}

// ptrF returns a pointer to f, for the nullable columns sqlc models with *float64.
func ptrF(f float64) *float64 { return &f }

// ptrI returns a pointer to i, for the nullable columns sqlc models with *int32.
func ptrI(i int32) *int32 { return &i }
