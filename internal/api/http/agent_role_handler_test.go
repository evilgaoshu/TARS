package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tars/internal/foundation/config"
	"tars/internal/modules/agentrole"
)

func TestAgentRolesCreateAcceptsStructuredModelBinding(t *testing.T) {
	t.Parallel()

	manager, err := agentrole.NewManager("", agentrole.Options{})
	if err != nil {
		t.Fatalf("new agent role manager: %v", err)
	}

	deps := Dependencies{
		Config:     config.Config{OpsAPI: config.OpsAPIConfig{Enabled: true, Token: "ops-token"}},
		AgentRoles: manager,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent-roles", strings.NewReader(`{
		"role_id":"custom-diagnosis",
		"display_name":"Custom Diagnosis",
		"status":"active",
		"profile":{"system_prompt":"diagnose"},
		"capability_binding":{"mode":"unrestricted"},
		"policy_binding":{"max_risk_level":"warning","max_action":"require_approval"},
		"model_binding":{
			"primary":{"provider_id":"openai-main","model":"gpt-4.1-mini"},
			"fallback":{"provider_id":"openai-backup","model":"gpt-4o-mini"},
			"inherit_platform_default":false
		}
	}`))
	req.Header.Set("Authorization", "Bearer ops-token")
	rec := httptest.NewRecorder()

	agentRolesListHandler(deps).ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected created response, got %d body=%s", rec.Code, rec.Body.String())
	}

	role, err := manager.Get("custom-diagnosis")
	if err != nil {
		t.Fatalf("get saved role: %v", err)
	}
	if role.ModelBinding.Primary == nil || role.ModelBinding.Primary.ProviderID != "openai-main" || role.ModelBinding.Primary.Model != "gpt-4.1-mini" {
		t.Fatalf("expected primary model binding to persist, got %+v", role.ModelBinding)
	}
	if role.ModelBinding.Fallback == nil || role.ModelBinding.Fallback.ProviderID != "openai-backup" || role.ModelBinding.Fallback.Model != "gpt-4o-mini" {
		t.Fatalf("expected fallback model binding to persist, got %+v", role.ModelBinding)
	}
	if role.ModelBinding.InheritPlatformDefault {
		t.Fatalf("expected inherit_platform_default=false, got %+v", role.ModelBinding)
	}
}

func TestAgentRolesUpdateAcceptsStructuredModelBinding(t *testing.T) {
	t.Parallel()

	manager, err := agentrole.NewManager("", agentrole.Options{})
	if err != nil {
		t.Fatalf("new agent role manager: %v", err)
	}

	deps := Dependencies{
		Config:     config.Config{OpsAPI: config.OpsAPIConfig{Enabled: true, Token: "ops-token"}},
		AgentRoles: manager,
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/agent-roles/diagnosis", strings.NewReader(`{
		"display_name":"Diagnosis Expert",
		"description":"Updated binding",
		"status":"active",
		"profile":{"system_prompt":"diagnose"},
		"capability_binding":{"mode":"whitelist","allowed_connector_capabilities":["metrics.query_instant"]},
		"policy_binding":{"max_risk_level":"warning","max_action":"require_approval"},
		"model_binding":{
			"primary":{"provider_id":"claude-main","model":"claude-3-7-sonnet"},
			"inherit_platform_default":false
		}
	}`))
	req.Header.Set("Authorization", "Bearer ops-token")
	rec := httptest.NewRecorder()

	agentRoleDetailHandler(deps, "diagnosis").ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected ok response, got %d body=%s", rec.Code, rec.Body.String())
	}

	role, err := manager.Get("diagnosis")
	if err != nil {
		t.Fatalf("get updated role: %v", err)
	}
	if role.ModelBinding.Primary == nil || role.ModelBinding.Primary.ProviderID != "claude-main" || role.ModelBinding.Primary.Model != "claude-3-7-sonnet" {
		t.Fatalf("expected updated primary model binding, got %+v", role.ModelBinding)
	}
}

func TestAgentRolesCreateRejectsLegacyProviderPreference(t *testing.T) {
	t.Parallel()

	manager, err := agentrole.NewManager("", agentrole.Options{})
	if err != nil {
		t.Fatalf("new agent role manager: %v", err)
	}

	deps := Dependencies{
		Config:     config.Config{OpsAPI: config.OpsAPIConfig{Enabled: true, Token: "ops-token"}},
		AgentRoles: manager,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent-roles", strings.NewReader(`{
		"role_id":"legacy-role",
		"display_name":"Legacy Role",
		"status":"active",
		"profile":{"system_prompt":"diagnose"},
		"capability_binding":{"mode":"unrestricted"},
		"policy_binding":{"max_risk_level":"warning","max_action":"require_approval"},
		"provider_preference":{"preferred_provider_id":"openai-main","preferred_model_role":"primary"}
	}`))
	req.Header.Set("Authorization", "Bearer ops-token")
	rec := httptest.NewRecorder()

	agentRolesListHandler(deps).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for legacy provider_preference, got %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "created") {
		t.Fatalf("expected rejection body, got %s", rec.Body.String())
	}
}

func TestAgentRolesCreateRejectsPartialPrimaryModelBinding(t *testing.T) {
	t.Parallel()

	manager, err := agentrole.NewManager("", agentrole.Options{})
	if err != nil {
		t.Fatalf("new agent role manager: %v", err)
	}

	deps := Dependencies{
		Config:     config.Config{OpsAPI: config.OpsAPIConfig{Enabled: true, Token: "ops-token"}},
		AgentRoles: manager,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent-roles", strings.NewReader(`{
		"role_id":"invalid-primary",
		"display_name":"Invalid Primary",
		"status":"active",
		"profile":{"system_prompt":"diagnose"},
		"capability_binding":{"mode":"unrestricted"},
		"policy_binding":{"max_risk_level":"warning","max_action":"require_approval"},
		"model_binding":{
			"primary":{"provider_id":"openai-main"},
			"inherit_platform_default":false
		}
	}`))
	req.Header.Set("Authorization", "Bearer ops-token")
	rec := httptest.NewRecorder()

	agentRolesListHandler(deps).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for partial primary model binding, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAgentRolesCreateRejectsFallbackWithoutPrimaryOrPlatformInheritance(t *testing.T) {
	t.Parallel()

	manager, err := agentrole.NewManager("", agentrole.Options{})
	if err != nil {
		t.Fatalf("new agent role manager: %v", err)
	}

	deps := Dependencies{
		Config:     config.Config{OpsAPI: config.OpsAPIConfig{Enabled: true, Token: "ops-token"}},
		AgentRoles: manager,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/agent-roles", strings.NewReader(`{
		"role_id":"invalid-fallback-only",
		"display_name":"Invalid Fallback Only",
		"status":"active",
		"profile":{"system_prompt":"diagnose"},
		"capability_binding":{"mode":"unrestricted"},
		"policy_binding":{"max_risk_level":"warning","max_action":"require_approval"},
		"model_binding":{
			"fallback":{"provider_id":"openai-backup","model":"gpt-4o-mini"},
			"inherit_platform_default":false
		}
	}`))
	req.Header.Set("Authorization", "Bearer ops-token")
	rec := httptest.NewRecorder()

	agentRolesListHandler(deps).ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request for fallback-only model binding without inheritance, got %d body=%s", rec.Code, rec.Body.String())
	}
}
