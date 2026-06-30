// Command ingester runs the NVE -> Watermill -> TimescaleDB ingestion pipeline.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/message/router/middleware"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"

	dbassets "github.com/KealanAU/water-go/db"
	"github.com/KealanAU/water-go/internal/config"
	"github.com/KealanAU/water-go/internal/nve"
	"github.com/KealanAU/water-go/internal/pipeline"
	"github.com/KealanAU/water-go/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	if err := run(logger); err != nil {
		logger.Error("ingester exited", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	st, err := connectWithRetry(ctx, cfg.DatabaseURL, logger)
	if err != nil {
		return err
	}
	defer st.Close()

	if err := st.Migrate(ctx, dbassets.Migrations); err != nil {
		return err
	}
	logger.Info("migrations applied")

	wmLogger := watermill.NewSlogLogger(logger)
	pubSub := gochannel.NewGoChannel(gochannel.Config{}, wmLogger)
	defer pubSub.Close()

	router, err := message.NewRouter(message.RouterConfig{}, wmLogger)
	if err != nil {
		return err
	}
	router.AddMiddleware(
		middleware.Recoverer,
		middleware.Retry{MaxRetries: 3, InitialInterval: time.Second, Logger: wmLogger}.Middleware,
	)

	normalizer := pipeline.NewNormalizer(st, logger)
	normalizer.Register(router, pubSub)

	client := nve.NewClient(cfg.NVEBaseURL, cfg.NVEAPIKey)
	poller := pipeline.NewPoller(cfg, client, pubSub, logger)

	go func() {
		select {
		case <-ctx.Done():
			return
		case <-router.Running():
		}
		logger.Info("poller started", "interval", cfg.PollInterval, "stations", cfg.StationIDs)
		if err := poller.Run(ctx); err != nil && ctx.Err() == nil {
			logger.Error("poller stopped", "err", err)
		}
	}()

	logger.Info("ingester running")
	return router.Run(ctx)
}

func connectWithRetry(ctx context.Context, dsn string, logger *slog.Logger) (*store.Store, error) {
	const attempts = 10
	var lastErr error
	for i := 0; i < attempts; i++ {
		st, err := store.New(ctx, dsn)
		if err == nil {
			return st, nil
		}
		lastErr = err
		logger.Warn("db not ready, retrying", "attempt", i+1, "err", err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil, lastErr
}
