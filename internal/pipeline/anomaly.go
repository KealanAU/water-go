package pipeline

import (
	"encoding/json"
	"log/slog"
	"math"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/KealanAU/water-go/internal/analytics"
	"github.com/KealanAU/water-go/internal/db"
	"github.com/KealanAU/water-go/internal/store"
)

// AnomalyDetector consumes stored observation points, computes a rolling
// z-score per (station, parameter) over a recent window, and records anomalies
// plus emits alerts when the score crosses the configured threshold.
type AnomalyDetector struct {
	store     *store.Store
	pub       message.Publisher
	log       *slog.Logger
	threshold float64
	window    int32
}

func NewAnomalyDetector(s *store.Store, pub message.Publisher, log *slog.Logger, threshold float64, window int) *AnomalyDetector {
	if window < 2 {
		window = 2
	}
	return &AnomalyDetector{store: s, pub: pub, log: log, threshold: threshold, window: int32(window)}
}

func (d *AnomalyDetector) Register(router *message.Router, sub message.Subscriber) {
	router.AddNoPublisherHandler("detect_anomalies", TopicStoredObservation, sub, d.handle)
}

func (d *AnomalyDetector) handle(msg *message.Message) error {
	var s StoredSeries
	if err := json.Unmarshal(msg.Payload, &s); err != nil {
		d.log.Error("decode stored series", "err", err)
		return nil // poison message: ack and drop
	}

	ctx := msg.Context()

	rows, err := d.store.Queries.RecentValues(ctx, db.RecentValuesParams{
		StationID: s.StationID,
		Parameter: s.Parameter,
		Limit:     d.window,
	})
	if err != nil {
		d.log.Error("load recent values", "station", s.StationID, "parameter", s.Parameter, "err", err)
		return err // retry
	}

	values := make([]float64, 0, len(rows))
	for _, v := range rows {
		if v != nil {
			values = append(values, *v)
		}
	}

	for _, p := range s.Points {
		mean, stddev, z, ok := analytics.ZScore(values, p.Value)
		if !ok || math.Abs(z) < d.threshold {
			continue
		}

		if err := d.store.Queries.InsertAnomaly(ctx, db.InsertAnomalyParams{
			Time:          p.Time,
			StationID:     s.StationID,
			Parameter:     s.Parameter,
			ParameterName: s.ParameterName,
			Value:         p.Value,
			Mean:          mean,
			Stddev:        stddev,
			Zscore:        z,
			Threshold:     d.threshold,
		}); err != nil {
			d.log.Error("insert anomaly", "station", s.StationID, "parameter", s.Parameter, "err", err)
			return err // retry
		}

		d.log.Info("anomaly detected",
			"station", s.StationID, "parameter", s.Parameter,
			"time", p.Time, "value", p.Value, "zscore", z, "threshold", d.threshold)

		if d.pub == nil {
			continue
		}
		alertMsg, err := newMessage(Alert{
			StationID:     s.StationID,
			Parameter:     s.Parameter,
			ParameterName: s.ParameterName,
			Time:          p.Time,
			Value:         p.Value,
			Mean:          mean,
			Stddev:        stddev,
			ZScore:        z,
			Threshold:     d.threshold,
		})
		if err != nil {
			d.log.Error("marshal alert", "station", s.StationID, "err", err)
			continue
		}
		if err := d.pub.Publish(TopicAlert, alertMsg); err != nil {
			d.log.Error("publish alert", "station", s.StationID, "err", err)
		}
	}
	return nil
}
