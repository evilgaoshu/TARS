package access

import (
	"strings"
	"testing"
	"time"
)

// ── helper ──────────────────────────────────────────────────────────────────

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	m, err := NewManager("")
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

// ── DefaultConfig & roles ────────────────────────────────────────────────────

func TestDefaultRoles(t *testing.T) {
	wantIDs := []string{"approver", "knowledge_admin", "ops_admin", "operator", "org_admin", "platform_admin", "tenant_admin", "viewer"}
	roles := defaultRoles()
	ids := make([]string, 0, len(roles))
	for _, r := range roles {
		ids = append(ids, r.ID)
	}
	// sort for comparison
	sortStrings(ids)
	sortStrings(wantIDs)
	if strings.Join(ids, ",") != strings.Join(wantIDs, ",") {
		t.Errorf("defaultRoles IDs = %v, want %v", ids, wantIDs)
	}
}

func TestPlatformAdminHasStarPermission(t *testing.T) {
	for _, role := range defaultRoles() {
		if role.ID == "platform_admin" {
			for _, perm := range role.Permissions {
				if perm == "*" {
					return
				}
			}
			t.Errorf("platform_admin does not have wildcard permission")
		}
	}
}

func TestViewerHasReadPermissions(t *testing.T) {
	m := newTestManager(t)
	user := User{UserID: "viewer-user", Username: "viewer-user", Status: "active", Source: "test", Roles: []string{"viewer"}}
	if _, err := m.UpsertUser(user); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	principal, ok := m.AuthenticateSession(issueTestSession(m, "viewer-user"))
	if !ok {
		t.Fatal("AuthenticateSession failed")
	}
	for _, perm := range []string{"platform.read", "sessions.read", "executions.read", "skills.read", "connectors.read", "providers.read", "people.read", "channels.read", "knowledge.read", "audit.read", "ssh_credentials.read"} {
		if !m.Evaluate(principal, perm) {
			t.Errorf("viewer should have permission %q", perm)
		}
	}
	// viewer should NOT have write permissions
	for _, perm := range []string{"configs.write", "users.write", "platform.write", "executions.write", "ssh_credentials.write", "ssh_credentials.use"} {
		if m.Evaluate(principal, perm) {
			t.Errorf("viewer should NOT have permission %q", perm)
		}
	}
}

func TestOpsAdminHasRequiredPermissions(t *testing.T) {
	m := newTestManager(t)
	user := User{UserID: "ops-admin-user", Username: "ops-admin-user", Status: "active", Source: "test", Roles: []string{"ops_admin"}}
	if _, err := m.UpsertUser(user); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	principal, ok := m.AuthenticateSession(issueTestSession(m, "ops-admin-user"))
	if !ok {
		t.Fatal("AuthenticateSession failed")
	}
	for _, perm := range []string{"platform.read", "platform.write", "sessions.read", "skills.write", "connectors.write", "configs.read", "configs.write", "users.read", "users.write", "auth.read", "ssh_credentials.read", "ssh_credentials.write", "ssh_credentials.use"} {
		if !m.Evaluate(principal, perm) {
			t.Errorf("ops_admin should have permission %q", perm)
		}
	}
}

// ── RBAC evaluation ──────────────────────────────────────────────────────────

func TestEvaluateWildcard(t *testing.T) {
	m := newTestManager(t)
	// platform_admin gets * -> should match anything
	user := User{UserID: "padmin", Username: "padmin", Status: "active", Source: "test", Roles: []string{"platform_admin"}}
	if _, err := m.UpsertUser(user); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	principal, ok := m.AuthenticateSession(issueTestSession(m, "padmin"))
	if !ok {
		t.Fatal("AuthenticateSession failed")
	}
	for _, perm := range []string{"*", "sessions.read", "users.write", "arbitrary.permission", "connectors.invoke"} {
		if !m.Evaluate(principal, perm) {
			t.Errorf("platform_admin should have %q via wildcard", perm)
		}
	}
}

