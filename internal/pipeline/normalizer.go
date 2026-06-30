package pipeline

import (
	"encoding/json"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/KealanAU/water-go/internal/db"
	"github.com/KealanAU/water-go/internal/nve"
	"github.com/KealanAU/water-go/internal/store"
)

// Normalizer is the downstream stage: it consumes raw NVE messages, cleans them,
// and persists them via the type-safe store. Times from the API are already
// timezone-aware (RFC3339); values/quality may be null and are stored as such.
type Normalizer struct {
	store *store.Store
	log   *slog.Logger
}

func NewNormalizer(s *store.Store, log *slog.Logger) *Normalizer {
	return &Normalizer{store: s, log: log}
}

func (n *Normalizer) Register(router *message.Router, sub message.Subscriber) {
	router.AddNoPublisherHandler("normalize_stations", TopicRawStations, sub, n.handleStation)
	router.AddNoPublisherHandler("normalize_observations", TopicRawObservation, sub, n.handleObservation)
}

func (n *Normalizer) handleStation(msg *message.Message) error {
	var s nve.Station
	if err := json.Unmarshal(msg.Payload, &s); err != nil {
		n.log.Error("decode station", "err", err)
		return nil // poison message: ack and drop rather than block the pipeline
	}
	err := n.store.Queries.UpsertStation(msg.Context(), db.UpsertStationParams{
		StationID: s.StationID,
		Name:      s.StationName,
		RiverName: s.RiverName,
		Latitude:  s.Latitude,
		Longitude: s.Longitude,
		Masl:      s.Masl,
	})
	if err != nil {
		n.log.Error("upsert station", "station", s.StationID, "err", err)
		return err // retry
	}
	return nil
}

func (n *Normalizer) handleObservation(msg *message.Message) error {
	var s nve.Series
	if err := json.Unmarshal(msg.Payload, &s); err != nil {
		n.log.Error("decode series", "err", err)
		return nil
	}

	ctx := msg.Context()

	// Ensure the station row exists to satisfy the foreign key, using whatever
	// metadata the observation carries.
	if err := n.store.Queries.UpsertStation(ctx, db.UpsertStationParams{
		StationID: s.StationID,
		Name:      s.StationName,
	}); err != nil {
		n.log.Error("upsert station from series", "station", s.StationID, "err", err)
		return err
	}

	var stored int
	for _, o := range s.Observations {
		err := n.store.Queries.InsertObservation(ctx, db.InsertObservationParams{
			Time:           o.Time,
			StationID:      s.StationID,
			Parameter:      s.Parameter,
			ParameterName:  s.ParameterName,
			Unit:           s.Unit,
			ResolutionTime: s.ResolutionTime,
			Value:          o.Value,
			Quality:        o.Quality,
			Correction:     o.Correction,
		})
		if err != nil {
			n.log.Error("insert observation", "station", s.StationID, "err", err)
			return err
		}
		stored++
	}
	n.log.Debug("stored observations", "station", s.StationID, "parameter", s.Parameter, "count", stored)
	return nil
}
