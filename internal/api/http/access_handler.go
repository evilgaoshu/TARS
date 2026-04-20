package httpapi

import (
	"net/http"
	"sort"
	"strings"

	"tars/internal/api/dto"
	"tars/internal/modules/access"
	"tars/internal/modules/org"
	"tars/internal/modules/reasoning"
)

// orgFilterFromRequest builds an access.OrgFilter from the request org context.
func orgFilterFromRequest(r *http.Request) access.OrgFilter {
	oc := org.RequestOrgContext(r)
	return access.OrgFilter{
		OrgID:       oc.OrgID,
		TenantID:    oc.TenantID,
		WorkspaceID: oc.WorkspaceID,
	}
}

func registerAccessRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/v1/auth/providers", instrumentHandler(deps, "/api/v1/auth/providers", authProvidersHandler(deps)))
	mux.HandleFunc("/api/v1/auth/providers/", instrumentHandler(deps, "/api/v1/auth/providers/*", authProviderDetailHandler(deps)))
	mux.HandleFunc("/api/v1/auth/login", instrumentHandler(deps, "/api/v1/auth/login", authLoginHandler(deps)))
	mux.HandleFunc("/api/v1/auth/challenge", instrumentHandler(deps, "/api/v1/auth/challenge", authChallengeHandler(deps)))
	mux.HandleFunc("/api/v1/auth/verify", instrumentHandler(deps, "/api/v1/auth/verify", authVerifyHandler(deps)))
	mux.HandleFunc("/api/v1/auth/mfa/verify", instrumentHandler(deps, "/api/v1/auth/mfa/verify", authMFAVerifyHandler(deps)))
	mux.HandleFunc("/api/v1/auth/logout", instrumentHandler(deps, "/api/v1/auth/logout", authLogoutHandler(deps)))
	mux.HandleFunc("/api/v1/auth/sessions", instrumentHandler(deps, "/api/v1/auth/sessions", authSessionsHandler(deps)))
	mux.HandleFunc("/api/v1/auth/callback/", instrumentHandler(deps, "/api/v1/auth/callback/*", authCallbackHandler(deps)))
	mux.HandleFunc("/api/v1/me", instrumentHandler(deps, "/api/v1/me", meHandler(deps)))
	mux.HandleFunc("/api/v1/users", instrumentHandler(deps, "/api/v1/users", usersHandler(deps)))
	mux.HandleFunc("/api/v1/users/", instrumentHandler(deps, "/api/v1/users/*", userDetailHandler(deps)))
	mux.HandleFunc("/api/v1/groups", instrumentHandler(deps, "/api/v1/groups", groupsHandler(deps)))
	mux.HandleFunc("/api/v1/groups/", instrumentHandler(deps, "/api/v1/groups/*", groupDetailHandler(deps)))
	mux.HandleFunc("/api/v1/roles", instrumentHandler(deps, "/api/v1/roles", rolesHandler(deps)))
	mux.HandleFunc("/api/v1/roles/", instrumentHandler(deps, "/api/v1/roles/*", roleDetailHandler(deps)))
	mux.HandleFunc("/api/v1/people", instrumentHandler(deps, "/api/v1/people", peopleHandler(deps)))
	mux.HandleFunc("/api/v1/people/", instrumentHandler(deps, "/api/v1/people/*", personDetailHandler(deps)))
	mux.HandleFunc("/api/v1/channels", instrumentHandler(deps, "/api/v1/channels", channelsHandler(deps)))
	mux.HandleFunc("/api/v1/channels/", instrumentHandler(deps, "/api/v1/channels/*", channelDetailHandler(deps)))
	mux.HandleFunc("/api/v1/providers", instrumentHandler(deps, "/api/v1/providers", providersRegistryHandler(deps)))
	mux.HandleFunc("/api/v1/providers/bindings", instrumentHandler(deps, "/api/v1/providers/bindings", providerBindingsHandler(deps)))
	mux.HandleFunc("/api/v1/providers/", instrumentHandler(deps, "/api/v1/providers/*", providerDetailHandler(deps)))
	mux.HandleFunc("/api/v1/config/auth", instrumentHandler(deps, "/api/v1/config/auth", authConfigHandler(deps)))
}

