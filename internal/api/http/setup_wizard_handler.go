package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"

	"tars/internal/api/dto"
	"tars/internal/modules/access"
	"tars/internal/modules/reasoning"
	postgresrepo "tars/internal/repo/postgres"
)

type setupWizardAdminRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Password    string `json:"password"`
}

type setupWizardAuthRequest struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type setupWizardProviderRequest struct {
	ProviderID string `json:"provider_id"`
	Vendor     string `json:"vendor"`
	Protocol   string `json:"protocol"`
	BaseURL    string `json:"base_url"`
	APIKey     string `json:"api_key"`
	APIKeyRef  string `json:"api_key_ref"`
	Model      string `json:"model"`
}

type setupWizardChannelRequest struct {
	ChannelID    string   `json:"channel_id"`
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	Usages       []string `json:"usages"`
	Capabilities []string `json:"capabilities"`
	Type         string   `json:"type"`
	Target       string   `json:"target"`
}

func setupWizardHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		initialization, err := buildSetupInitializationStatus(r, deps)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		if !setupWizardAccessAllowed(deps, r, initialization) && !requireOpsAccess(deps, w, r) {
			return
		}
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		writeJSON(w, http.StatusOK, buildSetupWizardResponse(deps, initialization))
	}
}

func setupWizardAdminHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		state, ok := requireSetupWizardWrite(deps, w, r)
		if !ok {
			return
		}
		var req setupWizardAdminRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
			return
		}
		username := strings.TrimSpace(req.Username)
		password := strings.TrimSpace(req.Password)
		if username == "" || password == "" {
			writeError(w, http.StatusBadRequest, "validation_failed", "username and password are required")
			return
		}
		if err := validateSetupAdminPassword(password); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		user, err := deps.Access.UpsertUser(access.User{
			UserID:               username,
			Username:             username,
			DisplayName:          strings.TrimSpace(req.DisplayName),
			Email:                strings.TrimSpace(req.Email),
			Status:               "active",
			Source:               "setup_wizard",
			PasswordHash:         string(hash),
			PasswordLoginEnabled: true,
			Roles:                []string{"platform_admin"},
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		authProvider, err := ensureSetupLocalPasswordProvider(deps)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		state.AdminUserID = user.UserID
		state.AuthProviderID = authProvider.ID
		state.CurrentStep = "provider"
		state.LoginHint.Provider = authProvider.ID
		state.LoginHint.Username = user.Username
		state.LoginHint.LoginURL = setupWizardLoginURL(authProvider.ID, user.Username)
		if err := persistAccessConfig(r.Context(), deps); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		if err := saveSetupState(r.Context(), deps, state); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, buildSetupWizardResponse(deps, setupInitializationFromState(state, deps)))
	}
}

func setupWizardAuthHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		state, ok := requireSetupWizardWrite(deps, w, r)
		if !ok {
			return
		}
		var req setupWizardAuthRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
			return
		}
		providerType := strings.TrimSpace(req.Type)
		if providerType == "" {
			providerType = "local_password"
		}
		if providerType != "local_password" && providerType != "local_token" {
			writeError(w, http.StatusBadRequest, "validation_failed", "unsupported auth provider type")
			return
		}
		provider, err := ensureSetupAuthProvider(deps, providerType, strings.TrimSpace(req.Name))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		state.AuthProviderID = provider.ID
		state.CurrentStep = "provider"
		state.LoginHint.Provider = provider.ID
		state.LoginHint.LoginURL = setupWizardLoginURL(provider.ID, state.AdminUserID)
		if err := persistAccessConfig(r.Context(), deps); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		if err := saveSetupState(r.Context(), deps, state); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, buildSetupWizardResponse(deps, setupInitializationFromState(state, deps)))
	}
}

func setupWizardProviderCheckHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if _, ok := requireSetupWizardWrite(deps, w, r); !ok {
			return
		}
		entry, model, err := decodeSetupWizardProviderEntry(r, deps, false)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		checkResult, err := executeSetupWizardProviderCheck(r.Context(), entry, model)
		if err != nil {
			writeError(w, http.StatusBadGateway, "provider_error", fmt.Sprintf("provider connectivity check failed: %v", err))
			return
		}
		writeJSON(w, http.StatusOK, checkResult)
	}
}

func setupWizardProviderHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		state, ok := requireSetupWizardWrite(deps, w, r)
		if !ok {
			return
		}
		entry, model, err := decodeSetupWizardProviderEntry(r, deps, true)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		checkResult, err := executeSetupWizardProviderCheck(r.Context(), entry, model)
		if err != nil {
			writeError(w, http.StatusBadGateway, "provider_error", fmt.Sprintf("provider connectivity check failed: %v", err))
			return
		}
		if !checkResult.Available {
			writeError(w, http.StatusBadRequest, "validation_failed", firstNonEmpty(strings.TrimSpace(checkResult.Detail), "provider connectivity check failed"))
			return
		}
		entry.APIKey = ""
		cfg := deps.Providers.Snapshot().Config
		updated := false
		for i := range cfg.Entries {
			if cfg.Entries[i].ID == entry.ID {
				cfg.Entries[i] = entry
				updated = true
				break
			}
		}
		if !updated {
			cfg.Entries = append(cfg.Entries, entry)
		}
		cfg.Primary = reasoning.ProviderBinding{ProviderID: entry.ID, Model: model}
		if err := deps.Providers.SaveConfig(cfg); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		state.PrimaryProviderID = entry.ID
		state.PrimaryModel = model
		state.ProviderChecked = true
		state.ProviderCheckOK = true
		state.ProviderCheckNote = firstNonEmpty(strings.TrimSpace(checkResult.Detail), "provider connectivity check succeeded")
		state.CurrentStep = "channel"
		if err := saveSetupState(r.Context(), deps, state); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, buildSetupWizardResponse(deps, setupInitializationFromState(state, deps)))
	}
}

func setupWizardChannelHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		state, ok := requireSetupWizardWrite(deps, w, r)
		if !ok {
			return
		}
		var req setupWizardChannelRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
			return
		}
		channelID := strings.TrimSpace(req.ChannelID)
		channelKind := firstNonEmpty(strings.TrimSpace(req.Kind), strings.TrimSpace(req.Type), "telegram")
		target := strings.TrimSpace(req.Target)
		if channelID == "" || target == "" {
			writeError(w, http.StatusBadRequest, "validation_failed", "channel_id and target are required")
			return
		}
		usages := cloneStrings(req.Usages)
		if len(usages) == 0 {
			usages = cloneStrings(req.Capabilities)
		}
		if len(usages) == 0 {
			usages = []string{"approval", "notifications"}
		}
		if err := validateSetupWizardChannelPrerequisites(deps, channelKind); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		channelUsages := make([]access.ChannelUsage, 0, len(usages))
		channelCapabilities := make([]access.ChannelCapability, 0, len(usages))
		for _, usage := range usages {
			trimmed := strings.TrimSpace(usage)
			if trimmed == "" {
				continue
			}
			channelUsages = append(channelUsages, access.ChannelUsage(trimmed))
			channelCapabilities = append(channelCapabilities, access.ChannelCapability(trimmed))
		}
		if err := validateChannelTarget(channelKind, target); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		ch, err := deps.Access.UpsertChannel(access.Channel{
			ID:           channelID,
			Name:         firstNonEmpty(strings.TrimSpace(req.Name), channelID),
			Kind:         channelKind,
			Type:         channelKind,
			Target:       target,
			Enabled:      true,
			Usages:       channelUsages,
			Capabilities: channelCapabilities,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		state.DefaultChannelID = ch.ID
		state.CurrentStep = "complete"
		if err := persistAccessConfig(r.Context(), deps); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		if err := saveSetupState(r.Context(), deps, state); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, buildSetupWizardResponse(deps, setupInitializationFromState(state, deps)))
	}
}

func setupWizardCompleteHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		state, ok := requireSetupWizardWrite(deps, w, r)
		if !ok {
			return
		}
		initialization := setupInitializationFromState(state, deps)
		if !initialization.AdminConfigured || !initialization.AuthConfigured || !initialization.ProviderReady || !initialization.ChannelReady || !initialization.ProviderChecked || !initialization.ProviderCheckOK {
			writeError(w, http.StatusBadRequest, "validation_failed", "required setup steps are incomplete")
			return
		}
		state.Initialized = true
		state.CurrentStep = ""
		state.CompletedAt = time.Now().UTC()
		state.UpdatedAt = state.CompletedAt
		state.LoginHint.LoginURL = setupWizardLoginURL(state.AuthProviderID, state.AdminUserID)
		if err := persistAccessConfig(r.Context(), deps); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		if err := saveSetupState(r.Context(), deps, state); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, buildSetupWizardResponse(deps, setupInitializationFromState(state, deps)))
	}
}

func buildSetupInitializationStatus(r *http.Request, deps Dependencies) (dto.SetupInitializationStatus, error) {
	state := postgresrepo.SetupState{}
	var err error
	if deps.RuntimeConfig != nil {
		state, err = deps.RuntimeConfig.LoadSetupState(r.Context())
		if err != nil {
			return dto.SetupInitializationStatus{}, err
		}
		return setupInitializationFromState(state, deps), nil
	}
	return setupInitializationFromState(state, deps), nil
}

