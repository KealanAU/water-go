//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KealanAU/water-go/internal/db"
)

func baseTime() time.Time {
	return time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
}

func seedStation(t *testing.T, id string) {
	t.Helper()
	err := testStore.Queries.UpsertStation(context.Background(), db.UpsertStationParams{
		StationID: id,
		Name:      "Station " + id,
		RiverName: strptr("Glomma"),
		Latitude:  ptrF(60.1),
		Longitude: ptrF(11.2),
		Masl:      ptrF(150),
	})
	require.NoError(t, err)
}

func strptr(s string) *string { return &s }

func TestUpsertStationRoundTrip(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	seedStation(t, "1.1.1")
	// Upsert again with changed metadata to verify it updates in place.
	require.NoError(t, testStore.Queries.UpsertStation(ctx, db.UpsertStationParams{
		StationID: "1.1.1",
		Name:      "Renamed",
		RiverName: strptr("Otra"),
	}))

	stations, err := testStore.Queries.ListStations(ctx)
	require.NoError(t, err)
	require.Len(t, stations, 1)
	assert.Equal(t, "1.1.1", stations[0].StationID)
	assert.Equal(t, "Renamed", stations[0].Name)
	require.NotNil(t, stations[0].RiverName)
	assert.Equal(t, "Otra", *stations[0].RiverName)
}

func TestInsertObservationAndLatest(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	seedStation(t, "2.2.2")

	for i := 0; i < 3; i++ {
		require.NoError(t, testStore.Queries.InsertObservation(ctx, db.InsertObservationParams{
			Time:           baseTime().Add(time.Duration(i) * time.Hour),
			StationID:      "2.2.2",
			Parameter:      1000,
			ParameterName:  "Water level",
			Unit:           "m",
			ResolutionTime: 60,
			Value:          ptrF(float64(i)),
			Quality:        ptrI(1),
		}))
	}

	latest, err := testStore.Queries.LatestObservations(ctx, "2.2.2")
	require.NoError(t, err)
	require.Len(t, latest, 1)
	require.NotNil(t, latest[0].Value)
	assert.Equal(t, float64(2), *latest[0].Value, "latest should be the newest timestamp")
}

func TestInsertObservationUpsertsInPlace(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	seedStation(t, "2.2.3")

	params := db.InsertObservationParams{
		Time:           baseTime(),
		StationID:      "2.2.3",
		Parameter:      1000,
		ParameterName:  "Water level",
		Unit:           "m",
		ResolutionTime: 60,
		Value:          ptrF(1.0),
	}
	require.NoError(t, testStore.Queries.InsertObservation(ctx, params))
	params.Value = ptrF(9.0)
	require.NoError(t, testStore.Queries.InsertObservation(ctx, params))

	latest, err := testStore.Queries.LatestObservations(ctx, "2.2.3")
	require.NoError(t, err)
	require.Len(t, latest, 1)
	require.NotNil(t, latest[0].Value)
	assert.Equal(t, 9.0, *latest[0].Value, "conflicting insert should update value")
}

func TestObservationsByStationPagination(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	seedStation(t, "3.3.3")

	const n = 5
	for i := 0; i < n; i++ {
		require.NoError(t, testStore.Queries.InsertObservation(ctx, db.InsertObservationParams{
			Time:           baseTime().Add(time.Duration(i) * time.Hour),
			StationID:      "3.3.3",
			Parameter:      1000,
			ParameterName:  "Water level",
			Unit:           "m",
			ResolutionTime: 60,
			Value:          ptrF(float64(i)),
		}))
	}

	from := baseTime().Add(-time.Hour)
	to := baseTime().Add(time.Duration(n) * time.Hour)

	page1, err := testStore.Queries.ObservationsByStation(ctx, db.ObservationsByStationParams{
		StationID: "3.3.3", Parameter: 1000, Time: from, Time_2: to, Limit: 2, Offset: 0,
	})
	require.NoError(t, err)
	require.Len(t, page1, 2)
	// Newest first.
	require.NotNil(t, page1[0].Value)
	assert.Equal(t, float64(4), *page1[0].Value)

	page2, err := testStore.Queries.ObservationsByStation(ctx, db.ObservationsByStationParams{
		StationID: "3.3.3", Parameter: 1000, Time: from, Time_2: to, Limit: 2, Offset: 2,
	})
	require.NoError(t, err)
	require.Len(t, page2, 2)
	require.NotNil(t, page2[0].Value)
	assert.Equal(t, float64(2), *page2[0].Value)

	// Time-range filter excludes out-of-window rows.
	narrow, err := testStore.Queries.ObservationsByStation(ctx, db.ObservationsByStationParams{
		StationID: "3.3.3", Parameter: 1000,
		Time:   baseTime().Add(time.Hour),
		Time_2: baseTime().Add(2 * time.Hour),
		Limit:  100, Offset: 0,
	})
	require.NoError(t, err)
	assert.Len(t, narrow, 2)
}

func TestInsertAnomalyAndList(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	seedStation(t, "4.4.4")

	for i := 0; i < 3; i++ {
		require.NoError(t, testStore.Queries.InsertAnomaly(ctx, db.InsertAnomalyParams{
			Time:          baseTime().Add(time.Duration(i) * time.Hour),
			StationID:     "4.4.4",
			Parameter:     1000,
			ParameterName: "Water level",
			Value:         float64(100 + i),
			Mean:          5,
			Stddev:        1,
			Zscore:        float64(95 + i),
			Threshold:     3,
		}))
	}

	rows, err := testStore.Queries.ListAnomaliesByStation(ctx, db.ListAnomaliesByStationParams{
		StationID: "4.4.4", Limit: 2,
	})
	require.NoError(t, err)
	require.Len(t, rows, 2, "limit should bound results")
	assert.Equal(t, float64(102), rows[0].Value, "newest first")
}

func TestRecentValuesSkipsNulls(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	seedStation(t, "5.5.5")

	vals := []*float64{ptrF(1), nil, ptrF(3)}
	for i, v := range vals {
		require.NoError(t, testStore.Queries.InsertObservation(ctx, db.InsertObservationParams{
			Time:           baseTime().Add(time.Duration(i) * time.Hour),
			StationID:      "5.5.5",
			Parameter:      1000,
			ParameterName:  "Water level",
			Unit:           "m",
			ResolutionTime: 60,
			Value:          v,
		}))
	}

	recent, err := testStore.Queries.RecentValues(ctx, db.RecentValuesParams{
		StationID: "5.5.5", Parameter: 1000, Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, recent, 2, "null values are filtered out")
}
