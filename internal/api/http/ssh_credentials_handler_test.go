package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tars/internal/foundation/audit"
	"tars/internal/foundation/config"
	"tars/internal/modules/sshcredentials"
)

func TestSSHCredentialAPIDoesNotReturnSecretMaterial(t *testing.T) {
	deps := Dependencies{
		Config: config.Config{OpsAPI: config.OpsAPIConfig{Enabled: true, Token: "local-test-token"}},
		SSHCredentials: sshcredentials.NewManager(
			sshcredentials.NewMemoryRepository(),
			sshcredentials.NewMemorySecretBackend(),
		),
	}
	body := []byte(`{
		"credential_id":"ops-key",
		"display_name":"Ops key",
		"connector_id":"ssh-main",
		"username":"root",
		"auth_type":"password",
		"password":"super-secret-password",
		"host_scope":"192.168.3.100",
		"operator_reason":"test"
	}`)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ssh-credentials", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer local-test-token")
	sshCredentialsListHandler(deps)(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertSSHCredentialResponseIsMetadataOnly(t, rec.Body.Bytes(), "super-secret-password")
	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created["status"] != sshcredentials.StatusActive {
		t.Fatalf("unexpected created metadata: %#v", created)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/ssh-credentials/ops-key", nil)
	req.Header.Set("Authorization", "Bearer local-test-token")
	sshCredentialDetailHandler(deps, "ops-key")(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 detail response, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertSSHCredentialResponseIsMetadataOnly(t, rec.Body.Bytes(), "super-secret-password")

	updateBody := []byte(`{
		"display_name":"Ops key rotated",
		"username":"admin",
		"password":"rotated-secret-password",
		"operator_reason":"rotate"
	}`)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/v1/ssh-credentials/ops-key", bytes.NewReader(updateBody))
	req.Header.Set("Authorization", "Bearer local-test-token")
	sshCredentialDetailHandler(deps, "ops-key")(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 update response, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertSSHCredentialResponseIsMetadataOnly(t, rec.Body.Bytes(), "rotated-secret-password")

	statusBody := []byte(`{"operator_reason":"rotate now"}`)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/ssh-credentials/ops-key/rotation-required", bytes.NewReader(statusBody))
	req.Header.Set("Authorization", "Bearer local-test-token")
	sshCredentialRouterHandler(deps)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 rotation_required response, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertSSHCredentialResponseIsMetadataOnly(t, rec.Body.Bytes(), "rotated-secret-password")
	var rotated map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rotated); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if rotated["status"] != sshcredentials.StatusRotationRequired {
		t.Fatalf("expected rotation_required status, got %#v", rotated)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/ssh-credentials", nil)
	req.Header.Set("Authorization", "Bearer local-test-token")
	sshCredentialsListHandler(deps)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertSSHCredentialListHasStatus(t, rec.Body.Bytes(), sshcredentials.StatusRotationRequired, "super-secret-password", "rotated-secret-password")

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/ssh-credentials/ops-key", bytes.NewReader([]byte(`{"operator_reason":"delete"}`)))
	req.Header.Set("Authorization", "Bearer local-test-token")
	sshCredentialDetailHandler(deps, "ops-key")(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 delete response, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertSSHCredentialResponseIsMetadataOnly(t, rec.Body.Bytes(), "rotated-secret-password")
}

func TestSSHCredentialAPIFailsClosedWhenCustodyNotConfigured(t *testing.T) {
	deps := Dependencies{Config: config.Config{OpsAPI: config.OpsAPIConfig{Enabled: true, Token: "local-test-token"}}}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ssh-credentials", strings.NewReader(`{"operator_reason":"test"}`))
	req.Header.Set("Authorization", "Bearer local-test-token")
	sshCredentialsListHandler(deps)(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 when custody backend is missing, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSSHCredentialStatusHandlerSupportsRotationRequiredAuditPath(t *testing.T) {
	logger := &captureSSHCredentialAuditLogger{}
	deps := Dependencies{
		Config: config.Config{OpsAPI: config.OpsAPIConfig{Enabled: true, Token: "local-test-token"}},
		Audit:  logger,
		SSHCredentials: sshcredentials.NewManager(
			sshcredentials.NewMemoryRepository(),
			sshcredentials.NewMemorySecretBackend(),
		),
	}
	if _, err := deps.SSHCredentials.Create(context.Background(), sshcredentials.CreateInput{
		CredentialID:   "ops-key",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       sshcredentials.AuthTypePassword,
		Password:       "super-secret-password",
		OperatorReason: "test",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/ssh-credentials/ops-key/rotation-required", bytes.NewReader([]byte(`{"operator_reason":"rotate now"}`)))
	req.Header.Set("Authorization", "Bearer local-test-token")
	sshCredentialRouterHandler(deps)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	assertSSHCredentialResponseIsMetadataOnly(t, rec.Body.Bytes(), "super-secret-password")
	if len(logger.entries) != 1 {
		t.Fatalf("expected one audit entry, got %+v", logger.entries)
	}
	if logger.entries[0].Action != "ssh_credential.rotation_required" {
		t.Fatalf("expected rotation_required audit action, got %+v", logger.entries[0])
	}
	encoded, err := json.Marshal(logger.entries[0].Metadata)
	if err != nil {
		t.Fatalf("marshal audit metadata: %v", err)
	}
	if strings.Contains(string(encoded), "super-secret-password") {
		t.Fatalf("audit metadata leaked secret material: %s", string(encoded))
	}
}

func assertSSHCredentialResponseIsMetadataOnly(t *testing.T, body []byte, forbiddenValues ...string) {
	t.Helper()
	raw := string(body)
	for _, key := range []string{"\"secret_ref\"", "\"passphrase_secret_ref\""} {
		if strings.Contains(raw, key) {
			t.Fatalf("ssh credential response leaked custody reference %s: %s", key, raw)
		}
	}
	for _, value := range forbiddenValues {
		if strings.Contains(raw, value) {
			t.Fatalf("ssh credential response leaked secret material %q: %s", value, raw)
		}
	}
}

func assertSSHCredentialListIsMetadataOnly(t *testing.T, body []byte, forbiddenValues ...string) {
	t.Helper()
	assertSSHCredentialListHasStatus(t, body, sshcredentials.StatusActive, forbiddenValues...)
}

func assertSSHCredentialListHasStatus(t *testing.T, body []byte, wantStatus string, forbiddenValues ...string) {
	t.Helper()
	assertSSHCredentialResponseIsMetadataOnly(t, body, forbiddenValues...)
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	items, ok := payload["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one list item, got %#v", payload)
	}
	item, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected list item object, got %#v", items[0])
	}
	if item["status"] != wantStatus {
		t.Fatalf("unexpected list item metadata: %#v", item)
	}
}

type captureSSHCredentialAuditLogger struct {
	entries []audit.Entry
}

func (c *captureSSHCredentialAuditLogger) Log(_ context.Context, entry audit.Entry) {
	c.entries = append(c.entries, entry)
}