func setupInitializationFromState(state postgresrepo.SetupState, deps Dependencies) dto.SetupInitializationStatus {
	adminConfigured := strings.TrimSpace(state.AdminUserID) != ""
	if !adminConfigured && deps.Access != nil {
		users := deps.Access.ListUsers()
		for _, item := range users {
			if item.Status == "active" && hasAnyRole(item.Roles, "platform_admin", "ops_admin") {
				state.AdminUserID = item.UserID
				adminConfigured = true
				break
			}
		}
	}
	authConfigured := strings.TrimSpace(state.AuthProviderID) != ""
	if !authConfigured && deps.Access != nil {
		for _, item := range deps.Access.ListAuthProviders() {
			if item.Enabled && (item.Type == "local_password" || item.Type == "local_token") {
				state.AuthProviderID = item.ID
				authConfigured = true
				break
			}
		}
	}
	providerReady := strings.TrimSpace(state.PrimaryProviderID) != "" && strings.TrimSpace(state.PrimaryModel) != ""
	if !providerReady && deps.Providers != nil {
		snapshot := deps.Providers.Snapshot()
		if strings.TrimSpace(snapshot.Config.Primary.ProviderID) != "" && strings.TrimSpace(snapshot.Config.Primary.Model) != "" {
			state.PrimaryProviderID = snapshot.Config.Primary.ProviderID
			state.PrimaryModel = snapshot.Config.Primary.Model
			providerReady = true
		}
	}
	channelReady := strings.TrimSpace(state.DefaultChannelID) != ""
	if !channelReady && deps.Access != nil {
		for _, item := range deps.Access.ListChannels() {
			if item.Enabled {
				state.DefaultChannelID = item.ID
				channelReady = true
				break
			}
		}
	}
	completed := make([]string, 0, 4)
	if adminConfigured {
		completed = append(completed, "admin")
	}
	if authConfigured {
		completed = append(completed, "auth")
	}
	if providerReady {
		completed = append(completed, "provider")
	}
	if channelReady {
		completed = append(completed, "channel")
	}
	nextStep := firstMissingStep(adminConfigured, authConfigured, providerReady, channelReady)
	mode := "wizard"
	if state.Initialized {
		mode = "runtime"
		nextStep = ""
	}
	return dto.SetupInitializationStatus{
		Initialized:       state.Initialized,
		Mode:              mode,
		CurrentStep:       firstNonEmpty(state.CurrentStep, nextStep),
		AdminConfigured:   adminConfigured,
		AuthConfigured:    authConfigured,
		ProviderReady:     providerReady,
		ChannelReady:      channelReady,
		ProviderChecked:   state.ProviderChecked,
		ProviderCheckOK:   state.ProviderCheckOK,
		ProviderCheckNote: state.ProviderCheckNote,
		AdminUserID:       state.AdminUserID,
		AuthProviderID:    state.AuthProviderID,
		PrimaryProviderID: state.PrimaryProviderID,
		PrimaryModel:      state.PrimaryModel,
		DefaultChannelID:  state.DefaultChannelID,
		LoginHint: dto.SetupLoginHint{
			Username: state.LoginHint.Username,
			Provider: state.LoginHint.Provider,
			LoginURL: state.LoginHint.LoginURL,
		},
		CompletedAt:    state.CompletedAt,
		UpdatedAt:      state.UpdatedAt,
		NextStep:       nextStep,
		RequiredSteps:  []string{"admin", "auth", "provider", "channel"},
		CompletedSteps: completed,
	}
}

func buildSetupWizardResponse(deps Dependencies, initialization dto.SetupInitializationStatus) dto.SetupWizardResponse {
	resp := dto.SetupWizardResponse{
		Initialization: initialization,
		Auth: dto.SetupWizardAuth{
			SupportedTypes:  []string{"local_password", "local_token"},
			RecommendedType: "local_password",
		},
	}
	if deps.Access != nil {
		if user, ok := deps.Access.GetUser(initialization.AdminUserID); ok {
			resp.Admin.User = userToDTO(user)
		}
		if provider, ok := deps.Access.GetAuthProvider(initialization.AuthProviderID); ok {
			resp.Auth.Provider = authProviderToDTO(provider)
		}
		if channel, ok := deps.Access.GetChannel(initialization.DefaultChannelID); ok {
			resp.Channel.Channel = channelToDTO(channel)
		}
	}
	if deps.Providers != nil {
		if entry, cfg, found := getProviderByID(deps.Providers, initialization.PrimaryProviderID); found {
			resp.Provider.Provider = providerRegistryEntryToDTO(entry, cfg, deps.Org)
		}
	}
	return resp
}

