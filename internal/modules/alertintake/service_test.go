package alertintake

import (
	"context"
	"strings"
	"testing"
)

func TestIngestVMAlertProducesStableIdempotencyKey(t *testing.T) {
	t.Parallel()

	service := NewService()
	payload := []byte(`{
		"status":"firing",
		"alerts":[
			{
				"labels":{"alertname":"HighCPU","instance":"host-1","severity":"critical"},
				"annotations":{"summary":"cpu too high"}
			}
		]
	}`)

	first, err := service.IngestVMAlert(context.Background(), payload)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	second, err := service.IngestVMAlert(context.Background(), payload)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}

	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("unexpected event count: %d / %d", len(first), len(second))
	}
	if first[0].IdempotencyKey == "" || first[0].RequestHash == "" {
		t.Fatalf("expected idempotency fields, got %+v", first[0])
	}
	if first[0].IdempotencyKey != second[0].IdempotencyKey {
		t.Fatalf("expected stable idempotency key, got %q vs %q", first[0].IdempotencyKey, second[0].IdempotencyKey)
	}
	if first[0].RequestHash != second[0].RequestHash {
		t.Fatalf("expected stable request hash, got %q vs %q", first[0].RequestHash, second[0].RequestHash)
	}
}

func TestIngestVMAlertChangesHashWhenPayloadChanges(t *testing.T) {
	t.Parallel()

	service := NewService()
	firstPayload := []byte(`{
		"status":"firing",
		"alerts":[
			{
				"labels":{"alertname":"HighCPU","instance":"host-1","severity":"critical"},
				"annotations":{"summary":"cpu too high"}
			}
		]
	}`)
	secondPayload := []byte(`{
		"status":"firing",
		"alerts":[
			{
				"labels":{"alertname":"HighCPU","instance":"host-1","severity":"critical"},
				"annotations":{"summary":"cpu too high again"}
			}
		]
	}`)

	first, err := service.IngestVMAlert(context.Background(), firstPayload)
	if err != nil {
		t.Fatalf("first ingest: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected 1 event from first payload, got %d", len(first))
	}
	second, err := service.IngestVMAlert(context.Background(), secondPayload)
	if err != nil {
		t.Fatalf("second ingest: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("expected 1 event from second payload, got %d", len(second))
	}

	if first[0].IdempotencyKey == second[0].IdempotencyKey {
		t.Fatalf("expected different idempotency keys, got %q", first[0].IdempotencyKey)
	}
	if first[0].RequestHash == second[0].RequestHash {
		t.Fatalf("expected different request hashes, got %q", first[0].RequestHash)
	}
}

func TestIngestVMAlertAcceptsAlertmanagerArrayPayload(t *testing.T) {
	t.Parallel()

	service := NewService()
	payload := []byte(`[
		{
			"labels":{"alertname":"TarsSmokeNodeUp","instance":"192.168.3.106","severity":"critical"},
			"annotations":{"summary":"array payload"}
		}
	]`)

	events, err := service.IngestVMAlert(context.Background(), payload)
	if err != nil {
		t.Fatalf("ingest array payload: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Fingerprint != "TarsSmokeNodeUp:192.168.3.106" {
		t.Fatalf("unexpected fingerprint: %+v", events[0])
	}
}

func TestIngestVMAlertRejectsMalformedPayload(t *testing.T) {
	t.Parallel()

	service := NewService()
	_, err := service.IngestVMAlert(context.Background(), []byte(`{"status":"firing","alerts":[`))
	if err == nil {
		t.Fatal("expected error for malformed payload")
	}
	if got := err.Error(); !strings.HasPrefix(got, "decode webhook") {
		t.Fatalf("expected decode error, got %q", got)
	}
}

func TestIngestVMAlertRejectsEmptyAlerts(t *testing.T) {
	t.Parallel()

	service := NewService()
	_, err := service.IngestVMAlert(context.Background(), []byte(`{"alerts":[]}`))
	if err == nil {
		t.Fatal("expected error for empty alerts")
	}
	if got := err.Error(); got != "alerts is required" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIngestVMAlertFallsBackToGeneratedFingerprint(t *testing.T) {
	t.Parallel()

	service := NewService()
	payload := []byte(`[
		{
			"labels":{},
			"annotations":{"summary":"missing identifying labels"}
		}
	]`)

	events, err := service.IngestVMAlert(context.Background(), payload)
	if err != nil {
		t.Fatalf("ingest fallback payload: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Fingerprint != "alert-0" {
		t.Fatalf("unexpected fallback fingerprint: %+v", events[0])
	}
}
