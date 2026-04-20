package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"tars/internal/api/dto"
	"tars/internal/modules/org"
)

// registerOrgRoutes mounts the Organization/Tenant/Workspace API.
func registerOrgRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/v1/organizations", instrumentHandler(deps, "/api/v1/organizations", orgsHandler(deps)))
	mux.HandleFunc("/api/v1/organizations/", instrumentHandler(deps, "/api/v1/organizations/*", orgDetailHandler(deps)))
	mux.HandleFunc("/api/v1/tenants", instrumentHandler(deps, "/api/v1/tenants", tenantsHandler(deps)))
	mux.HandleFunc("/api/v1/tenants/", instrumentHandler(deps, "/api/v1/tenants/*", tenantDetailHandler(deps)))
	mux.HandleFunc("/api/v1/workspaces", instrumentHandler(deps, "/api/v1/workspaces", workspacesHandler(deps)))
	mux.HandleFunc("/api/v1/workspaces/", instrumentHandler(deps, "/api/v1/workspaces/*", workspaceDetailHandler(deps)))
	mux.HandleFunc("/api/v1/org/context", instrumentHandler(deps, "/api/v1/org/context", orgContextHandler(deps)))
	// ORG-N5: policy endpoints
	mux.HandleFunc("/api/v1/org/policy", instrumentHandler(deps, "/api/v1/org/policy", orgPolicyHandler(deps)))
	mux.HandleFunc("/api/v1/org/policy/resolve", instrumentHandler(deps, "/api/v1/org/policy/resolve", orgPolicyResolveHandler(deps)))
	// Tenant policy is handled inside tenantDetailHandler via rest path matching (/tenants/{id}/policy)
}


// ---------------------------------------------------------------------------
// DTO helpers
// ---------------------------------------------------------------------------

func orgToDTO(o org.Organization) dto.Organization {
	return dto.Organization{
		ID:          o.ID,
		Name:        o.Name,
		Slug:        o.Slug,
		Status:      o.Status,
		Description: o.Description,
		Domain:      o.Domain,
		CreatedAt:   o.CreatedAt,
		UpdatedAt:   o.UpdatedAt,
	}
}

