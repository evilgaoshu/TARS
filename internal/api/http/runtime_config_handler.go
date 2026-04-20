package httpapi

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"tars/internal/api/dto"
	"tars/internal/foundation/audit"
	"tars/internal/foundation/secrets"
	"tars/internal/modules/approvalrouting"
	"tars/internal/modules/authorization"
	"tars/internal/modules/org"
	"tars/internal/modules/reasoning"
)

type authorizationConfigUpdateRequest struct {
	Content        string                         `json:"content"`
	Config         *dto.AuthorizationPolicyConfig `json:"config"`
	OperatorReason string                         `json:"operator_reason"`
}

type approvalRoutingConfigUpdateRequest struct {
	Content        string                     `json:"content"`
	Config         *dto.ApprovalRoutingConfig `json:"config"`
	OperatorReason string                     `json:"operator_reason"`
}

type reasoningPromptConfigUpdateRequest struct {
	Content        string                     `json:"content"`
	Config         *dto.ReasoningPromptConfig `json:"config"`
	OperatorReason string                     `json:"operator_reason"`
}

type desensitizationConfigUpdateRequest struct {
	Content        string                     `json:"content"`
	Config         *dto.DesensitizationConfig `json:"config"`
	OperatorReason string                     `json:"operator_reason"`
}

type providersConfigUpdateRequest struct {
	Content        string               `json:"content"`
	Config         *dto.ProvidersConfig `json:"config"`
	OperatorReason string               `json:"operator_reason"`
}

type providerBindingsUpdateRequest struct {
	Bindings       *dto.ProviderBindings `json:"bindings"`
	OperatorReason string                `json:"operator_reason"`
}

func authorizationConfigHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}

		switch r.Method {
		case http.MethodGet:
			handleAuthorizationConfigGet(w, r, deps)
		case http.MethodPut:
			handleAuthorizationConfigPut(w, r, deps)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func approvalRoutingConfigHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}

		switch r.Method {
		case http.MethodGet:
			handleApprovalRoutingConfigGet(w, r, deps)
		case http.MethodPut:
			handleApprovalRoutingConfigPut(w, r, deps)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func reasoningPromptConfigHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}

		switch r.Method {
		case http.MethodGet:
			handleReasoningPromptConfigGet(w, r, deps)
		case http.MethodPut:
			handleReasoningPromptConfigPut(w, r, deps)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func desensitizationConfigHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}

		switch r.Method {
		case http.MethodGet:
			handleDesensitizationConfigGet(w, r, deps)
		case http.MethodPut:
			handleDesensitizationConfigPut(w, r, deps)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func providersConfigHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			handleProvidersConfigGet(w, r, deps)
		case http.MethodPut:
			handleProvidersConfigPut(w, r, deps)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func providersModelsHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		handleProvidersModelsPost(w, r, deps)
	}
}

func providersCheckHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		handleProvidersCheckPost(w, r, deps)
	}
}

func handleAuthorizationConfigGet(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Authorization == nil {
		writeError(w, http.StatusConflict, "not_configured", "authorization manager is not configured")
		return
	}

	snapshot := deps.Authorization.Snapshot()
	auditOpsRead(r.Context(), deps, "authorization_config", fallbackString(snapshot.Path, "runtime"), "get", map[string]any{
		"configured": strings.TrimSpace(snapshot.Path) != "",
		"loaded":     snapshot.Loaded,
	})
	writeJSON(w, http.StatusOK, authorizationConfigResponse(snapshot))
}

func handleAuthorizationConfigPut(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Authorization == nil {
		writeError(w, http.StatusConflict, "not_configured", "authorization manager is not configured")
		return
	}

	var req authorizationConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	if strings.TrimSpace(req.OperatorReason) == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "operator_reason is required")
		return
	}

	switch {
	case strings.TrimSpace(req.Content) != "":
		if err := deps.Authorization.Save(req.Content); err != nil {
			handleAuthorizationSaveError(w, err)
			return
		}
	case req.Config != nil:
		if err := deps.Authorization.SaveConfig(authorizationConfigFromDTO(*req.Config)); err != nil {
			handleAuthorizationSaveError(w, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "validation_failed", "content or config is required")
		return
	}

	snapshot := deps.Authorization.Snapshot()
	logConfigUpdate(r, deps, "authorization_config", fallbackString(snapshot.Path, "runtime"), req.OperatorReason, map[string]any{
		"loaded": snapshot.Loaded,
	})
	writeJSON(w, http.StatusOK, authorizationConfigResponse(snapshot))
}

