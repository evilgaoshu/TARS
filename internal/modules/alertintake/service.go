package alertintake

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"tars/internal/contracts"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

type vmAlertRequest struct {
	Status string `json:"status"`
	Alerts []struct {
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
	} `json:"alerts"`
}

func (s *Service) IngestVMAlert(_ context.Context, rawPayload []byte) ([]contracts.AlertEvent, error) {
	alerts, err := decodeVMAlerts(rawPayload)
	if err != nil {
		return nil, fmt.Errorf("decode webhook: %w", err)
	}
	if len(alerts) == 0 {
		return nil, fmt.Errorf("alerts is required")
	}

	events := make([]contracts.AlertEvent, 0, len(alerts))
	for i, alert := range alerts {
		fingerprint := alert.Labels["alertname"] + ":" + alert.Labels["instance"]
		if fingerprint == ":" {
			fingerprint = fmt.Sprintf("alert-%d", i)
		}
		requestHash, err := hashVMAlert(alert.Labels, alert.Annotations)
		if err != nil {
			return nil, fmt.Errorf("hash alert: %w", err)
		}

		events = append(events, contracts.AlertEvent{
			Source:         "vmalert",
			Severity:       alert.Labels["severity"],
			Fingerprint:    fingerprint,
			IdempotencyKey: "vmalert:" + requestHash,
			RequestHash:    requestHash,
			Labels:         alert.Labels,
			Annotations:    alert.Annotations,
			ReceivedAt:     time.Now().UTC(),
		})
	}

	return events, nil
}

func decodeVMAlerts(rawPayload []byte) ([]struct {
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}, error) {
	var req vmAlertRequest
	if err := json.Unmarshal(rawPayload, &req); err == nil {
		return req.Alerts, nil
	}

	var alerts []struct {
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
	}
	if err := json.Unmarshal(rawPayload, &alerts); err != nil {
		return nil, err
	}
	return alerts, nil
}

func hashVMAlert(labels map[string]string, annotations map[string]string) (string, error) {
	payload, err := json.Marshal(struct {
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
	}{
		Labels:      labels,
		Annotations: annotations,
	})
	if err != nil {
		return "", err
	}

	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}
