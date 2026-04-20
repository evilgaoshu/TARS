package access

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"gopkg.in/yaml.v3"
)

var ErrConfigPathNotSet = errors.New("access config path is not set")
var ErrUserNotFound = errors.New("user not found")
var ErrAuthProviderNotFound = errors.New("auth provider not found")
var ErrRoleNotFound = errors.New("role not found")
var ErrGroupNotFound = errors.New("group not found")
var ErrPersonNotFound = errors.New("person not found")
var ErrChannelNotFound = errors.New("channel not found")
var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrAuthProviderDisabled = errors.New("auth provider is disabled")
var ErrOAuthStateInvalid = errors.New("oauth state is invalid or expired")
var ErrAuthFlowNotFound = errors.New("auth flow not found or expired")
var ErrChallengeNotFound = errors.New("challenge not found or expired")
var ErrChallengeRequired = errors.New("challenge verification required")
var ErrMFARequired = errors.New("mfa verification required")

type Snapshot struct {
	Path      string
	Content   string
	Config    Config
	UpdatedAt time.Time
	Loaded    bool
}

type Config struct {
	Users         []User
	Groups        []Group
	AuthProviders []AuthProvider
	Roles         []Role
	People        []Person
	Channels      []Channel
}

type User struct {
	UserID               string         `yaml:"user_id,omitempty"`
	Username             string         `yaml:"username,omitempty"`
	DisplayName          string         `yaml:"display_name,omitempty"`
	Email                string         `yaml:"email,omitempty"`
	Status               string         `yaml:"status,omitempty"`
	Source               string         `yaml:"source,omitempty"`
	PasswordHash         string         `yaml:"password_hash,omitempty"`
	PasswordLoginEnabled bool           `yaml:"password_login_enabled,omitempty"`
	PasswordUpdatedAt    time.Time      `yaml:"password_updated_at,omitempty"`
	ChallengeRequired    bool           `yaml:"challenge_required,omitempty"`
	MFAEnabled           bool           `yaml:"mfa_enabled,omitempty"`
	MFAMethod            string         `yaml:"mfa_method,omitempty"`
	TOTPSecret           string         `yaml:"totp_secret,omitempty"`
	Roles                []string       `yaml:"roles,omitempty"`
	Groups               []string       `yaml:"groups,omitempty"`
	Identities           []IdentityLink `yaml:"identities,omitempty"`
	// Org affiliation (ORG-N1) — omit when empty for single-tenant compat
	OrgID       string    `yaml:"org_id,omitempty"`
	TenantID    string    `yaml:"tenant_id,omitempty"`
	WorkspaceID string    `yaml:"workspace_id,omitempty"`
	CreatedAt   time.Time `yaml:"created_at,omitempty"`
	UpdatedAt   time.Time `yaml:"updated_at,omitempty"`
}

