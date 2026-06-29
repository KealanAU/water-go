package pipeline

import (
	"encoding/json"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
)

// Watermill topics for the ingestion pipeline.
const (
	TopicRawStations    = "raw_nve_station"
	TopicRawObservation = "raw_nve_observation"
)

// newMessage JSON-encodes v as a Watermill message with a fresh UUID.
func newMessage[T any](v T) (*message.Message, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return message.NewMessage(watermill.NewUUID(), payload), nil
}