func TestEvaluatePrefixWildcard(t *testing.T) {
	m := newTestManager(t)
	// ops_admin has sessions.* -> should match sessions.read, sessions.write etc.
	user := User{UserID: "opsadm", Username: "opsadm", Status: "active", Source: "test", Roles: []string{"ops_admin"}}
	if _, err := m.UpsertUser(user); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	principal, ok := m.AuthenticateSession(issueTestSession(m, "opsadm"))
	if !ok {
		t.Fatal("AuthenticateSession failed")
	}
	if !m.Evaluate(principal, "sessions.read") {
		t.Error("ops_admin should have sessions.read via sessions.*")
	}
	if !m.Evaluate(principal, "sessions.write") {
		t.Error("ops_admin should have sessions.write via sessions.*")
	}
}

func TestEvaluateEmptyPermissions(t *testing.T) {
	m := newTestManager(t)
	principal := Principal{Permission: map[string]struct{}{}}
	// empty permission string is always allowed (no restriction)
	if !m.Evaluate(principal, "") {
		t.Error("empty permission string should always be allowed (no restriction)")
	}
}

// ── User CRUD ────────────────────────────────────────────────────────────────

func TestUpsertAndGetUser(t *testing.T) {
	m := newTestManager(t)
	user := User{UserID: "u1", Username: "alice", DisplayName: "Alice", Email: "alice@example.com", Status: "active", Source: "local"}
	created, err := m.UpsertUser(user)
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if created.UserID != "u1" {
		t.Errorf("UserID = %q, want %q", created.UserID, "u1")
	}
	got, ok := m.GetUser("u1")
	if !ok {
		t.Fatal("GetUser: not found")
	}
	if got.Email != "alice@example.com" {
		t.Errorf("Email = %q, want alice@example.com", got.Email)
	}
}

func TestSetUserStatus(t *testing.T) {
	m := newTestManager(t)
	user := User{UserID: "u2", Username: "bob", Status: "active", Source: "local"}
	if _, err := m.UpsertUser(user); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	updated, err := m.SetUserStatus("u2", "disabled")
	if err != nil {
		t.Fatalf("SetUserStatus: %v", err)
	}
	if updated.Status != "disabled" {
		t.Errorf("Status = %q, want disabled", updated.Status)
	}
}

func TestGetUserNotFound(t *testing.T) {
	m := newTestManager(t)
	if _, ok := m.GetUser("nonexistent"); ok {
		t.Error("expected not found, got found")
	}
}

// ── Group CRUD ───────────────────────────────────────────────────────────────

func TestUpsertAndGetGroup(t *testing.T) {
	m := newTestManager(t)
	group := Group{GroupID: "g1", DisplayName: "SRE Team", Status: "active", Roles: []string{"operator"}}
	created, err := m.UpsertGroup(group)
	if err != nil {
		t.Fatalf("UpsertGroup: %v", err)
	}
	if created.GroupID != "g1" {
		t.Errorf("GroupID = %q, want g1", created.GroupID)
	}
	got, ok := m.GetGroup("g1")
	if !ok {
		t.Fatal("GetGroup: not found")
	}
	if len(got.Roles) == 0 || got.Roles[0] != "operator" {
		t.Errorf("Roles = %v, want [operator]", got.Roles)
	}
}

// ── Role binding ──────────────────────────────────────────────────────────────

func TestBindRoleToUser(t *testing.T) {
	m := newTestManager(t)
	user := User{UserID: "u3", Username: "carol", Status: "active", Source: "local", Roles: []string{"viewer"}}
	if _, err := m.UpsertUser(user); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := m.BindRole("ops_admin", []string{"u3"}, nil); err != nil {
		t.Fatalf("BindRole: %v", err)
	}
	got, ok := m.GetUser("u3")
	if !ok {
		t.Fatal("GetUser after BindRole: not found")
	}
	if !containsString(got.Roles, "ops_admin") {
		t.Errorf("user roles after BindRole = %v, expected ops_admin", got.Roles)
	}
}

