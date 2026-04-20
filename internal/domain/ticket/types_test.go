package ticket

import "testing"

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected string
	}{
		{"pending", StatusPending, "pending"},
		{"in_progress", StatusInProgress, "in_progress"},
		{"verifying", StatusVerifying, "verifying"},
		{"resolved", StatusResolved, "resolved"},
		{"closed", StatusClosed, "closed"},
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
		StatusPending, StatusInProgress, StatusVerifying,
		StatusResolved, StatusClosed,
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
		ID:     "ticket-001",
		Status: StatusPending,
	}
	if a.ID != "ticket-001" {
		t.Errorf("expected ID ticket-001, got %q", a.ID)
	}
	if a.Status != StatusPending {
		t.Errorf("expected Status pending, got %q", a.Status)
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

func TestTicketLifecycleOrder(t *testing.T) {
	lifecycle := []Status{
		StatusPending,
		StatusInProgress,
		StatusVerifying,
		StatusResolved,
		StatusClosed,
	}
	seen := make(map[Status]bool)
	for _, s := range lifecycle {
		if seen[s] {
			t.Errorf("lifecycle has duplicate status: %q", s)
		}
		seen[s] = true
	}
	if len(lifecycle) != 5 {
		t.Errorf("expected 5 lifecycle steps, got %d", len(lifecycle))
	}
}