func handleApprovalRoutingConfigGet(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Approval == nil {
		writeError(w, http.StatusConflict, "not_configured", "approval routing manager is not configured")
		return
	}

	snapshot := deps.Approval.Snapshot()
	auditOpsRead(r.Context(), deps, "approval_routing_config", fallbackString(snapshot.Path, "runtime"), "get", map[string]any{
		"configured": strings.TrimSpace(snapshot.Path) != "",
		"loaded":     snapshot.Loaded,
	})
	writeJSON(w, http.StatusOK, approvalRoutingConfigResponse(snapshot))
}

func handleApprovalRoutingConfigPut(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Approval == nil {
		writeError(w, http.StatusConflict, "not_configured", "approval routing manager is not configured")
		return
	}

	var req approvalRoutingConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	if strings.TrimSpace(req.OperatorReason) == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "operator_reason is required")
		return
	}

	switch {
	case strings.TrimSpace(req.Content) != "":
		if err := deps.Approval.Save(req.Content); err != nil {
			handleApprovalSaveError(w, err)
			return
		}
	case req.Config != nil:
		if err := deps.Approval.SaveConfig(approvalRoutingConfigFromDTO(*req.Config)); err != nil {
			handleApprovalSaveError(w, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "validation_failed", "content or config is required")
		return
	}

	snapshot := deps.Approval.Snapshot()
	logConfigUpdate(r, deps, "approval_routing_config", fallbackString(snapshot.Path, "runtime"), req.OperatorReason, map[string]any{
		"loaded": snapshot.Loaded,
	})
	writeJSON(w, http.StatusOK, approvalRoutingConfigResponse(snapshot))
}

func handleReasoningPromptConfigGet(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Prompts == nil {
		writeError(w, http.StatusConflict, "not_configured", "reasoning prompts manager is not configured")
		return
	}

	snapshot := deps.Prompts.Snapshot()
	auditOpsRead(r.Context(), deps, "reasoning_prompt_config", fallbackString(snapshot.Path, "runtime"), "get", map[string]any{
		"configured": strings.TrimSpace(snapshot.Path) != "",
		"loaded":     snapshot.Loaded,
	})
	writeJSON(w, http.StatusOK, reasoningPromptConfigResponse(snapshot))
}

func handleReasoningPromptConfigPut(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Prompts == nil {
		writeError(w, http.StatusConflict, "not_configured", "reasoning prompts manager is not configured")
		return
	}

	var req reasoningPromptConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	if strings.TrimSpace(req.OperatorReason) == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "operator_reason is required")
		return
	}

	switch {
	case strings.TrimSpace(req.Content) != "":
		if err := deps.Prompts.Save(req.Content); err != nil {
			handlePromptSaveError(w, err)
			return
		}
	case req.Config != nil:
		if err := deps.Prompts.SavePromptSet(reasoning.PromptSet{
			SystemPrompt:       req.Config.SystemPrompt,
			UserPromptTemplate: req.Config.UserPromptTemplate,
		}); err != nil {
			handlePromptSaveError(w, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "validation_failed", "content or config is required")
		return
	}

	snapshot := deps.Prompts.Snapshot()
	logConfigUpdate(r, deps, "reasoning_prompt_config", fallbackString(snapshot.Path, "runtime"), req.OperatorReason, map[string]any{
		"loaded": snapshot.Loaded,
	})
	writeJSON(w, http.StatusOK, reasoningPromptConfigResponse(snapshot))
}

func handleDesensitizationConfigGet(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Desense == nil {
		writeError(w, http.StatusConflict, "not_configured", "desensitization manager is not configured")
		return
	}

	snapshot := deps.Desense.Snapshot()
	auditOpsRead(r.Context(), deps, "desensitization_config", fallbackString(snapshot.Path, "runtime"), "get", map[string]any{
		"configured": strings.TrimSpace(snapshot.Path) != "",
		"loaded":     snapshot.Loaded,
	})
	writeJSON(w, http.StatusOK, desensitizationConfigResponse(snapshot))
}