func requireSetupWizardWrite(deps Dependencies, w http.ResponseWriter, r *http.Request) (postgresrepo.SetupState, bool) {
	status, err := buildSetupInitializationStatus(r, deps)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
		return postgresrepo.SetupState{}, false
	}
	if !setupWizardAccessAllowed(deps, r, status) && !requireOpsAccess(deps, w, r) {
		return postgresrepo.SetupState{}, false
	}
	state := postgresrepo.SetupState{}
	if deps.RuntimeConfig != nil {
		state, err = deps.RuntimeConfig.LoadSetupState(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return postgresrepo.SetupState{}, false
		}
	}
	if state.Initialized {
		writeError(w, http.StatusConflict, "already_initialized", "setup wizard is already completed")
		return postgresrepo.SetupState{}, false
	}
	return state, true
}

func setupWizardAccessAllowed(deps Dependencies, r *http.Request, initialization dto.SetupInitializationStatus) bool {
	if initialization.Initialized {
		return false
	}
	if principal, ok := authenticatedPrincipal(deps, r); ok {
		if deps.Access == nil || deps.Access.Evaluate(principal, "platform.read") {
			return true
		}
		return false
	}
	return !initialization.Initialized
}

func ensureSetupLocalPasswordProvider(deps Dependencies) (access.AuthProvider, error) {
	return ensureSetupAuthProvider(deps, "local_password", "Local Password")
}

func ensureSetupAuthProvider(deps Dependencies, providerType string, name string) (access.AuthProvider, error) {
	providerID := strings.TrimSpace(providerType)
	return deps.Access.UpsertAuthProvider(access.AuthProvider{
		ID:                providerID,
		Type:              providerType,
		Name:              firstNonEmpty(strings.TrimSpace(name), "Setup "+providerType),
		Enabled:           true,
		PasswordMinLength: 8,
		DefaultRoles:      []string{"platform_admin"},
	})
}

func saveSetupState(ctx context.Context, deps Dependencies, state postgresrepo.SetupState) error {
	if deps.RuntimeConfig == nil {
		return nil
	}
	return deps.RuntimeConfig.SaveSetupState(ctx, state)
}

func persistAccessConfig(ctx context.Context, deps Dependencies) error {
	if deps.RuntimeConfig == nil || deps.Access == nil {
		return nil
	}
	return deps.RuntimeConfig.SaveAccessConfig(ctx, deps.Access.Snapshot().Config)
}

func firstMissingStep(adminConfigured, authConfigured, providerReady, channelReady bool) string {
	switch {
	case !adminConfigured:
		return "admin"
	case !authConfigured:
		return "auth"
	case !providerReady:
		return "provider"
	case !channelReady:
		return "channel"
	default:
		return "complete"
	}
}

func isSecretRefFormatValid(ref string) bool {
	ref = strings.TrimSpace(ref)
	return strings.HasPrefix(ref, "secret://") && len(ref) > len("secret://")
}

func lookupRequiredSecret(deps Dependencies, ref string) (string, bool) {
	if deps.Secrets == nil {
		return "", false
	}
	value, ok := deps.Secrets.Get(strings.TrimSpace(ref))
	if !ok || strings.TrimSpace(value) == "" {
		return "", false
	}
	return value, true
}

func validateChannelTarget(channelType string, target string) error {
	channelType = strings.TrimSpace(channelType)
	target = strings.TrimSpace(target)
	if channelType != "telegram" {
		return nil
	}
	if strings.HasPrefix(target, "@") || strings.HasPrefix(target, "-100") {
		return nil
	}
	if strings.HasPrefix(target, "-") {
		target = strings.TrimPrefix(target, "-")
	}
	for _, ch := range target {
		if ch < '0' || ch > '9' {
			return fmt.Errorf("telegram target must be @channel, numeric chat id, or -100... id")
		}
	}
	return nil
}

func setupWizardLoginURL(providerID string, username string) string {
	values := url.Values{}
	if strings.TrimSpace(providerID) != "" {
		values.Set("provider_id", strings.TrimSpace(providerID))
	}
	if strings.TrimSpace(username) != "" {
		values.Set("username", strings.TrimSpace(username))
	}
	values.Set("next", "/runtime-checks")
	encoded := values.Encode()
	if encoded == "" {
		return "/login"
	}
	return "/login?" + encoded
}