func mapMethodPermission(method string, readPermission string, writePermission string) string {
	switch method {
	case http.MethodGet:
		return readPermission
	default:
		return writePermission
	}
}

func filterUsers(items []access.User, query string) []access.User {
	if strings.TrimSpace(query) == "" {
		return items
	}
	query = strings.ToLower(strings.TrimSpace(query))
	out := make([]access.User, 0, len(items))
	for _, item := range items {
		haystacks := []string{item.UserID, item.Username, item.DisplayName, item.Email, item.Status, item.Source, strings.Join(item.Roles, " ")}
		if containsQuery(query, haystacks...) {
			out = append(out, item)
		}
	}
	return out
}

func filterGroups(items []access.Group, query string) []access.Group {
	if strings.TrimSpace(query) == "" {
		return items
	}
	query = strings.ToLower(strings.TrimSpace(query))
	out := make([]access.Group, 0, len(items))
	for _, item := range items {
		if containsQuery(query, item.GroupID, item.DisplayName, item.Description, item.Status, strings.Join(item.Roles, " "), strings.Join(item.Members, " ")) {
			out = append(out, item)
		}
	}
	return out
}

func filterPeople(items []access.Person, query string) []access.Person {
	if strings.TrimSpace(query) == "" {
		return items
	}
	query = strings.ToLower(strings.TrimSpace(query))
	out := make([]access.Person, 0, len(items))
	for _, item := range items {
		if containsQuery(query, item.ID, item.DisplayName, item.Email, item.Status, item.LinkedUserID, strings.Join(item.ChannelIDs, " ")) {
			out = append(out, item)
		}
	}
	return out
}

func filterChannels(items []access.Channel, query string) []access.Channel {
	if strings.TrimSpace(query) == "" {
		return items
	}
	query = strings.ToLower(strings.TrimSpace(query))
	out := make([]access.Channel, 0, len(items))
	for _, item := range items {
		if containsQuery(query, item.ID, item.Kind, item.Type, item.Name, item.Target, strings.Join(item.LinkedUsers, " "), joinChannelUsages(item.Usages)) {
			out = append(out, item)
		}
	}
	return out
}

func filterProviders(items []reasoning.ProviderEntry, query string) []reasoning.ProviderEntry {
	if strings.TrimSpace(query) == "" {
		return items
	}
	query = strings.ToLower(strings.TrimSpace(query))
	out := make([]reasoning.ProviderEntry, 0, len(items))
	for _, item := range items {
		if containsQuery(query, item.ID, item.Vendor, item.Protocol, item.BaseURL) {
			out = append(out, item)
		}
	}
	return out
}

func containsQuery(query string, haystacks ...string) bool {
	for _, haystack := range haystacks {
		if strings.Contains(strings.ToLower(haystack), query) {
			return true
		}
	}
	return false
}

func authProviderToDTO(item access.AuthProvider) dto.AuthProvider {
	return dto.AuthProvider{ID: item.ID, Type: item.Type, Name: item.Name, Enabled: item.Enabled, IssuerURL: item.IssuerURL, ClientID: item.ClientID, ClientSecretSet: strings.TrimSpace(item.ClientSecret) != "" || strings.TrimSpace(item.ClientSecretRef) != "", ClientSecretRef: item.ClientSecretRef, AuthURL: item.AuthURL, TokenURL: item.TokenURL, UserInfoURL: item.UserInfoURL, SessionTTLSeconds: item.SessionTTLSeconds, LDAPURL: item.LDAPURL, BindDN: item.BindDN, BindPasswordSet: strings.TrimSpace(item.BindPassword) != "" || strings.TrimSpace(item.BindPasswordRef) != "", BindPasswordRef: item.BindPasswordRef, BaseDN: item.BaseDN, UserSearchFilter: item.UserSearchFilter, GroupSearchFilter: item.GroupSearchFilter, RedirectPath: item.RedirectPath, SuccessRedirect: item.SuccessRedirect, UserIDField: item.UserIDField, UsernameField: item.UsernameField, DisplayNameField: item.DisplayNameField, EmailField: item.EmailField, AllowedDomains: append([]string(nil), item.AllowedDomains...), Scopes: append([]string(nil), item.Scopes...), DefaultRoles: append([]string(nil), item.DefaultRoles...), AllowJIT: item.AllowJIT, PasswordMinLength: item.PasswordMinLength, RequireChallenge: item.RequireChallenge, ChallengeChannel: item.ChallengeChannel, ChallengeTTLSeconds: item.ChallengeTTLSeconds, ChallengeCodeLength: item.ChallengeCodeLength, RequireMFA: item.RequireMFA}
}

