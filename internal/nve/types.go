package nve

import "time"

// Parameter codes used by the NVE HydAPI. See https://hydapi.nve.no for the full list.
const (
	ParameterWaterLevel int32 = 1000 // Vannstand / water stage (m)
	ParameterDischarge  int32 = 1001 // Vannføring / water discharge (m³/s)
	ParameterWaterTemp  int32 = 1003 // Vanntemperatur / water temperature (°C)
)

type envelope[T any] struct {
	Data []T `json:"data"`
}

type Station struct {
	StationID   string       `json:"stationId"`
	StationName string       `json:"stationName"`
	Latitude    *float64     `json:"latitude"`
	Longitude   *float64     `json:"longitude"`
	Masl        *float64     `json:"masl"`
	RiverName   *string      `json:"riverName"`
	SeriesList  []SeriesInfo `json:"seriesList"`
}

type SeriesInfo struct {
	Parameter        int32  `json:"parameter"`
	ParameterName    string `json:"parameterName"`
	ParameterNameEng string `json:"parameterNameEng"`
}

type Series struct {
	StationID        string        `json:"stationId"`
	StationName      string        `json:"stationName"`
	Parameter        int32         `json:"parameter"`
	ParameterName    string        `json:"parameterName"`
	ParameterNameEng string        `json:"parameterNameEng"`
	Unit             string        `json:"unit"`
	Method           string        `json:"method"`
	ResolutionTime   int32         `json:"resolutionTime"`
	Observations     []Observation `json:"observations"`
}

// Observation is a single reading; pointer fields are null when the source
// reports no value.
type Observation struct {
	Time       time.Time `json:"time"`
	Value      *float64  `json:"value"`
	Correction *int32    `json:"correction"`
	Quality    *int32    `json:"quality"`
}