func TestBindRoleNotFound(t *testing.T) {
	m := newTestManager(t)
	if err := m.BindRole("nonexistent_role", []string{"u1"}, nil); err == nil {
		t.Error("expected error for nonexistent role, got nil")
	}
}

func TestSetRoleBindingsAddAndRemove(t *testing.T) {
	m := newTestManager(t)
	// Seed users and a group. Use approver (non-default role) so that removing
	// a user's only approver binding doesn't get re-added by normalizeUser,
	// which always assigns "viewer" as the floor role for users with no roles.
	for _, u := range []User{
		{UserID: "u-alpha", Username: "alpha", Status: "active", Source: "local", Roles: []string{"approver"}},
		{UserID: "u-beta", Username: "beta", Status: "active", Source: "local", Roles: []string{"approver"}},
		{UserID: "u-gamma", Username: "gamma", Status: "active", Source: "local", Roles: []string{"approver"}},
	} {
		if _, err := m.UpsertUser(u); err != nil {
			t.Fatalf("UpsertUser %s: %v", u.UserID, err)
		}
	}
	if _, err := m.UpsertGroup(Group{GroupID: "g-delta", DisplayName: "Delta", Roles: []string{"approver"}}); err != nil {
		t.Fatalf("UpsertGroup: %v", err)
	}

	// Step 1: set approver bindings to only alpha and beta (gamma should be removed).
	if err := m.SetRoleBindings("approver", []string{"u-alpha", "u-beta"}, nil); err != nil {
		t.Fatalf("SetRoleBindings (initial): %v", err)
	}
	b1, _ := m.GetRoleBindings("approver")
	if !containsString(b1.UserIDs, "u-alpha") || !containsString(b1.UserIDs, "u-beta") {
		t.Errorf("after initial bind: expected u-alpha and u-beta, got %v", b1.UserIDs)
	}
	if containsString(b1.UserIDs, "u-gamma") {
		t.Errorf("after initial bind: u-gamma should have been removed, got %v", b1.UserIDs)
	}

	// Step 2: replace with beta + gamma + group g-delta (alpha should be removed).
	if err := m.SetRoleBindings("approver", []string{"u-beta", "u-gamma"}, []string{"g-delta"}); err != nil {
		t.Fatalf("SetRoleBindings (replace): %v", err)
	}
	b2, _ := m.GetRoleBindings("approver")
	if containsString(b2.UserIDs, "u-alpha") {
		t.Errorf("after replace: u-alpha should have been removed, got %v", b2.UserIDs)
	}
	if !containsString(b2.UserIDs, "u-beta") {
		t.Errorf("after replace: u-beta should still be bound, got %v", b2.UserIDs)
	}
	if !containsString(b2.UserIDs, "u-gamma") {
		t.Errorf("after replace: u-gamma should be added, got %v", b2.UserIDs)
	}
	if !containsString(b2.GroupIDs, "g-delta") {
		t.Errorf("after replace: g-delta should be bound, got %v", b2.GroupIDs)
	}

	// Step 3: clear all approver bindings.
	if err := m.SetRoleBindings("approver", nil, nil); err != nil {
		t.Fatalf("SetRoleBindings (clear): %v", err)
	}
	b3, _ := m.GetRoleBindings("approver")
	if len(b3.UserIDs) != 0 || len(b3.GroupIDs) != 0 {
		t.Errorf("after clear: expected empty bindings, got users=%v groups=%v", b3.UserIDs, b3.GroupIDs)
	}
}

func TestUnbindRoleRemovesFromUser(t *testing.T) {
	m := newTestManager(t)
	user := User{UserID: "u-unbind", Username: "unbind-user", Status: "active", Source: "local", Roles: []string{"viewer", "approver"}}
	if _, err := m.UpsertUser(user); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if err := m.UnbindRole("viewer", []string{"u-unbind"}, nil); err != nil {
		t.Fatalf("UnbindRole: %v", err)
	}
	got, ok := m.GetUser("u-unbind")
	if !ok {
		t.Fatal("GetUser after UnbindRole: not found")
	}
	if containsString(got.Roles, "viewer") {
		t.Errorf("expected viewer to be removed, got roles=%v", got.Roles)
	}
	if !containsString(got.Roles, "approver") {
		t.Errorf("expected approver to be retained, got roles=%v", got.Roles)
	}
}