func handleDesensitizationConfigPut(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Desense == nil {
		writeError(w, http.StatusConflict, "not_configured", "desensitization manager is not configured")
		return
	}

	var req desensitizationConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	if strings.TrimSpace(req.OperatorReason) == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "operator_reason is required")
		return
	}

	switch {
	case strings.TrimSpace(req.Content) != "":
		if err := deps.Desense.Save(req.Content); err != nil {
			handleDesensitizationSaveError(w, err)
			return
		}
	case req.Config != nil:
		if err := deps.Desense.SaveConfig(desensitizationConfigFromDTO(*req.Config)); err != nil {
			handleDesensitizationSaveError(w, err)
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "validation_failed", "content or config is required")
		return
	}

	snapshot := deps.Desense.Snapshot()
	logConfigUpdate(r, deps, "desensitization_config", fallbackString(snapshot.Path, "runtime"), req.OperatorReason, map[string]any{
		"loaded": snapshot.Loaded,
	})
	writeJSON(w, http.StatusOK, desensitizationConfigResponse(snapshot))
}

func handleProvidersConfigGet(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Providers == nil {
		writeError(w, http.StatusConflict, "not_configured", "providers manager is not configured")
		return
	}
	snapshot := deps.Providers.Snapshot()
	auditOpsRead(r.Context(), deps, "providers_config", fallbackString(snapshot.Path, "runtime"), "get", map[string]any{
		"configured": strings.TrimSpace(snapshot.Path) != "",
		"loaded":     snapshot.Loaded,
	})
	writeJSON(w, http.StatusOK, providersConfigResponse(snapshot, deps.Org))
}

func handleProvidersConfigPut(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Providers == nil {
		writeError(w, http.StatusConflict, "not_configured", "providers manager is not configured")
		return
	}
	var req providersConfigUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	if strings.TrimSpace(req.OperatorReason) == "" {
		writeError(w, http.StatusBadRequest, "validation_failed", "operator_reason is required")
		return
	}
	switch {
	case strings.TrimSpace(req.Content) != "":
		if err := deps.Providers.Save(req.Content); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
	case req.Config != nil:
		if err := deps.Providers.SaveConfig(providersConfigFromDTO(*req.Config)); err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
	default:
		writeError(w, http.StatusBadRequest, "validation_failed", "content or config is required")
		return
	}
	snapshot := deps.Providers.Snapshot()
	if deps.RuntimeConfig != nil {
		if err := deps.RuntimeConfig.SaveProvidersConfig(r.Context(), snapshot.Config); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
	}
	metadata := map[string]any{
		"loaded": snapshot.Loaded,
	}
	if req.Config != nil {
		metadata["provider_ids"] = providerEntryIDs(req.Config.Entries)
		metadata["secret_refs"] = providerSecretRefIDs(req.Config.Entries)
	}
	logConfigUpdate(r, deps, "providers_config", fallbackString(snapshot.Path, "runtime"), req.OperatorReason, metadata)
	writeJSON(w, http.StatusOK, providersConfigResponse(snapshot, deps.Org))
}

