package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"
	"tars/internal/api/dto"
	"tars/internal/modules/msgtpl"
)

func registerNotificationTemplateRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/v1/notification-templates", instrumentHandler(deps, "/api/v1/notification-templates", notificationTemplatesHandler(deps)))
	mux.HandleFunc("/api/v1/notification-templates/", instrumentHandler(deps, "/api/v1/notification-templates/*", notificationTemplateDetailHandler(deps)))
	
	// Legacy aliases
	mux.HandleFunc("/api/v1/msg-templates", instrumentHandler(deps, "/api/v1/msg-templates", notificationTemplatesHandler(deps)))
	mux.HandleFunc("/api/v1/msg-templates/", instrumentHandler(deps, "/api/v1/msg-templates/*", notificationTemplateDetailHandler(deps)))
}

func notificationTemplatesHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		manager := deps.NotificationTemplates
		if manager == nil {
			writeError(w, http.StatusConflict, "not_configured", "notification templates manager is not configured")
			return
		}
		switch r.Method {
		case http.MethodGet:
			principal, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.read")
			if !ok {
				return
			}
			query := parseListQuery(r)
			items := filterNotificationTemplates(manager.List(), query.Query)
			pageItems, meta := paginateItems(items, query)
			resp := dto.NotificationTemplateListResponse{Items: make([]dto.NotificationTemplate, 0, len(pageItems)), ListPage: meta}
			for _, item := range pageItems {
				resp.Items = append(resp.Items, notificationTemplateToDTO(item))
			}
			auditOpsRead(r.Context(), deps, "notification_templates", "", "list", map[string]any{"actor": principal.User.UserID, "count": len(resp.Items)})
			writeJSON(w, http.StatusOK, resp)
		case http.MethodPost:
			principal, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.write")
			if !ok {
				return
			}
			var req dto.NotificationTemplateUpsertRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				writeValidationError(w, "operator_reason is required")
				return
			}
			tpl := notificationTemplateFromDTO(req.Template)
			saved, err := manager.Upsert(tpl)
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "notification_template", saved.ID, "notification_template_created", map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusCreated, notificationTemplateToDTO(saved))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func notificationTemplateDetailHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		manager := deps.NotificationTemplates
		if manager == nil {
			writeError(w, http.StatusConflict, "not_configured", "notification templates manager is not configured")
			return
		}
		// Support both route prefixes
		prefix := "/api/v1/notification-templates/"
		if strings.HasPrefix(r.URL.Path, "/api/v1/msg-templates/") {
			prefix = "/api/v1/msg-templates/"
		}
		id, action := nestedResourcePath(r.URL.Path, prefix)
		if id == "" {
			writeError(w, http.StatusNotFound, "not_found", "notification template not found")
			return
		}
		switch {
		case r.Method == http.MethodGet && action == "":
			principal, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.read")
			if !ok {
				return
			}
			tpl, found := manager.Get(id)
			if !found {
				writeError(w, http.StatusNotFound, "not_found", "notification template not found")
				return
			}
			auditOpsRead(r.Context(), deps, "notification_template", id, "get", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusOK, notificationTemplateToDTO(tpl))
		case r.Method == http.MethodPut && action == "":
			principal, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.write")
			if !ok {
				return
			}
			var req dto.NotificationTemplateUpsertRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				writeValidationError(w, "operator_reason is required")
				return
			}
			tpl := notificationTemplateFromDTO(req.Template)
			tpl.ID = id
			saved, err := manager.Upsert(tpl)
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "notification_template", id, "notification_template_updated", map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusOK, notificationTemplateToDTO(saved))
		case r.Method == http.MethodPost && (action == "enable" || action == "disable"):
			principal, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.write")
			if !ok {
				return
			}
			var req operatorReasonRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				writeValidationError(w, "operator_reason is required")
				return
			}
			updated, err := manager.SetEnabled(id, action == "enable")
			if err != nil {
				writeError(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "notification_template", id, "notification_template_"+action+"d", map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusOK, notificationTemplateToDTO(updated))

		case r.Method == http.MethodPost && action == "render":
			_, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.read")
			if !ok {
				return
			}
			tpl, found := manager.Get(id)
			if !found {
				writeError(w, http.StatusNotFound, "not_found", "notification template not found")
				return
			}
			var reqBody struct {
				Vars map[string]string `json:"vars"`
			}
			if r.ContentLength > 0 {
				_ = json.NewDecoder(r.Body).Decode(&reqBody)
			}
			if reqBody.Vars == nil {
				reqBody.Vars = map[string]string{}
			}
			subject, body := tpl.Render(reqBody.Vars)
			writeJSON(w, http.StatusOK, map[string]string{
				"template_id": id,
				"subject":     subject,
				"body":        body,
			})

		case r.Method == http.MethodGet && action == "export":
			_, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.read")
			if !ok {
				return
			}
			tpl, found := manager.Get(id)
			if !found {
				writeError(w, http.StatusNotFound, "not_found", "notification template not found")
				return
			}
			format := r.URL.Query().Get("format")
			if format == "" {
				format = "json"
			}
			d := notificationTemplateToDTO(tpl)
			switch format {
			case "yaml":
				out, err := yaml.Marshal(d)
				if err != nil {
					writeError(w, http.StatusInternalServerError, "marshal_error", err.Error())
					return
				}
				w.Header().Set("Content-Type", "application/yaml")
				w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="notification-template-%s.yaml"`, id))
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(out)
			default: // json
				out, err := json.MarshalIndent(d, "", "  ")
				if err != nil {
					writeError(w, http.StatusInternalServerError, "marshal_error", err.Error())
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="notification-template-%s.json"`, id))
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(out)
			}

		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func filterNotificationTemplates(items []msgtpl.NotificationTemplate, query string) []msgtpl.NotificationTemplate {
	if strings.TrimSpace(query) == "" {
		return items
	}
	query = strings.ToLower(strings.TrimSpace(query))
	out := make([]msgtpl.NotificationTemplate, 0, len(items))
	for _, item := range items {
		if containsQuery(query, item.ID, item.Kind, item.Locale, item.Name) {
			out = append(out, item)
		}
	}
	return out
}

func notificationTemplateToDTO(tpl msgtpl.NotificationTemplate) dto.NotificationTemplate {
	status := strings.TrimSpace(tpl.Status)
	if status == "" {
		if tpl.Enabled {
			status = "active"
		} else {
			status = "disabled"
		}
	}
	return dto.NotificationTemplate{
		ID:             tpl.ID,
		Kind:           tpl.Kind,
		Locale:         tpl.Locale,
		Name:           tpl.Name,
		Status:         status,
		Enabled:        tpl.Enabled,
		VariableSchema: cloneStringMap(tpl.VariableSchema),
		UsageRefs:      append([]string(nil), tpl.UsageRefs...),
		Content: dto.NotificationTemplateContent{
			Subject: tpl.Content.Subject,
			Body:    tpl.Content.Body,
		},
		UpdatedAt: tpl.UpdatedAt,
	}
}

func notificationTemplateFromDTO(d dto.NotificationTemplate) msgtpl.NotificationTemplate {
	return msgtpl.NotificationTemplate{
		ID:             d.ID,
		Kind:           d.Kind,
		Locale:         d.Locale,
		Name:           d.Name,
		Status:         d.Status,
		Enabled:        d.Enabled,
		VariableSchema: cloneStringMap(d.VariableSchema),
		UsageRefs:      append([]string(nil), d.UsageRefs...),
		Content: msgtpl.TemplateContent{
			Subject: d.Content.Subject,
			Body:    d.Content.Body,
		},
	}
}
