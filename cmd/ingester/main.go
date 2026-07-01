// Command ingester runs the NVE -> Watermill -> TimescaleDB ingestion pipeline.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
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
	"github.com/KealanAU/water-go/internal/metrics"
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

	normalizer := pipeline.NewNormalizer(st, pubSub, logger)
	normalizer.Register(router, pubSub)

	detector := pipeline.NewAnomalyDetector(st, pubSub, logger, cfg.AnomalyThreshold, cfg.AnomalyWindow)
	detector.Register(router, pubSub)

	alerter := pipeline.NewAlerter(logger, cfg.AlertWebhookURL)
	alerter.Register(router, pubSub)

	// Lightweight metrics-only HTTP server. The ingester produces most of the
	// pipeline metrics, so Prometheus scrapes it here.
	metricsSrv := startMetricsServer(ctx, cfg.MetricsAddr, logger)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = metricsSrv.Shutdown(shutdownCtx)
	}()

	client := nve.NewClient(cfg.NVEBaseURL, cfg.NVEAPIKey,
		nve.WithMaxRetries(cfg.NVEMaxRetries),
		nve.WithRateLimit(cfg.NVERateLimit),
		nve.WithTimeout(cfg.NVETimeout),
	)
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

// startMetricsServer launches a background HTTP server exposing Prometheus
// metrics at /metrics and a liveness probe at /healthz. It shuts down when the
// signal-driven context is cancelled.
func startMetricsServer(ctx context.Context, addr string, logger *slog.Logger) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", metrics.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}

	go func() {
		logger.Info("metrics server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("metrics server failed", "err", err)
		}
	}()

	return srv
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
