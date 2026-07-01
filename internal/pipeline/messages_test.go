package pipeline

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMessageRoundTripsStoredSeries(t *testing.T) {
	in := StoredSeries{
		StationID:     "1.2.3",
		Parameter:     1000,
		ParameterName: "Water level",
		Points: []StoredPoint{
			{Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), Value: 1.5},
			{Time: time.Date(2024, 1, 1, 1, 0, 0, 0, time.UTC), Value: 2.5},
		},
	}
	msg, err := newMessage(in)
	require.NoError(t, err)
	require.NotEmpty(t, msg.UUID)

	var out StoredSeries
	require.NoError(t, json.Unmarshal(msg.Payload, &out))
	assert.Equal(t, in, out)
}

func TestNewMessageRoundTripsAlert(t *testing.T) {
	in := Alert{
		StationID:     "1.2.3",
		Parameter:     1001,
		ParameterName: "Discharge",
		Time:          time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		Value:         99,
		Mean:          10,
		Stddev:        3,
		ZScore:        29.6,
		Threshold:     3,
	}
	msg, err := newMessage(in)
	require.NoError(t, err)

	var out Alert
	require.NoError(t, json.Unmarshal(msg.Payload, &out))
	assert.Equal(t, in, out)
}

func TestStoredSeriesJSONFieldNames(t *testing.T) {
	msg, err := newMessage(StoredSeries{StationID: "x", Parameter: 1000})
	require.NoError(t, err)
	var m map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(msg.Payload, &m))
	for _, key := range []string{"station_id", "parameter", "parameter_name", "points"} {
		_, ok := m[key]
		assert.True(t, ok, "expected snake_case field %q in payload", key)
	}
}
