package metrics

import (
	"bytes"
	"strings"
	"testing"
)

func TestRegistryWritesPrometheusMetrics(t *testing.T) {
	t.Parallel()

	reg := New()
	reg.AddAlertEvents("vmalert", "accepted", 2)
	reg.IncExecution("completed")
	reg.SetFeatureFlag("execution_enabled", true)
	reg.SetRolloutMode("execution_beta")

	var buf bytes.Buffer
	if err := reg.WritePrometheus(&buf); err != nil {
		t.Fatalf("write prometheus metrics: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `tars_alert_events_total{result="accepted",source="vmalert"} 2`) {
		t.Fatalf("expected alert events metric, got:\n%s", output)
	}
	if !strings.Contains(output, `tars_execution_requests_total{status="completed"} 1`) {
		t.Fatalf("expected execution metric, got:\n%s", output)
	}
	if !strings.Contains(output, `tars_feature_flag_enabled{flag="execution_enabled"} 1`) {
		t.Fatalf("expected feature flag gauge, got:\n%s", output)
	}
	if !strings.Contains(output, `tars_rollout_mode_info{mode="execution_beta"} 1`) {
		t.Fatalf("expected rollout mode gauge, got:\n%s", output)
	}
}

func TestRegistryWritesHistogramMetrics(t *testing.T) {
	t.Parallel()

	reg := New()
	reg.ObserveHistogram("tars_observability_store_append_duration_seconds", "Append duration for observability store writes.", Labels{"signal": "logs"}, []float64{0.001, 0.01, 0.1}, 0.005)
	reg.ObserveHistogram("tars_observability_store_append_duration_seconds", "Append duration for observability store writes.", Labels{"signal": "logs"}, []float64{0.001, 0.01, 0.1}, 0.05)

	var buf bytes.Buffer
	if err := reg.WritePrometheus(&buf); err != nil {
		t.Fatalf("write prometheus metrics: %v", err)
	}

	output := buf.String()
	for _, want := range []string{
		`tars_observability_store_append_duration_seconds_bucket{le="0.001",signal="logs"} 0`,
		`tars_observability_store_append_duration_seconds_bucket{le="0.01",signal="logs"} 1`,
		`tars_observability_store_append_duration_seconds_bucket{le="0.1",signal="logs"} 2`,
		`tars_observability_store_append_duration_seconds_bucket{le="+Inf",signal="logs"} 2`,
		`tars_observability_store_append_duration_seconds_sum{signal="logs"} 0.055`,
		`tars_observability_store_append_duration_seconds_count{signal="logs"} 2`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected histogram output to contain %q, got:\n%s", want, output)
		}
	}
}
