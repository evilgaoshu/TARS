package sshcredentials

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"tars/internal/foundation/audit"
)

func TestManagerCreatePrivateKeyCredentialStoresMetadataOnlyAndResolves(t *testing.T) {
	repo := NewMemoryRepository()
	vault := NewMemorySecretBackend()
	mgr := NewManager(repo, vault)

	created, err := mgr.Create(context.Background(), CreateInput{
		CredentialID:   "prod-root",
		DisplayName:    "Production root key",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       AuthTypePrivateKey,
		PrivateKey:     "-----BEGIN OPENSSH PRIVATE KEY-----\nsecret material\n-----END OPENSSH PRIVATE KEY-----",
		Passphrase:     "key-passphrase",
		HostScope:      "192.168.3.100,192.168.3.9",
		OperatorReason: "initial custody",
		ActorID:        "admin",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.SecretRef != "" || created.PassphraseSecretRef != "" {
		t.Fatalf("expected sanitized credential to clear custody refs, got %#v", created)
	}

	stored, ok, err := repo.Get(context.Background(), "prod-root")
	if err != nil || !ok {
		t.Fatalf("repo.Get() found=%v err=%v", ok, err)
	}
	if stored.Status != StatusActive || stored.SecretRef == "" || stored.PassphraseSecretRef == "" {
		t.Fatalf("unexpected stored metadata: %#v", stored)
	}
	if strings.Contains(stored.SecretRef, "secret material") || strings.Contains(stored.PassphraseSecretRef, "key-passphrase") {
		t.Fatalf("metadata must not contain plaintext secret material: %#v", stored)
	}

	resolved, err := mgr.Resolve(context.Background(), "prod-root", "192.168.3.100")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if resolved.PrivateKey == "" || resolved.Passphrase != "key-passphrase" || resolved.Username != "root" {
		t.Fatalf("unexpected resolved credential: %#v", resolved)
	}
}

func TestManagerRejectsDisabledCredential(t *testing.T) {
	mgr := NewManager(NewMemoryRepository(), NewMemorySecretBackend())
	if _, err := mgr.Create(context.Background(), CreateInput{
		CredentialID:   "disabled-key",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       AuthTypePassword,
		Password:       "local-only-password",
		OperatorReason: "test",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := mgr.SetStatus(context.Background(), "disabled-key", StatusDisabled, "admin", "pause access"); err != nil {
		t.Fatalf("SetStatus() error = %v", err)
	}
	if _, err := mgr.Resolve(context.Background(), "disabled-key", "192.168.3.100"); err == nil {
		t.Fatalf("expected disabled credential to be rejected")
	}
}

func TestManagerRejectsRotationRequiredCredential(t *testing.T) {
	mgr := NewManager(NewMemoryRepository(), NewMemorySecretBackend())
	if _, err := mgr.Create(context.Background(), CreateInput{
		CredentialID:   "rotation-key",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       AuthTypePassword,
		Password:       "local-only-password",
		OperatorReason: "test",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := mgr.SetStatus(context.Background(), "rotation-key", StatusRotationRequired, "admin", "rotation overdue"); err != nil {
		t.Fatalf("SetStatus() error = %v", err)
	}
	if _, err := mgr.Resolve(context.Background(), "rotation-key", "192.168.3.100"); err == nil || !errors.Is(err, ErrDisabled) || !strings.Contains(err.Error(), StatusRotationRequired) {
		t.Fatalf("expected rotation_required credential to fail closed with inactive error, got %v", err)
	}
}

func TestSanitizeCredentialClearsCustodyRefs(t *testing.T) {
	cred := sanitizeCredential(Credential{
		CredentialID:        "ops-key",
		SecretRef:           " ssh/ssh-main/ops-key/material ",
		PassphraseSecretRef: " ssh/ssh-main/ops-key/passphrase ",
	})
	if cred.SecretRef != "" || cred.PassphraseSecretRef != "" {
		t.Fatalf("expected sanitizeCredential to clear custody refs, got %#v", cred)
	}
}

func TestManagerRejectsHostOutsideScope(t *testing.T) {
	mgr := NewManager(NewMemoryRepository(), NewMemorySecretBackend())
	if _, err := mgr.Create(context.Background(), CreateInput{
		CredentialID:   "scoped-key",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       AuthTypePassword,
		Password:       "local-only-password",
		HostScope:      "192.168.3.0/24",
		OperatorReason: "test",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := mgr.Resolve(context.Background(), "scoped-key", "10.0.0.10"); err == nil {
		t.Fatalf("expected host outside scope to be rejected")
	}
	if _, err := mgr.Resolve(context.Background(), "scoped-key", "192.168.3.100"); err != nil {
		t.Fatalf("expected host inside CIDR to resolve, got %v", err)
	}
}

func TestManagerResolveAuditsCredentialUseWithoutSecretMaterial(t *testing.T) {
	logger := &captureAuditLogger{}
	mgr := NewManager(NewMemoryRepository(), NewMemorySecretBackend())
	mgr.SetAudit(logger)
	if _, err := mgr.Create(context.Background(), CreateInput{
		CredentialID:   "audited-key",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       AuthTypePassword,
		Password:       "super-secret-password",
		HostScope:      "192.168.3.100",
		OperatorReason: "test",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if _, err := mgr.Resolve(context.Background(), "audited-key", "192.168.3.100"); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(logger.entries) != 1 {
		t.Fatalf("expected one audit entry, got %+v", logger.entries)
	}
	entry := logger.entries[0]
	if entry.Action != "ssh_credential.used" || entry.Actor != "ssh_runtime" || entry.ResourceID != "audited-key" {
		t.Fatalf("unexpected audit entry: %+v", entry)
	}
	encoded, err := json.Marshal(entry.Metadata)
	if err != nil {
		t.Fatalf("marshal audit metadata: %v", err)
	}
	if strings.Contains(string(encoded), "super-secret-password") {
		t.Fatalf("audit metadata leaked secret material: %s", string(encoded))
	}
}

func TestManagerResolveFailsClosedWhenExpiredCredentialNeedsRotation(t *testing.T) {
	repo := NewMemoryRepository()
	vault := NewMemorySecretBackend()
	mgr := NewManager(repo, vault)
	logger := &captureAuditLogger{}
	mgr.SetAudit(logger)
	now := time.Date(2026, 4, 24, 10, 0, 0, 0, time.UTC)
	mgr.now = func() time.Time { return now }
	expiresAt := now.Add(-time.Minute)
	if _, err := mgr.Create(context.Background(), CreateInput{
		CredentialID:   "expired-key",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       AuthTypePassword,
		Password:       "local-only-password",
		ExpiresAt:      &expiresAt,
		OperatorReason: "test",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := mgr.Resolve(context.Background(), "expired-key", "192.168.3.100"); err == nil || !errors.Is(err, ErrRotationRequired) {
		t.Fatalf("expected expired credential to fail with rotation required, got %v", err)
	}
	stored, ok, err := repo.Get(context.Background(), "expired-key")
	if err != nil || !ok {
		t.Fatalf("repo.Get() found=%v err=%v", ok, err)
	}
	if stored.Status != StatusRotationRequired {
		t.Fatalf("expected stored status to become rotation_required, got %#v", stored)
	}
	if len(logger.entries) != 1 {
		t.Fatalf("expected one audit entry, got %+v", logger.entries)
	}
	entry := logger.entries[0]
	if entry.Action != "ssh_credential.rotation_auto_triggered" || entry.Actor != "system" {
		t.Fatalf("unexpected audit entry: %+v", entry)
	}
	if entry.Metadata["trigger"] != "expires_at" || entry.Metadata["rotation_required"] != true {
		t.Fatalf("expected expiry-trigger metadata, got %+v", entry)
	}
}

func TestManagerUpdateWithMaterialClearsRotationRequired(t *testing.T) {
	mgr := NewManager(NewMemoryRepository(), NewMemorySecretBackend())
	if _, err := mgr.Create(context.Background(), CreateInput{
		CredentialID:   "rotating-key",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       AuthTypePassword,
		Password:       "old-password",
		OperatorReason: "test",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := mgr.SetStatus(context.Background(), "rotating-key", StatusRotationRequired, "admin", "rotation overdue"); err != nil {
		t.Fatalf("SetStatus() error = %v", err)
	}
	updated, err := mgr.Update(context.Background(), "rotating-key", UpdateInput{
		Password:       "new-password",
		OperatorReason: "rotate",
		ActorID:        "admin",
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Status != StatusActive {
		t.Fatalf("expected rotated credential to return active, got %#v", updated)
	}
	if updated.LastRotatedAt.IsZero() {
		t.Fatalf("expected LastRotatedAt to be updated, got %#v", updated)
	}
}

func TestManagerResolveFailsClosedWhenSecretMetadataMissing(t *testing.T) {
	repo := NewMemoryRepository()
	vault := NewMemorySecretBackend()
	mgr := NewManager(repo, vault)
	if _, err := mgr.Create(context.Background(), CreateInput{
		CredentialID:   "missing-secret",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       AuthTypePassword,
		Password:       "secret",
		OperatorReason: "test",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	vault.Delete(context.Background(), "ssh/ssh-main/missing-secret/material")
	if _, err := mgr.Resolve(context.Background(), "missing-secret", "192.168.3.100"); err == nil || !errors.Is(err, ErrSecretMissing) {
		t.Fatalf("expected missing metadata to fail closed, got %v", err)
	}
}

func TestManagerResolveFailsClosedWhenKeyIDMismatches(t *testing.T) {
	repo := NewMemoryRepository()
	vault := NewMemorySecretBackend()
	vault.keyID = "current-key"
	mgr := NewManager(repo, vault)
	if _, err := mgr.Create(context.Background(), CreateInput{
		CredentialID:   "mismatch-key",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       AuthTypePassword,
		Password:       "secret",
		OperatorReason: "test",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	vault.mu.Lock()
	vault.meta["ssh/ssh-main/mismatch-key/material"] = SecretMetadata{Ref: "ssh/ssh-main/mismatch-key/material", Set: true, KeyID: "old-key", UpdatedAt: time.Now().UTC(), Algorithm: "memory"}
	vault.mu.Unlock()
	if _, err := mgr.Resolve(context.Background(), "mismatch-key", "192.168.3.100"); err == nil || !errors.Is(err, ErrKeyIDMismatch) {
		t.Fatalf("expected key_id mismatch to fail closed, got %v", err)
	}
}

func TestManagerResolveDeniesBreakGlassMaterialAccess(t *testing.T) {
	mgr := NewManager(NewMemoryRepository(), NewMemorySecretBackend())
	if _, err := mgr.Create(context.Background(), CreateInput{
		CredentialID:   "break-glass-key",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       AuthTypePassword,
		Password:       "secret",
		OperatorReason: "test",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := mgr.ResolveWithOptions(context.Background(), "break-glass-key", ResolveOptions{TargetHost: "192.168.3.100", BreakGlassAccess: true, ActorSource: "ops-token", ActorID: "ops-admin"}); err == nil || !errors.Is(err, ErrBreakGlassDenied) {
		t.Fatalf("expected break-glass resolve to be denied, got %v", err)
	}
}

type captureAuditLogger struct {
	entries []audit.Entry
}

func (c *captureAuditLogger) Log(_ context.Context, entry audit.Entry) {
	c.entries = append(c.entries, entry)
}

func (b *MemorySecretBackend) KeyID() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.keyID
}
