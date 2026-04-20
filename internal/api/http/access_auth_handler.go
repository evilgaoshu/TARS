package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"tars/internal/api/dto"
)

type authLoginRequest struct {
	ProviderID string `json:"provider_id,omitempty"`
	Token      string `json:"token,omitempty"`
	Username   string `json:"username,omitempty"`
	Password   string `json:"password,omitempty"`
	BaseURL    string `json:"base_url,omitempty"`
}

type authConfigUpdateRequest struct {
	Content        string            `json:"content"`
	Config         *dto.AccessConfig `json:"config"`
	OperatorReason string            `json:"operator_reason"`
}

type operatorReasonRequest struct {
	OperatorReason string `json:"operator_reason"`
}

func authProvidersHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		switch r.Method {
		case http.MethodGet:
			items := deps.Access.ListAuthProviders()
			resp := dto.AuthProviderListResponse{Items: make([]dto.AuthProvider, 0, len(items))}
			baseURL := strings.TrimSpace(deps.Config.Server.PublicBaseURL)
			for _, item := range items {
				public := authProviderToDTO(item)
				if (item.Type == "oauth2" || item.Type == "oidc") && item.Enabled {
					public.LoginURL = "/api/v1/auth/login?provider_id=" + item.ID
					if baseURL != "" {
						public.LoginURL = strings.TrimRight(baseURL, "/") + public.LoginURL
					}
				}
				resp.Items = append(resp.Items, public)
			}
			writeJSON(w, http.StatusOK, resp)
		case http.MethodPost:
			principal, ok := requireAuthenticatedPrincipal(deps, w, r, "auth.write")
			if !ok {
				return
			}
			var req struct {
				Provider       dto.AuthProvider `json:"provider"`
				OperatorReason string           `json:"operator_reason"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				req.OperatorReason = "system: " + principal.User.UserID + " created auth_provider"
			}
			provider, err := deps.Access.UpsertAuthProvider(authProviderFromDTO(req.Provider))
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "auth_provider", provider.ID, "auth_provider_created", map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusCreated, authProviderToDTO(provider))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func authProviderDetailHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, mapMethodPermission(r.Method, "auth.read", "auth.write"))
		if !ok {
			return
		}
		providerID, action := nestedResourcePath(r.URL.Path, "/api/v1/auth/providers/")
		if providerID == "" {
			writeError(w, http.StatusNotFound, "not_found", "auth provider not found")
			return
		}
		switch {
		case r.Method == http.MethodGet && action == "":
			provider, found := deps.Access.GetAuthProvider(providerID)
			if !found {
				writeError(w, http.StatusNotFound, "not_found", "auth provider not found")
				return
			}
			auditOpsRead(r.Context(), deps, "auth_provider", providerID, "get", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusOK, authProviderToDTO(provider))
		case r.Method == http.MethodPut && action == "":
			var req struct {
				Provider       dto.AuthProvider `json:"provider"`
				OperatorReason string           `json:"operator_reason"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				req.OperatorReason = "system: " + principal.User.UserID + " updated auth_provider"
			}
			provider := authProviderFromDTO(req.Provider)
			provider.ID = providerID
			updated, err := deps.Access.UpsertAuthProvider(provider)
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "auth_provider", providerID, "auth_provider_updated", map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusOK, authProviderToDTO(updated))
		case r.Method == http.MethodPost && (action == "enable" || action == "disable"):
			var req operatorReasonRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				req.OperatorReason = "system: " + principal.User.UserID + " " + action + "d auth_provider"
			}
			updated, err := deps.Access.SetAuthProviderEnabled(providerID, action == "enable")
			if err != nil {
				writeError(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "auth_provider", providerID, "auth_provider_"+action+"d", map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusOK, authProviderToDTO(updated))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func authLoginHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		if r.Method == http.MethodGet {
			providerID := strings.TrimSpace(r.URL.Query().Get("provider_id"))
			if providerID == "" {
				writeValidationError(w, "provider_id is required")
				return
			}
			url, err := deps.Access.StartOAuthLogin(providerID, deps.Config.Server.PublicBaseURL)
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			http.Redirect(w, r, url, http.StatusFound)
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req authLoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		providerID := firstNonEmpty(strings.TrimSpace(req.ProviderID), "local_token")
		if req.Password != "" || providerID == "local_password" {
			result, err := deps.Access.LoginWithLocalPassword(providerID, req.Username, req.Password)
			if err != nil {
				auditOpsWrite(r.Context(), deps, "auth", providerID, "auth_login_failed", map[string]any{"provider_id": providerID})
				writeError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
				return
			}
			if result.Session.Token != "" {
				principal := principalFromSession(deps.Access, result.Session, result.User)
				auditOpsWrite(r.Context(), deps, "auth", providerID, "auth_login_succeeded", map[string]any{"provider_id": providerID, "user_id": result.User.UserID})
				writeJSON(w, http.StatusOK, dto.AuthLoginResponse{SessionToken: result.Session.Token, User: userToDTO(result.User), Roles: append([]string(nil), principal.RoleIDs...), Permissions: sortedPermissions(principal.Permission), ProviderID: providerID})
				return
			}
			writeJSON(w, http.StatusOK, dto.AuthLoginResponse{ProviderID: providerID, User: userToDTO(result.User), PendingToken: result.PendingToken, NextStep: result.NextStep, ChallengeID: result.ChallengeID, ChallengeChannel: result.ChallengeChannel, ChallengeCode: result.ChallengeCode, ChallengeExpiresAt: result.ChallengeExpiresAt})
			return
		}
		if providerID == "local_token" || req.Token != "" {
			session, user, err := deps.Access.LoginWithLocalToken(req.Token, deps.Config.OpsAPI.Token)
			if err != nil {
				auditOpsWrite(r.Context(), deps, "auth", providerID, "auth_login_failed", map[string]any{"provider_id": providerID})
				writeError(w, http.StatusUnauthorized, "unauthorized", "invalid credentials")
				return
			}
			principal := principalFromSession(deps.Access, session, user)
			auditOpsWrite(r.Context(), deps, "auth", providerID, "auth_login_succeeded", map[string]any{"provider_id": providerID, "user_id": user.UserID})
			writeJSON(w, http.StatusOK, dto.AuthLoginResponse{SessionToken: session.Token, User: userToDTO(user), Roles: append([]string(nil), principal.RoleIDs...), Permissions: sortedPermissions(principal.Permission), ProviderID: providerID})
			return
		}
		url, err := deps.Access.StartOAuthLogin(providerID, firstNonEmpty(req.BaseURL, deps.Config.Server.PublicBaseURL))
		if err != nil {
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, dto.AuthLoginResponse{ProviderID: providerID, RedirectURL: url})
	}
}

func authChallengeHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req dto.AuthChallengeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		result, err := deps.Access.StartChallenge(req.PendingToken)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, dto.AuthChallengeResponse{ProviderID: result.ProviderID, PendingToken: result.PendingToken, NextStep: result.NextStep, ChallengeID: result.ChallengeID, ChallengeChannel: result.ChallengeChannel, ChallengeCode: result.ChallengeCode, ChallengeExpiresAt: result.ChallengeExpiresAt})
	}
}

func authVerifyHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req dto.AuthVerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		result, err := deps.Access.VerifyChallenge(req.PendingToken, req.ChallengeID, req.Code)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		if result.Session.Token != "" {
			principal := principalFromSession(deps.Access, result.Session, result.User)
			writeJSON(w, http.StatusOK, dto.AuthLoginResponse{SessionToken: result.Session.Token, User: userToDTO(result.User), Roles: append([]string(nil), principal.RoleIDs...), Permissions: sortedPermissions(principal.Permission), ProviderID: result.ProviderID})
			return
		}
		writeJSON(w, http.StatusOK, dto.AuthLoginResponse{ProviderID: result.ProviderID, User: userToDTO(result.User), PendingToken: result.PendingToken, NextStep: result.NextStep})
	}
}

func authMFAVerifyHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req dto.AuthMFAVerifyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		result, err := deps.Access.VerifyMFA(req.PendingToken, req.Code)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		principal := principalFromSession(deps.Access, result.Session, result.User)
		writeJSON(w, http.StatusOK, dto.AuthLoginResponse{SessionToken: result.Session.Token, User: userToDTO(result.User), Roles: append([]string(nil), principal.RoleIDs...), Permissions: sortedPermissions(principal.Permission), ProviderID: result.ProviderID})
	}
}

func authCallbackHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		providerID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/auth/callback/"), "/")
		if providerID == "" {
			writeError(w, http.StatusNotFound, "not_found", "auth provider not found")
			return
		}
		session, user, err := deps.Access.CompleteOAuthCallback(r.Context(), providerID, deps.Config.Server.PublicBaseURL, r.URL.Query().Get("state"), r.URL.Query().Get("code"))
		if err != nil {
			auditOpsWrite(r.Context(), deps, "auth", providerID, "auth_login_failed", map[string]any{"provider_id": providerID, "error": err.Error()})
			writeError(w, http.StatusUnauthorized, "unauthorized", err.Error())
			return
		}
		provider, _ := deps.Access.GetAuthProvider(providerID)
		auditOpsWrite(r.Context(), deps, "auth", providerID, "auth_login_succeeded", map[string]any{"provider_id": providerID, "user_id": user.UserID})
		redirectTarget := firstNonEmpty(provider.SuccessRedirect, "/login")
		separator := "?"
		if strings.Contains(redirectTarget, "?") {
			separator = "&"
		}
		http.Redirect(w, r, redirectTarget+separator+"session_token="+session.Token, http.StatusFound)
	}
}

func authLogoutHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, "")
		if !ok {
			return
		}
		if principal.Kind == "session" && deps.Access != nil {
			deps.Access.Logout(principal.Token)
		}
		auditOpsWrite(r.Context(), deps, "auth", principal.Source, "auth_logout", map[string]any{"user_id": principal.User.UserID})
		writeJSON(w, http.StatusOK, dto.AcceptedResponse{Accepted: true})
	}
}

func authSessionsHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, "")
		if !ok {
			return
		}
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		sessions := deps.Access.ListSessions()
		items := make([]dto.SessionInventoryItem, 0, len(sessions))
		for _, session := range sessions {
			if session.UserID != principal.User.UserID && !deps.Access.Evaluate(principal, "auth.write") {
				continue
			}
			items = append(items, dto.SessionInventoryItem{
				TokenMasked: maskSessionToken(session.Token),
				UserID:      session.UserID,
				ProviderID:  session.ProviderID,
				CreatedAt:   session.CreatedAt,
				ExpiresAt:   session.ExpiresAt,
				LastSeenAt:  session.LastSeenAt,
			})
		}
		auditOpsRead(r.Context(), deps, "auth_sessions", principal.User.UserID, "list", map[string]any{"actor": principal.User.UserID, "count": len(items)})
		writeJSON(w, http.StatusOK, dto.SessionInventoryResponse{Items: items})
	}
}

func meHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, "")
		if !ok {
			return
		}
		resp := dto.MeResponse{User: userToDTO(*principal.User), Roles: append([]string(nil), principal.RoleIDs...), Permissions: sortedPermissions(principal.Permission), AuthSource: principal.Source, BreakGlass: principal.Source == "ops-token"}
		if deps.Access != nil {
			if session, found := deps.Access.GetSession(principal.Token); found {
				resp.SessionToken = session.Token
				resp.SessionExpiresAt = session.ExpiresAt
			}
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

func authConfigHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, mapMethodPermission(r.Method, "auth.read", "configs.write"))
		if !ok {
			return
		}
		if deps.Access == nil {
			writeError(w, http.StatusConflict, "not_configured", "access manager is not configured")
			return
		}
		switch r.Method {
		case http.MethodGet:
			snapshot := deps.Access.Snapshot()
			writeJSON(w, http.StatusOK, dto.AccessConfigResponse{Configured: strings.TrimSpace(snapshot.Path) != "", Loaded: snapshot.Loaded, Path: snapshot.Path, UpdatedAt: snapshot.UpdatedAt, Content: snapshot.Content, Config: accessConfigToDTO(snapshot.Config)})
		case http.MethodPut:
			var req authConfigUpdateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				req.OperatorReason = "system: " + principal.User.UserID + " updated auth config"
			}
			switch {
			case strings.TrimSpace(req.Content) != "":
				if err := deps.Access.Save(req.Content); err != nil {
					writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
					return
				}
			case req.Config != nil:
				if err := deps.Access.SaveConfig(accessConfigFromDTO(*req.Config)); err != nil {
					writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
					return
				}
			default:
				writeValidationError(w, "content or config is required")
				return
			}
			snapshot := deps.Access.Snapshot()
			auditOpsWrite(r.Context(), deps, "auth_config", snapshot.Path, "update", map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusOK, dto.AccessConfigResponse{Configured: strings.TrimSpace(snapshot.Path) != "", Loaded: snapshot.Loaded, Path: snapshot.Path, UpdatedAt: snapshot.UpdatedAt, Content: snapshot.Content, Config: accessConfigToDTO(snapshot.Config)})
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}
