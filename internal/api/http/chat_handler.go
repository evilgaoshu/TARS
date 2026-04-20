package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"tars/internal/api/dto"
	"tars/internal/contracts"
	"tars/internal/foundation/audit"
)

func registerChatRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/v1/chat/messages", instrumentHandler(deps, "/api/v1/chat/messages", chatMessagesHandler(deps)))
	mux.HandleFunc("/api/v1/chat/sessions", instrumentHandler(deps, "/api/v1/chat/sessions", chatSessionsHandler(deps)))
}

// chatMessagesHandler handles POST /api/v1/chat/messages
// Accepts a user message, constructs an AlertEvent with source="web_chat",
// dispatches into the workflow, and returns a ChatMessageResponse.
func chatMessagesHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		principal, ok := requireAuthenticatedPrincipal(deps, w, r, "")
		if !ok {
			return
		}

		var req dto.ChatMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		if strings.TrimSpace(req.Message) == "" {
			writeValidationError(w, "message is required")
			return
		}

		userID := principal.User.UserID
		if userID == "" {
			userID = principal.User.Username
		}
		if userID == "" {
			userID = "web_user"
		}

		host := strings.TrimSpace(req.Host)
		if host == "" && len(deps.Config.SSH.AllowedHosts) > 0 {
			host = deps.Config.SSH.AllowedHosts[0]
		}
		if host == "" {
			host = "localhost"
		}

		service := strings.TrimSpace(req.Service)
		severity := strings.TrimSpace(req.Severity)
		if severity == "" {
			severity = "info"
		}

		alertName := webChatAlertName(req.Message, service)
		fingerprint := fmt.Sprintf("web-chat:%s:%s:%s:%s", userID, host, service, req.Message)
		if len(fingerprint) > 128 {
			fingerprint = fingerprint[:128]
		}
		idempotencyKey := fmt.Sprintf("web_chat:%s:%s", userID, fingerprint)

		event := contracts.AlertEvent{
			Source:         "web_chat",
			Severity:       severity,
			Fingerprint:    fingerprint,
			IdempotencyKey: idempotencyKey,
			RequestHash:    idempotencyKey,
			Labels: map[string]string{
				"alertname":      alertName,
				"instance":       host,
				"host":           host,
				"service":        service,
				"severity":       severity,
				"tars_chat":      "true",
				"tars_generated": "web_chat",
				"chat_id":        userID,
			},
			Annotations: map[string]string{
				"summary":      req.Message,
				"user_request": req.Message,
				"requested_by": userID,
			},
		}

		result, err := deps.Workflow.HandleAlertEvent(r.Context(), event)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "workflow_error", err.Error())
			return
		}

		auditChatEvent(r.Context(), deps, "web_chat_message", result.SessionID, "receive", userID, map[string]any{
			"host":         host,
			"service":      service,
			"user_request": req.Message,
			"alert_name":   alertName,
			"duplicated":   result.Duplicated,
		})

		ackMessage := fmt.Sprintf("已收到您的请求。正在分析并生成建议，session: %s", result.SessionID)
		if result.Duplicated {
			ackMessage = fmt.Sprintf("这条请求已经在处理中，session: %s", result.SessionID)
		}

		writeJSON(w, http.StatusOK, dto.ChatMessageResponse{
			SessionID:  result.SessionID,
			Status:     result.Status,
			Duplicated: result.Duplicated,
			AckMessage: ackMessage,
		})
	}
}

// chatSessionsHandler handles GET /api/v1/chat/sessions
// Returns recent sessions that originated from web_chat source.
func chatSessionsHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}

		_, ok := requireAuthenticatedPrincipal(deps, w, r, "")
		if !ok {
			return
		}

		sessions, err := deps.Workflow.ListSessions(r.Context(), contracts.ListSessionsFilter{
			Query: "web_chat",
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
			return
		}

		// Filter to only web_chat sessions client-side since the filter is a text search
		items := make([]dto.ChatSessionSummary, 0, len(sessions))
		for _, s := range sessions {
			source := labelStringFromMap(s.Alert, "tars_generated")
			if source != "web_chat" {
				continue
			}
			summary := annotationStringFromMap(s.Alert, "user_request")
			if summary == "" {
				summary = annotationStringFromMap(s.Alert, "summary")
			}
			items = append(items, dto.ChatSessionSummary{
				SessionID:   s.SessionID,
				Status:      s.Status,
				UserRequest: summary,
				Host:        labelStringFromMap(s.Alert, "host"),
				Service:     labelStringFromMap(s.Alert, "service"),
			})
		}

		writeJSON(w, http.StatusOK, dto.ChatSessionsResponse{
			Items: items,
			Total: len(items),
		})
	}
}

// --- helpers ---

func webChatAlertName(text string, service string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "load") || strings.Contains(lower, "uptime") || strings.Contains(text, "负载"):
		return "TarsChatLoadRequest"
	case strings.Contains(lower, "memory") || strings.Contains(text, "内存"):
		return "TarsChatMemoryRequest"
	case strings.Contains(lower, "disk") || strings.Contains(text, "磁盘"):
		return "TarsChatDiskRequest"
	case service != "" || strings.Contains(lower, "status") || strings.Contains(text, "状态"):
		return "TarsChatServiceStatus"
	default:
		return "TarsChatRequest"
	}
}

func annotationStringFromMap(alert map[string]interface{}, key string) string {
	if alert == nil {
		return ""
	}
	switch anns := alert["annotations"].(type) {
	case map[string]interface{}:
		if v, ok := anns[key].(string); ok {
			return v
		}
	case map[string]string:
		return anns[key]
	}
	return ""
}

func labelStringFromMap(alert map[string]interface{}, key string) string {
	if alert == nil {
		return ""
	}
	switch labels := alert["labels"].(type) {
	case map[string]interface{}:
		if v, ok := labels[key].(string); ok {
			return v
		}
	case map[string]string:
		return labels[key]
	}
	return ""
}

func auditChatEvent(ctx context.Context, deps Dependencies, resourceType string, resourceID string, action string, actor string, metadata map[string]any) {
	if deps.Audit == nil {
		return
	}
	deps.Audit.Log(ctx, audit.Entry{
		ResourceType: resourceType,
		ResourceID:   fallbackString(resourceID, "unknown"),
		Action:       action,
		Actor:        actor,
		Metadata:     metadata,
	})
}
