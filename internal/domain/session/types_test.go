package session

import "testing"

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected string
	}{
		{"open", StatusOpen, "open"},
		{"analyzing", StatusAnalyzing, "analyzing"},
		{"pending_approval", StatusPendingApproval, "pending_approval"},
		{"executing", StatusExecuting, "executing"},
		{"verifying", StatusVerifying, "verifying"},
		{"resolved", StatusResolved, "resolved"},
		{"failed", StatusFailed, "failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, string(tt.status))
			}
		})
	}
}

func TestStatusAllUnique(t *testing.T) {
	all := []Status{
		StatusOpen, StatusAnalyzing, StatusPendingApproval,
		StatusExecuting, StatusVerifying, StatusResolved, StatusFailed,
	}
	seen := make(map[Status]bool)
	for _, s := range all {
		if seen[s] {
			t.Errorf("duplicate status value: %q", s)
		}
		seen[s] = true
	}
}

func TestAggregateFields(t *testing.T) {
	a := Aggregate{
		ID:     "sess-001",
		Status: StatusOpen,
	}
	if a.ID != "sess-001" {
		t.Errorf("expected ID sess-001, got %q", a.ID)
	}
	if a.Status != StatusOpen {
		t.Errorf("expected Status open, got %q", a.Status)
	}
}

func TestAggregateZeroValue(t *testing.T) {
	var a Aggregate
	if a.ID != "" {
		t.Errorf("expected empty ID, got %q", a.ID)
	}
	if a.Status != "" {
		t.Errorf("expected empty Status, got %q", a.Status)
	}
}

func TestSessionLifecycleOrder(t *testing.T) {
	// Verify the conceptual lifecycle ordering exists as distinct statuses.
	lifecycle := []Status{
		StatusOpen,
		StatusAnalyzing,
		StatusPendingApproval,
		StatusExecuting,
		StatusVerifying,
		StatusResolved,
	}
	seen := make(map[Status]bool)
	for _, s := range lifecycle {
		if seen[s] {
			t.Errorf("lifecycle has duplicate status: %q", s)
		}
		seen[s] = true
	}
	if len(lifecycle) != 6 {
		t.Errorf("expected 6 lifecycle steps, got %d", len(lifecycle))
	}
}
