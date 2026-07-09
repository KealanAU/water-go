package pipeline

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/KealanAU/water-go/internal/config"
	"github.com/KealanAU/water-go/internal/metrics"
	"github.com/KealanAU/water-go/internal/nve"
)

type Poller struct {
	cfg       *config.Config
	client    *nve.Client
	publisher message.Publisher
	log       *slog.Logger

	// stationIDs is the effective set of stations to poll, resolved once at
	// startup from either the explicit config list or auto-discovery.
	stationIDs []string
}

func NewPoller(cfg *config.Config, client *nve.Client, pub message.Publisher, log *slog.Logger) *Poller {
	return &Poller{cfg: cfg, client: client, publisher: pub, log: log}
}

// Run polls immediately, then on every tick of the configured interval until
// ctx is canceled.
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

// syncStations records the effective station set on the poller for
// pollObservations to use, besides publishing station metadata.
func (p *Poller) syncStations(ctx context.Context) {
	start := time.Now()
	stations, err := p.client.Stations(ctx, true)
	metrics.ObserveRequest("stations", start)
	if err != nil {
		p.log.Error("station sync failed", "err", err)
		return
	}

	var tracked []nve.Station
	if p.cfg.DiscoverStations {
		tracked = discover(stations, p.cfg.MaxStations)
		p.log.Info("discovered stations", "count", len(tracked), "max", p.cfg.MaxStations)
	} else {
		wanted := make(map[string]bool, len(p.cfg.StationIDs))
		for _, id := range p.cfg.StationIDs {
			wanted[id] = true
		}
		for _, s := range stations {
			if wanted[s.StationID] {
				tracked = append(tracked, s)
			}
		}
	}

	ids := make([]string, 0, len(tracked))
	var published int
	for _, s := range tracked {
		ids = append(ids, s.StationID)
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
	p.stationIDs = ids
	p.log.Info("synced stations", "published", published, "tracked", len(ids))
}

// discover returns up to limit active stations in a deterministic order.
func discover(stations []nve.Station, limit int) []nve.Station {
	sorted := make([]nve.Station, len(stations))
	copy(sorted, stations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].StationID < sorted[j].StationID })
	if limit > 0 && len(sorted) > limit {
		sorted = sorted[:limit]
	}
	return sorted
}

func (p *Poller) pollObservations(ctx context.Context) {
	// NVE requires an explicit ISO-8601 start/end interval.
	now := time.Now().UTC()
	referenceTime := now.Add(-p.cfg.Lookback).Format(time.RFC3339) + "/" + now.Format(time.RFC3339)

	stationIDs := p.stationIDs
	if len(stationIDs) == 0 && !p.cfg.DiscoverStations {
		// Fall back to the configured list if station sync has not populated one
		// (e.g. the initial /Stations call failed) but an explicit list exists.
		stationIDs = p.cfg.StationIDs
	}

	var published int
	for _, stationID := range stationIDs {
		for _, param := range p.cfg.Parameters {
			metrics.PollTotal.Inc()
			start := time.Now()
			series, err := p.client.Observations(ctx, nve.ObservationsParams{
				StationID:      stationID,
				Parameter:      param,
				ResolutionTime: p.cfg.ResolutionTime,
				ReferenceTime:  referenceTime,
			})
			metrics.ObserveRequest("observations", start)
			if err != nil {
				metrics.IncFetchError(param)
				p.log.Error("fetch observations", "station", stationID, "parameter", param, "err", err)
				continue
			}
			for i := range series {
				msg, err := newMessage(series[i])
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
	metrics.MarkPoll()
	p.log.Info("polled observations", "messages", published, "stations", len(stationIDs))
}
