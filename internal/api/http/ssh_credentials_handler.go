package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"tars/internal/api/dto"
	"tars/internal/modules/access"
	"tars/internal/modules/sshcredentials"
)

func sshCredentialsListHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			if deps.SSHCredentials == nil || !deps.SSHCredentials.Configured() {
				writeJSON(w, http.StatusOK, dto.SSHCredentialListResponse{Configured: false, Items: []dto.SSHCredential{}})
				return
			}
			items, err := deps.SSHCredentials.List(r.Context())
			if err != nil {
				writeSSHCredentialError(w, err)
				return
			}
			out := make([]dto.SSHCredential, 0, len(items))
			for _, item := range items {
				out = append(out, sshCredentialDTO(item))
			}
			auditOpsRead(r.Context(), deps, "ssh_credential", "*", "list", map[string]any{"count": len(out)})
			writeJSON(w, http.StatusOK, dto.SSHCredentialListResponse{Configured: true, Items: out})
		case http.MethodPost:
			if deps.SSHCredentials == nil || !deps.SSHCredentials.Configured() {
				writeSSHCredentialError(w, sshcredentials.ErrNotConfigured)
				return
			}
			var req dto.SSHCredentialUpsertRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				writeValidationError(w, "operator_reason is required")
				return
			}
			principal, _ := authenticatedPrincipal(deps, r)
			created, err := deps.SSHCredentials.Create(r.Context(), sshcredentials.CreateInput{
				CredentialID:   req.CredentialID,
				DisplayName:    req.DisplayName,
				OwnerType:      req.OwnerType,
				OwnerID:        req.OwnerID,
				ConnectorID:    req.ConnectorID,
				Username:       req.Username,
				AuthType:       req.AuthType,
				Password:       req.Password,
				PrivateKey:     req.PrivateKey,
				Passphrase:     req.Passphrase,
				HostScope:      req.HostScope,
				ExpiresAt:      req.ExpiresAt,
				OperatorReason: req.OperatorReason,
				ActorID:        principalUserID(principal),
			})
			if err != nil {
				writeSSHCredentialError(w, err)
				return
			}
			auditOpsWrite(r.Context(), deps, "ssh_credential", created.CredentialID, "ssh_credential.created", sshCredentialAuditMetadata(req.OperatorReason, created))
			writeJSON(w, http.StatusCreated, sshCredentialDTO(created))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func sshCredentialRouterHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if deps.SSHCredentials == nil || !deps.SSHCredentials.Configured() {
			writeSSHCredentialError(w, sshcredentials.ErrNotConfigured)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/ssh-credentials/")
		path = strings.TrimSuffix(path, "/")
		if path == "" {
			writeError(w, http.StatusNotFound, "not_found", "ssh credential not found")
			return
		}
		if strings.HasSuffix(path, "/enable") {
			sshCredentialStatusHandler(deps, strings.TrimSuffix(path, "/enable"), sshcredentials.StatusActive)(w, r)
			return
		}
		if strings.HasSuffix(path, "/disable") {
			sshCredentialStatusHandler(deps, strings.TrimSuffix(path, "/disable"), sshcredentials.StatusDisabled)(w, r)
			return
		}
		if strings.HasSuffix(path, "/rotation-required") {
			sshCredentialStatusHandler(deps, strings.TrimSuffix(path, "/rotation-required"), sshcredentials.StatusRotationRequired)(w, r)
			return
		}
		sshCredentialDetailHandler(deps, path)(w, r)
	}
}

