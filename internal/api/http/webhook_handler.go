package httpapi

import (
	"net/http"
	"strings"

	"tars/internal/api/dto"
)

func webhookHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		signature := r.Header.Get("X-Tars-Signature")
		if secret := strings.TrimSpace(deps.Config.VMAlert.WebhookSecret); secret != "" {
			if signature == "" || signature != secret {
				if deps.Metrics != nil {
					deps.Metrics.AddAlertEvents("vmalert", "invalid_signature", 1)
				}
				writeError(w, http.StatusUnauthorized, "invalid_signature", "signature verification failed")
				return
			}
		}

		body, err := readBody(r)
		if err != nil {
			if deps.Metrics != nil {
				deps.Metrics.AddAlertEvents("vmalert", "invalid_body", 1)
			}
			writeError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
			return
		}

		events, err := deps.AlertIngest.IngestVMAlert(r.Context(), body)
		if err != nil {
			if deps.Metrics != nil {
				deps.Metrics.AddAlertEvents("vmalert", "validation_failed", 1)
			}
			writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
			return
		}

		sessionIDs := make([]string, 0, len(events))
		duplicated := false
		for _, event := range events {
			result, handleErr := deps.Workflow.HandleAlertEvent(r.Context(), event)
			if handleErr != nil {
				if deps.Metrics != nil {
					deps.Metrics.AddAlertEvents("vmalert", "internal_error", 1)
				}
				writeError(w, http.StatusInternalServerError, "internal_error", handleErr.Error())
				return
			}

			sessionIDs = append(sessionIDs, result.SessionID)
			duplicated = duplicated || result.Duplicated
			if deps.Metrics != nil {
				resultLabel := "accepted"
				if result.Duplicated {
					resultLabel = "duplicated"
				}
				deps.Metrics.AddAlertEvents("vmalert", resultLabel, 1)
			}
		}

		writeJSON(w, http.StatusOK, dto.VMAlertWebhookResponse{
			Accepted:   true,
			Duplicated: duplicated,
			EventCount: len(events),
			SessionIDs: sessionIDs,
		})
	}
}