func handleProvidersModelsPost(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Providers == nil {
		writeError(w, http.StatusConflict, "not_configured", "providers manager is not configured")
		return
	}
	var req dto.ProviderListModelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	var entry reasoning.ProviderEntry
	var err error
	if req.Provider != nil {
		entry = providerRegistryEntryFromDTO(*req.Provider, deps.Org)
	} else {
		entry, err = resolveProviderEntry(deps.Providers, req.ProviderID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
	}
	entry.APIKey = resolveProviderAPIKey(entry, deps.Secrets)
	models, err := reasoning.ListProviderModels(r.Context(), entry)
	if err != nil {
		writeError(w, http.StatusBadGateway, "provider_error", err.Error())
		return
	}
	items := make([]dto.ProviderModelInfo, 0, len(models))
	for _, item := range models {
		items = append(items, dto.ProviderModelInfo{ID: item.ID, Name: item.Name})
	}
	writeJSON(w, http.StatusOK, dto.ProviderListModelsResponse{
		ProviderID: entry.ID,
		Models:     items,
	})
}

func handleProvidersCheckPost(w http.ResponseWriter, r *http.Request, deps Dependencies) {
	if deps.Providers == nil {
		writeError(w, http.StatusConflict, "not_configured", "providers manager is not configured")
		return
	}
	var req dto.ProviderCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
		return
	}
	var entry reasoning.ProviderEntry
	var err error
	if req.Provider != nil {
		entry = providerRegistryEntryFromDTO(*req.Provider, deps.Org)
	} else {
		entry, err = resolveProviderEntry(deps.Providers, req.ProviderID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
	}
	entry.APIKey = resolveProviderAPIKey(entry, deps.Secrets)
	result, err := reasoning.CheckProviderAvailability(r.Context(), entry, strings.TrimSpace(req.Model))
	if err != nil {
		writeError(w, http.StatusBadGateway, "provider_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, dto.ProviderCheckResponse{
		ProviderID: entry.ID,
		Available:  result.Available,
		Detail:     result.Detail,
	})
}

func resolveProviderAPIKey(entry reasoning.ProviderEntry, store *secrets.Store) string {
	if ref := strings.TrimSpace(entry.APIKeyRef); ref != "" && store != nil {
		if value, ok := store.Get(ref); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return strings.TrimSpace(entry.APIKey)
}

func providerEntryIDs(items []dto.ProviderEntry) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		if id := strings.TrimSpace(item.ID); id != "" {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

func providerSecretRefIDs(items []dto.ProviderEntry) []string {
	refs := make([]string, 0, len(items))
	for _, item := range items {
		if ref := strings.TrimSpace(item.APIKeyRef); ref != "" {
			refs = append(refs, ref)
		}
	}
	sort.Strings(refs)
	return refs
}

func authorizationConfigResponse(snapshot authorization.Snapshot) dto.AuthorizationConfigResponse {
	return dto.AuthorizationConfigResponse{
		Configured: strings.TrimSpace(snapshot.Path) != "",
		Loaded:     snapshot.Loaded,
		Path:       snapshot.Path,
		UpdatedAt:  snapshot.UpdatedAt,
		Content:    snapshot.Content,
		Config:     authorizationConfigToDTO(snapshot.Config),
	}
}

func approvalRoutingConfigResponse(snapshot approvalrouting.Snapshot) dto.ApprovalRoutingConfigResponse {
	return dto.ApprovalRoutingConfigResponse{
		Configured: strings.TrimSpace(snapshot.Path) != "",
		Loaded:     snapshot.Loaded,
		Path:       snapshot.Path,
		UpdatedAt:  snapshot.UpdatedAt,
		Content:    snapshot.Content,
		Config:     approvalRoutingConfigToDTO(snapshot.Config),
	}
}

func reasoningPromptConfigResponse(snapshot reasoning.PromptSnapshot) dto.ReasoningPromptConfigResponse {
	return dto.ReasoningPromptConfigResponse{
		Configured: strings.TrimSpace(snapshot.Path) != "",
		Loaded:     snapshot.Loaded,
		Path:       snapshot.Path,
		UpdatedAt:  snapshot.UpdatedAt,
		Content:    snapshot.Content,
		Config: dto.ReasoningPromptConfig{
			SystemPrompt:       snapshot.PromptSet.SystemPrompt,
			UserPromptTemplate: snapshot.PromptSet.UserPromptTemplate,
		},
	}
}

func desensitizationConfigResponse(snapshot reasoning.DesensitizationSnapshot) dto.DesensitizationConfigResponse {
	return dto.DesensitizationConfigResponse{
		Configured: strings.TrimSpace(snapshot.Path) != "",
		Loaded:     snapshot.Loaded,
		Path:       snapshot.Path,
		UpdatedAt:  snapshot.UpdatedAt,
		Content:    snapshot.Content,
		Config:     desensitizationConfigToDTO(snapshot.Config),
	}
}

func providersConfigResponse(snapshot reasoning.ProvidersSnapshot, orgManager *org.Manager) dto.ProvidersConfigResponse {
	return dto.ProvidersConfigResponse{
		Configured: strings.TrimSpace(snapshot.Path) != "",
		Loaded:     snapshot.Loaded,
		Path:       snapshot.Path,
		UpdatedAt:  snapshot.UpdatedAt,
		Content:    redactProvidersContent(snapshot.Content),
		Config:     providersConfigToDTO(snapshot.Config, orgManager),
	}
}

func providerBindingsResponse(snapshot reasoning.ProvidersSnapshot) dto.ProviderBindingsResponse {
	return dto.ProviderBindingsResponse{
		Configured: strings.TrimSpace(snapshot.Path) != "",
		Loaded:     snapshot.Loaded,
		Path:       snapshot.Path,
		UpdatedAt:  snapshot.UpdatedAt,
		Bindings: dto.ProviderBindings{
			Primary: dto.ProviderBinding{
				ProviderID: snapshot.Config.Primary.ProviderID,
				Model:      snapshot.Config.Primary.Model,
			},
			Assist: dto.ProviderBinding{
				ProviderID: snapshot.Config.Assist.ProviderID,
				Model:      snapshot.Config.Assist.Model,
			},
		},
	}
}

func authorizationConfigToDTO(cfg authorization.Config) dto.AuthorizationPolicyConfig {
	whitelistAction := string(cfg.Defaults.WhitelistAction)
	if whitelistAction == "" {
		whitelistAction = string(authorization.ActionDirectExecute)
	}
	blacklistAction := string(cfg.Defaults.BlacklistAction)
	if blacklistAction == "" {
		blacklistAction = string(authorization.ActionSuggestOnly)
	}
	unmatchedAction := string(cfg.Defaults.UnmatchedAction)
	if unmatchedAction == "" {
		unmatchedAction = string(authorization.ActionRequireApproval)
	}
	overrides := make([]dto.AuthorizationOverrideConfig, 0, len(cfg.SSH.Overrides))
	for _, item := range cfg.SSH.Overrides {
		overrides = append(overrides, dto.AuthorizationOverrideConfig{
			ID:           item.ID,
			Services:     cloneStrings(item.Services),
			Hosts:        cloneStrings(item.Hosts),
			Channels:     cloneStrings(item.Channels),
			CommandGlobs: cloneStrings(item.CommandGlobs),
			Action:       string(item.Action),
		})
	}
	return dto.AuthorizationPolicyConfig{
		WhitelistAction:     whitelistAction,
		BlacklistAction:     blacklistAction,
		UnmatchedAction:     unmatchedAction,
		NormalizeWhitespace: cfg.SSH.NormalizeWhitespace,
		HardDenySSHCommand:  cloneStrings(cfg.HardDeny.SSHCommand),
		HardDenyMCPSkill:    cloneStrings(cfg.HardDeny.MCPSkill),
		Whitelist:           cloneStrings(cfg.SSH.Whitelist),
		Blacklist:           cloneStrings(cfg.SSH.Blacklist),
		Overrides:           overrides,
	}
}

func authorizationConfigFromDTO(cfg dto.AuthorizationPolicyConfig) authorization.Config {
	overrides := make([]authorization.OverrideConfig, 0, len(cfg.Overrides))
	for _, item := range cfg.Overrides {
		overrides = append(overrides, authorization.OverrideConfig{
			ID:           strings.TrimSpace(item.ID),
			Services:     cloneStrings(item.Services),
			Hosts:        cloneStrings(item.Hosts),
			Channels:     cloneStrings(item.Channels),
			CommandGlobs: cloneStrings(item.CommandGlobs),
			Action:       authorization.Action(strings.TrimSpace(item.Action)),
		})
	}
	return authorization.Config{
		Defaults: authorization.Defaults{
			WhitelistAction: authorization.Action(strings.TrimSpace(cfg.WhitelistAction)),
			BlacklistAction: authorization.Action(strings.TrimSpace(cfg.BlacklistAction)),
			UnmatchedAction: authorization.Action(strings.TrimSpace(cfg.UnmatchedAction)),
		},
		HardDeny: authorization.HardDenyConfig{
			SSHCommand: cloneStrings(cfg.HardDenySSHCommand),
			MCPSkill:   cloneStrings(cfg.HardDenyMCPSkill),
		},
		SSH: authorization.SSHCommandConfig{
			NormalizeWhitespace: cfg.NormalizeWhitespace,
			Whitelist:           cloneStrings(cfg.Whitelist),
			Blacklist:           cloneStrings(cfg.Blacklist),
			Overrides:           overrides,
		},
	}
}

func desensitizationConfigToDTO(cfg reasoning.DesensitizationConfig) dto.DesensitizationConfig {
	return dto.DesensitizationConfig{
		Enabled: cfg.Enabled,
		Secrets: dto.DesensitizationSecretConfig{
			KeyNames:           cloneStrings(cfg.Secrets.KeyNames),
			QueryKeyNames:      cloneStrings(cfg.Secrets.QueryKeyNames),
			AdditionalPatterns: cloneStrings(cfg.Secrets.AdditionalPatterns),
			RedactBearer:       cfg.Secrets.RedactBearer,
			RedactBasicAuthURL: cfg.Secrets.RedactBasicAuthURL,
			RedactSKTokens:     cfg.Secrets.RedactSKTokens,
		},
		Placeholders: dto.DesensitizationPlaceholderConfig{
			HostKeyFragments:  cloneStrings(cfg.Placeholders.HostKeyFragments),
			PathKeyFragments:  cloneStrings(cfg.Placeholders.PathKeyFragments),
			ReplaceInlineIP:   cfg.Placeholders.ReplaceInlineIP,
			ReplaceInlineHost: cfg.Placeholders.ReplaceInlineHost,
			ReplaceInlinePath: cfg.Placeholders.ReplaceInlinePath,
		},
		Rehydration: dto.DesensitizationRehydrationConfig{
			Host: cfg.Rehydration.Host,
			IP:   cfg.Rehydration.IP,
			Path: cfg.Rehydration.Path,
		},
		LocalLLMAssist: dto.LocalLLMAssistConfig{
			Enabled:  cfg.LocalLLMAssist.Enabled,
			Provider: cfg.LocalLLMAssist.Provider,
			BaseURL:  cfg.LocalLLMAssist.BaseURL,
			Model:    cfg.LocalLLMAssist.Model,
			Mode:     cfg.LocalLLMAssist.Mode,
		},
	}
}

func desensitizationConfigFromDTO(cfg dto.DesensitizationConfig) reasoning.DesensitizationConfig {
	return reasoning.DesensitizationConfig{
		Enabled: cfg.Enabled,
		Secrets: reasoning.SecretRedactionConfig{
			KeyNames:           cloneStrings(cfg.Secrets.KeyNames),
			QueryKeyNames:      cloneStrings(cfg.Secrets.QueryKeyNames),
			AdditionalPatterns: cloneStrings(cfg.Secrets.AdditionalPatterns),
			RedactBearer:       cfg.Secrets.RedactBearer,
			RedactBasicAuthURL: cfg.Secrets.RedactBasicAuthURL,
			RedactSKTokens:     cfg.Secrets.RedactSKTokens,
		},
		Placeholders: reasoning.PlaceholderConfig{
			HostKeyFragments:  cloneStrings(cfg.Placeholders.HostKeyFragments),
			PathKeyFragments:  cloneStrings(cfg.Placeholders.PathKeyFragments),
			ReplaceInlineIP:   cfg.Placeholders.ReplaceInlineIP,
			ReplaceInlineHost: cfg.Placeholders.ReplaceInlineHost,
			ReplaceInlinePath: cfg.Placeholders.ReplaceInlinePath,
		},
		Rehydration: reasoning.RehydrationConfig{
			Host: cfg.Rehydration.Host,
			IP:   cfg.Rehydration.IP,
			Path: cfg.Rehydration.Path,
		},
		LocalLLMAssist: reasoning.LocalLLMAssistConfig{
			Enabled:  cfg.LocalLLMAssist.Enabled,
			Provider: strings.TrimSpace(cfg.LocalLLMAssist.Provider),
			BaseURL:  strings.TrimSpace(cfg.LocalLLMAssist.BaseURL),
			Model:    strings.TrimSpace(cfg.LocalLLMAssist.Model),
			Mode:     strings.TrimSpace(cfg.LocalLLMAssist.Mode),
		},
	}
}

func providersConfigToDTO(cfg reasoning.ProvidersConfig, orgManager *org.Manager) dto.ProvidersConfig {
	defaults := defaultAffiliation(orgManager)
	out := dto.ProvidersConfig{
		Primary: dto.ProviderBinding{
			ProviderID: cfg.Primary.ProviderID,
			Model:      cfg.Primary.Model,
		},
		Assist: dto.ProviderBinding{
			ProviderID: cfg.Assist.ProviderID,
			Model:      cfg.Assist.Model,
		},
		Entries: make([]dto.ProviderEntry, 0, len(cfg.Entries)),
	}
	for _, item := range cfg.Entries {
		out.Entries = append(out.Entries, dto.ProviderEntry{
			ID:          item.ID,
			Vendor:      item.Vendor,
			Protocol:    item.Protocol,
			BaseURL:     item.BaseURL,
			APIKeyRef:   item.APIKeyRef,
			APIKeySet:   strings.TrimSpace(item.APIKey) != "",
			OrgID:       ownershipValue(item.OrgID, defaults.OrgID),
			TenantID:    ownershipValue(item.TenantID, defaults.TenantID),
			WorkspaceID: ownershipValue(item.WorkspaceID, defaults.WorkspaceID),
			Enabled:     item.Enabled,
			Templates:   providerTemplatesToDTO(item.Templates),
		})
	}
	return out
}

func redactProvidersContent(content string) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}
	cfg, _, err := reasoning.ParseProvidersConfig([]byte(content))
	if err != nil || cfg == nil {
		return ""
	}
	for i := range cfg.Entries {
		cfg.Entries[i].APIKey = ""
	}
	encoded, err := reasoning.EncodeProvidersConfig(cfg)
	if err != nil {
		return ""
	}
	return encoded
}

func providersConfigFromDTO(cfg dto.ProvidersConfig) reasoning.ProvidersConfig {
	out := reasoning.ProvidersConfig{
		Primary: reasoning.ProviderBinding{
			ProviderID: strings.TrimSpace(cfg.Primary.ProviderID),
			Model:      strings.TrimSpace(cfg.Primary.Model),
		},
		Assist: reasoning.ProviderBinding{
			ProviderID: strings.TrimSpace(cfg.Assist.ProviderID),
			Model:      strings.TrimSpace(cfg.Assist.Model),
		},
		Entries: make([]reasoning.ProviderEntry, 0, len(cfg.Entries)),
	}
	for _, item := range cfg.Entries {
		out.Entries = append(out.Entries, reasoning.ProviderEntry{
			ID:          strings.TrimSpace(item.ID),
			Vendor:      strings.TrimSpace(item.Vendor),
			Protocol:    strings.TrimSpace(item.Protocol),
			BaseURL:     strings.TrimSpace(item.BaseURL),
			APIKey:      strings.TrimSpace(item.APIKey),
			APIKeyRef:   strings.TrimSpace(item.APIKeyRef),
			OrgID:       strings.TrimSpace(item.OrgID),
			TenantID:    strings.TrimSpace(item.TenantID),
			WorkspaceID: strings.TrimSpace(item.WorkspaceID),
			Enabled:     item.Enabled,
			Templates:   providerTemplatesFromDTO(item.Templates),
		})
	}
	return out
}

func providerTemplatesToDTO(items []reasoning.ProviderTemplate) []dto.ProviderTemplate {
	if len(items) == 0 {
		return nil
	}
	out := make([]dto.ProviderTemplate, 0, len(items))
	for _, item := range items {
		out = append(out, dto.ProviderTemplate{
			ID:          item.ID,
			Name:        item.Name,
			Description: item.Description,
			Values:      cloneConnectorStringMap(item.Values),
			CreatedAt:   item.CreatedAt,
		})
	}
	return out
}

func providerTemplatesFromDTO(items []dto.ProviderTemplate) []reasoning.ProviderTemplate {
	if len(items) == 0 {
		return nil
	}
	out := make([]reasoning.ProviderTemplate, 0, len(items))
	for _, item := range items {
		out = append(out, reasoning.ProviderTemplate{
			ID:          strings.TrimSpace(item.ID),
			Name:        strings.TrimSpace(item.Name),
			Description: strings.TrimSpace(item.Description),
			Values:      cloneConnectorStringMap(item.Values),
			CreatedAt:   item.CreatedAt,
		})
	}
	return out
}

func resolveProviderEntry(manager *reasoning.ProviderManager, providerID string) (reasoning.ProviderEntry, error) {
	snapshot := manager.Snapshot()
	targetID := strings.TrimSpace(providerID)
	if targetID == "" {
		targetID = strings.TrimSpace(snapshot.Config.Primary.ProviderID)
	}
	if targetID == "" {
		return reasoning.ProviderEntry{}, reasoning.ErrProviderNotConfigured
	}
	for _, item := range snapshot.Config.Entries {
		if item.ID == targetID {
			return item, nil
		}
	}
	return reasoning.ProviderEntry{}, reasoning.ErrProviderNotConfigured
}

func approvalRoutingConfigToDTO(cfg approvalrouting.Config) dto.ApprovalRoutingConfig {
	return dto.ApprovalRoutingConfig{
		ProhibitSelfApproval: cfg.ProhibitSelfApproval,
		ServiceOwners:        routeEntriesFromMap(cfg.ServiceOwners),
		OncallGroups:         routeEntriesFromMap(cfg.OncallGroups),
		CommandAllowlist:     routeEntriesFromMap(cfg.CommandAllowlist),
	}
}

func approvalRoutingConfigFromDTO(cfg dto.ApprovalRoutingConfig) approvalrouting.Config {
	return approvalrouting.Config{
		ProhibitSelfApproval: cfg.ProhibitSelfApproval,
		ServiceOwners:        routeEntriesToMap(cfg.ServiceOwners),
		OncallGroups:         routeEntriesToMap(cfg.OncallGroups),
		CommandAllowlist:     routeEntriesToMap(cfg.CommandAllowlist),
	}
}

func routeEntriesFromMap(in map[string][]string) []dto.RouteEntry {
	if len(in) == 0 {
		return nil
	}
	keys := make([]string, 0, len(in))
	for key := range in {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]dto.RouteEntry, 0, len(keys))
	for _, key := range keys {
		out = append(out, dto.RouteEntry{
			Key:     key,
			Targets: cloneStrings(in[key]),
		})
	}
	return out
}

