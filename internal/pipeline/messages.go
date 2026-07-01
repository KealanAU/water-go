package pipeline

import (
	"encoding/json"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
)

const (
	TopicRawStations    = "raw_nve_station"
	TopicRawObservation = "raw_nve_observation"
	// TopicStoredObservation carries points the Normalizer has successfully
	// persisted, so downstream stages (anomaly detection) can react to them.
	TopicStoredObservation = "stored_observation"
	// TopicAlert carries alert payloads emitted when an anomaly is detected.
	TopicAlert = "alert"
)

// StoredSeries is published on TopicStoredObservation after the Normalizer
// persists a series. It carries the metadata plus the points the anomaly
// detector needs, decoupling downstream stages from the raw NVE shape.
type StoredSeries struct {
	StationID     string        `json:"station_id"`
	Parameter     int32         `json:"parameter"`
	ParameterName string        `json:"parameter_name"`
	Points        []StoredPoint `json:"points"`
}

// StoredPoint is a single persisted observation value at a timestamp.
type StoredPoint struct {
	Time  time.Time `json:"time"`
	Value float64   `json:"value"`
}

// Alert is the payload published on TopicAlert when an anomaly is detected and
// dispatched by the Alerter (logged, and optionally POSTed to a webhook).
type Alert struct {
	StationID     string    `json:"station_id"`
	Parameter     int32     `json:"parameter"`
	ParameterName string    `json:"parameter_name"`
	Time          time.Time `json:"time"`
	Value         float64   `json:"value"`
	Mean          float64   `json:"mean"`
	Stddev        float64   `json:"stddev"`
	ZScore        float64   `json:"zscore"`
	Threshold     float64   `json:"threshold"`
}

func newMessage[T any](v T) (*message.Message, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return message.NewMessage(watermill.NewUUID(), payload), nil
}