// ── Auth providers ───────────────────────────────────────────────────────────

func TestUpsertAuthProvider(t *testing.T) {
	m := newTestManager(t)
	provider := AuthProvider{ID: "google-oidc", Type: "oidc", Name: "Google", Enabled: true, ClientID: "cid", AuthURL: "https://accounts.google.com/auth", TokenURL: "https://oauth2.googleapis.com/token", UserInfoURL: "https://openidconnect.googleapis.com/v1/userinfo", Scopes: []string{"openid", "email", "profile"}, AllowJIT: true, DefaultRoles: []string{"viewer"}}
	created, err := m.UpsertAuthProvider(provider)
	if err != nil {
		t.Fatalf("UpsertAuthProvider: %v", err)
	}
	if created.ID != "google-oidc" {
		t.Errorf("ID = %q, want google-oidc", created.ID)
	}
	got, ok := m.GetAuthProvider("google-oidc")
	if !ok {
		t.Fatal("GetAuthProvider: not found")
	}
	if !got.Enabled {
		t.Error("expected provider to be enabled")
	}
}

func TestSetAuthProviderEnabled(t *testing.T) {
	m := newTestManager(t)
	provider := AuthProvider{ID: "ldap-main", Type: "local_token", Name: "LDAP Main", Enabled: true}
	if _, err := m.UpsertAuthProvider(provider); err != nil {
		t.Fatalf("UpsertAuthProvider: %v", err)
	}
	updated, err := m.SetAuthProviderEnabled("ldap-main", false)
	if err != nil {
		t.Fatalf("SetAuthProviderEnabled: %v", err)
	}
	if updated.Enabled {
		t.Error("expected provider to be disabled after SetAuthProviderEnabled(false)")
	}
}

func TestParseConfigAuthProviderSnakeCaseFields(t *testing.T) {
	content := []byte(`access:
  auth_providers:
    - id: dex-local
      type: oidc
      name: Dex Shared OIDC
      enabled: true
      issuer_url: http://127.0.0.1:15556/dex
      client_id: tars-shared
      client_secret: tars-shared-secret
      redirect_path: /api/v1/auth/callback/dex-local
      success_redirect: http://127.0.0.1:8081/login
      session_ttl_seconds: 43200
      user_id_field: sub
      username_field: preferred_username
      display_name_field: name
      email_field: email
      default_roles:
        - viewer
      allow_jit: true
`)

	cfg, _, err := ParseConfig(content)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if len(cfg.AuthProviders) != 1 {
		t.Fatalf("AuthProviders length = %d, want 1", len(cfg.AuthProviders))
	}
	provider := cfg.AuthProviders[0]
	if provider.ClientID != "tars-shared" {
		t.Fatalf("ClientID = %q, want tars-shared", provider.ClientID)
	}
	if provider.IssuerURL != "http://127.0.0.1:15556/dex" {
		t.Fatalf("IssuerURL = %q, want dex issuer", provider.IssuerURL)
	}
	if provider.SuccessRedirect != "http://127.0.0.1:8081/login" {
		t.Fatalf("SuccessRedirect = %q, want login redirect", provider.SuccessRedirect)
	}
	if provider.SessionTTLSeconds != 43200 {
		t.Fatalf("SessionTTLSeconds = %d, want 43200", provider.SessionTTLSeconds)
	}
	if !provider.AllowJIT {
		t.Fatal("AllowJIT = false, want true")
	}
}