func routeEntriesToMap(in []dto.RouteEntry) map[string][]string {
	out := make(map[string][]string)
	for _, item := range in {
		key := strings.TrimSpace(item.Key)
		values := cloneStrings(item.Targets)
		if key == "" || len(values) == 0 {
			continue
		}
		out[key] = values
	}
	return out
}

func handleAuthorizationSaveError(w http.ResponseWriter, err error) {
	switch {
	case err == authorization.ErrConfigPathNotSet:
		writeError(w, http.StatusConflict, "not_configured", err.Error())
	default:
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
	}
}

func handleApprovalSaveError(w http.ResponseWriter, err error) {
	switch {
	case err == approvalrouting.ErrConfigPathNotSet:
		writeError(w, http.StatusConflict, "not_configured", err.Error())
	default:
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
	}
}

func handlePromptSaveError(w http.ResponseWriter, err error) {
	switch {
	case err == reasoning.ErrPromptsConfigPathNotSet:
		writeError(w, http.StatusConflict, "not_configured", err.Error())
	default:
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
	}
}

func handleDesensitizationSaveError(w http.ResponseWriter, err error) {
	switch {
	case err == reasoning.ErrDesensitizationConfigPathNotSet:
		writeError(w, http.StatusConflict, "not_configured", err.Error())
	default:
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
	}
}

func logConfigUpdate(r *http.Request, deps Dependencies, resourceType string, resourceID string, operatorReason string, metadata map[string]any) {
	if deps.Audit == nil {
		return
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadata["operator_reason"] = operatorReason
	deps.Audit.Log(r.Context(), audit.Entry{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       "update",
		Actor:        "ops_api",
		Metadata:     metadata,
	})
}
