package alert

import "testing"

func TestEventFields(t *testing.T) {
	e := Event{
		ID:          "evt-001",
		Source:      "vmalert",
		Severity:    "critical",
		Fingerprint: "CPUHigh:host-1",
	}

	if e.ID != "evt-001" {
		t.Errorf("expected ID evt-001, got %q", e.ID)
	}
	if e.Source != "vmalert" {
		t.Errorf("expected Source vmalert, got %q", e.Source)
	}
	if e.Severity != "critical" {
		t.Errorf("expected Severity critical, got %q", e.Severity)
	}
	if e.Fingerprint != "CPUHigh:host-1" {
		t.Errorf("expected Fingerprint CPUHigh:host-1, got %q", e.Fingerprint)
	}
}

func TestEventZeroValue(t *testing.T) {
	var e Event
	if e.ID != "" {
		t.Errorf("expected empty ID, got %q", e.ID)
	}
	if e.Severity != "" {
		t.Errorf("expected empty Severity, got %q", e.Severity)
	}
}

func TestEventSeverityValues(t *testing.T) {
	severities := []string{"critical", "warning", "info"}
	for _, sev := range severities {
		e := Event{Severity: sev}
		if e.Severity != sev {
			t.Errorf("severity round-trip failed: expected %q, got %q", sev, e.Severity)
		}
	}
}

func TestEventFingerprintUniqueness(t *testing.T) {
	a := Event{Fingerprint: "CPUHigh:host-1"}
	b := Event{Fingerprint: "MemLow:host-2"}
	if a.Fingerprint == b.Fingerprint {
		t.Error("distinct events should have different fingerprints")
	}
}