func authProviderFromDTO(item dto.AuthProvider) access.AuthProvider {
	return access.AuthProvider{ID: item.ID, Type: item.Type, Name: item.Name, Enabled: item.Enabled, IssuerURL: item.IssuerURL, ClientID: item.ClientID, ClientSecret: item.ClientSecret, ClientSecretRef: item.ClientSecretRef, AuthURL: item.AuthURL, TokenURL: item.TokenURL, UserInfoURL: item.UserInfoURL, SessionTTLSeconds: item.SessionTTLSeconds, LDAPURL: item.LDAPURL, BindDN: item.BindDN, BindPassword: item.BindPassword, BindPasswordRef: item.BindPasswordRef, BaseDN: item.BaseDN, UserSearchFilter: item.UserSearchFilter, GroupSearchFilter: item.GroupSearchFilter, RedirectPath: item.RedirectPath, SuccessRedirect: item.SuccessRedirect, UserIDField: item.UserIDField, UsernameField: item.UsernameField, DisplayNameField: item.DisplayNameField, EmailField: item.EmailField, AllowedDomains: append([]string(nil), item.AllowedDomains...), Scopes: append([]string(nil), item.Scopes...), DefaultRoles: append([]string(nil), item.DefaultRoles...), AllowJIT: item.AllowJIT, PasswordMinLength: item.PasswordMinLength, RequireChallenge: item.RequireChallenge, ChallengeChannel: item.ChallengeChannel, ChallengeTTLSeconds: item.ChallengeTTLSeconds, ChallengeCodeLength: item.ChallengeCodeLength, RequireMFA: item.RequireMFA}
}

func userToDTO(item access.User) dto.User {
	identities := make([]dto.IdentityLink, 0, len(item.Identities))
	for _, identity := range item.Identities {
		identities = append(identities, dto.IdentityLink{ProviderType: identity.ProviderType, ProviderID: identity.ProviderID, ExternalSubject: identity.ExternalSubject, ExternalUsername: identity.ExternalUsername, ExternalEmail: identity.ExternalEmail})
	}
	return dto.User{UserID: item.UserID, Username: item.Username, DisplayName: item.DisplayName, Email: item.Email, Status: item.Status, Source: item.Source, Roles: append([]string(nil), item.Roles...), Groups: append([]string(nil), item.Groups...), Identities: identities, OrgID: item.OrgID, TenantID: item.TenantID, WorkspaceID: item.WorkspaceID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, PasswordLoginEnabled: item.PasswordLoginEnabled, ChallengeRequired: item.ChallengeRequired, MFAEnabled: item.MFAEnabled, MFAMethod: item.MFAMethod, TOTPSecret: item.TOTPSecret}
}

func userFromDTO(item dto.User) access.User {
	identities := make([]access.IdentityLink, 0, len(item.Identities))
	for _, identity := range item.Identities {
		identities = append(identities, access.IdentityLink{ProviderType: identity.ProviderType, ProviderID: identity.ProviderID, ExternalSubject: identity.ExternalSubject, ExternalUsername: identity.ExternalUsername, ExternalEmail: identity.ExternalEmail})
	}
	return access.User{UserID: item.UserID, Username: item.Username, DisplayName: item.DisplayName, Email: item.Email, Status: item.Status, Source: item.Source, Roles: append([]string(nil), item.Roles...), Groups: append([]string(nil), item.Groups...), Identities: identities, OrgID: item.OrgID, TenantID: item.TenantID, WorkspaceID: item.WorkspaceID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt, PasswordHash: item.PasswordHash, PasswordLoginEnabled: item.PasswordLoginEnabled, PasswordUpdatedAt: item.PasswordUpdatedAt, ChallengeRequired: item.ChallengeRequired, MFAEnabled: item.MFAEnabled, MFAMethod: item.MFAMethod, TOTPSecret: item.TOTPSecret}
}