func tenantToDTO(t org.Tenant) dto.Tenant {
	return dto.Tenant{
		ID:              t.ID,
		OrgID:           t.OrgID,
		Name:            t.Name,
		Slug:            t.Slug,
		Status:          t.Status,
		Description:     t.Description,
		DefaultLocale:   t.DefaultLocale,
		DefaultTimezone: t.DefaultTimezone,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

func workspaceToDTO(ws org.Workspace) dto.Workspace {
	return dto.Workspace{
		ID:          ws.ID,
		TenantID:    ws.TenantID,
		OrgID:       ws.OrgID,
		Name:        ws.Name,
		Slug:        ws.Slug,
		Status:      ws.Status,
		Description: ws.Description,
		CreatedAt:   ws.CreatedAt,
		UpdatedAt:   ws.UpdatedAt,
	}
}

func dtoToOrg(d dto.Organization) org.Organization {
	return org.Organization{
		ID:          d.ID,
		Name:        d.Name,
		Slug:        d.Slug,
		Status:      d.Status,
		Description: d.Description,
		Domain:      d.Domain,
	}
}

func dtoToTenant(d dto.Tenant) org.Tenant {
	return org.Tenant{
		ID:              d.ID,
		OrgID:           d.OrgID,
		Name:            d.Name,
		Slug:            d.Slug,
		Status:          d.Status,
		Description:     d.Description,
		DefaultLocale:   d.DefaultLocale,
		DefaultTimezone: d.DefaultTimezone,
	}
}

func dtoToWorkspace(d dto.Workspace) org.Workspace {
	return org.Workspace{
		ID:          d.ID,
		TenantID:    d.TenantID,
		OrgID:       d.OrgID,
		Name:        d.Name,
		Slug:        d.Slug,
		Status:      d.Status,
		Description: d.Description,
	}
}

// ---------------------------------------------------------------------------
// Organizations
// ---------------------------------------------------------------------------

func orgsHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Org == nil {
			writeError(w, http.StatusConflict, "not_configured", "org manager is not configured")
			return
		}
		switch r.Method {
		case http.MethodGet:
			items := deps.Org.ListOrganizations()
			resp := dto.OrganizationListResponse{
				Items:    make([]dto.Organization, 0, len(items)),
				ListPage: dto.ListPage{Page: 1, Limit: len(items) + 1, Total: len(items)},
			}
			for _, item := range items {
				resp.Items = append(resp.Items, orgToDTO(item))
			}
			writeJSON(w, http.StatusOK, resp)

		case http.MethodPost:
			_, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.write")
			if !ok {
				return
			}
			var input dto.Organization
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
				return
			}
			created, err := deps.Org.CreateOrganization(dtoToOrg(input))
			if err != nil {
				if errors.Is(err, org.ErrIDRequired) {
					writeError(w, http.StatusBadRequest, "validation_error", err.Error())
				} else {
					writeError(w, http.StatusConflict, "conflict", err.Error())
				}
				return
			}
			writeJSON(w, http.StatusCreated, orgToDTO(created))

		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func orgDetailHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Org == nil {
			writeError(w, http.StatusConflict, "not_configured", "org manager is not configured")
			return
		}
		orgID := strings.TrimPrefix(r.URL.Path, "/api/v1/organizations/")
		orgID = strings.SplitN(orgID, "/", 2)[0]
		if orgID == "" {
			writeError(w, http.StatusBadRequest, "missing_id", "organization id is required")
			return
		}

		// Check for sub-resources like /api/v1/organizations/{id}/enable
		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/organizations/"+orgID)
		rest = strings.TrimPrefix(rest, "/")

		if rest == "enable" || rest == "disable" {
			_, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.write")
			if !ok {
				return
			}
			status := "active"
			if rest == "disable" {
				status = "disabled"
			}
			updated, err := deps.Org.SetOrganizationStatus(orgID, status)
			if err != nil {
				orgDetailError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, orgToDTO(updated))
			return
		}

		switch r.Method {
		case http.MethodGet:
			item, err := deps.Org.GetOrganization(orgID)
			if err != nil {
				orgDetailError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, orgToDTO(item))

		case http.MethodPut:
			_, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.write")
			if !ok {
				return
			}
			var input dto.Organization
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
				return
			}
			updated, err := deps.Org.UpdateOrganization(orgID, dtoToOrg(input))
			if err != nil {
				orgDetailError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, orgToDTO(updated))

		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func orgDetailError(w http.ResponseWriter, err error) {
	if errors.Is(err, org.ErrOrgNotFound) {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	} else {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Tenants
// ---------------------------------------------------------------------------

func tenantsHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Org == nil {
			writeError(w, http.StatusConflict, "not_configured", "org manager is not configured")
			return
		}
		switch r.Method {
		case http.MethodGet:
			orgID := r.URL.Query().Get("org_id")
			items := deps.Org.ListTenants(orgID)
			resp := dto.TenantListResponse{
				Items:    make([]dto.Tenant, 0, len(items)),
				ListPage: dto.ListPage{Page: 1, Limit: len(items) + 1, Total: len(items)},
			}
			for _, item := range items {
				resp.Items = append(resp.Items, tenantToDTO(item))
			}
			writeJSON(w, http.StatusOK, resp)

		case http.MethodPost:
			_, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.write")
			if !ok {
				return
			}
			var input dto.Tenant
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
				return
			}
			created, err := deps.Org.CreateTenant(dtoToTenant(input))
			if err != nil {
				if errors.Is(err, org.ErrIDRequired) {
					writeError(w, http.StatusBadRequest, "validation_error", err.Error())
				} else {
					writeError(w, http.StatusConflict, "conflict", err.Error())
				}
				return
			}
			writeJSON(w, http.StatusCreated, tenantToDTO(created))

		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func tenantDetailHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Org == nil {
			writeError(w, http.StatusConflict, "not_configured", "org manager is not configured")
			return
		}
		tenantID := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/")
		tenantID = strings.SplitN(tenantID, "/", 2)[0]
		if tenantID == "" {
			writeError(w, http.StatusBadRequest, "missing_id", "tenant id is required")
			return
		}

		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/tenants/"+tenantID)
		rest = strings.TrimPrefix(rest, "/")

		if rest == "enable" || rest == "disable" {
			_, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.write")
			if !ok {
				return
			}
			status := "active"
			if rest == "disable" {
				status = "disabled"
			}
			updated, err := deps.Org.SetTenantStatus(tenantID, status)
			if err != nil {
				tenantDetailError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, tenantToDTO(updated))
			return
		}

		// ORG-N5: tenant policy sub-resource
		if rest == "policy" {
			tenantPolicySubHandler(deps, w, r, tenantID)
			return
		}

		switch r.Method {
		case http.MethodGet:
			item, err := deps.Org.GetTenant(tenantID)
			if err != nil {
				tenantDetailError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, tenantToDTO(item))

		case http.MethodPut:
			_, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.write")
			if !ok {
				return
			}
			var input dto.Tenant
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
				return
			}
			updated, err := deps.Org.UpdateTenant(tenantID, dtoToTenant(input))
			if err != nil {
				tenantDetailError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, tenantToDTO(updated))

		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func tenantDetailError(w http.ResponseWriter, err error) {
	if errors.Is(err, org.ErrTenantNotFound) {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	} else {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Workspaces
// ---------------------------------------------------------------------------

func workspacesHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Org == nil {
			writeError(w, http.StatusConflict, "not_configured", "org manager is not configured")
			return
		}
		switch r.Method {
		case http.MethodGet:
			tenantID := r.URL.Query().Get("tenant_id")
			items := deps.Org.ListWorkspaces(tenantID)
			resp := dto.WorkspaceListResponse{
				Items:    make([]dto.Workspace, 0, len(items)),
				ListPage: dto.ListPage{Page: 1, Limit: len(items) + 1, Total: len(items)},
			}
			for _, item := range items {
				resp.Items = append(resp.Items, workspaceToDTO(item))
			}
			writeJSON(w, http.StatusOK, resp)

		case http.MethodPost:
			_, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.write")
			if !ok {
				return
			}
			var input dto.Workspace
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
				return
			}
			created, err := deps.Org.CreateWorkspace(dtoToWorkspace(input))
			if err != nil {
				if errors.Is(err, org.ErrIDRequired) {
					writeError(w, http.StatusBadRequest, "validation_error", err.Error())
				} else {
					writeError(w, http.StatusConflict, "conflict", err.Error())
				}
				return
			}
			writeJSON(w, http.StatusCreated, workspaceToDTO(created))

		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func workspaceDetailHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Org == nil {
			writeError(w, http.StatusConflict, "not_configured", "org manager is not configured")
			return
		}
		wsID := strings.TrimPrefix(r.URL.Path, "/api/v1/workspaces/")
		wsID = strings.SplitN(wsID, "/", 2)[0]
		if wsID == "" {
			writeError(w, http.StatusBadRequest, "missing_id", "workspace id is required")
			return
		}

		rest := strings.TrimPrefix(r.URL.Path, "/api/v1/workspaces/"+wsID)
		rest = strings.TrimPrefix(rest, "/")

		if rest == "enable" || rest == "disable" {
			_, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.write")
			if !ok {
				return
			}
			status := "active"
			if rest == "disable" {
				status = "disabled"
			}
			updated, err := deps.Org.SetWorkspaceStatus(wsID, status)
			if err != nil {
				workspaceDetailError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, workspaceToDTO(updated))
			return
		}

		switch r.Method {
		case http.MethodGet:
			item, err := deps.Org.GetWorkspace(wsID)
			if err != nil {
				workspaceDetailError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, workspaceToDTO(item))

		case http.MethodPut:
			_, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.write")
			if !ok {
				return
			}
			var input dto.Workspace
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
				return
			}
			updated, err := deps.Org.UpdateWorkspace(wsID, dtoToWorkspace(input))
			if err != nil {
				workspaceDetailError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, workspaceToDTO(updated))

		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func workspaceDetailError(w http.ResponseWriter, err error) {
	if errors.Is(err, org.ErrWorkspaceNotFound) {
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	} else {
		writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
	}
}

// ---------------------------------------------------------------------------
// Org context (single-tenant compat helper)
// ---------------------------------------------------------------------------

func orgContextHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Org == nil {
			writeError(w, http.StatusConflict, "not_configured", "org manager is not configured")
			return
		}
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		writeJSON(w, http.StatusOK, dto.OrgContextResponse{
			DefaultOrg:       orgToDTO(deps.Org.DefaultOrg()),
			DefaultTenant:    tenantToDTO(deps.Org.DefaultTenant()),
			DefaultWorkspace: workspaceToDTO(deps.Org.DefaultWorkspace()),
		})
	}
}

// ---------------------------------------------------------------------------
// ORG-N5: Policy helpers
// ---------------------------------------------------------------------------

func orgPolicyToDTO(p org.OrgPolicy) dto.OrgPolicyDTO {
	return dto.OrgPolicyDTO{
		OrgID:                       p.OrgID,
		AllowedAuthMethods:          p.AllowedAuthMethods,
		RequireMFA:                  p.RequireMFA,
		MaxSessionDurationS:         int64(p.MaxSessionDuration.Seconds()),
		IdleSessionTimeoutS:         int64(p.IdleSessionTimeout.Seconds()),
		RequireApprovalForExecution: p.RequireApprovalForExecution,
		ProhibitSelfApproval:        p.ProhibitSelfApproval,
		DefaultJITRoles:             p.DefaultJITRoles,
		SkillAllowlist:              p.SkillAllowlist,
		SkillBlocklist:              p.SkillBlocklist,
		AuditRetentionDays:          p.AuditRetentionDays,
		KnowledgeRetentionDays:      p.KnowledgeRetentionDays,
		UpdatedAt:                   p.UpdatedAt,
	}
}

func dtoToOrgPolicy(d dto.OrgPolicyDTO) org.OrgPolicy {
	return org.OrgPolicy{
		OrgID:                       d.OrgID,
		AllowedAuthMethods:          d.AllowedAuthMethods,
		RequireMFA:                  d.RequireMFA,
		MaxSessionDuration:          time.Duration(d.MaxSessionDurationS) * time.Second,
		IdleSessionTimeout:          time.Duration(d.IdleSessionTimeoutS) * time.Second,
		RequireApprovalForExecution: d.RequireApprovalForExecution,
		ProhibitSelfApproval:        d.ProhibitSelfApproval,
		DefaultJITRoles:             d.DefaultJITRoles,
		SkillAllowlist:              d.SkillAllowlist,
		SkillBlocklist:              d.SkillBlocklist,
		AuditRetentionDays:          d.AuditRetentionDays,
		KnowledgeRetentionDays:      d.KnowledgeRetentionDays,
	}
}

func tenantPolicyToDTO(p org.TenantPolicy) dto.TenantPolicyDTO {
	out := dto.TenantPolicyDTO{
		TenantID:           p.TenantID,
		OrgID:              p.OrgID,
		AllowedAuthMethods: p.AllowedAuthMethods,
		RequireMFA:         p.RequireMFA,
		DefaultJITRoles:    p.DefaultJITRoles,
		SkillAllowlist:     p.SkillAllowlist,
		SkillBlocklist:     p.SkillBlocklist,
		UpdatedAt:          p.UpdatedAt,
	}
	if p.RequireApprovalForExecution != nil {
		out.RequireApprovalForExecution = p.RequireApprovalForExecution
	}
	if p.ProhibitSelfApproval != nil {
		out.ProhibitSelfApproval = p.ProhibitSelfApproval
	}
	if p.MaxSessionDuration != nil {
		s := int64(p.MaxSessionDuration.Seconds())
		out.MaxSessionDurationS = &s
	}
	if p.IdleSessionTimeout != nil {
		s := int64(p.IdleSessionTimeout.Seconds())
		out.IdleSessionTimeoutS = &s
	}
	return out
}

func dtoToTenantPolicy(d dto.TenantPolicyDTO) org.TenantPolicy {
	p := org.TenantPolicy{
		TenantID:                    d.TenantID,
		OrgID:                       d.OrgID,
		AllowedAuthMethods:          d.AllowedAuthMethods,
		RequireMFA:                  d.RequireMFA,
		RequireApprovalForExecution: d.RequireApprovalForExecution,
		ProhibitSelfApproval:        d.ProhibitSelfApproval,
		DefaultJITRoles:             d.DefaultJITRoles,
		SkillAllowlist:              d.SkillAllowlist,
		SkillBlocklist:              d.SkillBlocklist,
	}
	if d.MaxSessionDurationS != nil {
		dur := time.Duration(*d.MaxSessionDurationS) * time.Second
		p.MaxSessionDuration = &dur
	}
	if d.IdleSessionTimeoutS != nil {
		dur := time.Duration(*d.IdleSessionTimeoutS) * time.Second
		p.IdleSessionTimeout = &dur
	}
	return p
}

func resolvedPolicyToDTO(p org.ResolvedPolicy) dto.ResolvedPolicyDTO {
	return dto.ResolvedPolicyDTO{
		OrgID:                       p.OrgID,
		TenantID:                    p.TenantID,
		AllowedAuthMethods:          p.AllowedAuthMethods,
		RequireMFA:                  p.RequireMFA,
		MaxSessionDurationS:         int64(p.MaxSessionDuration.Seconds()),
		IdleSessionTimeoutS:         int64(p.IdleSessionTimeout.Seconds()),
		RequireApprovalForExecution: p.RequireApprovalForExecution,
		ProhibitSelfApproval:        p.ProhibitSelfApproval,
		DefaultJITRoles:             p.DefaultJITRoles,
		SkillAllowlist:              p.SkillAllowlist,
		SkillBlocklist:              p.SkillBlocklist,
		AuditRetentionDays:          p.AuditRetentionDays,
		KnowledgeRetentionDays:      p.KnowledgeRetentionDays,
	}
}

func secondsToDuration(s int64) time.Duration {
	return time.Duration(s) * time.Second
}

// ---------------------------------------------------------------------------
// ORG-N5: Policy HTTP handlers
// ---------------------------------------------------------------------------

// orgPolicyHandler handles GET /api/v1/org/policy?org_id=... and
// PUT /api/v1/org/policy to set the org-level policy.
func orgPolicyHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Org == nil {
			writeError(w, http.StatusConflict, "not_configured", "org manager is not configured")
			return
		}
		switch r.Method {
		case http.MethodGet:
			orgID := r.URL.Query().Get("org_id")
			if orgID == "" {
				orgID = org.RequestOrgContext(r).OrgID
			}
			if orgID == "" {
				orgID = deps.Org.DefaultOrg().ID
			}
			writeJSON(w, http.StatusOK, orgPolicyToDTO(deps.Org.GetOrgPolicy(orgID)))

		case http.MethodPut:
			_, ok := requireAuthenticatedPrincipal(deps, w, r, "org_policy.write")
			if !ok {
				return
			}
			var input dto.OrgPolicyDTO
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
				return
			}
			if input.OrgID == "" {
				writeError(w, http.StatusBadRequest, "validation_error", "org_id is required")
				return
			}
			if err := deps.Org.SetOrgPolicy(dtoToOrgPolicy(input)); err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
				return
			}
			writeJSON(w, http.StatusOK, orgPolicyToDTO(deps.Org.GetOrgPolicy(input.OrgID)))

		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

// orgPolicyResolveHandler handles GET /api/v1/org/policy/resolve?org_id=...&tenant_id=...
// Returns the effective merged policy.
func orgPolicyResolveHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Org == nil {
			writeError(w, http.StatusConflict, "not_configured", "org manager is not configured")
			return
		}
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		orgCtx := org.RequestOrgContext(r)
		orgID := firstNonEmptyStr(r.URL.Query().Get("org_id"), orgCtx.OrgID, deps.Org.DefaultOrg().ID)
		tenantID := firstNonEmptyStr(r.URL.Query().Get("tenant_id"), orgCtx.TenantID, deps.Org.DefaultTenant().ID)
		writeJSON(w, http.StatusOK, resolvedPolicyToDTO(deps.Org.ResolvePolicy(orgID, tenantID)))
	}
}

// tenantPolicySubHandler handles GET+PUT /api/v1/tenants/{id}/policy.
// Called from tenantDetailHandler when rest == "policy".
func tenantPolicySubHandler(deps Dependencies, w http.ResponseWriter, r *http.Request, tenantID string) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, tenantPolicyToDTO(deps.Org.GetTenantPolicy(tenantID)))
	case http.MethodPut:
		_, ok := requireAuthenticatedPrincipal(deps, w, r, "org_policy.write")
		if !ok {
			return
		}
		var input dto.TenantPolicyDTO
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
			return
		}
		input.TenantID = tenantID
		if err := deps.Org.SetTenantPolicy(dtoToTenantPolicy(input)); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, tenantPolicyToDTO(deps.Org.GetTenantPolicy(tenantID)))
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
