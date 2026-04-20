package org

import (
	"testing"
	"time"
)

func TestManagerDefaults_NoPath(t *testing.T) {
	m, err := NewManager("")
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	orgs := m.ListOrganizations()
	if len(orgs) != 1 {
		t.Fatalf("expected 1 default org, got %d", len(orgs))
	}
	if orgs[0].ID != DefaultOrgID {
		t.Errorf("expected org id %q, got %q", DefaultOrgID, orgs[0].ID)
	}

	tenants := m.ListTenants("")
	if len(tenants) != 1 {
		t.Fatalf("expected 1 default tenant, got %d", len(tenants))
	}
	workspaces := m.ListWorkspaces("")
	if len(workspaces) != 1 {
		t.Fatalf("expected 1 default workspace, got %d", len(workspaces))
	}
}

func TestManagerCRUD(t *testing.T) {
	m, _ := NewManager("")

	// Create org
	org, err := m.CreateOrganization(Organization{
		ID: "acme", Name: "Acme Corp", Slug: "acme",
	})
	if err != nil {
		t.Fatalf("CreateOrganization: %v", err)
	}
	if org.Status != "active" {
		t.Errorf("expected status active, got %q", org.Status)
	}
	if org.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}

	// Get org
	got, err := m.GetOrganization("acme")
	if err != nil {
		t.Fatalf("GetOrganization: %v", err)
	}
	if got.Name != "Acme Corp" {
		t.Errorf("expected name %q, got %q", "Acme Corp", got.Name)
	}

	// Duplicate rejects
	_, err = m.CreateOrganization(Organization{ID: "acme"})
	if err == nil {
		t.Error("expected error for duplicate org id")
	}

	// Update org
	org.Name = "Acme Inc"
	updated, err := m.UpdateOrganization("acme", org)
	if err != nil {
		t.Fatalf("UpdateOrganization: %v", err)
	}
	if updated.Name != "Acme Inc" {
		t.Errorf("expected %q, got %q", "Acme Inc", updated.Name)
	}

	// Set status
	_, err = m.SetOrganizationStatus("acme", "disabled")
	if err != nil {
		t.Fatalf("SetOrganizationStatus: %v", err)
	}
	got2, _ := m.GetOrganization("acme")
	if got2.Status != "disabled" {
		t.Errorf("expected disabled, got %q", got2.Status)
	}

	// Tenant CRUD
	tenant, err := m.CreateTenant(Tenant{ID: "acme-prod", OrgID: "acme", Name: "Production"})
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if tenant.Status != "active" {
		t.Errorf("expected active, got %q", tenant.Status)
	}

	tenants := m.ListTenants("acme")
	if len(tenants) != 1 {
		t.Errorf("expected 1 tenant for acme, got %d", len(tenants))
	}

	// Workspace CRUD
	ws, err := m.CreateWorkspace(Workspace{
		ID: "acme-prod-main", TenantID: "acme-prod", OrgID: "acme", Name: "Main",
	})
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if ws.Status != "active" {
		t.Errorf("expected active, got %q", ws.Status)
	}

	got3, err := m.GetWorkspace("acme-prod-main")
	if err != nil {
		t.Fatalf("GetWorkspace: %v", err)
	}
	if got3.TenantID != "acme-prod" {
		t.Errorf("expected tenant_id acme-prod, got %q", got3.TenantID)
	}

	// Unknown ID errors
	_, err = m.GetOrganization("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent org")
	}
	_, err = m.GetTenant("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent tenant")
	}
	_, err = m.GetWorkspace("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent workspace")
	}

	// Empty ID rejects
	_, err = m.CreateOrganization(Organization{ID: ""})
	if err == nil {
		t.Error("expected error for empty org id")
	}
	_, err = m.CreateTenant(Tenant{ID: "  "})
	if err == nil {
		t.Error("expected error for blank tenant id")
	}
	_, err = m.CreateWorkspace(Workspace{ID: ""})
	if err == nil {
		t.Error("expected error for empty workspace id")
	}
}

func TestManagerDefaults_WithPath_MissingFile(t *testing.T) {
	tmpPath := t.TempDir() + "/nonexistent/org.yaml"
	m, err := NewManager(tmpPath)
	if err != nil {
		t.Fatalf("NewManager with missing path: %v", err)
	}
	// Should have booted defaults
	orgs := m.ListOrganizations()
	if len(orgs) == 0 {
		t.Fatal("expected at least 1 default org when config file is missing")
	}
}

func TestManagerSetWorkspaceStatus(t *testing.T) {
	m, _ := NewManager("")
	ws, err := m.SetWorkspaceStatus(DefaultWorkspaceID, "disabled")
	if err != nil {
		t.Fatalf("SetWorkspaceStatus: %v", err)
	}
	if ws.Status != "disabled" {
		t.Errorf("expected disabled, got %q", ws.Status)
	}
}

func TestManagerDefaultHelpers(t *testing.T) {
	m, _ := NewManager("")
	org := m.DefaultOrg()
	if org.ID == "" {
		t.Error("DefaultOrg should not return empty ID")
	}
	tenant := m.DefaultTenant()
	if tenant.ID == "" {
		t.Error("DefaultTenant should not return empty ID")
	}
	ws := m.DefaultWorkspace()
	if ws.ID == "" {
		t.Error("DefaultWorkspace should not return empty ID")
	}
	_ = m.UpdatedAt().Add(time.Second)
}
