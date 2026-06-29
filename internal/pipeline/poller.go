package pipeline

import (
	"context"
	"log/slog"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/KealanAU/water-go/internal/config"
	"github.com/KealanAU/water-go/internal/nve"
)

// Poller periodically queries the NVE HydAPI and publishes raw messages onto
// the Watermill pipeline. It is the ingestion stage; normalisation/storage
// happens downstream in the Normalizer.
type Poller struct {
	cfg       *config.Config
	client    *nve.Client
	publisher message.Publisher
	log       *slog.Logger
}

// NewPoller constructs a Poller.
func NewPoller(cfg *config.Config, client *nve.Client, pub message.Publisher, log *slog.Logger) *Poller {
	return &Poller{cfg: cfg, client: client, publisher: pub, log: log}
}

// Run syncs station metadata once, then polls observations on PollInterval
// until the context is cancelled.
func (p *Poller) Run(ctx context.Context) error {
	p.syncStations(ctx)
	p.pollObservations(ctx)

	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			p.pollObservations(ctx)
		}
	}
}

func (p *Poller) syncStations(ctx context.Context) {
	stations, err := p.client.Stations(ctx, true)
	if err != nil {
		p.log.Error("station sync failed", "err", err)
		return
	}
	wanted := make(map[string]bool, len(p.cfg.StationIDs))
	for _, id := range p.cfg.StationIDs {
		wanted[id] = true
	}
	var published int
	for _, s := range stations {
		if !wanted[s.StationID] {
			continue
		}
		msg, err := newMessage(s)
		if err != nil {
			p.log.Error("marshal station", "station", s.StationID, "err", err)
			continue
		}
		if err := p.publisher.Publish(TopicRawStations, msg); err != nil {
			p.log.Error("publish station", "station", s.StationID, "err", err)
			continue
		}
		published++
	}
	p.log.Info("synced stations", "published", published)
}

func (p *Poller) pollObservations(ctx context.Context) {
	// NVE requires an explicit ISO-8601 start/end interval.
	now := time.Now().UTC()
	referenceTime := now.Add(-p.cfg.Lookback).Format(time.RFC3339) + "/" + now.Format(time.RFC3339)

	var published int
	for _, stationID := range p.cfg.StationIDs {
		for _, param := range p.cfg.Parameters {
			series, err := p.client.Observations(ctx, nve.ObservationsParams{
				StationID:      stationID,
				Parameter:      param,
				ResolutionTime: p.cfg.ResolutionTime,
				ReferenceTime:  referenceTime,
			})
			if err != nil {
				p.log.Error("fetch observations", "station", stationID, "parameter", param, "err", err)
				continue
			}
			for _, s := range series {
				msg, err := newMessage(s)
				if err != nil {
					p.log.Error("marshal series", "station", stationID, "err", err)
					continue
				}
				if err := p.publisher.Publish(TopicRawObservation, msg); err != nil {
					p.log.Error("publish observation", "station", stationID, "err", err)
					continue
				}
				published++
			}
		}
	}
	p.log.Info("polled observations", "messages", published)
}
