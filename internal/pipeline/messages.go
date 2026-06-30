package pipeline

import (
	"encoding/json"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
)

const (
	TopicRawStations    = "raw_nve_station"
	TopicRawObservation = "raw_nve_observation"
)

func newMessage[T any](v T) (*message.Message, error) {
	payload, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return message.NewMessage(watermill.NewUUID(), payload), nil
}
