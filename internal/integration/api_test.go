//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KealanAU/water-go/internal/api"
	"github.com/KealanAU/water-go/internal/db"
)

// apiServer builds a chi handler against the shared store with small pagination
// bounds so clamping is easy to exercise.
func apiServer() http.Handler {
	return api.NewServer(testStore, discardLogger(), api.Options{
		DefaultPageSize: 2,
		MaxPageSize:     3,
	}).Routes()
}

func doGet(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestAPIHealthAndReady(t *testing.T) {
	h := apiServer()

	live := doGet(t, h, "/healthz")
	assert.Equal(t, http.StatusOK, live.Code)

	ready := doGet(t, h, "/readyz")
	assert.Equal(t, http.StatusOK, ready.Code)
	var body map[string]string
	require.NoError(t, json.Unmarshal(ready.Body.Bytes(), &body))
	assert.Equal(t, "ok", body["status"])
}

func TestAPIListStations(t *testing.T) {
	resetDB(t)
	seedStation(t, "a.1")
	seedStation(t, "a.2")
	h := apiServer()

	rec := doGet(t, h, "/stations")
	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var stations []db.Station
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stations))
	require.Len(t, stations, 2)
	assert.Equal(t, "a.1", stations[0].StationID)
}

func TestAPILatest(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	seedStation(t, "b.1")
	for i := 0; i < 2; i++ {
		require.NoError(t, testStore.Queries.InsertObservation(ctx, db.InsertObservationParams{
			Time: baseTime().Add(time.Duration(i) * time.Hour), StationID: "b.1",
			Parameter: 1000, ParameterName: "Water level", Unit: "m", ResolutionTime: 60,
			Value: ptrF(float64(i)),
		}))
	}
	h := apiServer()

	rec := doGet(t, h, "/stations/b.1/latest")
	require.Equal(t, http.StatusOK, rec.Code)
	var rows []db.Observation
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].Value)
	assert.Equal(t, float64(1), *rows[0].Value)
}

func TestAPIObservationsPaginationClamp(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	seedStation(t, "c.1")
	for i := 0; i < 6; i++ {
		require.NoError(t, testStore.Queries.InsertObservation(ctx, db.InsertObservationParams{
			Time: baseTime().Add(time.Duration(i) * time.Hour), StationID: "c.1",
			Parameter: 1000, ParameterName: "Water level", Unit: "m", ResolutionTime: 60,
			Value: ptrF(float64(i)),
		}))
	}
	h := apiServer()

	from := baseTime().Add(-time.Hour).Format(time.RFC3339)
	to := baseTime().Add(10 * time.Hour).Format(time.RFC3339)

	rec := doGet(t, h, "/stations/c.1/observations?parameter=1000&limit=100&from="+from+"&to="+to)
	require.Equal(t, http.StatusOK, rec.Code)
	var rows []db.Observation
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Len(t, rows, 3, "limit should be clamped to MaxPageSize")

	recDefault := doGet(t, h, "/stations/c.1/observations?parameter=1000&from="+from+"&to="+to)
	require.Equal(t, http.StatusOK, recDefault.Code)
	var defRows []db.Observation
	require.NoError(t, json.Unmarshal(recDefault.Body.Bytes(), &defRows))
	assert.Len(t, defRows, 2, "default page size applies")
}

func TestAPIObservationsBadParams(t *testing.T) {
	resetDB(t)
	seedStation(t, "d.1")
	h := apiServer()

	cases := []struct {
		name string
		path string
	}{
		{"missing parameter", "/stations/d.1/observations"},
		{"non-int parameter", "/stations/d.1/observations?parameter=abc"},
		{"bad limit", "/stations/d.1/observations?parameter=1000&limit=abc"},
		{"negative limit", "/stations/d.1/observations?parameter=1000&limit=-1"},
		{"bad from", "/stations/d.1/observations?parameter=1000&from=notatime"},
		{"to before from", "/stations/d.1/observations?parameter=1000&from=2024-02-01T00:00:00Z&to=2024-01-01T00:00:00Z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := doGet(t, h, tc.path)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestAPIAnomalies(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	seedStation(t, "e.1")
	for i := 0; i < 4; i++ {
		require.NoError(t, testStore.Queries.InsertAnomaly(ctx, db.InsertAnomalyParams{
			Time: baseTime().Add(time.Duration(i) * time.Hour), StationID: "e.1",
			Parameter: 1000, ParameterName: "Water level", Value: float64(100 + i),
			Mean: 5, Stddev: 1, Zscore: float64(95 + i), Threshold: 3,
		}))
	}
	h := apiServer()

	rec := doGet(t, h, "/stations/e.1/anomalies?limit=100")
	require.Equal(t, http.StatusOK, rec.Code)
	var rows []db.Anomaly
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &rows))
	assert.Len(t, rows, 3, "anomaly limit clamped to MaxPageSize")
}