func TestParseConfigUserSnakeCaseFields(t *testing.T) {
	content := []byte(`access:
  users:
    - user_id: alice
      username: alice
      display_name: Alice
      email: alice@example.com
      status: active
      source: local_password
      password_hash: hash-value
      password_login_enabled: true
      challenge_required: true
      mfa_enabled: true
      mfa_method: totp
      totp_secret: JBSWY3DPEHPK3PXP
      roles:
        - viewer
`)

	cfg, _, err := ParseConfig(content)
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	if len(cfg.Users) != 1 {
		t.Fatalf("Users length = %d, want 1", len(cfg.Users))
	}
	user := cfg.Users[0]
	if user.UserID != "alice" {
		t.Fatalf("UserID = %q, want alice", user.UserID)
	}
	if user.PasswordHash != "hash-value" {
		t.Fatalf("PasswordHash = %q, want hash-value", user.PasswordHash)
	}
	if !user.PasswordLoginEnabled || !user.ChallengeRequired || !user.MFAEnabled {
		t.Fatalf("expected password/challenge/mfa flags set, got %+v", user)
	}
	if user.MFAMethod != "totp" || user.TOTPSecret != "JBSWY3DPEHPK3PXP" {
		t.Fatalf("unexpected mfa fields: %+v", user)
	}
}

// ── Local token login (break-glass) ─────────────────────────────────────────

func TestLoginWithLocalToken(t *testing.T) {
	m := newTestManager(t)
	provider := AuthProvider{ID: "local-ops", Type: "local_token", Name: "Local Ops", Enabled: true, ClientSecret: "super-secret-token", DefaultRoles: []string{"ops_admin"}}
	if _, err := m.UpsertAuthProvider(provider); err != nil {
		t.Fatalf("UpsertAuthProvider: %v", err)
	}
	session, user, err := m.LoginWithLocalToken("super-secret-token", "")
	if err != nil {
		t.Fatalf("LoginWithLocalToken: %v", err)
	}
	if session.Token == "" {
		t.Error("expected non-empty session token")
	}
	if user.Source != "local-ops" {
		t.Errorf("Source = %q, want local-ops", user.Source)
	}
}

func TestLoginWithWrongTokenFails(t *testing.T) {
	m := newTestManager(t)
	provider := AuthProvider{ID: "local-ops2", Type: "local_token", Name: "Local Ops 2", Enabled: true, ClientSecret: "correct-token"}
	if _, err := m.UpsertAuthProvider(provider); err != nil {
		t.Fatalf("UpsertAuthProvider: %v", err)
	}
	_, _, err := m.LoginWithLocalToken("wrong-token", "")
	if err == nil {
		t.Error("expected error for wrong token, got nil")
	}
}

func TestLoginWithOpsFallbackToken(t *testing.T) {
	m := newTestManager(t)
	session, user, err := m.LoginWithLocalToken("ops-break-glass-token", "ops-break-glass-token")
	if err != nil {
		t.Fatalf("LoginWithLocalToken fallback: %v", err)
	}
	if session.Token == "" {
		t.Error("expected non-empty fallback session token")
	}
	if user.UserID != "ops-admin" {
		t.Errorf("UserID = %q, want ops-admin", user.UserID)
	}
	if !containsString(user.Roles, "platform_admin") {
		t.Errorf("expected platform_admin role on ops-admin, got %v", user.Roles)
	}
}

// ── Session auth ─────────────────────────────────────────────────────────────

func TestAuthenticateValidSession(t *testing.T) {
	m := newTestManager(t)
	user := User{UserID: "sess-user", Username: "sess-user", Status: "active", Source: "test", Roles: []string{"operator"}}
	if _, err := m.UpsertUser(user); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	token := issueTestSession(m, "sess-user")
	principal, ok := m.AuthenticateSession(token)
	if !ok {
		t.Fatal("AuthenticateSession: not ok")
	}
	if principal.User == nil || principal.User.UserID != "sess-user" {
		t.Errorf("User = %v, want sess-user", principal.User)
	}
	if principal.Kind != "session" {
		t.Errorf("Kind = %q, want session", principal.Kind)
	}
}

