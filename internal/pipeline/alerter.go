package pipeline

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"

	"github.com/KealanAU/water-go/internal/metrics"
)

// Alerter consumes alert messages and dispatches them: it always logs the alert
// at WARN, and additionally POSTs the JSON payload to a webhook when one is
// configured. Webhook delivery is best-effort with a short timeout so a slow or
// broken endpoint never stalls the pipeline.
type Alerter struct {
	log        *slog.Logger
	webhookURL string
	client     *http.Client
}

func NewAlerter(log *slog.Logger, webhookURL string) *Alerter {
	return &Alerter{
		log:        log,
		webhookURL: webhookURL,
		client:     &http.Client{Timeout: 5 * time.Second},
	}
}

func (a *Alerter) Register(router *message.Router, sub message.Subscriber) {
	router.AddNoPublisherHandler("dispatch_alerts", TopicAlert, sub, a.handle)
}

func (a *Alerter) handle(msg *message.Message) error {
	var alert Alert
	if err := json.Unmarshal(msg.Payload, &alert); err != nil {
		a.log.Error("decode alert", "err", err)
		return nil // poison message: ack and drop
	}

	a.log.Warn("anomaly alert",
		"station", alert.StationID,
		"parameter", alert.Parameter,
		"parameter_name", alert.ParameterName,
		"time", alert.Time,
		"value", alert.Value,
		"mean", alert.Mean,
		"stddev", alert.Stddev,
		"zscore", alert.ZScore,
		"threshold", alert.Threshold,
	)
	metrics.AlertDispatched("log", "success")

	if a.webhookURL == "" {
		return nil
	}

	// Best-effort webhook delivery: log failures but ack the message so a broken
	// endpoint doesn't wedge the pipeline behind endless retries.
	if err := a.postWebhook(msg.Context(), msg.Payload); err != nil {
		metrics.AlertDispatched("webhook", "error")
		a.log.Error("alert webhook failed", "station", alert.StationID, "err", err)
	} else {
		metrics.AlertDispatched("webhook", "success")
	}
	return nil
}

func (a *Alerter) postWebhook(ctx context.Context, payload []byte) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.webhookURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
