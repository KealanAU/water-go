package pipeline

import (
	"encoding/json"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/kealanclarke/water-go/internal/nve"
)

// Watermill topics for the ingestion pipeline.
const (
	TopicRawStations    = "raw_nve_station"
	TopicRawObservation = "raw_nve_observation"
)

func newSeriesMessage(s nve.Series) (*message.Message, error) {
	payload, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return message.NewMessage(watermill.NewUUID(), payload), nil
}

func newStationMessage(s nve.Station) (*message.Message, error) {
	payload, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return message.NewMessage(watermill.NewUUID(), payload), nil
}