func TestAuthenticateSessionAfterLogout(t *testing.T) {
	m := newTestManager(t)
	user := User{UserID: "logout-user", Username: "logout-user", Status: "active", Source: "test", Roles: []string{"viewer"}}
	if _, err := m.UpsertUser(user); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	token := issueTestSession(m, "logout-user")
	m.Logout(token)
	if _, ok := m.AuthenticateSession(token); ok {
		t.Error("expected session to be invalid after logout")
	}
}

func TestAuthenticateExpiredSession(t *testing.T) {
	m := newTestManager(t)
	// Override the clock to a time in the past so sessions are immediately expired
	pastNow := time.Now().UTC().Add(-24 * time.Hour)
	m.now = func() time.Time { return pastNow }
	user := User{UserID: "exp-user", Username: "exp-user", Status: "active", Source: "test"}
	if _, err := m.UpsertUser(user); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	token := issueTestSession(m, "exp-user")
	// Restore time to now
	m.now = func() time.Time { return time.Now().UTC() }
	// Session should now be expired
	if _, ok := m.AuthenticateSession(token); ok {
		t.Error("expected expired session to be rejected")
	}
}

func TestAuthenticateDisabledUser(t *testing.T) {
	m := newTestManager(t)
	user := User{UserID: "disabled-user", Username: "disabled-user", Status: "active", Source: "test"}
	if _, err := m.UpsertUser(user); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	token := issueTestSession(m, "disabled-user")
	// Disable the user
	if _, err := m.SetUserStatus("disabled-user", "disabled"); err != nil {
		t.Fatalf("SetUserStatus: %v", err)
	}
	if _, ok := m.AuthenticateSession(token); ok {
		t.Error("expected disabled user to fail authentication")
	}
}

// ── Group membership effect on roles ─────────────────────────────────────────

func TestGroupRolesInheritedByUser(t *testing.T) {
	m := newTestManager(t)
	// Create a group with approver role
	grp := Group{GroupID: "approvers-group", DisplayName: "Approvers", Status: "active", Roles: []string{"approver"}, Members: []string{"member-user"}}
	if _, err := m.UpsertGroup(grp); err != nil {
		t.Fatalf("UpsertGroup: %v", err)
	}
	// Create user with viewer role only
	user := User{UserID: "member-user", Username: "member-user", Status: "active", Source: "test", Roles: []string{"viewer"}}
	if _, err := m.UpsertUser(user); err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	token := issueTestSession(m, "member-user")
	principal, ok := m.AuthenticateSession(token)
	if !ok {
		t.Fatal("AuthenticateSession failed")
	}
	// User should inherit approver permissions from group
	if !m.Evaluate(principal, "executions.read") {
		t.Error("user should have executions.read from group approver role")
	}
}

// ── People & Channels ─────────────────────────────────────────────────────────

func TestUpsertAndGetPerson(t *testing.T) {
	m := newTestManager(t)
	person := Person{ID: "p1", DisplayName: "Dave", Email: "dave@example.com", Status: "active", Team: "platform", ApprovalTarget: "team-lead"}
	created, err := m.UpsertPerson(person)
	if err != nil {
		t.Fatalf("UpsertPerson: %v", err)
	}
	if created.ID != "p1" {
		t.Errorf("ID = %q, want p1", created.ID)
	}
	got, ok := m.GetPerson("p1")
	if !ok {
		t.Fatal("GetPerson: not found")
	}
	if got.Team != "platform" {
		t.Errorf("Team = %q, want platform", got.Team)
	}
}

func TestUpsertAndGetChannel(t *testing.T) {
	m := newTestManager(t)
	ch := Channel{ID: "ch1", Type: "telegram", Name: "Telegram Main", Target: "-100123456789", Enabled: true}
	created, err := m.UpsertChannel(ch)
	if err != nil {
		t.Fatalf("UpsertChannel: %v", err)
	}
	if created.ID != "ch1" {
		t.Errorf("ID = %q, want ch1", created.ID)
	}
	got, ok := m.GetChannel("ch1")
	if !ok {
		t.Fatal("GetChannel: not found")
	}
	if !got.Enabled {
		t.Error("expected channel to be enabled")
	}
}