func sshCredentialDetailHandler(deps Dependencies, credentialID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			item, ok, err := deps.SSHCredentials.Get(r.Context(), credentialID)
			if err != nil {
				writeSSHCredentialError(w, err)
				return
			}
			if !ok {
				writeSSHCredentialError(w, sshcredentials.ErrNotFound)
				return
			}
			auditOpsRead(r.Context(), deps, "ssh_credential", item.CredentialID, "get", nil)
			writeJSON(w, http.StatusOK, sshCredentialDTO(item))
		case http.MethodPut:
			var req dto.SSHCredentialUpsertRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				writeValidationError(w, "operator_reason is required")
				return
			}
			principal, _ := authenticatedPrincipal(deps, r)
			updated, err := deps.SSHCredentials.Update(r.Context(), credentialID, sshcredentials.UpdateInput{
				DisplayName:    req.DisplayName,
				OwnerType:      req.OwnerType,
				OwnerID:        req.OwnerID,
				ConnectorID:    req.ConnectorID,
				Username:       req.Username,
				Password:       req.Password,
				PrivateKey:     req.PrivateKey,
				Passphrase:     req.Passphrase,
				HostScope:      req.HostScope,
				ExpiresAt:      req.ExpiresAt,
				OperatorReason: req.OperatorReason,
				ActorID:        principalUserID(principal),
			})
			if err != nil {
				writeSSHCredentialError(w, err)
				return
			}
			auditOpsWrite(r.Context(), deps, "ssh_credential", updated.CredentialID, "ssh_credential.updated", sshCredentialAuditMetadata(req.OperatorReason, updated))
			writeJSON(w, http.StatusOK, sshCredentialDTO(updated))
		case http.MethodDelete:
			var req dto.SSHCredentialStatusRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			if strings.TrimSpace(req.OperatorReason) == "" {
				writeValidationError(w, "operator_reason is required")
				return
			}
			principal, _ := authenticatedPrincipal(deps, r)
			deleted, err := deps.SSHCredentials.Delete(r.Context(), credentialID, principalUserID(principal), req.OperatorReason)
			if err != nil {
				writeSSHCredentialError(w, err)
				return
			}
			auditOpsWrite(r.Context(), deps, "ssh_credential", deleted.CredentialID, "ssh_credential.deleted", sshCredentialAuditMetadata(req.OperatorReason, deleted))
			writeJSON(w, http.StatusOK, sshCredentialDTO(deleted))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func sshCredentialStatusHandler(deps Dependencies, credentialID string, status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		var req dto.SSHCredentialStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		if strings.TrimSpace(req.OperatorReason) == "" {
			writeValidationError(w, "operator_reason is required")
			return
		}
		principal, _ := authenticatedPrincipal(deps, r)
		updated, err := deps.SSHCredentials.SetStatus(r.Context(), credentialID, status, principalUserID(principal), req.OperatorReason)
		if err != nil {
			writeSSHCredentialError(w, err)
			return
		}
		action := "ssh_credential.disabled"
		switch status {
		case sshcredentials.StatusActive:
			action = "ssh_credential.enabled"
		case sshcredentials.StatusRotationRequired:
			action = "ssh_credential.rotation_required"
		}
		auditOpsWrite(r.Context(), deps, "ssh_credential", updated.CredentialID, action, sshCredentialAuditMetadata(req.OperatorReason, updated))
		writeJSON(w, http.StatusOK, sshCredentialDTO(updated))
	}
}

func writeSSHCredentialError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sshcredentials.ErrNotConfigured):
		writeError(w, http.StatusConflict, "not_configured", "encrypted ssh credential custody is not configured")
	case errors.Is(err, sshcredentials.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "ssh credential not found")
	case errors.Is(err, sshcredentials.ErrRotationRequired):
		writeError(w, http.StatusConflict, "credential_rotation_required", "ssh credential rotation is required before use")
	case errors.Is(err, sshcredentials.ErrDisabled):
		writeError(w, http.StatusConflict, "credential_disabled", "ssh credential is not active")
	case errors.Is(err, sshcredentials.ErrHostScope):
		writeError(w, http.StatusForbidden, "host_scope_denied", "target host is outside credential scope")
	default:
		writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
	}
}

func sshCredentialDTO(in sshcredentials.Credential) dto.SSHCredential {
	return dto.SSHCredential{
		CredentialID:  in.CredentialID,
		DisplayName:   in.DisplayName,
		OwnerType:     in.OwnerType,
		OwnerID:       in.OwnerID,
		ConnectorID:   in.ConnectorID,
		Username:      in.Username,
		AuthType:      in.AuthType,
		HostScope:     in.HostScope,
		Status:        in.Status,
		CreatedBy:     in.CreatedBy,
		UpdatedBy:     in.UpdatedBy,
		CreatedAt:     in.CreatedAt,
		UpdatedAt:     in.UpdatedAt,
		LastRotatedAt: in.LastRotatedAt,
		ExpiresAt:     in.ExpiresAt,
	}
}

func sshCredentialAuditMetadata(reason string, cred sshcredentials.Credential) map[string]any {
	return map[string]any{
		"operator_reason": reason,
		"credential_id":   cred.CredentialID,
		"connector_id":    cred.ConnectorID,
		"owner_type":      cred.OwnerType,
		"owner_id":        cred.OwnerID,
		"auth_type":       cred.AuthType,
		"host_scope":      cred.HostScope,
		"status":          cred.Status,
	}
}

func principalUserID(principal access.Principal) string {
	if principal.User != nil {
		return principal.User.UserID
	}
	return principal.Source
}
