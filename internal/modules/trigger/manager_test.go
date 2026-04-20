package trigger

import (
	"context"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func newMemManager() *Manager {
	return NewManager(nil)
}

func boolPtr(v bool) *bool { return &v }

// ---------------------------------------------------------------------------
// seedDefaults
// ---------------------------------------------------------------------------

func TestSeedDefaults_SixBuiltInTriggers(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	triggers, err := m.List(context.Background(), ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(triggers) != 6 {
		t.Fatalf("expected 6 default triggers, got %d", len(triggers))
	}
}

func TestSeedDefaults_AllEnabled(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	triggers, _ := m.List(context.Background(), ListFilter{})
	for _, tr := range triggers {
		if !tr.Enabled {
			t.Errorf("expected trigger %s to be enabled by default", tr.ID)
		}
	}
}

func TestSeedDefaults_AllPointToInAppInbox(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	triggers, _ := m.List(context.Background(), ListFilter{})
	for _, tr := range triggers {
		if tr.ChannelID != "in_app_inbox" {
			t.Errorf("expected trigger %s channel_id=in_app_inbox, got %q", tr.ID, tr.ChannelID)
		}
	}
}

// ---------------------------------------------------------------------------
// MatchEnabled
// ---------------------------------------------------------------------------

func TestMatchEnabled_ReturnsEnabledForEventType(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	matches, err := m.MatchEnabled(context.Background(), "default", EventOnSkillCompleted)
	if err != nil {
		t.Fatalf("MatchEnabled: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected 1 match for on_skill_completed, got %d", len(matches))
	}
	if matches[0].EventType != EventOnSkillCompleted {
		t.Fatalf("unexpected event type: %s", matches[0].EventType)
	}
}

func TestMatchEnabled_DisabledTriggerNotReturned(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	// Disable the skill_completed trigger
	_, err := m.SetEnabled(context.Background(), "trg_skill_completed", false)
	if err != nil {
		t.Fatalf("SetEnabled: %v", err)
	}
	matches, err := m.MatchEnabled(context.Background(), "default", EventOnSkillCompleted)
	if err != nil {
		t.Fatalf("MatchEnabled: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches for disabled trigger, got %d", len(matches))
	}
}

func TestMatchEnabled_UnknownEventTypeReturnsEmpty(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	matches, err := m.MatchEnabled(context.Background(), "default", "on_unknown_event")
	if err != nil {
		t.Fatalf("MatchEnabled: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(matches))
	}
}

func TestMatchEnabled_AudienceRuleRequiresMatchingAudience(t *testing.T) {
	t.Parallel()
	m := newMemManager()

	_, err := m.Upsert(context.Background(), Trigger{
		ID:             "trg_audience_only",
		TenantID:       "default",
		DisplayName:    "Audience only",
		Enabled:        true,
		EventType:      EventOnExecutionCompleted,
		TargetAudience: "ops.leads",
		ChannelID:      "telegram",
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	matches, err := m.MatchEnabled(context.Background(), MatchRequest{
		TenantID:       "default",
		EventType:      EventOnExecutionCompleted,
		TargetAudience: "ops.engineering",
	})
	if err != nil {
		t.Fatalf("MatchEnabled: %v", err)
	}
	for _, match := range matches {
		if match.ID == "trg_audience_only" {
			t.Fatalf("expected audience-mismatched trigger to be filtered out, got %+v", match)
		}
	}

	matches, err = m.MatchEnabled(context.Background(), MatchRequest{
		TenantID:       "default",
		EventType:      EventOnExecutionCompleted,
		TargetAudience: "ops.leads",
	})
	if err != nil {
		t.Fatalf("MatchEnabled: %v", err)
	}
	found := false
	for _, match := range matches {
		if match.ID == "trg_audience_only" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected audience-matched trigger to be returned")
	}
}

func TestMatchEnabled_FilterExprMatchesEventData(t *testing.T) {
	t.Parallel()
	m := newMemManager()

	_, err := m.Upsert(context.Background(), Trigger{
		ID:          "trg_filter_expr",
		TenantID:    "default",
		DisplayName: "Filter expr",
		Enabled:     true,
		EventType:   EventOnApprovalRequested,
		FilterExpr:  "risk_level == 'critical'",
		ChannelID:   "telegram",
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	matches, err := m.MatchEnabled(context.Background(), MatchRequest{
		TenantID:  "default",
		EventType: EventOnApprovalRequested,
		EventData: map[string]string{"risk_level": "warning"},
	})
	if err != nil {
		t.Fatalf("MatchEnabled: %v", err)
	}
	for _, match := range matches {
		if match.ID == "trg_filter_expr" {
			t.Fatalf("expected non-matching filter_expr trigger to be filtered out, got %+v", match)
		}
	}

	matches, err = m.MatchEnabled(context.Background(), MatchRequest{
		TenantID:  "default",
		EventType: EventOnApprovalRequested,
		EventData: map[string]string{"risk_level": "critical"},
	})
	if err != nil {
		t.Fatalf("MatchEnabled: %v", err)
	}
	found := false
	for _, match := range matches {
		if match.ID == "trg_filter_expr" {
			found = true
		}
	}
	if !found {
		t.Fatal("expected filter_expr-matched trigger to be returned")
	}
}

func TestMatchEnabled_InvalidFilterExprSkipsTrigger(t *testing.T) {
	t.Parallel()
	m := newMemManager()

	_, err := m.Upsert(context.Background(), Trigger{
		ID:          "trg_invalid_filter_expr",
		TenantID:    "default",
		DisplayName: "Invalid filter expr",
		Enabled:     true,
		EventType:   EventOnApprovalRequested,
		FilterExpr:  "risk_level ~= critical",
		ChannelID:   "telegram",
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	matches, err := m.MatchEnabled(context.Background(), MatchRequest{
		TenantID:  "default",
		EventType: EventOnApprovalRequested,
		EventData: map[string]string{"risk_level": "critical"},
	})
	if err != nil {
		t.Fatalf("MatchEnabled: %v", err)
	}
	for _, match := range matches {
		if match.ID == "trg_invalid_filter_expr" {
			t.Fatalf("expected invalid filter_expr trigger to be skipped, got %+v", match)
		}
	}
}

func TestUpsert_PreservesChannelID(t *testing.T) {
	t.Parallel()
	m := newMemManager()

	saved, err := m.Upsert(context.Background(), Trigger{
		ID:          "trg_custom_channel_ref",
		TenantID:    "default",
		DisplayName: "Channel ref compatibility",
		Enabled:     true,
		EventType:   EventOnSkillCompleted,
		ChannelID:   "inbox-primary",
	})
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if saved.ChannelID != "inbox-primary" {
		t.Fatalf("expected channel_id to be preserved, got %q", saved.ChannelID)
	}

	stored, err := m.Get(context.Background(), saved.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.ChannelID != "inbox-primary" {
		t.Fatalf("expected stored channel_id to remain persisted, got %q", stored.ChannelID)
	}
}

// ---------------------------------------------------------------------------
// IsOnCooldown
// ---------------------------------------------------------------------------

func TestIsOnCooldown_NoCooldown(t *testing.T) {
	t.Parallel()
	tr := Trigger{CooldownSec: 0, LastFiredAt: time.Now()}
	if IsOnCooldown(tr, time.Now()) {
		t.Fatal("expected no cooldown when CooldownSec=0")
	}
}

func TestIsOnCooldown_NotFiredYet(t *testing.T) {
	t.Parallel()
	tr := Trigger{CooldownSec: 60} // LastFiredAt is zero
	if IsOnCooldown(tr, time.Now()) {
		t.Fatal("expected no cooldown when LastFiredAt is zero")
	}
}

func TestIsOnCooldown_StillWithin(t *testing.T) {
	t.Parallel()
	firedAt := time.Now().Add(-30 * time.Second)
	tr := Trigger{CooldownSec: 60, LastFiredAt: firedAt}
	if !IsOnCooldown(tr, time.Now()) {
		t.Fatal("expected cooldown to be active (30s elapsed, 60s cooldown)")
	}
}

func TestIsOnCooldown_Expired(t *testing.T) {
	t.Parallel()
	firedAt := time.Now().Add(-120 * time.Second)
	tr := Trigger{CooldownSec: 60, LastFiredAt: firedAt}
	if IsOnCooldown(tr, time.Now()) {
		t.Fatal("expected cooldown to be expired (120s elapsed, 60s cooldown)")
	}
}

// ---------------------------------------------------------------------------
// Upsert (in-memory)
// ---------------------------------------------------------------------------

func TestUpsert_Create(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	tr := Trigger{
		ID:        "trg_custom",
		TenantID:  "acme",
		EventType: "on_custom_event",
		ChannelID: "email",
		Enabled:   true,
	}
	got, err := m.Upsert(context.Background(), tr)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if got.ID != "trg_custom" {
		t.Fatalf("unexpected ID: %s", got.ID)
	}
	if got.TenantID != "acme" {
		t.Fatalf("unexpected TenantID: %s", got.TenantID)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("expected UpdatedAt to be set")
	}
}

func TestUpsert_GeneratesIDIfEmpty(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	tr := Trigger{EventType: "on_custom_event", ChannelID: "email"}
	got, err := m.Upsert(context.Background(), tr)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if got.ID == "" {
		t.Fatal("expected a generated ID")
	}
}

func TestUpsert_DefaultsTenantID(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	tr := Trigger{EventType: "on_custom_event", ChannelID: "email"}
	got, err := m.Upsert(context.Background(), tr)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if got.TenantID != "default" {
		t.Fatalf("expected TenantID=default, got %q", got.TenantID)
	}
}

func TestUpsert_UpdateExisting(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	// Update an existing built-in trigger
	original, err := m.Get(context.Background(), "trg_skill_completed")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	original.CooldownSec = 300
	updated, err := m.Upsert(context.Background(), original)
	if err != nil {
		t.Fatalf("Upsert update: %v", err)
	}
	if updated.CooldownSec != 300 {
		t.Fatalf("expected CooldownSec=300, got %d", updated.CooldownSec)
	}
}

// ---------------------------------------------------------------------------
// Get
// ---------------------------------------------------------------------------

func TestGet_NotFound(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	_, err := m.Get(context.Background(), "does-not-exist")
	if err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	}
}

// ---------------------------------------------------------------------------
// List with filter
// ---------------------------------------------------------------------------

func TestList_FilterByEventType(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	results, err := m.List(context.Background(), ListFilter{EventType: EventOnApprovalRequested})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].EventType != EventOnApprovalRequested {
		t.Fatalf("unexpected event type: %s", results[0].EventType)
	}
}

func TestList_FilterByEnabled_False(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	// Disable one trigger
	_, _ = m.SetEnabled(context.Background(), "trg_session_closed", false)
	results, err := m.List(context.Background(), ListFilter{Enabled: boolPtr(false)})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected exactly 1 disabled trigger, got %d", len(results))
	}
	if results[0].ID != "trg_session_closed" {
		t.Fatalf("unexpected disabled trigger ID: %s", results[0].ID)
	}
}

func TestList_Pagination(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	page1, _ := m.List(context.Background(), ListFilter{Limit: 2, Offset: 0})
	page2, _ := m.List(context.Background(), ListFilter{Limit: 2, Offset: 2})
	if len(page1) != 2 || len(page2) != 2 {
		t.Fatalf("expected 2 items per page, got page1=%d page2=%d", len(page1), len(page2))
	}
	if page1[0].ID == page2[0].ID {
		t.Fatal("pages should not overlap")
	}
}

// ---------------------------------------------------------------------------
// SetEnabled
// ---------------------------------------------------------------------------

func TestSetEnabled_EnableDisable(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	// Disable
	got, err := m.SetEnabled(context.Background(), "trg_approval_requested", false)
	if err != nil || got.Enabled {
		t.Fatalf("expected disabled, err=%v, enabled=%v", err, got.Enabled)
	}
	// Re-enable
	got, err = m.SetEnabled(context.Background(), "trg_approval_requested", true)
	if err != nil || !got.Enabled {
		t.Fatalf("expected enabled, err=%v, enabled=%v", err, got.Enabled)
	}
}

func TestSetEnabled_NotFound(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	_, err := m.SetEnabled(context.Background(), "no-such-trigger", true)
	if err == nil {
		t.Fatal("expected error for missing trigger")
	}
}

// ---------------------------------------------------------------------------
// RecordFired
// ---------------------------------------------------------------------------

func TestRecordFired_UpdatesLastFiredAt(t *testing.T) {
	t.Parallel()
	m := newMemManager()
	before := time.Now().Add(-time.Second)
	err := m.RecordFired(context.Background(), "trg_skill_completed")
	if err != nil {
		t.Fatalf("RecordFired: %v", err)
	}
	got, err := m.Get(context.Background(), "trg_skill_completed")
	if err != nil {
		t.Fatalf("Get after RecordFired: %v", err)
	}
	if !got.LastFiredAt.After(before) {
		t.Fatalf("expected LastFiredAt after %v, got %v", before, got.LastFiredAt)
	}
}
