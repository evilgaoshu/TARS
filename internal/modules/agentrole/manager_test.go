package agentrole

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuiltinRolesExist(t *testing.T) {
	m, err := NewManager("", Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	expected := []string{"diagnosis", "automation_operator", "reviewer", "knowledge_curator"}
	for _, id := range expected {
		r, err := m.Get(id)
		if err != nil {
			t.Errorf("Get(%q): %v", id, err)
			continue
		}
		if r.Status != "active" {
			t.Errorf("Get(%q).Status = %q, want active", id, r.Status)
		}
		if !r.IsBuiltin {
			t.Errorf("Get(%q).IsBuiltin = false, want true", id)
		}
	}
}

func TestCRUDLifecycle(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent_roles.yaml")

	m, err := NewManager(path, Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Verify built-in roles were persisted
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not created: %v", err)
	}

	// Create a custom role
	custom := AgentRole{
		RoleID:      "custom_test",
		DisplayName: "Custom Test Role",
		Description: "A test role",
		Profile: Profile{
			SystemPrompt: "Test prompt",
			PersonaTags:  []string{"test"},
		},
		CapabilityBinding: CapabilityBinding{
			Mode: "unrestricted",
		},
		PolicyBinding: PolicyBinding{
			MaxRiskLevel: "info",
			MaxAction:    "suggest_only",
		},
	}
	created, err := m.Create(custom)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.RoleID != "custom_test" {
		t.Errorf("created.RoleID = %q, want custom_test", created.RoleID)
	}
	if created.Status != "active" {
		t.Errorf("created.Status = %q, want active", created.Status)
	}
	if created.CreatedAt.IsZero() {
		t.Error("created.CreatedAt is zero")
	}

	// Verify list includes builtin + custom
	all := m.List()
	if len(all) != 5 { // 4 builtin + 1 custom
		t.Errorf("List() len = %d, want 5", len(all))
	}

	// Update the custom role
	custom.DisplayName = "Updated Name"
	updated, err := m.Update(custom)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.DisplayName != "Updated Name" {
		t.Errorf("updated.DisplayName = %q, want Updated Name", updated.DisplayName)
	}

	// Disable
	disabled, err := m.SetEnabled("custom_test", false)
	if err != nil {
		t.Fatalf("SetEnabled(false): %v", err)
	}
	if disabled.Status != "disabled" {
		t.Errorf("disabled.Status = %q, want disabled", disabled.Status)
	}

	// Enable
	enabled, err := m.SetEnabled("custom_test", true)
	if err != nil {
		t.Fatalf("SetEnabled(true): %v", err)
	}
	if enabled.Status != "active" {
		t.Errorf("enabled.Status = %q, want active", enabled.Status)
	}

	// Delete custom role
	if err := m.Delete("custom_test"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := m.Get("custom_test"); err != ErrRoleNotFound {
		t.Errorf("Get after delete: got %v, want ErrRoleNotFound", err)
	}

	// Cannot delete builtin
	if err := m.Delete("diagnosis"); err != ErrRoleIsBuiltin {
		t.Errorf("Delete builtin: got %v, want ErrRoleIsBuiltin", err)
	}

	// Cannot create duplicate
	if _, err := m.Create(AgentRole{RoleID: "diagnosis"}); err != ErrRoleIDConflict {
		t.Errorf("Create duplicate: got %v, want ErrRoleIDConflict", err)
	}

	// Role ID required
	if _, err := m.Create(AgentRole{}); err != ErrRoleIDRequired {
		t.Errorf("Create empty ID: got %v, want ErrRoleIDRequired", err)
	}
}

func TestResolveForSession(t *testing.T) {
	m, err := NewManager("", Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Empty role ID falls back to diagnosis
	r := m.ResolveForSession("")
	if r.RoleID != "diagnosis" {
		t.Errorf("ResolveForSession(\"\") = %q, want diagnosis", r.RoleID)
	}

	// Unknown role ID falls back to diagnosis
	r = m.ResolveForSession("nonexistent")
	if r.RoleID != "diagnosis" {
		t.Errorf("ResolveForSession(nonexistent) = %q, want diagnosis", r.RoleID)
	}

	// Known role ID
	r = m.ResolveForSession("reviewer")
	if r.RoleID != "reviewer" {
		t.Errorf("ResolveForSession(reviewer) = %q, want reviewer", r.RoleID)
	}
}

func TestResolveForAutomation(t *testing.T) {
	m, err := NewManager("", Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	r := m.ResolveForAutomation("")
	if r.RoleID != "automation_operator" {
		t.Errorf("ResolveForAutomation(\"\") = %q, want automation_operator", r.RoleID)
	}

	r = m.ResolveForAutomation("diagnosis")
	if r.RoleID != "diagnosis" {
		t.Errorf("ResolveForAutomation(diagnosis) = %q, want diagnosis", r.RoleID)
	}
}

func TestReloadFromDisk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent_roles.yaml")

	m1, err := NewManager(path, Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	// Create a custom role with m1
	_, err = m1.Create(AgentRole{
		RoleID:            "persisted_role",
		DisplayName:       "Persisted",
		CapabilityBinding: CapabilityBinding{Mode: "unrestricted"},
		PolicyBinding:     PolicyBinding{MaxRiskLevel: "info", MaxAction: "suggest_only"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Load with a new manager instance
	m2, err := NewManager(path, Options{})
	if err != nil {
		t.Fatalf("NewManager(reload): %v", err)
	}
	r, err := m2.Get("persisted_role")
	if err != nil {
		t.Fatalf("Get(persisted_role) after reload: %v", err)
	}
	if r.DisplayName != "Persisted" {
		t.Errorf("r.DisplayName = %q, want Persisted", r.DisplayName)
	}
}

func TestSnapshot(t *testing.T) {
	m, err := NewManager("", Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	snap := m.Snapshot()
	if !snap.Loaded {
		t.Error("Snapshot.Loaded = false, want true")
	}
	if len(snap.Config.AgentRoles) != 4 {
		t.Errorf("Snapshot roles count = %d, want 4", len(snap.Config.AgentRoles))
	}
}

func TestSortedRoles(t *testing.T) {
	roles := []AgentRole{
		{RoleID: "z", DisplayName: "Zulu"},
		{RoleID: "a", DisplayName: "Alpha"},
		{RoleID: "m", DisplayName: "Mike"},
	}
	sorted := SortedRoles(roles)
	if sorted[0].RoleID != "a" || sorted[1].RoleID != "m" || sorted[2].RoleID != "z" {
		t.Errorf("SortedRoles order incorrect: %v", sorted)
	}
}

func TestReloadFailsFastOnLegacyProviderPreferenceKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent_roles.yaml")
	raw := `agent_roles:
  agent_roles:
    - role_id: legacy
      display_name: Legacy
      status: active
      profile:
        system_prompt: diagnose
      capability_binding:
        mode: unrestricted
      policy_binding:
        max_risk_level: warning
        max_action: require_approval
      provider_preference:
        preferred_provider_id: openai-main
        preferred_model_role: primary
`
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := NewManager(path, Options{})
	if err == nil {
		t.Fatal("expected fail-fast error for legacy provider_preference")
	}
	if !strings.Contains(err.Error(), "provider_preference") {
		t.Fatalf("expected provider_preference failure, got %v", err)
	}
}

func TestCreateRejectsInvalidModelBinding(t *testing.T) {
	m, err := NewManager("", Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	_, err = m.Create(AgentRole{
		RoleID:            "invalid-binding",
		DisplayName:       "Invalid Binding",
		CapabilityBinding: CapabilityBinding{Mode: "unrestricted"},
		PolicyBinding:     PolicyBinding{MaxRiskLevel: "warning", MaxAction: "require_approval"},
		ModelBinding: ModelBinding{
			Primary: &ModelTargetBinding{ProviderID: "openai-main"},
		},
	})
	if err == nil {
		t.Fatal("expected invalid partial primary model binding to be rejected")
	}
	if !strings.Contains(err.Error(), "primary.model") {
		t.Fatalf("expected primary.model validation error, got %v", err)
	}

	_, err = m.Create(AgentRole{
		RoleID:            "invalid-fallback-only",
		DisplayName:       "Invalid Fallback Only",
		CapabilityBinding: CapabilityBinding{Mode: "unrestricted"},
		PolicyBinding:     PolicyBinding{MaxRiskLevel: "warning", MaxAction: "require_approval"},
		ModelBinding: ModelBinding{
			Fallback: &ModelTargetBinding{ProviderID: "openai-backup", Model: "gpt-4o-mini"},
		},
	})
	if err == nil {
		t.Fatal("expected fallback-only model binding to be rejected")
	}
	if !strings.Contains(err.Error(), "fallback requires either primary or inherit_platform_default=true") {
		t.Fatalf("expected fallback validation error, got %v", err)
	}
}
