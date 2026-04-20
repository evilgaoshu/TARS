package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"tars/internal/api/dto"
	"tars/internal/modules/agentrole"
)

var errLegacyProviderPreference = errors.New("provider_preference has been removed; use model_binding")

func agentRolesListHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if deps.AgentRoles == nil {
			writeError(w, http.StatusConflict, "not_configured", "agent role manager is not configured")
			return
		}
		switch r.Method {
		case http.MethodGet:
			query := parseListQuery(r)
			roles := deps.AgentRoles.List()
			items := filterAgentRoles(roles, query.Query)
			sortAgentRoles(items, query.SortBy, query.SortOrder)
			dtos := make([]dto.AgentRole, 0, len(items))
			for _, ar := range items {
				dtos = append(dtos, agentRoleDTO(ar))
			}
			pageItems, meta := paginateItems(dtos, query)
			auditOpsRead(r.Context(), deps, "agent_role", "", "agent_roles_listed", map[string]any{"count": len(pageItems)})
			writeJSON(w, http.StatusOK, dto.AgentRoleListResponse{Items: pageItems, ListPage: meta})
		case http.MethodPost:
			req, err := decodeAgentRoleRequest(r)
			if err != nil {
				if errors.Is(err, errLegacyProviderPreference) {
					writeError(w, http.StatusBadRequest, "invalid_agent_role", err.Error())
					return
				}
				writeValidationError(w, "invalid request body")
				return
			}
			created, err := deps.AgentRoles.Create(agentRoleFromDTO(req))
			if err != nil {
				if errors.Is(err, agentrole.ErrRoleIDRequired) {
					writeValidationError(w, "role_id is required")
					return
				}
				if errors.Is(err, agentrole.ErrRoleIDConflict) {
					writeError(w, http.StatusConflict, "conflict", "agent role already exists")
					return
				}
				writeError(w, http.StatusBadRequest, "invalid_agent_role", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "agent_role", created.RoleID, "agent_role_created", nil)
			writeJSON(w, http.StatusCreated, agentRoleDTO(created))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func agentRoleRouterHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		path := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/agent-roles/"), "/")
		if path == "" {
			writeError(w, http.StatusNotFound, "not_found", "agent role not found")
			return
		}
		switch {
		case strings.HasSuffix(path, "/enable"):
			agentRoleEnableHandler(deps, strings.TrimSuffix(path, "/enable"), true)(w, r)
		case strings.HasSuffix(path, "/disable"):
			agentRoleEnableHandler(deps, strings.TrimSuffix(path, "/disable"), false)(w, r)
		default:
			agentRoleDetailHandler(deps, path)(w, r)
		}
	}
}