func TestUpsertChannelNormalizesTypedLists(t *testing.T) {
	m := newTestManager(t)
	ch := Channel{
		ID:           "ch-typed",
		Kind:         "slack",
		Name:         "Slack Alerts",
		Target:       "#ops",
		Enabled:      true,
		Usages:       []ChannelUsage{" approval ", "approval", "alert"},
		Capabilities: []ChannelCapability{},
	}
	created, err := m.UpsertChannel(ch)
	if err != nil {
		t.Fatalf("UpsertChannel: %v", err)
	}
	if len(created.Usages) != 2 {
		t.Fatalf("Usages len = %d, want 2", len(created.Usages))
	}
	if created.Usages[0] != ChannelUsageApproval || created.Usages[1] != ChannelUsageAlert {
		t.Fatalf("Usages = %#v, want [approval alert]", created.Usages)
	}
	if len(created.Capabilities) != 2 {
		t.Fatalf("Capabilities len = %d, want 2", len(created.Capabilities))
	}
	if created.Capabilities[0] != ChannelCapability("approval") || created.Capabilities[1] != ChannelCapability("alert") {
		t.Fatalf("Capabilities = %#v, want [approval alert]", created.Capabilities)
	}

	got, ok := m.GetChannel("ch-typed")
	if !ok {
		t.Fatal("GetChannel: not found")
	}
	if len(got.Usages) != 2 || len(got.Capabilities) != 2 {
		t.Fatalf("stored channel lists = %#v / %#v, want both cloned with two items", got.Usages, got.Capabilities)
	}
}

// ── Ops-token break-glass compatibility (MUST NOT BREAK) ─────────────────────

func TestOpsTokenBreakGlassIsAlwaysAllowed(t *testing.T) {
	m := newTestManager(t)
	// Simulate the ops-token path from breakGlassPrincipal
	user := User{UserID: "ops-admin", Username: "ops-admin", DisplayName: "Ops Admin", Status: "active", Source: "ops_token", Roles: []string{"platform_admin"}}
	permission := map[string]struct{}{"*": {}}
	principal := Principal{Kind: "ops_token", Token: "the-ops-token", User: &user, RoleIDs: []string{"platform_admin"}, Permission: permission, Source: "ops-token"}
	// Should be allowed to do anything
	for _, perm := range []string{"*", "sessions.write", "users.delete", "configs.admin", "arbitrary"} {
		if !m.Evaluate(principal, perm) {
			t.Errorf("ops_token break-glass should be allowed %q", perm)
		}
	}
}

// ── IdentityLink (user_identities) ───────────────────────────────────────────

func TestUserWithIdentityLink(t *testing.T) {
	m := newTestManager(t)
	user := User{
		UserID:   "linked-user",
		Username: "oauth-user",
		Status:   "active",
		Source:   "google-oidc",
		Roles:    []string{"viewer"},
		Identities: []IdentityLink{
			{ProviderType: "oidc", ProviderID: "google-oidc", ExternalSubject: "1234567890", ExternalUsername: "user@example.com", ExternalEmail: "user@example.com"},
		},
	}
	created, err := m.UpsertUser(user)
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	if len(created.Identities) == 0 {
		t.Fatal("expected user to have identities")
	}
	if created.Identities[0].ExternalSubject != "1234567890" {
		t.Errorf("ExternalSubject = %q, want 1234567890", created.Identities[0].ExternalSubject)
	}
}

// ── helper functions ─────────────────────────────────────────────────────────

// issueTestSession directly calls issueSession (unexported helper); we wrap it.
func issueTestSession(m *Manager, userID string) string {
	session := m.issueSession(userID, "test", 3600)
	return session.Token
}

func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[i] > s[j] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