func hasAnyRole(items []string, roles ...string) bool {
	for _, item := range items {
		for _, role := range roles {
			if strings.EqualFold(strings.TrimSpace(item), role) {
				return true
			}
		}
	}
	return false
}

var _ = context.Background

func decodeSetupWizardProviderEntry(r *http.Request, deps Dependencies, requireModel bool) (reasoning.ProviderEntry, string, error) {
	var req setupWizardProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return reasoning.ProviderEntry{}, "", fmt.Errorf("invalid request body")
	}
	providerID := strings.TrimSpace(req.ProviderID)
	baseURL := strings.TrimSpace(req.BaseURL)
	model := strings.TrimSpace(req.Model)
	if providerID == "" || baseURL == "" {
		return reasoning.ProviderEntry{}, "", fmt.Errorf("provider_id and base_url are required")
	}
	if requireModel && model == "" {
		return reasoning.ProviderEntry{}, "", fmt.Errorf("provider_id, base_url and model are required")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return reasoning.ProviderEntry{}, "", fmt.Errorf("base_url must be a valid URL")
	}
	apiKeyRef, err := resolveSetupWizardProviderSecretRef(deps, providerID, strings.TrimSpace(req.APIKeyRef), strings.TrimSpace(req.APIKey))
	if err != nil {
		return reasoning.ProviderEntry{}, "", err
	}
	if !isSecretRefFormatValid(apiKeyRef) {
		return reasoning.ProviderEntry{}, "", fmt.Errorf("api_key_ref must use secret://... format")
	}
	secretValue, ok := lookupRequiredSecret(deps, apiKeyRef)
	if !ok {
		return reasoning.ProviderEntry{}, "", fmt.Errorf("api_key_ref is missing from secret store")
	}
	return reasoning.ProviderEntry{
		ID:        providerID,
		Vendor:    firstNonEmpty(strings.TrimSpace(req.Vendor), "openai"),
		Protocol:  firstNonEmpty(strings.TrimSpace(req.Protocol), "openai_compatible"),
		BaseURL:   baseURL,
		APIKeyRef: apiKeyRef,
		APIKey:    secretValue,
		Enabled:   true,
	}, model, nil
}

func resolveSetupWizardProviderSecretRef(deps Dependencies, providerID string, apiKeyRef string, rawAPIKey string) (string, error) {
	apiKeyRef = strings.TrimSpace(apiKeyRef)
	rawAPIKey = strings.TrimSpace(rawAPIKey)
	if rawAPIKey == "" {
		if apiKeyRef == "" {
			return "", fmt.Errorf("api_key or api_key_ref is required")
		}
		return apiKeyRef, nil
	}
	if deps.Secrets == nil || strings.TrimSpace(deps.Secrets.Snapshot().Path) == "" {
		return "", fmt.Errorf("secret store is not configured")
	}
	if apiKeyRef == "" {
		apiKeyRef = fmt.Sprintf("secret://providers/%s/api-key", providerID)
	}
	if !isSecretRefFormatValid(apiKeyRef) {
		return "", fmt.Errorf("api_key_ref must use secret://... format")
	}
	if _, err := deps.Secrets.Apply(map[string]string{apiKeyRef: rawAPIKey}, nil, time.Now().UTC()); err != nil {
		return "", fmt.Errorf("failed to store provider api key: %w", err)
	}
	return apiKeyRef, nil
}

func executeSetupWizardProviderCheck(ctx context.Context, entry reasoning.ProviderEntry, model string) (dto.ProviderCheckResponse, error) {
	result, err := reasoning.CheckProviderAvailability(ctx, entry, model)
	if err != nil {
		return dto.ProviderCheckResponse{}, err
	}
	return dto.ProviderCheckResponse{
		ProviderID: entry.ID,
		Available:  result.Available,
		Detail:     result.Detail,
	}, nil
}

func validateSetupAdminPassword(password string) error {
	var missing []string
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}
	if len([]rune(password)) < 8 {
		missing = append(missing, "use at least 8 characters")
	}
	if !hasUpper {
		missing = append(missing, "include an uppercase letter")
	}
	if !hasLower {
		missing = append(missing, "include a lowercase letter")
	}
	if !hasDigit {
		missing = append(missing, "include a number")
	}
	if !hasSpecial {
		missing = append(missing, "include a symbol")
	}
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("password must %s", strings.Join(missing, ", "))
}

func validateSetupWizardChannelPrerequisites(deps Dependencies, channelType string) error {
	if strings.TrimSpace(channelType) == "telegram" && strings.TrimSpace(deps.Config.Telegram.BotToken) == "" {
		return fmt.Errorf("telegram bot token must be configured before selecting Telegram as the primary channel")
	}
	return nil
}
