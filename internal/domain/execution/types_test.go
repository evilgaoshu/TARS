package execution

import "testing"

func TestStatusConstants(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected string
	}{
		{"pending", StatusPending, "pending"},
		{"approved", StatusApproved, "approved"},
		{"executing", StatusExecuting, "executing"},
		{"completed", StatusCompleted, "completed"},
		{"failed", StatusFailed, "failed"},
		{"timeout", StatusTimeout, "timeout"},
		{"rejected", StatusRejected, "rejected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, string(tt.status))
			}
		})
	}
}

func TestStatusAllDefined(t *testing.T) {
	// Ensure no two statuses share the same string value.
	all := []Status{
		StatusPending, StatusApproved, StatusExecuting,
		StatusCompleted, StatusFailed, StatusTimeout, StatusRejected,
	}
	seen := make(map[Status]bool)
	for _, s := range all {
		if seen[s] {
			t.Errorf("duplicate status value: %q", s)
		}
		seen[s] = true
	}
}

func TestRequestFields(t *testing.T) {
	r := Request{
		ID:         "req-001",
		SessionID:  "sess-abc",
		TargetHost: "host-1",
		Command:    "systemctl restart api",
		Status:     StatusPending,
	}

	if r.ID != "req-001" {
		t.Errorf("expected ID req-001, got %q", r.ID)
	}
	if r.SessionID != "sess-abc" {
		t.Errorf("expected SessionID sess-abc, got %q", r.SessionID)
	}
	if r.TargetHost != "host-1" {
		t.Errorf("expected TargetHost host-1, got %q", r.TargetHost)
	}
	if r.Command != "systemctl restart api" {
		t.Errorf("expected Command 'systemctl restart api', got %q", r.Command)
	}
	if r.Status != StatusPending {
		t.Errorf("expected Status pending, got %q", r.Status)
	}
}

func TestRequestZeroValue(t *testing.T) {
	var r Request
	if r.ID != "" {
		t.Errorf("expected empty ID, got %q", r.ID)
	}
	if r.Status != "" {
		t.Errorf("expected empty Status, got %q", r.Status)
	}
}

func TestStatusTransitions(t *testing.T) {
	// Validate that the expected terminal and non-terminal states are correct.
	terminalStates := map[Status]bool{
		StatusCompleted: true,
		StatusFailed:    true,
		StatusTimeout:   true,
		StatusRejected:  true,
	}
	activeStates := map[Status]bool{
		StatusPending:   true,
		StatusApproved:  true,
		StatusExecuting: true,
	}

	for s := range terminalStates {
		if activeStates[s] {
			t.Errorf("status %q should not be both terminal and active", s)
		}
	}
	for s := range activeStates {
		if terminalStates[s] {
			t.Errorf("status %q should not be both active and terminal", s)
		}
	}
}

func TestStatusEquality(t *testing.T) {
	s := StatusPending
	if s != StatusPending {
		t.Error("same status should be equal")
	}
	if s == StatusCompleted {
		t.Error("different statuses should not be equal")
	}
}
