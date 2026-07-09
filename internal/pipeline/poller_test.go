package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KealanAU/water-go/internal/nve"
)

func stations(ids ...string) []nve.Station {
	out := make([]nve.Station, len(ids))
	for i, id := range ids {
		out[i] = nve.Station{StationID: id}
	}
	return out
}

func stationIDs(s []nve.Station) []string {
	out := make([]string, len(s))
	for i, st := range s {
		out[i] = st.StationID
	}
	return out
}

func TestDiscoverSortsDeterministically(t *testing.T) {
	got := discover(stations("2.1.0", "1.5.0", "3.0.0", "1.1.0"), 0)
	assert.Equal(t, []string{"1.1.0", "1.5.0", "2.1.0", "3.0.0"}, stationIDs(got))
}

func TestDiscoverAppliesLimit(t *testing.T) {
	got := discover(stations("c", "a", "d", "b"), 2)
	require.Len(t, got, 2)
	assert.Equal(t, []string{"a", "b"}, stationIDs(got))
}

func TestDiscoverLimitZeroReturnsAll(t *testing.T) {
	got := discover(stations("b", "a", "c"), 0)
	assert.Len(t, got, 3)
}

func TestDiscoverLimitLargerThanInput(t *testing.T) {
	in := stations("b", "a")
	got := discover(in, 100)
	assert.Equal(t, []string{"a", "b"}, stationIDs(got))
}

func TestDiscoverDoesNotMutateInput(t *testing.T) {
	in := stations("c", "a", "b")
	_ = discover(in, 0)
	assert.Equal(t, []string{"c", "a", "b"}, stationIDs(in))
}