func agentRoleDetailHandler(deps Dependencies, roleID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.AgentRoles == nil {
			writeError(w, http.StatusConflict, "not_configured", "agent role manager is not configured")
			return
		}
		switch r.Method {
		case http.MethodGet:
			ar, err := deps.AgentRoles.Get(roleID)
			if err != nil {
				writeError(w, http.StatusNotFound, "not_found", "agent role not found")
				return
			}
			auditOpsRead(r.Context(), deps, "agent_role", roleID, "agent_role_viewed", nil)
			writeJSON(w, http.StatusOK, agentRoleDTO(ar))
		case http.MethodPut:
			req, err := decodeAgentRoleRequest(r)
			if err != nil {
				if errors.Is(err, errLegacyProviderPreference) {
					writeError(w, http.StatusBadRequest, "invalid_agent_role", err.Error())
					return
				}
				writeValidationError(w, "invalid request body")
				return
			}
			req.RoleID = roleID
			updated, err := deps.AgentRoles.Update(agentRoleFromDTO(req))
			if err != nil {
				if errors.Is(err, agentrole.ErrRoleNotFound) {
					writeError(w, http.StatusNotFound, "not_found", "agent role not found")
					return
				}
				writeError(w, http.StatusBadRequest, "invalid_agent_role", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "agent_role", roleID, "agent_role_updated", nil)
			writeJSON(w, http.StatusOK, agentRoleDTO(updated))
		case http.MethodDelete:
			if err := deps.AgentRoles.Delete(roleID); err != nil {
				if errors.Is(err, agentrole.ErrRoleNotFound) {
					writeError(w, http.StatusNotFound, "not_found", "agent role not found")
					return
				}
				if errors.Is(err, agentrole.ErrRoleIsBuiltin) {
					writeError(w, http.StatusForbidden, "forbidden", "cannot delete a built-in agent role")
					return
				}
				writeError(w, http.StatusInternalServerError, "internal", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "agent_role", roleID, "agent_role_deleted", nil)
			writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func agentRoleEnableHandler(deps Dependencies, roleID string, enable bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if deps.AgentRoles == nil {
			writeError(w, http.StatusConflict, "not_configured", "agent role manager is not configured")
			return
		}
		updated, err := deps.AgentRoles.SetEnabled(roleID, enable)
		if err != nil {
			if errors.Is(err, agentrole.ErrRoleNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "agent role not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		action := "agent_role_disabled"
		if enable {
			action = "agent_role_enabled"
		}
		auditOpsWrite(r.Context(), deps, "agent_role", roleID, action, nil)
		writeJSON(w, http.StatusOK, agentRoleDTO(updated))
	}
}

// ---------- helpers ----------

func agentRoleDTO(ar agentrole.AgentRole) dto.AgentRole {
	return dto.AgentRole{
		RoleID:      ar.RoleID,
		DisplayName: ar.DisplayName,
		Description: ar.Description,
		Status:      ar.Status,
		IsBuiltin:   ar.IsBuiltin,
		Profile: dto.AgentRoleProfile{
			SystemPrompt: ar.Profile.SystemPrompt,
			PersonaTags:  ar.Profile.PersonaTags,
		},
		CapabilityBinding: dto.AgentRoleCapabilityBinding{
			AllowedConnectorCapabilities: ar.CapabilityBinding.AllowedConnectorCapabilities,
			DeniedConnectorCapabilities:  ar.CapabilityBinding.DeniedConnectorCapabilities,
			AllowedSkills:                ar.CapabilityBinding.AllowedSkills,
			AllowedSkillTags:             ar.CapabilityBinding.AllowedSkillTags,
			Mode:                         ar.CapabilityBinding.Mode,
		},
		PolicyBinding: dto.AgentRolePolicyBinding{
			MaxRiskLevel:       ar.PolicyBinding.MaxRiskLevel,
			MaxAction:          ar.PolicyBinding.MaxAction,
			RequireApprovalFor: ar.PolicyBinding.RequireApprovalFor,
			HardDeny:           ar.PolicyBinding.HardDeny,
		},
		ModelBinding: agentRoleModelBindingDTO(ar.ModelBinding),
		OrgID:        ar.OrgID,
		TenantID:     ar.TenantID,
		CreatedAt:    ar.CreatedAt,
		UpdatedAt:    ar.UpdatedAt,
	}
}

func agentRoleFromDTO(role dto.AgentRole) agentrole.AgentRole {
	return agentrole.AgentRole{
		RoleID:      strings.TrimSpace(role.RoleID),
		DisplayName: strings.TrimSpace(role.DisplayName),
		Description: strings.TrimSpace(role.Description),
		Status:      strings.TrimSpace(role.Status),
		IsBuiltin:   role.IsBuiltin,
		Profile: agentrole.Profile{
			SystemPrompt: role.Profile.SystemPrompt,
			PersonaTags:  cloneStrings(role.Profile.PersonaTags),
		},
		CapabilityBinding: agentrole.CapabilityBinding{
			AllowedConnectorCapabilities: cloneStrings(role.CapabilityBinding.AllowedConnectorCapabilities),
			DeniedConnectorCapabilities:  cloneStrings(role.CapabilityBinding.DeniedConnectorCapabilities),
			AllowedSkills:                cloneStrings(role.CapabilityBinding.AllowedSkills),
			AllowedSkillTags:             cloneStrings(role.CapabilityBinding.AllowedSkillTags),
			Mode:                         strings.TrimSpace(role.CapabilityBinding.Mode),
		},
		PolicyBinding: agentrole.PolicyBinding{
			MaxRiskLevel:       strings.TrimSpace(role.PolicyBinding.MaxRiskLevel),
			MaxAction:          strings.TrimSpace(role.PolicyBinding.MaxAction),
			RequireApprovalFor: cloneStrings(role.PolicyBinding.RequireApprovalFor),
			HardDeny:           cloneStrings(role.PolicyBinding.HardDeny),
		},
		ModelBinding: agentRoleModelBindingFromDTO(role.ModelBinding),
		OrgID:        strings.TrimSpace(role.OrgID),
		TenantID:     strings.TrimSpace(role.TenantID),
		CreatedAt:    role.CreatedAt,
		UpdatedAt:    role.UpdatedAt,
	}
}

func decodeAgentRoleRequest(r *http.Request) (dto.AgentRole, error) {
	var req dto.AgentRole
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return dto.AgentRole{}, err
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		return dto.AgentRole{}, err
	}
	if _, ok := payload["provider_preference"]; ok {
		return dto.AgentRole{}, errLegacyProviderPreference
	}
	if rawModelBinding, ok := payload["model_binding"]; ok {
		if err := validateAgentRoleModelBindingPayload(rawModelBinding); err != nil {
			return dto.AgentRole{}, err
		}
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return dto.AgentRole{}, err
	}
	return req, nil
}

func validateAgentRoleModelBindingPayload(raw json.RawMessage) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}
	var binding map[string]json.RawMessage
	if err := json.Unmarshal(raw, &binding); err != nil {
		return fmt.Errorf("%w: model_binding must be an object", agentrole.ErrInvalidModelBinding)
	}
	primaryConfigured, err := validateAgentRoleModelTargetBindingPayload("primary", binding["primary"])
	if err != nil {
		return err
	}
	fallbackConfigured, err := validateAgentRoleModelTargetBindingPayload("fallback", binding["fallback"])
	if err != nil {
		return err
	}
	hasInherit := false
	inheritPlatformDefault := false
	if rawInherit, ok := binding["inherit_platform_default"]; ok {
		hasInherit = true
		if err := json.Unmarshal(rawInherit, &inheritPlatformDefault); err != nil {
			return fmt.Errorf("%w: inherit_platform_default must be a boolean", agentrole.ErrInvalidModelBinding)
		}
	}
	if !primaryConfigured && fallbackConfigured && !inheritPlatformDefault {
		return fmt.Errorf("%w: fallback requires either primary or inherit_platform_default=true", agentrole.ErrInvalidModelBinding)
	}
	if !primaryConfigured && !fallbackConfigured && hasInherit && !inheritPlatformDefault {
		return fmt.Errorf("%w: primary is required when inherit_platform_default=false", agentrole.ErrInvalidModelBinding)
	}
	return nil
}

func validateAgentRoleModelTargetBindingPayload(name string, raw json.RawMessage) (bool, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return false, nil
	}
	var target map[string]json.RawMessage
	if err := json.Unmarshal(raw, &target); err != nil {
		return false, fmt.Errorf("%w: %s must be an object", agentrole.ErrInvalidModelBinding, name)
	}
	var providerID string
	if rawProviderID, ok := target["provider_id"]; ok {
		if err := json.Unmarshal(rawProviderID, &providerID); err != nil {
			return false, fmt.Errorf("%w: %s.provider_id must be a string", agentrole.ErrInvalidModelBinding, name)
		}
	}
	var model string
	if rawModel, ok := target["model"]; ok {
		if err := json.Unmarshal(rawModel, &model); err != nil {
			return false, fmt.Errorf("%w: %s.model must be a string", agentrole.ErrInvalidModelBinding, name)
		}
	}
	providerID = strings.TrimSpace(providerID)
	model = strings.TrimSpace(model)
	if providerID == "" && model == "" {
		return false, nil
	}
	if providerID == "" {
		return false, fmt.Errorf("%w: %s.provider_id is required when %s.model is set", agentrole.ErrInvalidModelBinding, name, name)
	}
	if model == "" {
		return false, fmt.Errorf("%w: %s.model is required when %s.provider_id is set", agentrole.ErrInvalidModelBinding, name, name)
	}
	return true, nil
}

func agentRoleModelBindingDTO(binding agentrole.ModelBinding) dto.AgentRoleModelBinding {
	return dto.AgentRoleModelBinding{
		Primary:                agentRoleModelTargetBindingDTO(binding.Primary),
		Fallback:               agentRoleModelTargetBindingDTO(binding.Fallback),
		InheritPlatformDefault: binding.InheritPlatformDefault,
	}
}

func agentRoleModelTargetBindingDTO(binding *agentrole.ModelTargetBinding) *dto.AgentRoleModelTargetBinding {
	if binding == nil {
		return nil
	}
	return &dto.AgentRoleModelTargetBinding{
		ProviderID: strings.TrimSpace(binding.ProviderID),
		Model:      strings.TrimSpace(binding.Model),
	}
}

func agentRoleModelBindingFromDTO(binding dto.AgentRoleModelBinding) agentrole.ModelBinding {
	return agentrole.ModelBinding{
		Primary:                agentRoleModelTargetBindingFromDTO(binding.Primary),
		Fallback:               agentRoleModelTargetBindingFromDTO(binding.Fallback),
		InheritPlatformDefault: binding.InheritPlatformDefault,
	}
}

func agentRoleModelTargetBindingFromDTO(binding *dto.AgentRoleModelTargetBinding) *agentrole.ModelTargetBinding {
	if binding == nil {
		return nil
	}
	target := &agentrole.ModelTargetBinding{
		ProviderID: strings.TrimSpace(binding.ProviderID),
		Model:      strings.TrimSpace(binding.Model),
	}
	if target.ProviderID == "" && target.Model == "" {
		return nil
	}
	return target
}

func filterAgentRoles(roles []agentrole.AgentRole, query string) []agentrole.AgentRole {
	if query == "" {
		return roles
	}
	q := strings.ToLower(query)
	var out []agentrole.AgentRole
	for _, r := range roles {
		if strings.Contains(strings.ToLower(r.RoleID), q) ||
			strings.Contains(strings.ToLower(r.DisplayName), q) ||
			strings.Contains(strings.ToLower(r.Description), q) {
			out = append(out, r)
		}
	}
	return out
}

func sortAgentRoles(roles []agentrole.AgentRole, sortBy, order string) {
	less := func(i, j int) bool {
		switch sortBy {
		case "role_id":
			return strings.ToLower(roles[i].RoleID) < strings.ToLower(roles[j].RoleID)
		case "status":
			return roles[i].Status < roles[j].Status
		case "updated_at":
			return roles[i].UpdatedAt.Before(roles[j].UpdatedAt)
		default:
			return strings.ToLower(roles[i].DisplayName) < strings.ToLower(roles[j].DisplayName)
		}
	}
	if order == "desc" {
		sort.Slice(roles, func(i, j int) bool { return !less(i, j) })
	} else {
		sort.Slice(roles, less)
	}
}
