package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"tars/internal/api/dto"
	"tars/internal/modules/trigger"
)

func registerTriggerRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/v1/triggers", instrumentHandler(deps, "/api/v1/triggers", triggersListHandler(deps)))
	mux.HandleFunc("/api/v1/triggers/", instrumentHandler(deps, "/api/v1/triggers/*", triggerDetailHandler(deps)))
}

func triggersListHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Trigger == nil {
			writeError(w, http.StatusConflict, "not_configured", "trigger service not configured")
			return
		}
		switch r.Method {
		case http.MethodGet:
			principal, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.read")
			if !ok {
				return
			}
			query := parseListQuery(r)
			eventType := r.URL.Query().Get("event_type")
			items, err := deps.Trigger.List(r.Context(), trigger.ListFilter{
				TenantID:  "default",
				EventType: eventType,
				Limit:     query.Limit,
				Offset:    (query.Page - 1) * query.Limit,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
				return
			}
			filtered := filterTriggers(items, query.Query)
			pageItems, meta := paginateItems(filtered, query)
			resp := dto.TriggerListResponse{Items: make([]dto.TriggerDTO, 0, len(pageItems)), ListPage: meta}
			for _, t := range pageItems {
				resp.Items = append(resp.Items, triggerToDTO(t))
			}
			auditOpsRead(r.Context(), deps, "triggers", "", "list", map[string]any{"actor": principal.User.UserID, "count": len(resp.Items)})
			writeJSON(w, http.StatusOK, resp)

		case http.MethodPost:
			principal, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.write")
			if !ok {
				return
			}
			var req dto.TriggerUpsertRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if req.Trigger.ID == "" && req.Trigger.DisplayName == "" && req.Trigger.EventType == "" {
				req.Trigger = dto.TriggerDTO{
					ID:              req.ID,
					TenantID:        req.TenantID,
					DisplayName:     req.DisplayName,
					Description:     req.Description,
					Enabled:         req.Enabled,
					EventType:       req.EventType,
					ChannelID:       req.ChannelID,
					AutomationJobID: req.AutomationJobID,
					Governance:      req.Governance,
					FilterExpr:      req.FilterExpr,
					TargetAudience:  req.TargetAudience,
					TemplateID:      req.TemplateID,
					CooldownSec:     req.CooldownSec,
				}
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				writeValidationError(w, "operator_reason is required")
				return
			}
			t := triggerFromDTO(req.Trigger)
			if err := validateTriggerDeliveryChannel(deps, t); err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			saved, err := deps.Trigger.Upsert(r.Context(), t)
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "trigger", saved.ID, "trigger_created",
				map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusCreated, triggerToDTO(saved))

		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func triggerDetailHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Trigger == nil {
			writeError(w, http.StatusConflict, "not_configured", "trigger service not configured")
			return
		}
		id, action := nestedResourcePath(r.URL.Path, "/api/v1/triggers/")
		if id == "" {
			writeError(w, http.StatusNotFound, "not_found", "trigger not found")
			return
		}

		switch {
		case r.Method == http.MethodGet && action == "":
			principal, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.read")
			if !ok {
				return
			}
			t, err := deps.Trigger.Get(r.Context(), id)
			if err != nil {
				writeError(w, http.StatusNotFound, "not_found", "trigger not found")
				return
			}
			auditOpsRead(r.Context(), deps, "trigger", id, "get", map[string]any{"actor": principal.User.UserID})
			writeJSON(w, http.StatusOK, triggerToDTO(t))

		case r.Method == http.MethodPut && action == "":
			principal, ok := requireAuthenticatedPrincipal(deps, w, r, "configs.write")
			if !ok {
				return
			}
			var req dto.TriggerUpsertRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if req.Trigger.ID == "" && req.Trigger.DisplayName == "" && req.Trigger.EventType == "" {
				req.Trigger = dto.TriggerDTO{
					ID:              req.ID,
					TenantID:        req.TenantID,
					DisplayName:     req.DisplayName,
					Description:     req.Description,
					Enabled:         req.Enabled,
					EventType:       req.EventType,
					ChannelID:       req.ChannelID,
					AutomationJobID: req.AutomationJobID,
					Governance:      req.Governance,
					FilterExpr:      req.FilterExpr,
					TargetAudience:  req.TargetAudience,
					TemplateID:      req.TemplateID,
					CooldownSec:     req.CooldownSec,
				}
			}
			if strings.TrimSpace(req.OperatorReason) == "" {
				writeValidationError(w, "operator_reason is required")
				return
			}
			t := triggerFromDTO(req.Trigger)
			t.ID = id
			if err := validateTriggerDeliveryChannel(deps, t); err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			saved, err := deps.Trigger.Upsert(r.Context(), t)
			if err != nil {
				writeError(w, http.StatusBadRequest, "validation_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "trigger", id, "trigger_updated",
				map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusOK, triggerToDTO(saved))

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
			updated, err := deps.Trigger.SetEnabled(r.Context(), id, action == "enable")
			if err != nil {
				writeError(w, http.StatusNotFound, "not_found", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "trigger", id, "trigger_"+action+"d",
				map[string]any{"actor": principal.User.UserID, "reason": req.OperatorReason})
			writeJSON(w, http.StatusOK, triggerToDTO(updated))

		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func validateTriggerDeliveryChannel(deps Dependencies, t trigger.Trigger) error {
	if trigger.IsDirectDeliveryChannelKind(t.ChannelID) {
		return nil
	}
	channelID := strings.TrimSpace(t.ChannelID)
	if channelID == "" {
		return nil
	}
	if deps.Access == nil {
		return nil
	}
	item, ok := deps.Access.GetChannel(channelID)
	if !ok {
		return nil
	}
	kind := strings.TrimSpace(item.Kind)
	if kind == "" {
		kind = strings.TrimSpace(item.Type)
	}
	if !trigger.IsDirectDeliveryChannelKind(kind) {
		return fmt.Errorf("unsupported trigger delivery channel kind: %s", kind)
	}
	return nil
}

// --- helpers ---

func triggerToDTO(t trigger.Trigger) dto.TriggerDTO {
	return dto.TriggerDTO{
		ID:              t.ID,
		TenantID:        t.TenantID,
		DisplayName:     t.DisplayName,
		Description:     t.Description,
		Enabled:         t.Enabled,
		EventType:       t.EventType,
		ChannelID:       t.ChannelID,
		AutomationJobID: t.AutomationJobID,
		Governance:      t.Governance,
		FilterExpr:      t.FilterExpr,
		TargetAudience:  t.TargetAudience,
		TemplateID:      t.TemplateID,
		CooldownSec:     t.CooldownSec,
		LastFiredAt:     t.LastFiredAt,
		CreatedAt:       t.CreatedAt,
		UpdatedAt:       t.UpdatedAt,
	}
}

func triggerFromDTO(d dto.TriggerDTO) trigger.Trigger {
	return trigger.Trigger{
		ID:              d.ID,
		TenantID:        d.TenantID,
		DisplayName:     d.DisplayName,
		Description:     d.Description,
		Enabled:         d.Enabled,
		EventType:       d.EventType,
		ChannelID:       d.ChannelID,
		AutomationJobID: d.AutomationJobID,
		Governance:      d.Governance,
		FilterExpr:      d.FilterExpr,
		TargetAudience:  d.TargetAudience,
		TemplateID:      d.TemplateID,
		CooldownSec:     d.CooldownSec,
	}
}

func filterTriggers(items []trigger.Trigger, query string) []trigger.Trigger {
	if strings.TrimSpace(query) == "" {
		return items
	}
	q := strings.ToLower(strings.TrimSpace(query))
	out := make([]trigger.Trigger, 0, len(items))
	for _, t := range items {
		if containsQuery(q, t.ID, t.DisplayName, t.EventType, t.ChannelID, t.TargetAudience) {
			out = append(out, t)
		}
	}
	return out
}