func roleToDTO(item access.Role) dto.Role {
	return dto.Role{ID: item.ID, DisplayName: item.DisplayName, Permissions: append([]string(nil), item.Permissions...)}
}

func roleFromDTO(item dto.Role) access.Role {
	return access.Role{ID: item.ID, DisplayName: item.DisplayName, Permissions: append([]string(nil), item.Permissions...)}
}

func groupToDTO(item access.Group) dto.Group {
	return dto.Group{GroupID: item.GroupID, DisplayName: item.DisplayName, Description: item.Description, Status: item.Status, Roles: append([]string(nil), item.Roles...), Members: append([]string(nil), item.Members...), OrgID: item.OrgID, TenantID: item.TenantID, WorkspaceID: item.WorkspaceID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func groupFromDTO(item dto.Group) access.Group {
	return access.Group{GroupID: item.GroupID, DisplayName: item.DisplayName, Description: item.Description, Status: item.Status, Roles: append([]string(nil), item.Roles...), Members: append([]string(nil), item.Members...), OrgID: item.OrgID, TenantID: item.TenantID, WorkspaceID: item.WorkspaceID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func personToDTO(item access.Person) dto.Person {
	return dto.Person{ID: item.ID, DisplayName: item.DisplayName, Email: item.Email, Status: item.Status, LinkedUserID: item.LinkedUserID, ChannelIDs: append([]string(nil), item.ChannelIDs...), Team: item.Team, ApprovalTarget: item.ApprovalTarget, OncallSchedule: item.OncallSchedule, Preferences: cloneStringMap(item.Preferences), OrgID: item.OrgID, TenantID: item.TenantID, WorkspaceID: item.WorkspaceID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func personFromDTO(item dto.Person) access.Person {
	return access.Person{ID: item.ID, DisplayName: item.DisplayName, Email: item.Email, Status: item.Status, LinkedUserID: item.LinkedUserID, ChannelIDs: append([]string(nil), item.ChannelIDs...), Team: item.Team, ApprovalTarget: item.ApprovalTarget, OncallSchedule: item.OncallSchedule, Preferences: cloneStringMap(item.Preferences), OrgID: item.OrgID, TenantID: item.TenantID, WorkspaceID: item.WorkspaceID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func channelToDTO(item access.Channel) dto.Channel {
	kind := firstNonEmpty(strings.TrimSpace(item.Kind), strings.TrimSpace(item.Type))
	usages := make([]dto.ChannelUsage, 0, len(item.Usages))
	for _, u := range item.Usages {
		usages = append(usages, dto.ChannelUsage(u))
	}
	if len(usages) == 0 {
		for _, c := range item.Capabilities {
			usages = append(usages, dto.ChannelUsage(c))
		}
	}
	capabilities := make([]dto.ChannelCapability, 0, len(item.Capabilities))
	for _, c := range item.Capabilities {
		capabilities = append(capabilities, dto.ChannelCapability(c))
	}
	return dto.Channel{ID: item.ID, Kind: kind, Type: firstNonEmpty(strings.TrimSpace(item.Type), kind), Name: item.Name, Target: item.Target, Enabled: item.Enabled, LinkedUsers: append([]string(nil), item.LinkedUsers...), Usages: usages, Capabilities: capabilities, OrgID: item.OrgID, TenantID: item.TenantID, WorkspaceID: item.WorkspaceID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func channelFromDTO(item dto.Channel) access.Channel {
	usages := make([]access.ChannelUsage, 0, len(item.Usages))
	for _, u := range item.Usages {
		usages = append(usages, access.ChannelUsage(u))
	}
	capabilities := make([]access.ChannelCapability, 0, len(item.Capabilities))
	for _, c := range item.Capabilities {
		capabilities = append(capabilities, access.ChannelCapability(c))
	}
	return access.Channel{ID: item.ID, Kind: item.Kind, Type: item.Type, Name: item.Name, Target: item.Target, Enabled: item.Enabled, LinkedUsers: append([]string(nil), item.LinkedUsers...), Usages: usages, Capabilities: capabilities, OrgID: item.OrgID, TenantID: item.TenantID, WorkspaceID: item.WorkspaceID, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
}

func joinChannelUsages(items []access.ChannelUsage) string {
	parts := make([]string, 0, len(items))
	for _, item := range items {
		parts = append(parts, string(item))
	}
	return strings.Join(parts, " ")
}

func providerRegistryEntryToDTO(item reasoning.ProviderEntry, cfg reasoning.ProvidersConfig, orgManager *org.Manager) dto.ProviderRegistryEntry {
	defaults := defaultAffiliation(orgManager)
	_ = cfg
	return dto.ProviderRegistryEntry{ID: item.ID, Vendor: item.Vendor, Protocol: item.Protocol, BaseURL: item.BaseURL, APIKeyRef: item.APIKeyRef, APIKeySet: strings.TrimSpace(item.APIKey) != "" || strings.TrimSpace(item.APIKeyRef) != "", Enabled: item.Enabled, OrgID: ownershipValue(item.OrgID, defaults.OrgID), TenantID: ownershipValue(item.TenantID, defaults.TenantID), WorkspaceID: ownershipValue(item.WorkspaceID, defaults.WorkspaceID), Templates: providerTemplatesToDTO(item.Templates)}
}

func providerRegistryEntryFromDTO(item dto.ProviderRegistryEntry, orgManager *org.Manager) reasoning.ProviderEntry {
	defaults := defaultAffiliation(orgManager)
	return reasoning.ProviderEntry{ID: item.ID, Vendor: item.Vendor, Protocol: item.Protocol, BaseURL: item.BaseURL, APIKey: item.APIKey, APIKeyRef: item.APIKeyRef, OrgID: ownershipValue(item.OrgID, defaults.OrgID), TenantID: ownershipValue(item.TenantID, defaults.TenantID), WorkspaceID: ownershipValue(item.WorkspaceID, defaults.WorkspaceID), Enabled: item.Enabled, Templates: providerTemplatesFromDTO(item.Templates)}
}

func getProviderByID(manager *reasoning.ProviderManager, providerID string) (reasoning.ProviderEntry, reasoning.ProvidersConfig, bool) {
	snapshot := manager.Snapshot()
	for _, item := range snapshot.Config.Entries {
		if item.ID == providerID {
			return item, snapshot.Config, true
		}
	}
	return reasoning.ProviderEntry{}, snapshot.Config, false
}

func upsertProviderEntry(manager *reasoning.ProviderManager, entry reasoning.ProviderEntry) (reasoning.ProviderEntry, error) {
	snapshot := manager.Snapshot()
	cfg := snapshot.Config
	replaced := false
	for i := range cfg.Entries {
		if cfg.Entries[i].ID == entry.ID {
			if strings.TrimSpace(entry.APIKey) == "" {
				entry.APIKey = cfg.Entries[i].APIKey
			}
			if strings.TrimSpace(entry.APIKeyRef) == "" {
				entry.APIKeyRef = cfg.Entries[i].APIKeyRef
			}
			cfg.Entries[i] = entry
			replaced = true
			break
		}
	}
	if !replaced {
		cfg.Entries = append(cfg.Entries, entry)
	}
	if err := manager.SaveConfig(cfg); err != nil {
		return reasoning.ProviderEntry{}, err
	}
	updated, _, _ := getProviderByID(manager, entry.ID)
	return updated, nil
}

func accessConfigToDTO(cfg access.Config) dto.AccessConfig {
	out := dto.AccessConfig{Users: make([]dto.User, 0, len(cfg.Users)), Groups: make([]dto.Group, 0, len(cfg.Groups)), AuthProviders: make([]dto.AuthProvider, 0, len(cfg.AuthProviders)), Roles: make([]dto.Role, 0, len(cfg.Roles)), People: make([]dto.Person, 0, len(cfg.People)), Channels: make([]dto.Channel, 0, len(cfg.Channels))}
	for _, item := range cfg.Users {
		out.Users = append(out.Users, userToDTO(item))
	}
	for _, item := range cfg.Groups {
		out.Groups = append(out.Groups, groupToDTO(item))
	}
	for _, item := range cfg.AuthProviders {
		out.AuthProviders = append(out.AuthProviders, authProviderToDTO(item))
	}
	for _, item := range cfg.Roles {
		out.Roles = append(out.Roles, roleToDTO(item))
	}
	for _, item := range cfg.People {
		out.People = append(out.People, personToDTO(item))
	}
	for _, item := range cfg.Channels {
		out.Channels = append(out.Channels, channelToDTO(item))
	}
	return out
}

func accessConfigFromDTO(cfg dto.AccessConfig) access.Config {
	out := access.Config{Users: make([]access.User, 0, len(cfg.Users)), Groups: make([]access.Group, 0, len(cfg.Groups)), AuthProviders: make([]access.AuthProvider, 0, len(cfg.AuthProviders)), Roles: make([]access.Role, 0, len(cfg.Roles)), People: make([]access.Person, 0, len(cfg.People)), Channels: make([]access.Channel, 0, len(cfg.Channels))}
	for _, item := range cfg.Users {
		out.Users = append(out.Users, userFromDTO(item))
	}
	for _, item := range cfg.Groups {
		out.Groups = append(out.Groups, groupFromDTO(item))
	}
	for _, item := range cfg.AuthProviders {
		out.AuthProviders = append(out.AuthProviders, access.AuthProvider{ID: item.ID, Type: item.Type, Name: item.Name, Enabled: item.Enabled, ClientID: item.ClientID, ClientSecret: item.ClientSecret, ClientSecretRef: item.ClientSecretRef, AuthURL: item.AuthURL, TokenURL: item.TokenURL, UserInfoURL: item.UserInfoURL, RedirectPath: item.RedirectPath, SuccessRedirect: item.SuccessRedirect, UserIDField: item.UserIDField, UsernameField: item.UsernameField, DisplayNameField: item.DisplayNameField, EmailField: item.EmailField, AllowedDomains: append([]string(nil), item.AllowedDomains...), Scopes: append([]string(nil), item.Scopes...), DefaultRoles: append([]string(nil), item.DefaultRoles...), AllowJIT: item.AllowJIT, PasswordMinLength: item.PasswordMinLength, RequireChallenge: item.RequireChallenge, ChallengeChannel: item.ChallengeChannel, ChallengeTTLSeconds: item.ChallengeTTLSeconds, ChallengeCodeLength: item.ChallengeCodeLength, RequireMFA: item.RequireMFA})
	}
	for _, item := range cfg.Roles {
		out.Roles = append(out.Roles, access.Role{ID: item.ID, DisplayName: item.DisplayName, Permissions: append([]string(nil), item.Permissions...)})
	}
	for _, item := range cfg.People {
		out.People = append(out.People, personFromDTO(item))
	}
	for _, item := range cfg.Channels {
		out.Channels = append(out.Channels, channelFromDTO(item))
	}
	return out
}

func principalFromSession(manager *access.Manager, session access.Session, user access.User) access.Principal {
	if manager != nil {
		if principal, ok := manager.AuthenticateSession(session.Token); ok {
			return principal
		}
	}
	roles := manager.ListRoles()
	perm := map[string]struct{}{}
	for _, role := range roles {
		for _, assigned := range user.Roles {
			if role.ID != assigned {
				continue
			}
			for _, p := range role.Permissions {
				perm[p] = struct{}{}
			}
		}
	}
	return access.Principal{Kind: "session", Token: session.Token, User: &user, RoleIDs: append([]string(nil), user.Roles...), Permission: perm, Source: session.ProviderID}
}

func nestedResourcePath(path string, prefix string) (string, string) {
	rest := strings.Trim(strings.TrimPrefix(path, prefix), "/")
	if rest == "" {
		return "", ""
	}
	parts := strings.Split(rest, "/")
	if len(parts) == 1 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func sortedPermissions(values map[string]struct{}) []string {
	items := make([]string, 0, len(values))
	for value := range values {
		items = append(items, value)
	}
	sort.Strings(items)
	return items
}

func maskSessionToken(token string) string {
	token = strings.TrimSpace(token)
	if len(token) <= 10 {
		return token
	}
	return token[:6] + "..." + token[len(token)-4:]
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}