type Group struct {
	GroupID     string   `yaml:"group_id,omitempty"`
	DisplayName string   `yaml:"display_name,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Status      string   `yaml:"status,omitempty"`
	Roles       []string `yaml:"roles,omitempty"`
	Members     []string `yaml:"members,omitempty"`
	// Org affiliation (ORG-N1)
	OrgID       string    `yaml:"org_id,omitempty"`
	TenantID    string    `yaml:"tenant_id,omitempty"`
	WorkspaceID string    `yaml:"workspace_id,omitempty"`
	CreatedAt   time.Time `yaml:"created_at,omitempty"`
	UpdatedAt   time.Time `yaml:"updated_at,omitempty"`
}

type IdentityLink struct {
	ProviderType     string `yaml:"provider_type,omitempty"`
	ProviderID       string `yaml:"provider_id,omitempty"`
	ExternalSubject  string `yaml:"external_subject,omitempty"`
	ExternalUsername string `yaml:"external_username,omitempty"`
	ExternalEmail    string `yaml:"external_email,omitempty"`
}

type AuthProvider struct {
	ID                  string   `yaml:"id,omitempty"`
	Type                string   `yaml:"type,omitempty"`
	Name                string   `yaml:"name,omitempty"`
	Enabled             bool     `yaml:"enabled,omitempty"`
	PasswordMinLength   int      `yaml:"password_min_length,omitempty"`
	RequireChallenge    bool     `yaml:"require_challenge,omitempty"`
	ChallengeChannel    string   `yaml:"challenge_channel,omitempty"`
	ChallengeTTLSeconds int      `yaml:"challenge_ttl_seconds,omitempty"`
	ChallengeCodeLength int      `yaml:"challenge_code_length,omitempty"`
	RequireMFA          bool     `yaml:"require_mfa,omitempty"`
	IssuerURL           string   `yaml:"issuer_url,omitempty"`
	ClientID            string   `yaml:"client_id,omitempty"`
	ClientSecret        string   `yaml:"client_secret,omitempty"`
	ClientSecretRef     string   `yaml:"client_secret_ref,omitempty"`
	AuthURL             string   `yaml:"auth_url,omitempty"`
	TokenURL            string   `yaml:"token_url,omitempty"`
	UserInfoURL         string   `yaml:"user_info_url,omitempty"`
	SessionTTLSeconds   int      `yaml:"session_ttl_seconds,omitempty"`
	LDAPURL             string   `yaml:"ldap_url,omitempty"`
	BindDN              string   `yaml:"bind_dn,omitempty"`
	BindPassword        string   `yaml:"bind_password,omitempty"`
	BindPasswordRef     string   `yaml:"bind_password_ref,omitempty"`
	BaseDN              string   `yaml:"base_dn,omitempty"`
	UserSearchFilter    string   `yaml:"user_search_filter,omitempty"`
	GroupSearchFilter   string   `yaml:"group_search_filter,omitempty"`
	Scopes              []string `yaml:"scopes,omitempty"`
	RedirectPath        string   `yaml:"redirect_path,omitempty"`
	SuccessRedirect     string   `yaml:"success_redirect,omitempty"`
	UserIDField         string   `yaml:"user_id_field,omitempty"`
	UsernameField       string   `yaml:"username_field,omitempty"`
	DisplayNameField    string   `yaml:"display_name_field,omitempty"`
	EmailField          string   `yaml:"email_field,omitempty"`
	AllowedDomains      []string `yaml:"allowed_domains,omitempty"`
	DefaultRoles        []string `yaml:"default_roles,omitempty"`
	AllowJIT            bool     `yaml:"allow_jit,omitempty"`
}

type Role struct {
	ID          string   `yaml:"id,omitempty"`
	DisplayName string   `yaml:"display_name,omitempty"`
	Permissions []string `yaml:"permissions,omitempty"`
}

type Person struct {
	ID             string            `yaml:"id,omitempty"`
	DisplayName    string            `yaml:"display_name,omitempty"`
	Email          string            `yaml:"email,omitempty"`
	Status         string            `yaml:"status,omitempty"`
	LinkedUserID   string            `yaml:"linked_user_id,omitempty"`
	ChannelIDs     []string          `yaml:"channel_ids,omitempty"`
	Team           string            `yaml:"team,omitempty"`
	ApprovalTarget string            `yaml:"approval_target,omitempty"`
	OncallSchedule string            `yaml:"oncall_schedule,omitempty"`
	Preferences    map[string]string `yaml:"preferences,omitempty"`
	// Org affiliation (ORG-N1)
	OrgID       string    `yaml:"org_id,omitempty"`
	TenantID    string    `yaml:"tenant_id,omitempty"`
	WorkspaceID string    `yaml:"workspace_id,omitempty"`
	CreatedAt   time.Time `yaml:"created_at,omitempty"`
	UpdatedAt   time.Time `yaml:"updated_at,omitempty"`
}

type ChannelUsage string

const (
	ChannelUsageApproval     ChannelUsage = "approval"
	ChannelUsageNotification ChannelUsage = "notification"
	ChannelUsageAlert        ChannelUsage = "alert"
)

type ChannelCapability string

const (
	ChannelCapabilityText   ChannelCapability = "text"
	ChannelCapabilityImage  ChannelCapability = "image"
	ChannelCapabilityFile   ChannelCapability = "file"
	ChannelCapabilityAction ChannelCapability = "action"
)

type Channel struct {
	ID           string              `yaml:"id,omitempty"`
	Kind         string              `yaml:"kind,omitempty"`
	Type         string              `yaml:"type,omitempty"` // Deprecated: use Kind
	Name         string              `yaml:"name,omitempty"`
	Target       string              `yaml:"target,omitempty"`
	Enabled      bool                `yaml:"enabled,omitempty"`
	LinkedUsers  []string            `yaml:"linked_users,omitempty"`
	Usages       []ChannelUsage      `yaml:"usages,omitempty"`
	Capabilities []ChannelCapability `yaml:"capabilities,omitempty"`
	// Org affiliation (ORG-N1)
	OrgID       string    `yaml:"org_id,omitempty"`
	TenantID    string    `yaml:"tenant_id,omitempty"`
	WorkspaceID string    `yaml:"workspace_id,omitempty"`
	CreatedAt   time.Time `yaml:"created_at,omitempty"`
	UpdatedAt   time.Time `yaml:"updated_at,omitempty"`
}

type Session struct {
	Token      string
	UserID     string
	ProviderID string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastSeenAt time.Time
}

type sessionClaims struct {
	ProviderID string `json:"provider_id,omitempty"`
	jwt.RegisteredClaims
}

type Principal struct {
	Kind       string
	Token      string
	User       *User
	RoleIDs    []string
	Permission map[string]struct{}
	Source     string
}

type OAuthState struct {
	ProviderID string
	State      string
	CreatedAt  time.Time
	ExpiresAt  time.Time
}

type PendingAuthFlow struct {
	Token             string
	UserID            string
	ProviderID        string
	CreatedAt         time.Time
	ExpiresAt         time.Time
	PasswordVerified  bool
	ChallengeRequired bool
	ChallengeVerified bool
	ChallengeID       string
	MFARequired       bool
	MFAVerified       bool
}

type ChallengeState struct {
	ID           string
	PendingToken string
	UserID       string
	ProviderID   string
	Code         string
	Channel      string
	CreatedAt    time.Time
	ExpiresAt    time.Time
	Attempts     int
	MaxAttempts  int
	Consumed     bool
}

type AuthFlowResult struct {
	Session            Session
	User               User
	ProviderID         string
	PendingToken       string
	NextStep           string
	ChallengeID        string
	ChallengeChannel   string
	ChallengeCode      string
	ChallengeExpiresAt time.Time
}

type fileConfig struct {
	Access struct {
		Users         []User         `yaml:"users,omitempty"`
		Groups        []Group        `yaml:"groups,omitempty"`
		AuthProviders []AuthProvider `yaml:"auth_providers,omitempty"`
		Roles         []Role         `yaml:"roles,omitempty"`
		People        []Person       `yaml:"people,omitempty"`
		Channels      []Channel      `yaml:"channels,omitempty"`
	} `yaml:"access"`
}

type Manager struct {
	mu          sync.RWMutex
	path        string
	content     string
	config      *Config
	enforcer    *casbin.Enforcer
	persist     func(Config) error
	updatedAt   time.Time
	sessions    map[string]Session
	oauthStates map[string]OAuthState
	pendingAuth map[string]PendingAuthFlow
	challenges  map[string]ChallengeState
	now         func() time.Time
	client      *http.Client
	sessionKey  []byte
}

func DefaultConfig() Config {
	return Config{Roles: defaultRoles()}
}

func NewManager(path string) (*Manager, error) {
	sessionKey := make([]byte, 32)
	if _, err := rand.Read(sessionKey); err != nil {
		return nil, err
	}
	m := &Manager{
		path:        strings.TrimSpace(path),
		sessions:    map[string]Session{},
		oauthStates: map[string]OAuthState{},
		pendingAuth: map[string]PendingAuthFlow{},
		challenges:  map[string]ChallengeState{},
		now:         func() time.Time { return time.Now().UTC() },
		client:      &http.Client{Timeout: 10 * time.Second},
		sessionKey:  sessionKey,
	}
	if m.path == "" {
		cfg := DefaultConfig()
		m.config = &cfg
		m.enforcer = buildPermissionEnforcer(cfg)
		m.updatedAt = m.now()
		m.content, _ = EncodeConfig(&cfg)
		return m, nil
	}
	if err := m.Reload(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Manager) Snapshot() Snapshot {
	if m == nil {
		return Snapshot{}
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot := Snapshot{Path: m.path, Content: m.content, UpdatedAt: m.updatedAt, Loaded: m.config != nil}
	if m.config != nil {
		snapshot.Config = cloneConfig(*m.config)
	}
	return snapshot
}

func (m *Manager) Reload() error {
	if m == nil {
		return nil
	}
	if strings.TrimSpace(m.path) == "" {
		cfg := DefaultConfig()
		content, _ := EncodeConfig(&cfg)
		m.mu.Lock()
		m.config = &cfg
		m.content = content
		m.updatedAt = m.now()
		m.mu.Unlock()
		return nil
	}
	content, err := os.ReadFile(m.path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			encoded, _ := EncodeConfig(&cfg)
			m.mu.Lock()
			m.config = &cfg
			m.enforcer = buildPermissionEnforcer(cfg)
			m.content = encoded
			m.updatedAt = m.now()
			m.mu.Unlock()
			return nil
		}
		return err
	}
	cfg, normalized, err := ParseConfig(content)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.config = cfg
	m.enforcer = buildPermissionEnforcer(*cfg)
	m.content = normalized
	m.updatedAt = m.now()
	m.mu.Unlock()
	return nil
}

func (m *Manager) SetPersistence(persist func(Config) error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.persist = persist
	m.mu.Unlock()
}

func (m *Manager) Save(content string) error {
	if m == nil || strings.TrimSpace(m.path) == "" {
		return ErrConfigPathNotSet
	}
	cfg, normalized, err := ParseConfig([]byte(content))
	if err != nil {
		return err
	}
	if err := writeFileAtomically(m.path, normalized); err != nil {
		return err
	}
	m.mu.Lock()
	m.config = cfg
	m.enforcer = buildPermissionEnforcer(*cfg)
	m.content = normalized
	m.updatedAt = m.now()
	persist := m.persist
	m.mu.Unlock()
	if persist != nil {
		return persist(cloneConfig(*cfg))
	}
	return nil
}

func (m *Manager) SaveConfig(cfg Config) error {
	if m == nil {
		return ErrConfigPathNotSet
	}
	if strings.TrimSpace(m.path) == "" {
		cfg = normalizeConfig(cfg)
		content, err := EncodeConfig(&cfg)
		if err != nil {
			return err
		}
		m.mu.Lock()
		m.config = &cfg
		m.enforcer = buildPermissionEnforcer(cfg)
		m.content = content
		m.updatedAt = m.now()
		persist := m.persist
		m.mu.Unlock()
		if persist != nil {
			return persist(cloneConfig(cfg))
		}
		return nil
	}
	content, err := EncodeConfig(&cfg)
	if err != nil {
		return err
	}
	return m.Save(content)
}

func (m *Manager) ListSessions() []Session {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]Session, 0, len(m.sessions))
	for _, session := range m.sessions {
		out = append(out, session)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].LastSeenAt.Equal(out[j].LastSeenAt) {
			return out[i].CreatedAt.After(out[j].CreatedAt)
		}
		return out[i].LastSeenAt.After(out[j].LastSeenAt)
	})
	return out
}

func (m *Manager) GetSession(token string) (Session, bool) {
	if m == nil {
		return Session{}, false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return Session{}, false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	session, ok := m.sessions[token]
	if !ok {
		return Session{}, false
	}
	return session, true
}

func (m *Manager) ListUsers() []User {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneUsers(m.config.Users)
}

func (m *Manager) SetUserStatus(id string, status string) (User, error) {
	user, ok := m.GetUser(id)
	if !ok {
		return User{}, ErrUserNotFound
	}
	user.Status = strings.TrimSpace(status)
	return m.UpsertUser(user)
}

func (m *Manager) GetUser(id string) (User, bool) {
	if m == nil {
		return User{}, false
	}
	id = strings.TrimSpace(id)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, user := range m.config.Users {
		if user.UserID == id || user.Username == id {
			return cloneUsers([]User{user})[0], true
		}
	}
	return User{}, false
}

func (m *Manager) UpsertUser(user User) (User, error) {
	if m == nil {
		return User{}, ErrUserNotFound
	}
	user = normalizeUser(user, m.now())
	m.mu.RLock()
	cfg := cloneConfig(*m.config)
	m.mu.RUnlock()
	replaced := false
	for i := range cfg.Users {
		if cfg.Users[i].UserID == user.UserID {
			user.CreatedAt = cfg.Users[i].CreatedAt
			cfg.Users[i] = user
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.Users = append(cfg.Users, user)
	}
	cfg = normalizeConfig(cfg)
	if err := m.SaveConfig(cfg); err != nil {
		return User{}, err
	}
	return user, nil
}

func (m *Manager) ListAuthProviders() []AuthProvider {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneAuthProviders(m.config.AuthProviders, true)
}

func (m *Manager) ListGroups() []Group {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneGroups(m.config.Groups)
}

func (m *Manager) GetGroup(id string) (Group, bool) {
	if m == nil {
		return Group{}, false
	}
	id = strings.TrimSpace(id)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, group := range m.config.Groups {
		if group.GroupID == id {
			return cloneGroups([]Group{group})[0], true
		}
	}
	return Group{}, false
}

func (m *Manager) UpsertGroup(group Group) (Group, error) {
	group = normalizeGroup(group, m.now())
	m.mu.RLock()
	cfg := cloneConfig(*m.config)
	m.mu.RUnlock()
	replaced := false
	for i := range cfg.Groups {
		if cfg.Groups[i].GroupID == group.GroupID {
			group.CreatedAt = cfg.Groups[i].CreatedAt
			cfg.Groups[i] = group
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.Groups = append(cfg.Groups, group)
	}
	cfg = normalizeConfig(cfg)
	if err := m.SaveConfig(cfg); err != nil {
		return Group{}, err
	}
	return group, nil
}

func (m *Manager) SetGroupStatus(id string, status string) (Group, error) {
	group, ok := m.GetGroup(id)
	if !ok {
		return Group{}, ErrGroupNotFound
	}
	group.Status = strings.TrimSpace(status)
	return m.UpsertGroup(group)
}

func (m *Manager) GetAuthProvider(id string) (AuthProvider, bool) {
	if m == nil {
		return AuthProvider{}, false
	}
	id = strings.TrimSpace(id)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, provider := range m.config.AuthProviders {
		if provider.ID == id {
			return normalizeAuthProvider(cloneAuthProviders([]AuthProvider{provider}, false)[0]), true
		}
	}
	return AuthProvider{}, false
}

func (m *Manager) UpsertAuthProvider(provider AuthProvider) (AuthProvider, error) {
	provider = normalizeAuthProvider(provider)
	m.mu.RLock()
	cfg := cloneConfig(*m.config)
	m.mu.RUnlock()
	replaced := false
	for i := range cfg.AuthProviders {
		if cfg.AuthProviders[i].ID == provider.ID {
			if provider.ClientSecret == "" {
				provider.ClientSecret = cfg.AuthProviders[i].ClientSecret
			}
			if provider.ClientSecretRef == "" {
				provider.ClientSecretRef = cfg.AuthProviders[i].ClientSecretRef
			}
			cfg.AuthProviders[i] = provider
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.AuthProviders = append(cfg.AuthProviders, provider)
	}
	cfg = normalizeConfig(cfg)
	if err := m.SaveConfig(cfg); err != nil {
		return AuthProvider{}, err
	}
	return provider, nil
}

func (m *Manager) SetAuthProviderEnabled(id string, enabled bool) (AuthProvider, error) {
	provider, ok := m.GetAuthProvider(id)
	if !ok {
		return AuthProvider{}, ErrAuthProviderNotFound
	}
	provider.Enabled = enabled
	return m.UpsertAuthProvider(provider)
}

func (m *Manager) ListRoles() []Role {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneRoles(m.config.Roles)
}

func (m *Manager) GetRole(id string) (Role, bool) {
	if m == nil {
		return Role{}, false
	}
	id = strings.TrimSpace(id)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, role := range m.config.Roles {
		if role.ID == id {
			return cloneRoles([]Role{role})[0], true
		}
	}
	return Role{}, false
}

func (m *Manager) UpsertRole(role Role) (Role, error) {
	role = normalizeRole(role)
	m.mu.RLock()
	cfg := cloneConfig(*m.config)
	m.mu.RUnlock()
	replaced := false
	for i := range cfg.Roles {
		if cfg.Roles[i].ID == role.ID {
			cfg.Roles[i] = role
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.Roles = append(cfg.Roles, role)
	}
	cfg = normalizeConfig(cfg)
	if err := m.SaveConfig(cfg); err != nil {
		return Role{}, err
	}
	return role, nil
}

// RoleBindings holds the users and groups bound to a specific role.
type RoleBindings struct {
	RoleID   string
	UserIDs  []string
	GroupIDs []string
}

// GetRoleBindings returns the users and groups that currently have the given role assigned.
func (m *Manager) GetRoleBindings(roleID string) (RoleBindings, bool) {
	if m == nil {
		return RoleBindings{}, false
	}
	roleID = strings.TrimSpace(roleID)
	m.mu.RLock()
	defer m.mu.RUnlock()
	roleFound := false
	for _, role := range m.config.Roles {
		if role.ID == roleID {
			roleFound = true
			break
		}
	}
	if !roleFound {
		return RoleBindings{}, false
	}
	var userIDs, groupIDs []string
	for _, user := range m.config.Users {
		for _, r := range user.Roles {
			if r == roleID {
				userIDs = append(userIDs, user.UserID)
				break
			}
		}
	}
	for _, group := range m.config.Groups {
		for _, r := range group.Roles {
			if r == roleID {
				groupIDs = append(groupIDs, group.GroupID)
				break
			}
		}
	}
	return RoleBindings{RoleID: roleID, UserIDs: userIDs, GroupIDs: groupIDs}, true
}

func (m *Manager) BindRole(roleID string, userIDs []string, groupIDs []string) error {
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return ErrRoleNotFound
	}
	m.mu.RLock()
	cfg := cloneConfig(*m.config)
	m.mu.RUnlock()
	roleFound := false
	for _, role := range cfg.Roles {
		if role.ID == roleID {
			roleFound = true
			break
		}
	}
	if !roleFound {
		return ErrRoleNotFound
	}
	for i := range cfg.Users {
		for _, userID := range userIDs {
			if cfg.Users[i].UserID == strings.TrimSpace(userID) {
				cfg.Users[i].Roles = dedupeStrings(append(cfg.Users[i].Roles, roleID))
			}
		}
	}
	for i := range cfg.Groups {
		for _, groupID := range groupIDs {
			if cfg.Groups[i].GroupID == strings.TrimSpace(groupID) {
				cfg.Groups[i].Roles = dedupeStrings(append(cfg.Groups[i].Roles, roleID))
			}
		}
	}
	cfg = normalizeConfig(cfg)
	return m.SaveConfig(cfg)
}

// UnbindRole removes roleID from the Roles list of each specified user and group.
func (m *Manager) UnbindRole(roleID string, userIDs []string, groupIDs []string) error {
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return ErrRoleNotFound
	}
	m.mu.RLock()
	cfg := cloneConfig(*m.config)
	m.mu.RUnlock()
	roleFound := false
	for _, role := range cfg.Roles {
		if role.ID == roleID {
			roleFound = true
			break
		}
	}
	if !roleFound {
		return ErrRoleNotFound
	}
	removeSet := make(map[string]bool, len(userIDs))
	for _, id := range userIDs {
		removeSet[strings.TrimSpace(id)] = true
	}
	removeGroupSet := make(map[string]bool, len(groupIDs))
	for _, id := range groupIDs {
		removeGroupSet[strings.TrimSpace(id)] = true
	}
	for i := range cfg.Users {
		if removeSet[cfg.Users[i].UserID] {
			cfg.Users[i].Roles = filterStrings(cfg.Users[i].Roles, roleID)
		}
	}
	for i := range cfg.Groups {
		if removeGroupSet[cfg.Groups[i].GroupID] {
			cfg.Groups[i].Roles = filterStrings(cfg.Groups[i].Roles, roleID)
		}
	}
	cfg = normalizeConfig(cfg)
	return m.SaveConfig(cfg)
}

// SetRoleBindings replaces the bindings for roleID to exactly the given userIDs/groupIDs.
func (m *Manager) SetRoleBindings(roleID string, userIDs []string, groupIDs []string) error {
	roleID = strings.TrimSpace(roleID)
	if roleID == "" {
		return ErrRoleNotFound
	}

	m.mu.RLock()
	cfg := cloneConfig(*m.config)
	m.mu.RUnlock()

	roleFound := false
	for _, role := range cfg.Roles {
		if role.ID == roleID {
			roleFound = true
			break
		}
	}
	if !roleFound {
		return ErrRoleNotFound
	}

	desiredUsers := make(map[string]bool, len(userIDs))
	for _, id := range userIDs {
		desiredUsers[strings.TrimSpace(id)] = true
	}
	desiredGroups := make(map[string]bool, len(groupIDs))
	for _, id := range groupIDs {
		desiredGroups[strings.TrimSpace(id)] = true
	}

	// Update all users: add role if in desired, remove if not.
	// Note: we only touch users that EITHER have the role OR are in the desired list.
	for i := range cfg.Users {
		u := &cfg.Users[i]
		hasRole := false
		for _, r := range u.Roles {
			if r == roleID {
				hasRole = true
				break
			}
		}

		if desiredUsers[u.UserID] {
			if !hasRole {
				u.Roles = dedupeStrings(append(u.Roles, roleID))
			}
		} else {
			if hasRole {
				u.Roles = filterStrings(u.Roles, roleID)
			}
		}
	}

	// Update all groups
	for i := range cfg.Groups {
		g := &cfg.Groups[i]
		hasRole := false
		for _, r := range g.Roles {
			if r == roleID {
				hasRole = true
				break
			}
		}

		if desiredGroups[g.GroupID] {
			if !hasRole {
				g.Roles = dedupeStrings(append(g.Roles, roleID))
			}
		} else {
			if hasRole {
				g.Roles = filterStrings(g.Roles, roleID)
			}
		}
	}

	cfg = normalizeConfig(cfg)
	return m.SaveConfig(cfg)
}

func (m *Manager) ListPeople() []Person {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return clonePeople(m.config.People)
}

func (m *Manager) GetPerson(id string) (Person, bool) {
	id = strings.TrimSpace(id)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, person := range m.config.People {
		if person.ID == id {
			return clonePeople([]Person{person})[0], true
		}
	}
	return Person{}, false
}

func (m *Manager) UpsertPerson(person Person) (Person, error) {
	person = normalizePerson(person, m.now())
	m.mu.RLock()
	cfg := cloneConfig(*m.config)
	m.mu.RUnlock()
	replaced := false
	for i := range cfg.People {
		if cfg.People[i].ID == person.ID {
			person.CreatedAt = cfg.People[i].CreatedAt
			cfg.People[i] = person
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.People = append(cfg.People, person)
	}
	cfg = normalizeConfig(cfg)
	if err := m.SaveConfig(cfg); err != nil {
		return Person{}, err
	}
	return person, nil
}

func (m *Manager) SetPersonStatus(id string, status string) (Person, error) {
	person, ok := m.GetPerson(id)
	if !ok {
		return Person{}, ErrPersonNotFound
	}
	person.Status = strings.TrimSpace(status)
	return m.UpsertPerson(person)
}

func (m *Manager) ListChannels() []Channel {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneChannels(m.config.Channels)
}

func (m *Manager) GetChannel(id string) (Channel, bool) {
	id = strings.TrimSpace(id)
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, channel := range m.config.Channels {
		if channel.ID == id {
			return cloneChannels([]Channel{channel})[0], true
		}
	}
	return Channel{}, false
}

func (m *Manager) UpsertChannel(channel Channel) (Channel, error) {
	channel = normalizeChannel(channel, m.now())
	m.mu.RLock()
	cfg := cloneConfig(*m.config)
	m.mu.RUnlock()
	replaced := false
	for i := range cfg.Channels {
		if cfg.Channels[i].ID == channel.ID {
			channel.CreatedAt = cfg.Channels[i].CreatedAt
			cfg.Channels[i] = channel
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.Channels = append(cfg.Channels, channel)
	}
	cfg = normalizeConfig(cfg)
	if err := m.SaveConfig(cfg); err != nil {
		return Channel{}, err
	}
	return channel, nil
}

func (m *Manager) SetChannelEnabled(id string, enabled bool) (Channel, error) {
	channel, ok := m.GetChannel(id)
	if !ok {
		return Channel{}, ErrChannelNotFound
	}
	channel.Enabled = enabled
	return m.UpsertChannel(channel)
}

func (m *Manager) LoginWithLocalToken(token string, fallbackToken string) (Session, User, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Session{}, User{}, ErrInvalidCredentials
	}
	m.mu.RLock()
	providers := cloneAuthProviders(m.config.AuthProviders, false)
	users := cloneUsers(m.config.Users)
	m.mu.RUnlock()
	for _, provider := range providers {
		if provider.Type != "local_token" || !provider.Enabled {
			continue
		}
		if token == strings.TrimSpace(provider.ClientSecret) {
			user := resolveLocalTokenUser(users, provider)
			if _, found := lookupUser(users, user.UserID); !found {
				persisted, err := m.UpsertUser(user)
				if err != nil {
					return Session{}, User{}, err
				}
				user = persisted
			}
			session := m.issueSession(user.UserID, provider.ID, provider.SessionTTLSeconds)
			return session, user, nil
		}
	}
	if fallbackToken != "" && token == strings.TrimSpace(fallbackToken) {
		user := User{UserID: "ops-admin", Username: "ops-admin", DisplayName: "Ops Admin", Email: "", Status: "active", Source: "ops_token", Roles: []string{"platform_admin"}, CreatedAt: m.now(), UpdatedAt: m.now()}
		session := m.issueSession(user.UserID, "ops-token", 24*60*60)
		return session, user, nil
	}
	return Session{}, User{}, ErrInvalidCredentials
}

func (m *Manager) LoginWithLocalPassword(providerID string, usernameOrEmail string, password string) (AuthFlowResult, error) {
	provider, err := m.localPasswordProvider(providerID)
	if err != nil {
		return AuthFlowResult{}, err
	}
	usernameOrEmail = strings.TrimSpace(usernameOrEmail)
	password = strings.TrimSpace(password)
	if usernameOrEmail == "" || password == "" {
		return AuthFlowResult{}, ErrInvalidCredentials
	}

	m.mu.RLock()
	users := cloneUsers(m.config.Users)
	m.mu.RUnlock()
	user, ok := lookupUserByCredentials(users, usernameOrEmail)
	if !ok || user.Status != "active" || !user.PasswordLoginEnabled || strings.TrimSpace(user.PasswordHash) == "" {
		return AuthFlowResult{}, ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return AuthFlowResult{}, ErrInvalidCredentials
	}

	result := AuthFlowResult{User: user, ProviderID: provider.ID}
	challengeRequired := provider.RequireChallenge || user.ChallengeRequired
	mfaRequired := provider.RequireMFA || (user.MFAEnabled && strings.EqualFold(user.MFAMethod, "totp") && strings.TrimSpace(user.TOTPSecret) != "")
	if !challengeRequired && !mfaRequired {
		result.Session = m.issueSession(user.UserID, provider.ID, provider.SessionTTLSeconds)
		return result, nil
	}

	flow := PendingAuthFlow{
		UserID:            user.UserID,
		ProviderID:        provider.ID,
		PasswordVerified:  true,
		ChallengeRequired: challengeRequired,
		ChallengeVerified: !challengeRequired,
		MFARequired:       mfaRequired,
		MFAVerified:       !mfaRequired,
	}
	flow = m.storePendingFlow(flow)
	result.PendingToken = flow.Token
	if challengeRequired {
		challenge, err := m.issueChallenge(flow.Token, provider, user)
		if err != nil {
			return AuthFlowResult{}, err
		}
		result.NextStep = "challenge"
		result.ChallengeID = challenge.ID
		result.ChallengeChannel = challenge.Channel
		result.ChallengeCode = challenge.Code
		result.ChallengeExpiresAt = challenge.ExpiresAt
		return result, nil
	}
	result.NextStep = "mfa"
	return result, nil
}

func (m *Manager) StartChallenge(pendingToken string) (AuthFlowResult, error) {
	flow, user, provider, err := m.pendingFlowContext(pendingToken)
	if err != nil {
		return AuthFlowResult{}, err
	}
	if !flow.ChallengeRequired {
		return AuthFlowResult{}, ErrChallengeNotFound
	}
	challenge, err := m.issueChallenge(flow.Token, provider, user)
	if err != nil {
		return AuthFlowResult{}, err
	}
	return AuthFlowResult{
		User:               user,
		ProviderID:         provider.ID,
		PendingToken:       flow.Token,
		NextStep:           "challenge",
		ChallengeID:        challenge.ID,
		ChallengeChannel:   challenge.Channel,
		ChallengeCode:      challenge.Code,
		ChallengeExpiresAt: challenge.ExpiresAt,
	}, nil
}

func (m *Manager) VerifyChallenge(pendingToken string, challengeID string, code string) (AuthFlowResult, error) {
	flow, user, provider, err := m.pendingFlowContext(pendingToken)
	if err != nil {
		return AuthFlowResult{}, err
	}
	if !flow.ChallengeRequired {
		return AuthFlowResult{}, ErrChallengeNotFound
	}
	challenge, err := m.consumeChallenge(flow.Token, challengeID, code)
	if err != nil {
		return AuthFlowResult{}, err
	}
	flow.ChallengeVerified = true
	flow.ChallengeID = challenge.ID
	flow = m.storePendingFlow(flow)
	result := AuthFlowResult{User: user, ProviderID: provider.ID, PendingToken: flow.Token}
	if flow.MFARequired && !flow.MFAVerified {
		result.NextStep = "mfa"
		return result, nil
	}
	result.Session = m.completePendingFlow(flow)
	return result, nil
}

func (m *Manager) VerifyMFA(pendingToken string, code string) (AuthFlowResult, error) {
	flow, user, provider, err := m.pendingFlowContext(pendingToken)
	if err != nil {
		return AuthFlowResult{}, err
	}
	if flow.ChallengeRequired && !flow.ChallengeVerified {
		return AuthFlowResult{}, ErrChallengeRequired
	}
	if !flow.MFARequired {
		return AuthFlowResult{}, ErrMFARequired
	}
	if !strings.EqualFold(user.MFAMethod, "totp") || strings.TrimSpace(user.TOTPSecret) == "" {
		return AuthFlowResult{}, ErrInvalidCredentials
	}
	if !totp.Validate(strings.TrimSpace(code), strings.TrimSpace(user.TOTPSecret)) {
		return AuthFlowResult{}, ErrInvalidCredentials
	}
	flow.MFAVerified = true
	flow = m.storePendingFlow(flow)
	return AuthFlowResult{User: user, ProviderID: provider.ID, PendingToken: flow.Token, Session: m.completePendingFlow(flow)}, nil
}

func (m *Manager) AuthenticateSession(token string) (Principal, bool) {
	if m == nil {
		return Principal{}, false
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return Principal{}, false
	}
	parsed := sessionClaims{}
	if _, err := jwt.ParseWithClaims(token, &parsed, func(t *jwt.Token) (any, error) {
		return m.sessionKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()})); err != nil {
		return Principal{}, false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[token]
	if !ok || session.ExpiresAt.Before(m.now()) {
		delete(m.sessions, token)
		return Principal{}, false
	}
	user, ok := lookupUser(m.config.Users, session.UserID)
	if !ok || user.Status != "active" {
		return Principal{}, false
	}
	session.LastSeenAt = m.now()
	m.sessions[token] = session
	return buildPrincipal("session", token, &user, m.config.Roles, m.config.Groups, session.ProviderID), true
}

func (m *Manager) Logout(token string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	delete(m.sessions, strings.TrimSpace(token))
	m.mu.Unlock()
}

func (m *Manager) StartOAuthLogin(providerID string, baseURL string) (string, error) {
	provider, ok := m.GetAuthProvider(providerID)
	if !ok {
		return "", ErrAuthProviderNotFound
	}
	if !provider.Enabled {
		return "", ErrAuthProviderDisabled
	}
	if provider.Type != "oauth2" && provider.Type != "oidc" {
		return "", ErrInvalidCredentials
	}
	provider, err := m.hydrateOAuthProvider(context.Background(), provider)
	if err != nil {
		return "", err
	}
	state, err := randomToken(24)
	if err != nil {
		return "", err
	}
	redirectURL := strings.TrimRight(baseURL, "/") + firstNonEmpty(provider.RedirectPath, "/api/v1/auth/callback/"+provider.ID)
	conf := oauth2.Config{ClientID: provider.ClientID, ClientSecret: provider.ClientSecret, Endpoint: oauth2.Endpoint{AuthURL: provider.AuthURL, TokenURL: provider.TokenURL}, RedirectURL: redirectURL, Scopes: provider.Scopes}
	m.mu.Lock()
	m.oauthStates[state] = OAuthState{ProviderID: provider.ID, State: state, CreatedAt: m.now(), ExpiresAt: m.now().Add(10 * time.Minute)}
	m.mu.Unlock()
	return conf.AuthCodeURL(state), nil
}

func (m *Manager) CompleteOAuthCallback(ctx context.Context, providerID string, baseURL string, state string, code string) (Session, User, error) {
	provider, ok := m.GetAuthProvider(providerID)
	if !ok {
		return Session{}, User{}, ErrAuthProviderNotFound
	}
	provider, err := m.hydrateOAuthProvider(ctx, provider)
	if err != nil {
		return Session{}, User{}, err
	}
	m.mu.Lock()
	stored, ok := m.oauthStates[strings.TrimSpace(state)]
	if !ok || stored.ProviderID != provider.ID || stored.ExpiresAt.Before(m.now()) {
		m.mu.Unlock()
		return Session{}, User{}, ErrOAuthStateInvalid
	}
	delete(m.oauthStates, state)
	m.mu.Unlock()
	redirectURL := strings.TrimRight(baseURL, "/") + firstNonEmpty(provider.RedirectPath, "/api/v1/auth/callback/"+provider.ID)
	conf := oauth2.Config{ClientID: provider.ClientID, ClientSecret: provider.ClientSecret, Endpoint: oauth2.Endpoint{AuthURL: provider.AuthURL, TokenURL: provider.TokenURL}, RedirectURL: redirectURL, Scopes: provider.Scopes}
	tok, err := conf.Exchange(ctx, code)
	if err != nil {
		return Session{}, User{}, err
	}
	profile, err := m.fetchOAuthProfile(ctx, provider, tok)
	if err != nil {
		return Session{}, User{}, err
	}
	user, err := m.resolveOAuthUser(provider, profile)
	if err != nil {
		return Session{}, User{}, err
	}
	session := m.issueSession(user.UserID, provider.ID, provider.SessionTTLSeconds)
	return session, user, nil
}

func (m *Manager) fetchOAuthProfile(ctx context.Context, provider AuthProvider, token *oauth2.Token) (map[string]any, error) {
	if provider.Type == "oidc" && strings.TrimSpace(provider.IssuerURL) != "" {
		issuerCtx := oidc.ClientContext(ctx, m.client)
		verifier, err := m.oidcVerifier(issuerCtx, provider)
		if err != nil {
			return nil, err
		}
		if rawIDToken, ok := token.Extra("id_token").(string); ok && strings.TrimSpace(rawIDToken) != "" {
			idToken, err := verifier.Verify(issuerCtx, rawIDToken)
			if err == nil {
				var claims map[string]any
				if err := idToken.Claims(&claims); err == nil {
					return claims, nil
				}
			}
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.UserInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("Accept", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("userinfo request failed: %s", resp.Status)
	}
	var profile map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}
	return profile, nil
}

func (m *Manager) resolveOAuthUser(provider AuthProvider, profile map[string]any) (User, error) {
	subject := profileString(profile, provider.UserIDField, "sub", "id")
	username := profileString(profile, provider.UsernameField, "preferred_username", "login", "username")
	displayName := profileString(profile, provider.DisplayNameField, "name")
	email := strings.ToLower(profileString(profile, provider.EmailField, "email"))
	if subject == "" {
		return User{}, ErrInvalidCredentials
	}
	if len(provider.AllowedDomains) > 0 && email != "" {
		allowed := false
		for _, domain := range provider.AllowedDomains {
			if strings.HasSuffix(email, "@"+strings.ToLower(strings.TrimSpace(domain))) {
				allowed = true
				break
			}
		}
		if !allowed {
			return User{}, ErrInvalidCredentials
		}
	}
	m.mu.RLock()
	cfg := cloneConfig(*m.config)
	m.mu.RUnlock()
	for _, user := range cfg.Users {
		for _, identity := range user.Identities {
			if identity.ProviderID == provider.ID && identity.ExternalSubject == subject {
				if identity.ExternalUsername != username || identity.ExternalEmail != email {
					updated := user
					for i := range updated.Identities {
						if updated.Identities[i].ProviderID == provider.ID && updated.Identities[i].ExternalSubject == subject {
							updated.Identities[i].ExternalUsername = username
							updated.Identities[i].ExternalEmail = email
							updated.Source = provider.ID
							if updated.Email == "" && email != "" {
								updated.Email = email
							}
							break
						}
					}
					persisted, err := m.UpsertUser(updated)
					if err != nil {
						return User{}, err
					}
					return persisted, nil
				}
				return user, nil
			}
		}
	}
	if !provider.AllowJIT {
		return User{}, ErrUserNotFound
	}
	user := normalizeUser(User{
		UserID:      firstNonEmpty(slugify(username), slugify(email), slugify(subject)),
		Username:    firstNonEmpty(username, email, subject),
		DisplayName: firstNonEmpty(displayName, username, email, subject),
		Email:       email,
		Status:      "active",
		Source:      provider.ID,
		Roles:       cloneStrings(provider.DefaultRoles),
		Identities: []IdentityLink{{
			ProviderType:     provider.Type,
			ProviderID:       provider.ID,
			ExternalSubject:  subject,
			ExternalUsername: username,
			ExternalEmail:    email,
		}},
	}, m.now())
	if len(user.Roles) == 0 {
		user.Roles = []string{"viewer"}
	}
	created, err := m.UpsertUser(user)
	if err != nil {
		return User{}, err
	}
	return created, nil
}

func (m *Manager) issueSession(userID string, providerID string, ttlSeconds int) Session {
	expiresAt := m.now().Add(sessionTTL(ttlSeconds))
	claims := sessionClaims{
		ProviderID: providerID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(m.now()),
			NotBefore: jwt.NewNumericDate(m.now()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			Issuer:    "tars/access",
			ID:        firstNonEmpty(providerID, "session") + ":" + userID + ":" + fmt.Sprintf("%d", m.now().UnixNano()),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.sessionKey)
	if err != nil {
		token, _ = randomToken(32)
	}
	session := Session{Token: token, UserID: userID, ProviderID: providerID, CreatedAt: m.now(), ExpiresAt: expiresAt, LastSeenAt: m.now()}
	m.mu.Lock()
	m.sessions[token] = session
	m.mu.Unlock()
	return session
}

func (m *Manager) Evaluate(principal Principal, permission string) bool {
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return true
	}
	if _, ok := principal.Permission["*"]; ok {
		return true
	}
	if _, ok := principal.Permission[permission]; ok {
		return true
	}
	m.mu.RLock()
	enforcer := m.enforcer
	m.mu.RUnlock()
	normalizedPermission := normalizePermissionPattern(permission)
	if enforcer == nil {
		parts := strings.Split(permission, ".")
		for i := len(parts) - 1; i > 0; i-- {
			candidate := strings.Join(parts[:i], ".") + ".*"
			if _, ok := principal.Permission[candidate]; ok {
				return true
			}
		}
		return false
	}
	for _, roleID := range principal.RoleIDs {
		ok, err := enforcer.Enforce(roleID, normalizedPermission)
		if err == nil && ok {
			return true
		}
	}
	return false
}

func ParseConfig(content []byte) (*Config, string, error) {
	var raw fileConfig
	if err := yaml.Unmarshal(content, &raw); err != nil {
		return nil, "", err
	}
	cfg := Config{Users: cloneUsers(raw.Access.Users), Groups: cloneGroups(raw.Access.Groups), AuthProviders: cloneAuthProviders(raw.Access.AuthProviders, false), Roles: cloneRoles(raw.Access.Roles), People: clonePeople(raw.Access.People), Channels: cloneChannels(raw.Access.Channels)}
	normalized := normalizeConfig(cfg)
	encoded, err := EncodeConfig(&normalized)
	if err != nil {
		return nil, "", err
	}
	return &normalized, encoded, nil
}

func EncodeConfig(cfg *Config) (string, error) {
	current := DefaultConfig()
	if cfg != nil {
		current = normalizeConfig(*cfg)
	}
	var raw fileConfig
	raw.Access.Users = cloneUsers(current.Users)
	raw.Access.Groups = cloneGroups(current.Groups)
	raw.Access.AuthProviders = cloneAuthProviders(current.AuthProviders, false)
	raw.Access.Roles = cloneRoles(current.Roles)
	raw.Access.People = clonePeople(current.People)
	raw.Access.Channels = cloneChannels(current.Channels)
	bytes, err := yaml.Marshal(raw)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func buildPrincipal(kind string, token string, user *User, roles []Role, groups []Group, source string) Principal {
	permission := map[string]struct{}{}
	roleIDs := []string{}
	if user != nil {
		roleIDs = effectiveRoleIDs(*user, groups)
		for _, role := range roles {
			for _, assigned := range roleIDs {
				if role.ID != assigned {
					continue
				}
				for _, perm := range role.Permissions {
					permission[perm] = struct{}{}
				}
			}
		}
	}
	return Principal{Kind: kind, Token: token, User: user, RoleIDs: roleIDs, Permission: permission, Source: source}
}

func cloneConfig(cfg Config) Config {
	return Config{Users: cloneUsers(cfg.Users), Groups: cloneGroups(cfg.Groups), AuthProviders: cloneAuthProviders(cfg.AuthProviders, false), Roles: cloneRoles(cfg.Roles), People: clonePeople(cfg.People), Channels: cloneChannels(cfg.Channels)}
}

func normalizeConfig(cfg Config) Config {
	now := time.Now().UTC()
	seenRoles := map[string]struct{}{}
	roles := cloneRoles(cfg.Roles)
	for _, role := range defaultRoles() {
		seenRoles[role.ID] = struct{}{}
	}
	for _, role := range roles {
		seenRoles[role.ID] = struct{}{}
	}
	roles = append(roles, defaultRoles()...)
	for i := range roles {
		roles[i] = normalizeRole(roles[i])
	}
	roles = dedupeRoles(roles)
	users := cloneUsers(cfg.Users)
	for i := range users {
		users[i] = normalizeUser(users[i], now)
	}
	groups := cloneGroups(cfg.Groups)
	for i := range groups {
		groups[i] = normalizeGroup(groups[i], now)
	}
	providers := cloneAuthProviders(cfg.AuthProviders, false)
	for i := range providers {
		providers[i] = normalizeAuthProvider(providers[i])
	}
	users, groups = syncGroupMemberships(users, groups)
	people := clonePeople(cfg.People)
	for i := range people {
		people[i] = normalizePerson(people[i], now)
	}
	channels := cloneChannels(cfg.Channels)
	for i := range channels {
		channels[i] = normalizeChannel(channels[i], now)
	}
	sort.SliceStable(users, func(i, j int) bool { return users[i].UserID < users[j].UserID })
	sort.SliceStable(groups, func(i, j int) bool { return groups[i].GroupID < groups[j].GroupID })
	sort.SliceStable(providers, func(i, j int) bool { return providers[i].ID < providers[j].ID })
	sort.SliceStable(roles, func(i, j int) bool { return roles[i].ID < roles[j].ID })
	sort.SliceStable(people, func(i, j int) bool { return people[i].ID < people[j].ID })
	sort.SliceStable(channels, func(i, j int) bool { return channels[i].ID < channels[j].ID })
	return Config{Users: dedupeUsers(users), Groups: dedupeGroups(groups), AuthProviders: dedupeProviders(providers), Roles: roles, People: dedupePeople(people), Channels: dedupeChannels(channels)}
}

func normalizeUser(user User, now time.Time) User {
	user.UserID = firstNonEmpty(strings.TrimSpace(user.UserID), slugify(user.Username), slugify(user.Email))
	user.Username = firstNonEmpty(strings.TrimSpace(user.Username), user.UserID)
	user.DisplayName = firstNonEmpty(strings.TrimSpace(user.DisplayName), user.Username, user.UserID)
	user.Email = strings.ToLower(strings.TrimSpace(user.Email))
	user.Status = firstNonEmpty(strings.TrimSpace(user.Status), "active")
	user.Source = firstNonEmpty(strings.TrimSpace(user.Source), "local")
	user.PasswordHash = strings.TrimSpace(user.PasswordHash)
	user.MFAMethod = strings.TrimSpace(user.MFAMethod)
	user.TOTPSecret = strings.TrimSpace(user.TOTPSecret)
	user.Roles = dedupeStrings(user.Roles)
	user.Groups = dedupeStrings(user.Groups)
	if len(user.Roles) == 0 {
		user.Roles = []string{"viewer"}
	}
	if user.PasswordHash != "" && user.PasswordUpdatedAt.IsZero() {
		user.PasswordUpdatedAt = now
	}
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now
	return user
}

func normalizeGroup(group Group, now time.Time) Group {
	group.GroupID = firstNonEmpty(strings.TrimSpace(group.GroupID), slugify(group.DisplayName))
	group.DisplayName = firstNonEmpty(strings.TrimSpace(group.DisplayName), group.GroupID)
	group.Description = strings.TrimSpace(group.Description)
	group.Status = firstNonEmpty(strings.TrimSpace(group.Status), "active")
	group.Roles = dedupeStrings(group.Roles)
	group.Members = dedupeStrings(group.Members)
	if group.CreatedAt.IsZero() {
		group.CreatedAt = now
	}
	group.UpdatedAt = now
	return group
}

func normalizeAuthProvider(provider AuthProvider) AuthProvider {
	provider.ID = strings.TrimSpace(provider.ID)
	provider.Type = firstNonEmpty(strings.TrimSpace(provider.Type), "oauth2")
	provider.Name = firstNonEmpty(strings.TrimSpace(provider.Name), provider.ID)
	provider.ChallengeChannel = firstNonEmpty(strings.TrimSpace(provider.ChallengeChannel), "builtin")
	provider.IssuerURL = strings.TrimSpace(provider.IssuerURL)
	provider.AuthURL = strings.TrimSpace(provider.AuthURL)
	provider.TokenURL = strings.TrimSpace(provider.TokenURL)
	provider.UserInfoURL = strings.TrimSpace(provider.UserInfoURL)
	provider.ClientID = strings.TrimSpace(provider.ClientID)
	provider.ClientSecret = strings.TrimSpace(provider.ClientSecret)
	provider.ClientSecretRef = strings.TrimSpace(provider.ClientSecretRef)
	provider.LDAPURL = strings.TrimSpace(provider.LDAPURL)
	provider.BindDN = strings.TrimSpace(provider.BindDN)
	provider.BindPassword = strings.TrimSpace(provider.BindPassword)
	provider.BindPasswordRef = strings.TrimSpace(provider.BindPasswordRef)
	provider.BaseDN = strings.TrimSpace(provider.BaseDN)
	provider.UserSearchFilter = strings.TrimSpace(provider.UserSearchFilter)
	provider.GroupSearchFilter = strings.TrimSpace(provider.GroupSearchFilter)
	provider.RedirectPath = firstNonEmpty(strings.TrimSpace(provider.RedirectPath), "/api/v1/auth/callback/"+provider.ID)
	provider.SuccessRedirect = firstNonEmpty(strings.TrimSpace(provider.SuccessRedirect), "/login")
	provider.UserIDField = strings.TrimSpace(provider.UserIDField)
	provider.UsernameField = strings.TrimSpace(provider.UsernameField)
	provider.DisplayNameField = strings.TrimSpace(provider.DisplayNameField)
	provider.EmailField = strings.TrimSpace(provider.EmailField)
	provider.AllowedDomains = dedupeStrings(provider.AllowedDomains)
	provider.DefaultRoles = dedupeStrings(provider.DefaultRoles)
	provider.Scopes = dedupeStrings(provider.Scopes)
	if provider.PasswordMinLength <= 0 {
		provider.PasswordMinLength = 8
	}
	if provider.ChallengeTTLSeconds <= 0 {
		provider.ChallengeTTLSeconds = 300
	}
	if provider.ChallengeCodeLength <= 0 {
		provider.ChallengeCodeLength = 6
	}
	if provider.Type == "local_token" && len(provider.DefaultRoles) == 0 {
		provider.DefaultRoles = []string{"ops_admin"}
	}
	return provider
}

func normalizeRole(role Role) Role {
	role.ID = strings.TrimSpace(role.ID)
	role.DisplayName = firstNonEmpty(strings.TrimSpace(role.DisplayName), role.ID)
	role.Permissions = dedupeStrings(role.Permissions)
	return role
}

func normalizePerson(person Person, now time.Time) Person {
	person.ID = firstNonEmpty(strings.TrimSpace(person.ID), slugify(person.DisplayName), slugify(person.Email))
	person.DisplayName = firstNonEmpty(strings.TrimSpace(person.DisplayName), person.ID)
	person.Email = strings.ToLower(strings.TrimSpace(person.Email))
	person.Status = firstNonEmpty(strings.TrimSpace(person.Status), "active")
	person.LinkedUserID = strings.TrimSpace(person.LinkedUserID)
	person.ChannelIDs = dedupeStrings(person.ChannelIDs)
	person.Team = strings.TrimSpace(person.Team)
	person.ApprovalTarget = strings.TrimSpace(person.ApprovalTarget)
	person.OncallSchedule = strings.TrimSpace(person.OncallSchedule)
	person.Preferences = cloneStringMap(person.Preferences)
	if person.CreatedAt.IsZero() {
		person.CreatedAt = now
	}
	person.UpdatedAt = now
	return person
}

func normalizeChannel(channel Channel, now time.Time) Channel {
	channel.ID = firstNonEmpty(strings.TrimSpace(channel.ID), slugify(channel.Name), slugify(channel.Target))
	channel.Kind = firstNonEmpty(strings.TrimSpace(channel.Kind), strings.TrimSpace(channel.Type), "web")
	channel.Type = firstNonEmpty(strings.TrimSpace(channel.Type), channel.Kind, "web")
	channel.Name = firstNonEmpty(strings.TrimSpace(channel.Name), channel.ID)
	channel.Target = strings.TrimSpace(channel.Target)
	channel.LinkedUsers = dedupeStrings(channel.LinkedUsers)
	if len(channel.Usages) == 0 {
		channel.Usages = channelUsagesFromCapabilities(channel.Capabilities)
	}
	if len(channel.Capabilities) == 0 {
		channel.Capabilities = channelCapabilitiesFromUsages(channel.Usages)
	}
	channel.Usages = dedupeChannelUsages(channel.Usages)
	channel.Capabilities = dedupeChannelCapabilities(channel.Capabilities)
	if channel.CreatedAt.IsZero() {
		channel.CreatedAt = now
	}
	channel.UpdatedAt = now
	return channel
}

func cloneUsers(items []User) []User {
	if len(items) == 0 {
		return nil
	}
	out := make([]User, 0, len(items))
	for _, item := range items {
		copy := item
		copy.Roles = cloneStrings(item.Roles)
		copy.Groups = cloneStrings(item.Groups)
		copy.Identities = append([]IdentityLink(nil), item.Identities...)
		out = append(out, copy)
	}
	return out
}

func cloneGroups(items []Group) []Group {
	if len(items) == 0 {
		return nil
	}
	out := make([]Group, 0, len(items))
	for _, item := range items {
		copy := item
		copy.Roles = cloneStrings(item.Roles)
		copy.Members = cloneStrings(item.Members)
		out = append(out, copy)
	}
	return out
}

func cloneAuthProviders(items []AuthProvider, public bool) []AuthProvider {
	if len(items) == 0 {
		return nil
	}
	out := make([]AuthProvider, 0, len(items))
	for _, item := range items {
		copy := item
		copy.Scopes = cloneStrings(item.Scopes)
		copy.AllowedDomains = cloneStrings(item.AllowedDomains)
		copy.DefaultRoles = cloneStrings(item.DefaultRoles)
		if public {
			copy.ClientSecret = ""
			copy.ClientSecretRef = ""
			copy.BindPassword = ""
			copy.BindPasswordRef = ""
		}
		out = append(out, copy)
	}
	return out
}

func cloneRoles(items []Role) []Role {
	if len(items) == 0 {
		return nil
	}
	out := make([]Role, 0, len(items))
	for _, item := range items {
		copy := item
		copy.Permissions = cloneStrings(item.Permissions)
		out = append(out, copy)
	}
	return out
}

func clonePeople(items []Person) []Person {
	if len(items) == 0 {
		return nil
	}
	out := make([]Person, 0, len(items))
	for _, item := range items {
		copy := item
		copy.ChannelIDs = cloneStrings(item.ChannelIDs)
		copy.Preferences = cloneStringMap(item.Preferences)
		out = append(out, copy)
	}
	return out
}

func cloneChannels(items []Channel) []Channel {
	if len(items) == 0 {
		return nil
	}
	out := make([]Channel, 0, len(items))
	for _, item := range items {
		copy := item
		copy.LinkedUsers = cloneStrings(item.LinkedUsers)
		copy.Usages = cloneChannelUsages(item.Usages)
		copy.Capabilities = cloneChannelCapabilities(item.Capabilities)
		out = append(out, copy)
	}
	return out
}

func dedupeUsers(items []User) []User {
	seen := map[string]struct{}{}
	out := make([]User, 0, len(items))
	for _, item := range items {
		if item.UserID == "" {
			continue
		}
		if _, ok := seen[item.UserID]; ok {
			continue
		}
		seen[item.UserID] = struct{}{}
		out = append(out, item)
	}
	return out
}

func dedupeProviders(items []AuthProvider) []AuthProvider {
	seen := map[string]struct{}{}
	out := make([]AuthProvider, 0, len(items))
	for _, item := range items {
		if item.ID == "" {
			continue
		}
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}
		out = append(out, item)
	}
	return out
}

func dedupeGroups(items []Group) []Group {
	seen := map[string]struct{}{}
	out := make([]Group, 0, len(items))
	for _, item := range items {
		if item.GroupID == "" {
			continue
		}
		if _, ok := seen[item.GroupID]; ok {
			continue
		}
		seen[item.GroupID] = struct{}{}
		out = append(out, item)
	}
	return out
}

func dedupeRoles(items []Role) []Role {
	seen := map[string]struct{}{}
	out := make([]Role, 0, len(items))
	for _, item := range items {
		if item.ID == "" {
			continue
		}
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}
		out = append(out, item)
	}
	return out
}

func dedupePeople(items []Person) []Person {
	seen := map[string]struct{}{}
	out := make([]Person, 0, len(items))
	for _, item := range items {
		if item.ID == "" {
			continue
		}
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}
		out = append(out, item)
	}
	return out
}

func dedupeChannels(items []Channel) []Channel {
	seen := map[string]struct{}{}
	out := make([]Channel, 0, len(items))
	for _, item := range items {
		if item.ID == "" {
			continue
		}
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}
		out = append(out, item)
	}
	return out
}

func dedupeStrings(items []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func cloneStrings(items []string) []string { return append([]string(nil), items...) }

func cloneChannelUsages(items []ChannelUsage) []ChannelUsage {
	return append([]ChannelUsage(nil), items...)
}

func cloneChannelCapabilities(items []ChannelCapability) []ChannelCapability {
	return append([]ChannelCapability(nil), items...)
}

func channelUsagesFromCapabilities(items []ChannelCapability) []ChannelUsage {
	out := make([]ChannelUsage, 0, len(items))
	for _, item := range items {
		out = append(out, ChannelUsage(item))
	}
	return out
}

func channelCapabilitiesFromUsages(items []ChannelUsage) []ChannelCapability {
	out := make([]ChannelCapability, 0, len(items))
	for _, item := range items {
		out = append(out, ChannelCapability(item))
	}
	return out
}

func dedupeChannelUsages(items []ChannelUsage) []ChannelUsage {
	seen := map[ChannelUsage]struct{}{}
	out := make([]ChannelUsage, 0, len(items))
	for _, item := range items {
		trimmed := ChannelUsage(strings.TrimSpace(string(item)))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func dedupeChannelCapabilities(items []ChannelCapability) []ChannelCapability {
	seen := map[ChannelCapability]struct{}{}
	out := make([]ChannelCapability, 0, len(items))
	for _, item := range items {
		trimmed := ChannelCapability(strings.TrimSpace(string(item)))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func cloneStringMap(items map[string]string) map[string]string {
	if len(items) == 0 {
		return nil
	}
	out := make(map[string]string, len(items))
	for key, value := range items {
		trimmed := strings.TrimSpace(key)
		if trimmed == "" {
			continue
		}
		out[trimmed] = strings.TrimSpace(value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func effectiveRoleIDs(user User, groups []Group) []string {
	roles := dedupeStrings(user.Roles)
	for _, group := range groups {
		if group.Status != "active" {
			continue
		}
		if containsString(user.Groups, group.GroupID) || containsString(group.Members, user.UserID) {
			roles = dedupeStrings(append(roles, group.Roles...))
		}
	}
	return roles
}

// filterStrings returns items with all occurrences of exclude removed.
func filterStrings(items []string, exclude string) []string {
	out := items[:0:0]
	for _, s := range items {
		if strings.TrimSpace(s) != strings.TrimSpace(exclude) {
			out = append(out, s)
		}
	}
	return out
}

func containsString(items []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, item := range items {
		if strings.TrimSpace(item) == want {
			return true
		}
	}
	return false
}

func syncGroupMemberships(users []User, groups []Group) ([]User, []Group) {
	userIndex := make(map[string]int, len(users))
	groupIndex := make(map[string]int, len(groups))
	for i := range users {
		userIndex[users[i].UserID] = i
	}
	for i := range groups {
		groupIndex[groups[i].GroupID] = i
	}
	for i := range users {
		for _, groupID := range users[i].Groups {
			if idx, ok := groupIndex[groupID]; ok {
				groups[idx].Members = dedupeStrings(append(groups[idx].Members, users[i].UserID))
			}
		}
	}
	for i := range groups {
		for _, userID := range groups[i].Members {
			if idx, ok := userIndex[userID]; ok {
				users[idx].Groups = dedupeStrings(append(users[idx].Groups, groups[i].GroupID))
			}
		}
	}
	return users, groups
}

func lookupUser(users []User, id string) (User, bool) {
	for _, user := range users {
		if user.UserID == id {
			return user, true
		}
	}
	return User{}, false
}

func lookupUserByCredentials(users []User, value string) (User, bool) {
	needle := strings.ToLower(strings.TrimSpace(value))
	for _, user := range users {
		if strings.EqualFold(user.UserID, needle) || strings.EqualFold(user.Username, needle) || strings.EqualFold(user.Email, needle) {
			return user, true
		}
	}
	return User{}, false
}

func (m *Manager) localPasswordProvider(providerID string) (AuthProvider, error) {
	providerID = strings.TrimSpace(providerID)
	m.mu.RLock()
	providers := cloneAuthProviders(m.config.AuthProviders, false)
	m.mu.RUnlock()
	for _, provider := range providers {
		if provider.Type != "local_password" {
			continue
		}
		if providerID != "" && provider.ID != providerID {
			continue
		}
		if !provider.Enabled {
			return AuthProvider{}, ErrAuthProviderDisabled
		}
		return provider, nil
	}
	return AuthProvider{}, ErrAuthProviderNotFound
}

func (m *Manager) pendingFlowContext(pendingToken string) (PendingAuthFlow, User, AuthProvider, error) {
	m.cleanupExpiredAuthState()
	pendingToken = strings.TrimSpace(pendingToken)
	if pendingToken == "" {
		return PendingAuthFlow{}, User{}, AuthProvider{}, ErrAuthFlowNotFound
	}
	m.mu.RLock()
	flow, ok := m.pendingAuth[pendingToken]
	users := cloneUsers(m.config.Users)
	providers := cloneAuthProviders(m.config.AuthProviders, false)
	m.mu.RUnlock()
	if !ok || flow.ExpiresAt.Before(m.now()) {
		return PendingAuthFlow{}, User{}, AuthProvider{}, ErrAuthFlowNotFound
	}
	user, ok := lookupUser(users, flow.UserID)
	if !ok {
		return PendingAuthFlow{}, User{}, AuthProvider{}, ErrUserNotFound
	}
	for _, provider := range providers {
		if provider.ID == flow.ProviderID {
			return flow, user, provider, nil
		}
	}
	return PendingAuthFlow{}, User{}, AuthProvider{}, ErrAuthProviderNotFound
}

func (m *Manager) storePendingFlow(flow PendingAuthFlow) PendingAuthFlow {
	m.cleanupExpiredAuthState()
	if strings.TrimSpace(flow.Token) == "" {
		flow.Token, _ = randomToken(24)
		flow.CreatedAt = m.now()
	}
	if flow.ExpiresAt.IsZero() {
		flow.ExpiresAt = m.now().Add(10 * time.Minute)
	}
	m.mu.Lock()
	m.pendingAuth[flow.Token] = flow
	m.mu.Unlock()
	return flow
}

func (m *Manager) issueChallenge(pendingToken string, provider AuthProvider, user User) (ChallengeState, error) {
	m.cleanupExpiredAuthState()
	code, err := randomDigits(provider.ChallengeCodeLength)
	if err != nil {
		return ChallengeState{}, err
	}
	challengeID, err := randomToken(12)
	if err != nil {
		return ChallengeState{}, err
	}
	challenge := ChallengeState{
		ID:           challengeID,
		PendingToken: pendingToken,
		UserID:       user.UserID,
		ProviderID:   provider.ID,
		Code:         code,
		Channel:      provider.ChallengeChannel,
		CreatedAt:    m.now(),
		ExpiresAt:    m.now().Add(time.Duration(provider.ChallengeTTLSeconds) * time.Second),
		MaxAttempts:  5,
	}
	m.mu.Lock()
	flow := m.pendingAuth[pendingToken]
	flow.ChallengeID = challengeID
	m.pendingAuth[pendingToken] = flow
	m.challenges[challengeID] = challenge
	m.mu.Unlock()
	return challenge, nil
}

func (m *Manager) consumeChallenge(pendingToken string, challengeID string, code string) (ChallengeState, error) {
	m.cleanupExpiredAuthState()
	challengeID = strings.TrimSpace(challengeID)
	code = strings.TrimSpace(code)
	m.mu.Lock()
	defer m.mu.Unlock()
	challenge, ok := m.challenges[challengeID]
	if !ok || challenge.PendingToken != strings.TrimSpace(pendingToken) || challenge.ExpiresAt.Before(m.now()) || challenge.Consumed {
		return ChallengeState{}, ErrChallengeNotFound
	}
	if challenge.Attempts >= challenge.MaxAttempts {
		delete(m.challenges, challengeID)
		delete(m.pendingAuth, pendingToken)
		return ChallengeState{}, ErrInvalidCredentials
	}
	if challenge.Code != code {
		challenge.Attempts++
		m.challenges[challengeID] = challenge
		return ChallengeState{}, ErrInvalidCredentials
	}
	challenge.Consumed = true
	delete(m.challenges, challengeID)
	return challenge, nil
}

func (m *Manager) completePendingFlow(flow PendingAuthFlow) Session {
	session := m.issueSession(flow.UserID, flow.ProviderID, 0)
	m.mu.Lock()
	delete(m.pendingAuth, flow.Token)
	if strings.TrimSpace(flow.ChallengeID) != "" {
		delete(m.challenges, flow.ChallengeID)
	}
	m.mu.Unlock()
	return session
}

func (m *Manager) cleanupExpiredAuthState() {
	if m == nil {
		return
	}
	now := m.now()
	m.mu.Lock()
	for token, flow := range m.pendingAuth {
		if flow.ExpiresAt.Before(now) {
			delete(m.pendingAuth, token)
		}
	}
	for id, challenge := range m.challenges {
		if challenge.ExpiresAt.Before(now) || challenge.Consumed {
			delete(m.challenges, id)
		}
	}
	m.mu.Unlock()
}

func resolveLocalTokenUser(users []User, provider AuthProvider) User {
	for _, user := range users {
		for _, identity := range user.Identities {
			if identity.ProviderID == provider.ID {
				return user
			}
		}
	}
	return normalizeUser(User{UserID: provider.ID + "-admin", Username: provider.ID + "-admin", DisplayName: provider.Name, Status: "active", Source: provider.ID, Roles: firstNonEmptySlice(provider.DefaultRoles, []string{"ops_admin"}), Identities: []IdentityLink{{ProviderType: provider.Type, ProviderID: provider.ID, ExternalSubject: provider.ID}}}, time.Now().UTC())
}

func defaultRoles() []Role {
	return []Role{
		{
			ID:          "platform_admin",
			DisplayName: "Platform Admin",
			Permissions: []string{"*"},
		},
		{
			ID:          "ops_admin",
			DisplayName: "Ops Admin",
			Permissions: []string{
				"platform.read", "platform.write",
				"sessions.*", "executions.*",
				"skills.*", "connectors.*", "providers.*", "channels.*",
				"people.*", "users.read", "users.write",
				"groups.read", "groups.write",
				"roles.read", "roles.write",
				"auth.read", "auth.write",
				"ssh_credentials.read", "ssh_credentials.write", "ssh_credentials.use",
				"audit.read", "knowledge.read", "knowledge.write",
				"outbox.*", "configs.*",
			},
		},
		{
			ID:          "approver",
			DisplayName: "Approver",
			Permissions: []string{
				"platform.read",
				"sessions.read", "executions.read", "executions.approve",
				"outbox.read",
				"people.read", "channels.read", "users.read", "groups.read", "roles.read",
				"skills.read", "connectors.read", "ssh_credentials.read",
			},
		},
		{
			ID:          "operator",
			DisplayName: "Operator",
			Permissions: []string{
				"platform.read",
				"sessions.read", "executions.read",
				"skills.read", "connectors.read", "providers.read", "ssh_credentials.read",
				"people.read", "channels.read", "users.read",
				"groups.read", "roles.read",
				"audit.read",
			},
		},
		{
			ID:          "viewer",
			DisplayName: "Viewer",
			Permissions: []string{
				"platform.read",
				"sessions.read", "executions.read",
				"skills.read", "connectors.read", "providers.read", "ssh_credentials.read",
				"people.read", "channels.read",
				"users.read", "groups.read", "roles.read",
				"auth.read",
				"knowledge.read", "audit.read",
			},
		},
		{
			ID:          "knowledge_admin",
			DisplayName: "Knowledge Admin",
			Permissions: []string{
				"platform.read",
				"knowledge.*", "audit.read",
				"sessions.read",
			},
		},
		// ORG-N3: Delegated Admin roles
		{
			ID:          "org_admin",
			DisplayName: "Organization Admin",
			Permissions: []string{
				"platform.read", "platform.write",
				"org.read", "org.write",
				"tenants.read", "tenants.write",
				"workspaces.read", "workspaces.write",
				"org_policy.read", "org_policy.write",
				"users.read", "users.write",
				"groups.read", "groups.write",
				"roles.read", "roles.write",
				"people.read", "people.write",
				"channels.read",
				"auth.read",
				"audit.read",
				"configs.read",
			},
		},
		{
			ID:          "tenant_admin",
			DisplayName: "Tenant Admin",
			Permissions: []string{
				"platform.read",
				"org.read",
				"tenants.read", "tenants.write",
				"workspaces.read", "workspaces.write",
				"org_policy.read",
				"users.read", "users.write",
				"groups.read", "groups.write",
				"roles.read",
				"people.read", "people.write",
				"channels.read",
				"auth.read",
				"audit.read",
			},
		},
	}
}

func buildPermissionEnforcer(cfg Config) *casbin.Enforcer {
	modelText := `
[request_definition]
r = sub, obj

[policy_definition]
p = sub, obj

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = (g(r.sub, p.sub) || r.sub == p.sub) && keyMatch2(r.obj, p.obj)
`
	m, err := casbinmodel.NewModelFromString(modelText)
	if err != nil {
		return nil
	}
	enforcer, err := casbin.NewEnforcer(m)
	if err != nil {
		return nil
	}
	for _, role := range cfg.Roles {
		for _, perm := range role.Permissions {
			_, _ = enforcer.AddPolicy(role.ID, normalizePermissionPattern(perm))
		}
	}
	return enforcer
}

func normalizePermissionPattern(permission string) string {
	permission = strings.TrimSpace(permission)
	if permission == "" {
		return ""
	}
	if permission == "*" {
		return "*"
	}
	if strings.HasSuffix(permission, ".*") {
		return strings.TrimSuffix(permission, ".*") + "/*"
	}
	return strings.ReplaceAll(permission, ".", "/")
}

func sessionTTL(ttlSeconds int) time.Duration {
	if ttlSeconds <= 0 {
		return 12 * time.Hour
	}
	return time.Duration(ttlSeconds) * time.Second
}

func (m *Manager) hydrateOAuthProvider(ctx context.Context, provider AuthProvider) (AuthProvider, error) {
	if provider.Type != "oidc" || strings.TrimSpace(provider.IssuerURL) == "" {
		return provider, nil
	}
	if strings.TrimSpace(provider.AuthURL) != "" && strings.TrimSpace(provider.TokenURL) != "" && strings.TrimSpace(provider.UserInfoURL) != "" {
		return provider, nil
	}
	providerCtx := oidc.ClientContext(ctx, m.client)
	discovery, err := oidc.NewProvider(providerCtx, provider.IssuerURL)
	if err != nil {
		return provider, err
	}
	provider.AuthURL = firstNonEmpty(provider.AuthURL, discovery.Endpoint().AuthURL)
	provider.TokenURL = firstNonEmpty(provider.TokenURL, discovery.Endpoint().TokenURL)
	var metadata struct {
		UserInfoEndpoint string `json:"userinfo_endpoint"`
	}
	if err := discovery.Claims(&metadata); err == nil {
		provider.UserInfoURL = firstNonEmpty(provider.UserInfoURL, metadata.UserInfoEndpoint)
	}
	provider.Scopes = firstNonEmptySlice(provider.Scopes, []string{"openid", "profile", "email"})
	return provider, nil
}

func (m *Manager) oidcVerifier(ctx context.Context, provider AuthProvider) (*oidc.IDTokenVerifier, error) {
	discovery, err := oidc.NewProvider(oidc.ClientContext(ctx, m.client), provider.IssuerURL)
	if err != nil {
		return nil, err
	}
	return discovery.Verifier(&oidc.Config{ClientID: provider.ClientID}), nil
}

func profileString(profile map[string]any, keys ...string) string {
	for _, key := range keys {
		if key == "" {
			continue
		}
		if value, ok := profile[key]; ok {
			switch typed := value.(type) {
			case string:
				if strings.TrimSpace(typed) != "" {
					return strings.TrimSpace(typed)
				}
			case float64:
				return fmt.Sprintf("%.0f", typed)
			}
		}
	}
	return ""
}

func slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func randomDigits(size int) (string, error) {
	if size <= 0 {
		size = 6
	}
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	var b strings.Builder
	b.Grow(size)
	for _, value := range buf {
		b.WriteByte('0' + (value % 10))
	}
	return b.String(), nil
}

func writeFileAtomically(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".access-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstNonEmptySlice(value []string, fallback []string) []string {
	if len(value) > 0 {
		return value
	}
	return fallback
}
