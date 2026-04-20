package httpapi

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"tars/internal/api/dto"
	"tars/internal/modules/org"
)

var syntheticIDPattern = regexp.MustCompile(`^[a-z]{3}-\d{6}$`)

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

func writeJSONAttachment(w http.ResponseWriter, status int, filename string, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)

	if payload != nil {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		_ = encoder.Encode(payload)
	}
}

func writeValidationError(w http.ResponseWriter, message string) {
	writeError(w, http.StatusBadRequest, "validation_failed", message)
}

func writeTooManyIDsError(w http.ResponseWriter, max int) {
	writeValidationError(w, "ids exceeds maximum batch size of "+strconv.Itoa(max))
}

func isValidResourceID(id string) bool {
	if _, err := uuid.Parse(id); err == nil {
		return true
	}
	return syntheticIDPattern.MatchString(id)
}

func writeError(w http.ResponseWriter, status int, code string, message string) {
	writeJSON(w, status, dto.ErrorEnvelope{
		Error: dto.ErrorBody{
			Code:    code,
			Message: message,
		},
	})
}

func ownershipValue(value string, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(fallback)
}

func defaultAffiliation(manager *org.Manager) dto.OrgAffiliation {
	if manager == nil {
		return dto.OrgAffiliation{}
	}
	return dto.OrgAffiliation{
		OrgID:       strings.TrimSpace(manager.DefaultOrg().ID),
		TenantID:    strings.TrimSpace(manager.DefaultTenant().ID),
		WorkspaceID: strings.TrimSpace(manager.DefaultWorkspace().ID),
	}
}
