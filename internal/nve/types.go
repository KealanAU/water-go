package nve

import "time"

type envelope[T any] struct {
	Data []T `json:"data"`
}

type Station struct {
	StationID   string   `json:"stationId"`
	StationName string   `json:"stationName"`
	Latitude    *float64 `json:"latitude"`
	Longitude   *float64 `json:"longitude"`
	Masl        *float64 `json:"masl"`
	RiverName   *string  `json:"riverName"`
}

type Series struct {
	StationID      string        `json:"stationId"`
	StationName    string        `json:"stationName"`
	Parameter      int32         `json:"parameter"`
	ParameterName  string        `json:"parameterName"`
	Unit           string        `json:"unit"`
	ResolutionTime int32         `json:"resolutionTime"`
	Observations   []Observation `json:"observations"`
}

// Observation is a single reading; pointer fields are null when the source
// reports no value.
type Observation struct {
	Time       time.Time `json:"time"`
	Value      *float64  `json:"value"`
	Correction *int32    `json:"correction"`
	Quality    *int32    `json:"quality"`
}
