package sshcredentials

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"tars/internal/foundation/audit"
)

const (
	AuthTypePassword   = "password"
	AuthTypePrivateKey = "private_key"

	StatusActive           = "active"
	StatusDisabled         = "disabled"
	StatusRotationRequired = "rotation_required"
	StatusDeleted          = "deleted"
)

var (
	ErrNotConfigured    = errors.New("ssh credential custody is not configured")
	ErrNotFound         = errors.New("ssh credential not found")
	ErrDisabled         = errors.New("ssh credential is not active")
	ErrRotationRequired = errors.New("ssh credential status is rotation_required")
	ErrHostScope        = errors.New("target host is outside credential scope")
	ErrSecretMissing    = errors.New("ssh credential secret material is missing")
	ErrKeyIDMismatch    = errors.New("ssh credential custody key_id does not match current configuration")
	ErrBreakGlassDenied = errors.New("break-glass access cannot resolve ssh credential material")
)

var credentialIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{1,127}$`)

type Credential struct {
	CredentialID        string     `json:"credential_id"`
	DisplayName         string     `json:"display_name,omitempty"`
	OwnerType           string     `json:"owner_type,omitempty"`
	OwnerID             string     `json:"owner_id,omitempty"`
	ConnectorID         string     `json:"connector_id,omitempty"`
	Username            string     `json:"username,omitempty"`
	AuthType            string     `json:"auth_type"`
	SecretRef           string     `json:"secret_ref,omitempty"`
	PassphraseSecretRef string     `json:"passphrase_secret_ref,omitempty"`
	HostScope           string     `json:"host_scope,omitempty"`
	Status              string     `json:"status"`
	CreatedBy           string     `json:"created_by,omitempty"`
	UpdatedBy           string     `json:"updated_by,omitempty"`
	CreatedAt           time.Time  `json:"created_at,omitempty"`
	UpdatedAt           time.Time  `json:"updated_at,omitempty"`
	LastRotatedAt       time.Time  `json:"last_rotated_at,omitempty"`
	ExpiresAt           *time.Time `json:"expires_at,omitempty"`
}

type CreateInput struct {
	CredentialID   string
	DisplayName    string
	OwnerType      string
	OwnerID        string
	ConnectorID    string
	Username       string
	AuthType       string
	Password       string
	PrivateKey     string
	Passphrase     string
	HostScope      string
	ExpiresAt      *time.Time
	OperatorReason string
	ActorID        string
}

type UpdateInput struct {
	DisplayName    string
	OwnerType      string
	OwnerID        string
	ConnectorID    string
	Username       string
	Password       string
	PrivateKey     string
	Passphrase     string
	HostScope      string
	ExpiresAt      *time.Time
	OperatorReason string
	ActorID        string
}

type ResolvedCredential struct {
	CredentialID string
	ConnectorID  string
	Username     string
	AuthType     string
	Password     string
	PrivateKey   string
	Passphrase   string
	HostScope    string
}

type Repository interface {
	List(ctx context.Context) ([]Credential, error)
	Get(ctx context.Context, credentialID string) (Credential, bool, error)
	Save(ctx context.Context, credential Credential) error
}

type SecretBackend interface {
	Put(ctx context.Context, ref string, value []byte, metadata map[string]string) error
	Get(ctx context.Context, ref string) ([]byte, error)
	Delete(ctx context.Context, ref string) error
	Metadata(ctx context.Context, ref string) (SecretMetadata, bool, error)
}

type SecretMetadata struct {
	Ref       string
	Set       bool
	UpdatedAt time.Time
	KeyID     string
	Algorithm string
}

type InventoryItem struct {
	CredentialID string
	Status       string
	UpdatedAt    time.Time
}

type ResolveOptions struct {
	TargetHost       string
	ActorID          string
	ActorSource      string
	BreakGlassAccess bool
}

type Manager struct {
	repo  Repository
	vault SecretBackend
	now   func() time.Time
	audit audit.Logger
}

func NewManager(repo Repository, vault SecretBackend) *Manager {
	return &Manager{repo: repo, vault: vault, now: func() time.Time { return time.Now().UTC() }}
}

func (m *Manager) SetAudit(logger audit.Logger) {
	if m == nil {
		return
	}
	m.audit = logger
}

func (m *Manager) Configured() bool {
	return m != nil && m.repo != nil && m.vault != nil
}

func (m *Manager) CurrentKeyID() string {
	if m == nil || m.vault == nil {
		return ""
	}
	return currentVaultKeyID(m.vault)
}

func (m *Manager) List(ctx context.Context) ([]Credential, error) {
	if !m.Configured() {
		return nil, ErrNotConfigured
	}
	items, err := m.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Credential, 0, len(items))
	for _, item := range items {
		if item.Status == StatusDeleted {
			continue
		}
		out = append(out, sanitizeCredential(item))
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].CredentialID < out[j].CredentialID
	})
	return out, nil
}

func (m *Manager) Inventory(ctx context.Context) ([]InventoryItem, error) {
	if !m.Configured() {
		return nil, ErrNotConfigured
	}
	items, err := m.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]InventoryItem, 0, len(items))
	currentKeyID := strings.TrimSpace(currentVaultKeyID(m.vault))
	for _, item := range items {
		if item.Status == StatusDeleted {
			continue
		}
		status := item.Status
		if status == "" {
			status = StatusActive
		}
		if item.ExpiresAt != nil && !item.ExpiresAt.IsZero() && !item.ExpiresAt.After(m.now()) {
			status = StatusRotationRequired
		}
		if strings.TrimSpace(item.SecretRef) == "" {
			status = "missing"
		} else {
			meta, ok, metaErr := m.vault.Metadata(ctx, item.SecretRef)
			switch {
			case metaErr != nil:
				status = "invalid_secret_ref"
			case !ok || !meta.Set:
				status = "missing"
			case currentKeyID != "" && strings.TrimSpace(meta.KeyID) != "" && strings.TrimSpace(meta.KeyID) != currentKeyID:
				status = "invalid_secret_ref"
			}
		}
		out = append(out, InventoryItem{CredentialID: item.CredentialID, Status: status, UpdatedAt: item.UpdatedAt})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CredentialID < out[j].CredentialID })
	return out, nil
}

func (m *Manager) Get(ctx context.Context, credentialID string) (Credential, bool, error) {
	if !m.Configured() {
		return Credential{}, false, ErrNotConfigured
	}
	item, ok, err := m.repo.Get(ctx, strings.TrimSpace(credentialID))
	if err != nil || !ok || item.Status == StatusDeleted {
		return Credential{}, false, err
	}
	return sanitizeCredential(item), true, nil
}

func (m *Manager) Create(ctx context.Context, input CreateInput) (Credential, error) {
	if !m.Configured() {
		return Credential{}, ErrNotConfigured
	}
	cred, material, err := normalizeCreateInput(input, m.now())
	if err != nil {
		return Credential{}, err
	}
	if existing, ok, err := m.repo.Get(ctx, cred.CredentialID); err != nil {
		return Credential{}, err
	} else if ok && existing.Status != StatusDeleted {
		return Credential{}, fmt.Errorf("ssh credential %s already exists", cred.CredentialID)
	}
	if err := m.storeMaterial(ctx, cred, material); err != nil {
		return Credential{}, err
	}
	if err := m.repo.Save(ctx, cred); err != nil {
		return Credential{}, err
	}
	return sanitizeCredential(cred), nil
}

func (m *Manager) Update(ctx context.Context, credentialID string, input UpdateInput) (Credential, error) {
	if !m.Configured() {
		return Credential{}, ErrNotConfigured
	}
	cred, ok, err := m.repo.Get(ctx, strings.TrimSpace(credentialID))
	if err != nil {
		return Credential{}, err
	}
	if !ok || cred.Status == StatusDeleted {
		return Credential{}, ErrNotFound
	}
	now := m.now()
	cred.DisplayName = firstNonEmpty(strings.TrimSpace(input.DisplayName), cred.DisplayName)
	cred.OwnerType = firstNonEmpty(strings.TrimSpace(input.OwnerType), cred.OwnerType)
	cred.OwnerID = firstNonEmpty(strings.TrimSpace(input.OwnerID), cred.OwnerID)
	cred.ConnectorID = firstNonEmpty(strings.TrimSpace(input.ConnectorID), cred.ConnectorID)
	cred.Username = firstNonEmpty(strings.TrimSpace(input.Username), cred.Username)
	cred.HostScope = strings.TrimSpace(input.HostScope)
	cred.ExpiresAt = input.ExpiresAt
	cred.UpdatedBy = strings.TrimSpace(input.ActorID)
	cred.UpdatedAt = now
	material := credentialMaterial{Password: input.Password, PrivateKey: input.PrivateKey, Passphrase: input.Passphrase}
	if material.hasPrimary() {
		if err := validateMaterial(cred.AuthType, material); err != nil {
			return Credential{}, err
		}
		if err := m.storeMaterial(ctx, cred, material); err != nil {
			return Credential{}, err
		}
		cred.Status = StatusActive
		cred.LastRotatedAt = now
	}
	if err := m.repo.Save(ctx, cred); err != nil {
		return Credential{}, err
	}
	return sanitizeCredential(cred), nil
}

func (m *Manager) SetStatus(ctx context.Context, credentialID string, status string, actorID string, reason string) (Credential, error) {
	if !m.Configured() {
		return Credential{}, ErrNotConfigured
	}
	status = normalizeStatus(status)
	if status == "" || status == StatusDeleted {
		return Credential{}, fmt.Errorf("unsupported ssh credential status %q", status)
	}
	cred, ok, err := m.repo.Get(ctx, strings.TrimSpace(credentialID))
	if err != nil {
		return Credential{}, err
	}
	if !ok || cred.Status == StatusDeleted {
		return Credential{}, ErrNotFound
	}
	if cred.Status == StatusRotationRequired && status == StatusActive {
		return Credential{}, ErrRotationRequired
	}
	cred.Status = status
	cred.UpdatedBy = strings.TrimSpace(actorID)
	cred.UpdatedAt = m.now()
	if err := m.repo.Save(ctx, cred); err != nil {
		return Credential{}, err
	}
	return sanitizeCredential(cred), nil
}

func (m *Manager) Delete(ctx context.Context, credentialID string, actorID string, reason string) (Credential, error) {
	if !m.Configured() {
		return Credential{}, ErrNotConfigured
	}
	cred, ok, err := m.repo.Get(ctx, strings.TrimSpace(credentialID))
	if err != nil {
		return Credential{}, err
	}
	if !ok || cred.Status == StatusDeleted {
		return Credential{}, ErrNotFound
	}
	cred.Status = StatusDeleted
	cred.UpdatedBy = strings.TrimSpace(actorID)
	cred.UpdatedAt = m.now()
	if cred.SecretRef != "" {
		_ = m.vault.Delete(ctx, cred.SecretRef)
	}
	if cred.PassphraseSecretRef != "" {
		_ = m.vault.Delete(ctx, cred.PassphraseSecretRef)
	}
	if err := m.repo.Save(ctx, cred); err != nil {
		return Credential{}, err
	}
	return sanitizeCredential(cred), nil
}

func (m *Manager) Resolve(ctx context.Context, credentialID string, targetHost string) (ResolvedCredential, error) {
	return m.ResolveWithOptions(ctx, credentialID, ResolveOptions{TargetHost: targetHost})
}

func (m *Manager) ResolveWithOptions(ctx context.Context, credentialID string, opts ResolveOptions) (ResolvedCredential, error) {
	if !m.Configured() {
		return ResolvedCredential{}, ErrNotConfigured
	}
	cred, ok, err := m.repo.Get(ctx, strings.TrimSpace(credentialID))
	if err != nil {
		return ResolvedCredential{}, err
	}
	if !ok || cred.Status == StatusDeleted {
		return ResolvedCredential{}, ErrNotFound
	}
	if opts.BreakGlassAccess {
		return ResolvedCredential{}, ErrBreakGlassDenied
	}
	if expiresAt := cred.ExpiresAt; expiresAt != nil && !expiresAt.IsZero() && !expiresAt.After(m.now()) {
		cred.Status = StatusRotationRequired
		cred.UpdatedBy = firstNonEmpty(strings.TrimSpace(opts.ActorID), "system")
		cred.UpdatedAt = m.now()
		if err := m.repo.Save(ctx, cred); err != nil {
			return ResolvedCredential{}, err
		}
		m.auditRotationAutoTriggered(ctx, cred, opts)
	}
	if cred.Status == StatusRotationRequired {
		return ResolvedCredential{}, errors.Join(ErrDisabled, ErrRotationRequired)
	}
	if cred.Status != StatusActive {
		return ResolvedCredential{}, ErrDisabled
	}
	if !hostAllowed(cred.HostScope, opts.TargetHost) {
		return ResolvedCredential{}, ErrHostScope
	}
	currentKeyID := strings.TrimSpace(currentVaultKeyID(m.vault))
	meta, ok, err := m.vault.Metadata(ctx, cred.SecretRef)
	if err != nil {
		return ResolvedCredential{}, err
	}
	if !ok || !meta.Set {
		return ResolvedCredential{}, ErrSecretMissing
	}
	if currentKeyID != "" && strings.TrimSpace(meta.KeyID) != "" && strings.TrimSpace(meta.KeyID) != currentKeyID {
		return ResolvedCredential{}, ErrKeyIDMismatch
	}
	material, err := m.vault.Get(ctx, cred.SecretRef)
	if err != nil {
		return ResolvedCredential{}, err
	}
	resolved := ResolvedCredential{
		CredentialID: cred.CredentialID,
		ConnectorID:  cred.ConnectorID,
		Username:     cred.Username,
		AuthType:     cred.AuthType,
		HostScope:    cred.HostScope,
	}
	switch cred.AuthType {
	case AuthTypePassword:
		resolved.Password = string(material)
	case AuthTypePrivateKey:
		resolved.PrivateKey = string(material)
		if cred.PassphraseSecretRef != "" {
			passphraseMeta, ok, err := m.vault.Metadata(ctx, cred.PassphraseSecretRef)
			if err != nil {
				return ResolvedCredential{}, err
			}
			if !ok || !passphraseMeta.Set {
				return ResolvedCredential{}, ErrSecretMissing
			}
			if currentKeyID != "" && strings.TrimSpace(passphraseMeta.KeyID) != "" && strings.TrimSpace(passphraseMeta.KeyID) != currentKeyID {
				return ResolvedCredential{}, ErrKeyIDMismatch
			}
			passphrase, err := m.vault.Get(ctx, cred.PassphraseSecretRef)
			if err != nil {
				return ResolvedCredential{}, err
			}
			resolved.Passphrase = string(passphrase)
		}
	default:
		return ResolvedCredential{}, fmt.Errorf("unsupported ssh auth type %q", cred.AuthType)
	}
	m.auditCredentialUse(ctx, cred, opts.TargetHost)
	return resolved, nil
}

func currentVaultKeyID(vault SecretBackend) string {
	type keyIDProvider interface{ KeyID() string }
	provider, ok := vault.(keyIDProvider)
	if !ok {
		return ""
	}
	return strings.TrimSpace(provider.KeyID())
}

func (m *Manager) auditCredentialUse(ctx context.Context, cred Credential, targetHost string) {
	if m == nil || m.audit == nil {
		return
	}
	m.audit.Log(ctx, audit.Entry{
		ResourceType: "ssh_credential",
		ResourceID:   cred.CredentialID,
		Action:       "ssh_credential.used",
		Actor:        "ssh_runtime",
		Metadata: map[string]any{
			"credential_id": cred.CredentialID,
			"connector_id":  cred.ConnectorID,
			"owner_type":    cred.OwnerType,
			"owner_id":      cred.OwnerID,
			"auth_type":     cred.AuthType,
			"host_scope":    cred.HostScope,
			"status":        cred.Status,
			"target_host":   strings.TrimSpace(targetHost),
		},
	})
}

func (m *Manager) auditRotationAutoTriggered(ctx context.Context, cred Credential, opts ResolveOptions) {
	if m == nil || m.audit == nil {
		return
	}
	m.audit.Log(ctx, audit.Entry{
		ResourceType: "ssh_credential",
		ResourceID:   cred.CredentialID,
		Action:       "ssh_credential.rotation_auto_triggered",
		Actor:        firstNonEmpty(strings.TrimSpace(opts.ActorID), "system"),
		Metadata: map[string]any{
			"credential_id":    cred.CredentialID,
			"connector_id":     cred.ConnectorID,
			"owner_type":       cred.OwnerType,
			"owner_id":         cred.OwnerID,
			"actor_source":     strings.TrimSpace(opts.ActorSource),
			"break_glass":      opts.BreakGlassAccess,
			"target_host":      strings.TrimSpace(opts.TargetHost),
			"trigger":          "expires_at",
			"rotation_required": true,
			"expires_at":       cred.ExpiresAt,
		},
	})
}

type credentialMaterial struct {
	Password   string
	PrivateKey string
	Passphrase string
}

func (m *Manager) storeMaterial(ctx context.Context, cred Credential, material credentialMaterial) error {
	metadata := map[string]string{
		"owner_type":    cred.OwnerType,
		"owner_id":      cred.OwnerID,
		"connector_id":  cred.ConnectorID,
		"credential_id": cred.CredentialID,
		"auth_type":     cred.AuthType,
	}
	switch cred.AuthType {
	case AuthTypePassword:
		return m.vault.Put(ctx, cred.SecretRef, []byte(material.Password), metadata)
	case AuthTypePrivateKey:
		if err := m.vault.Put(ctx, cred.SecretRef, []byte(material.PrivateKey), metadata); err != nil {
			return err
		}
		if strings.TrimSpace(material.Passphrase) != "" {
			return m.vault.Put(ctx, cred.PassphraseSecretRef, []byte(material.Passphrase), metadata)
		}
		return nil
	default:
		return fmt.Errorf("unsupported ssh auth type %q", cred.AuthType)
	}
}

func normalizeCreateInput(input CreateInput, now time.Time) (Credential, credentialMaterial, error) {
	id := strings.TrimSpace(input.CredentialID)
	if !credentialIDPattern.MatchString(id) {
		return Credential{}, credentialMaterial{}, fmt.Errorf("invalid ssh credential id")
	}
	authType := normalizeAuthType(input.AuthType)
	material := credentialMaterial{Password: input.Password, PrivateKey: input.PrivateKey, Passphrase: input.Passphrase}
	if err := validateMaterial(authType, material); err != nil {
		return Credential{}, credentialMaterial{}, err
	}
	connectorID := firstNonEmpty(strings.TrimSpace(input.ConnectorID), "ssh")
	cred := Credential{
		CredentialID:        id,
		DisplayName:         strings.TrimSpace(input.DisplayName),
		OwnerType:           firstNonEmpty(strings.TrimSpace(input.OwnerType), "connector"),
		OwnerID:             strings.TrimSpace(input.OwnerID),
		ConnectorID:         connectorID,
		Username:            strings.TrimSpace(input.Username),
		AuthType:            authType,
		SecretRef:           fmt.Sprintf("ssh/%s/%s/material", connectorID, id),
		PassphraseSecretRef: fmt.Sprintf("ssh/%s/%s/passphrase", connectorID, id),
		HostScope:           strings.TrimSpace(input.HostScope),
		Status:              StatusActive,
		CreatedBy:           strings.TrimSpace(input.ActorID),
		UpdatedBy:           strings.TrimSpace(input.ActorID),
		CreatedAt:           now,
		UpdatedAt:           now,
		LastRotatedAt:       now,
		ExpiresAt:           input.ExpiresAt,
	}
	if cred.DisplayName == "" {
		cred.DisplayName = id
	}
	if cred.OwnerID == "" {
		cred.OwnerID = connectorID
	}
	if cred.Username == "" {
		return Credential{}, credentialMaterial{}, fmt.Errorf("ssh username is required")
	}
	if authType != AuthTypePrivateKey || strings.TrimSpace(input.Passphrase) == "" {
		cred.PassphraseSecretRef = ""
	}
	return cred, material, nil
}

func validateMaterial(authType string, material credentialMaterial) error {
	switch authType {
	case AuthTypePassword:
		if strings.TrimSpace(material.Password) == "" {
			return fmt.Errorf("ssh password is required")
		}
	case AuthTypePrivateKey:
		if strings.TrimSpace(material.PrivateKey) == "" {
			return fmt.Errorf("ssh private key is required")
		}
		if !strings.Contains(material.PrivateKey, "PRIVATE KEY") {
			return fmt.Errorf("ssh private key must look like a PEM/OpenSSH private key")
		}
	default:
		return fmt.Errorf("unsupported ssh auth type %q", authType)
	}
	return nil
}

func (m credentialMaterial) hasPrimary() bool {
	return strings.TrimSpace(m.Password) != "" || strings.TrimSpace(m.PrivateKey) != ""
}

func normalizeAuthType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case AuthTypePassword:
		return AuthTypePassword
	case AuthTypePrivateKey, "key", "private-key":
		return AuthTypePrivateKey
	default:
		return ""
	}
}

func normalizeStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case StatusActive:
		return StatusActive
	case StatusDisabled:
		return StatusDisabled
	case StatusRotationRequired:
		return StatusRotationRequired
	case StatusDeleted:
		return StatusDeleted
	default:
		return ""
	}
}

func sanitizeCredential(cred Credential) Credential {
	cred.SecretRef = ""
	cred.PassphraseSecretRef = ""
	return cred
}

func hostAllowed(scope string, targetHost string) bool {
	scope = strings.TrimSpace(scope)
	if scope == "" || scope == "*" {
		return true
	}
	host := normalizeHost(targetHost)
	if host == "" {
		return false
	}
	for _, candidate := range strings.Split(scope, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if candidate == "*" || strings.EqualFold(candidate, host) || strings.EqualFold(candidate, targetHost) {
			return true
		}
		if _, network, err := net.ParseCIDR(candidate); err == nil {
			if ip := net.ParseIP(host); ip != nil && network.Contains(ip) {
				return true
			}
		}
	}
	return false
}

func normalizeHost(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if at := strings.LastIndex(value, "@"); at >= 0 && at < len(value)-1 {
		value = value[at+1:]
	}
	if host, _, err := net.SplitHostPort(value); err == nil {
		return strings.Trim(host, "[]")
	}
	return strings.Trim(value, "[]")
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type MemoryRepository struct {
	mu    sync.RWMutex
	items map[string]Credential
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{items: map[string]Credential{}}
}

func (r *MemoryRepository) List(ctx context.Context) ([]Credential, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Credential, 0, len(r.items))
	for _, item := range r.items {
		out = append(out, item)
	}
	return out, nil
}

func (r *MemoryRepository) Get(ctx context.Context, credentialID string) (Credential, bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[strings.TrimSpace(credentialID)]
	return item, ok, nil
}

func (r *MemoryRepository) Save(ctx context.Context, credential Credential) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[strings.TrimSpace(credential.CredentialID)] = credential
	return nil
}

type MemorySecretBackend struct {
	mu      sync.RWMutex
	values  map[string][]byte
	updated map[string]time.Time
	meta    map[string]SecretMetadata
	keyID   string
}

func NewMemorySecretBackend() *MemorySecretBackend {
	return &MemorySecretBackend{values: map[string][]byte{}, updated: map[string]time.Time{}, meta: map[string]SecretMetadata{}, keyID: "memory"}
}

func (b *MemorySecretBackend) Put(ctx context.Context, ref string, value []byte, metadata map[string]string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	trimmed := strings.TrimSpace(ref)
	now := time.Now().UTC()
	b.values[trimmed] = append([]byte(nil), value...)
	b.updated[trimmed] = now
	b.meta[trimmed] = SecretMetadata{Ref: trimmed, Set: true, UpdatedAt: now, KeyID: b.keyID, Algorithm: "memory"}
	return nil
}

func (b *MemorySecretBackend) Get(ctx context.Context, ref string) ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	value, ok := b.values[strings.TrimSpace(ref)]
	if !ok {
		return nil, ErrNotFound
	}
	return append([]byte(nil), value...), nil
}

func (b *MemorySecretBackend) Delete(ctx context.Context, ref string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.values, strings.TrimSpace(ref))
	delete(b.updated, strings.TrimSpace(ref))
	delete(b.meta, strings.TrimSpace(ref))
	return nil
}

func (b *MemorySecretBackend) Metadata(ctx context.Context, ref string) (SecretMetadata, bool, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	ref = strings.TrimSpace(ref)
	meta, ok := b.meta[ref]
	if !ok {
		return SecretMetadata{}, false, nil
	}
	return meta, true, nil
}
