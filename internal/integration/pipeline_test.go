//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KealanAU/water-go/internal/db"
	"github.com/KealanAU/water-go/internal/nve"
	"github.com/KealanAU/water-go/internal/pipeline"
)

// stage is anything that registers handlers on the router (Normalizer,
// AnomalyDetector, ...).
type stage interface {
	Register(*message.Router, message.Subscriber)
}

// runStage wires a gochannel pub/sub, builds the stage with that pub/sub as its
// downstream publisher, registers it, subscribes to captureTopic, and starts the
// router. It returns the pub/sub (used to feed the stage's input topic) and a
// channel of captured downstream messages. Cleanup stops the router.
func runStage(t *testing.T, build func(pub message.Publisher) stage, captureTopic string) (*gochannel.GoChannel, <-chan *message.Message) {
	t.Helper()
	logger := watermill.NopLogger{}
	ps := gochannel.NewGoChannel(gochannel.Config{}, logger)

	router, err := message.NewRouter(message.RouterConfig{}, logger)
	require.NoError(t, err)
	build(ps).Register(router, ps)

	ctx, cancel := context.WithCancel(context.Background())
	captured, err := ps.Subscribe(ctx, captureTopic)
	require.NoError(t, err)

	go func() { _ = router.Run(ctx) }()
	select {
	case <-router.Running():
	case <-time.After(10 * time.Second):
		t.Fatal("router did not start")
	}

	t.Cleanup(func() {
		cancel()
		_ = router.Close()
		_ = ps.Close()
	})
	return ps, captured
}

func publishJSON(t *testing.T, ps *gochannel.GoChannel, topic string, v any) {
	t.Helper()
	payload, err := json.Marshal(v)
	require.NoError(t, err)
	require.NoError(t, ps.Publish(topic, message.NewMessage(watermill.NewUUID(), payload)))
}

func TestNormalizerPersistsAndPublishes(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	ps, stored := runStage(t, func(pub message.Publisher) stage {
		return pipeline.NewNormalizer(testStore, pub, discardLogger())
	}, pipeline.TopicStoredObservation)

	series := nve.Series{
		StationID:      "9.9.9",
		StationName:    "Test",
		Parameter:      1000,
		ParameterName:  "Water level",
		Unit:           "m",
		ResolutionTime: 60,
		Observations: []nve.Observation{
			{Time: baseTime(), Value: ptrF(1.5)},
			{Time: baseTime().Add(time.Hour), Value: ptrF(2.5)},
			{Time: baseTime().Add(2 * time.Hour), Value: nil}, // null: stored but not forwarded
		},
	}
	publishJSON(t, ps, pipeline.TopicRawObservation, series)

	var msg *message.Message
	select {
	case msg = <-stored:
		msg.Ack()
	case <-time.After(15 * time.Second):
		t.Fatal("no stored_observation published")
	}

	var out pipeline.StoredSeries
	require.NoError(t, json.Unmarshal(msg.Payload, &out))
	assert.Equal(t, "9.9.9", out.StationID)
	assert.Len(t, out.Points, 2, "only non-null points are forwarded")

	// All three observation rows landed; the station was auto-created via FK upsert.
	rows, err := testStore.Queries.ObservationsByStation(ctx, db.ObservationsByStationParams{
		StationID: "9.9.9", Parameter: 1000,
		Time: baseTime().Add(-time.Hour), Time_2: baseTime().Add(3 * time.Hour),
		Limit: 100, Offset: 0,
	})
	require.NoError(t, err)
	assert.Len(t, rows, 3)

	stations, err := testStore.Queries.ListStations(ctx)
	require.NoError(t, err)
	require.Len(t, stations, 1)
	assert.Equal(t, "9.9.9", stations[0].StationID)
}

func TestAnomalyDetectorEmitsAlert(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	seedStation(t, "7.7.7")

	// Seed a tight distribution so a large point yields a big z-score.
	for i := 0; i < 20; i++ {
		v := 5.0
		if i%2 == 0 {
			v = 5.2
		}
		require.NoError(t, testStore.Queries.InsertObservation(ctx, db.InsertObservationParams{
			Time:           baseTime().Add(time.Duration(i) * time.Minute),
			StationID:      "7.7.7",
			Parameter:      1000,
			ParameterName:  "Water level",
			Unit:           "m",
			ResolutionTime: 60,
			Value:          ptrF(v),
		}))
	}

	ps, alerts := runStage(t, func(pub message.Publisher) stage {
		return pipeline.NewAnomalyDetector(testStore, pub, discardLogger(), 3.0, 100)
	}, pipeline.TopicAlert)

	publishJSON(t, ps, pipeline.TopicStoredObservation, pipeline.StoredSeries{
		StationID:     "7.7.7",
		Parameter:     1000,
		ParameterName: "Water level",
		Points:        []pipeline.StoredPoint{{Time: baseTime().Add(time.Hour), Value: 100}},
	})

	select {
	case msg := <-alerts:
		msg.Ack()
		var a pipeline.Alert
		require.NoError(t, json.Unmarshal(msg.Payload, &a))
		assert.Equal(t, "7.7.7", a.StationID)
		assert.Greater(t, a.ZScore, 3.0)
	case <-time.After(15 * time.Second):
		t.Fatal("no alert published")
	}

	rows, err := testStore.Queries.ListAnomaliesByStation(ctx, db.ListAnomaliesByStationParams{
		StationID: "7.7.7", Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, float64(100), rows[0].Value)
}
