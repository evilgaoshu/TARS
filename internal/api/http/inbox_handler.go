package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"

	"tars/internal/api/dto"
	"tars/internal/modules/access"
	"tars/internal/modules/inbox"
)

func registerInboxRoutes(mux *http.ServeMux, deps Dependencies) {
	mux.HandleFunc("/api/v1/inbox", instrumentHandler(deps, "/api/v1/inbox", inboxListHandler(deps)))
	mux.HandleFunc("/api/v1/inbox/mark-all-read", instrumentHandler(deps, "/api/v1/inbox/mark-all-read", inboxMarkAllReadHandler(deps)))
	mux.HandleFunc("/api/v1/inbox/", instrumentHandler(deps, "/api/v1/inbox/*", inboxDetailHandler(deps)))
}

func inboxListHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Inbox == nil {
			writeError(w, http.StatusConflict, "not_configured", "inbox not configured")
			return
		}
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, "")
		if !ok {
			return
		}
		tenantID := principalTenantID(principal)

		switch r.Method {
		case http.MethodGet:
			limit := 50
			offset := 0
			unreadOnly := false
			if v := r.URL.Query().Get("limit"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n > 0 {
					limit = n
				}
			}
			if v := r.URL.Query().Get("offset"); v != "" {
				if n, err := strconv.Atoi(v); err == nil && n >= 0 {
					offset = n
				}
			}
			if v := r.URL.Query().Get("unread_only"); v == "true" || v == "1" {
				unreadOnly = true
			}
			items, err := deps.Inbox.List(r.Context(), inbox.ListFilter{
				TenantID:   tenantID,
				UnreadOnly: unreadOnly,
				Limit:      limit,
				Offset:     offset,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "list_failed", err.Error())
				return
			}
			unread, _ := deps.Inbox.CountUnread(r.Context(), tenantID)
			resp := dto.InboxListResponse{
				Items:       make([]dto.InboxMessage, 0, len(items)),
				UnreadCount: unread,
			}
			resp.ListPage = dto.ListPage{
				Page:  offset/limit + 1,
				Limit: limit,
				Total: unread, // approximate; unread is most useful total
			}
			for _, m := range items {
				resp.Items = append(resp.Items, inboxToDTO(m))
			}
			writeJSON(w, http.StatusOK, resp)

		case http.MethodPost:
			// Create a manual inbox message (admin tool / debug)
			principal2, ok2 := requireAuthenticatedPrincipal(deps, w, r, "configs.write")
			if !ok2 {
				return
			}
			var req dto.InboxCreateRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			created, err := deps.Inbox.Create(r.Context(), inbox.Message{
				TenantID: principalTenantID(principal2),
				Subject:  req.Subject,
				Body:     req.Body,
				RefType:  req.RefType,
				RefID:    req.RefID,
				Source:   req.Source,
			})
			if err != nil {
				writeError(w, http.StatusInternalServerError, "create_failed", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "inbox_message", created.ID, "inbox_message_created",
				map[string]any{"actor": principal2.User.UserID})
			writeJSON(w, http.StatusCreated, inboxToDTO(created))

		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func inboxMarkAllReadHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Inbox == nil {
			writeError(w, http.StatusConflict, "not_configured", "inbox not configured")
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		principal, ok := requireAuthenticatedPrincipal(deps, w, r, "")
		if !ok {
			return
		}
		n, err := deps.Inbox.MarkAllRead(r.Context(), principalTenantID(principal))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "mark_all_read_failed", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"marked_read": n})
	}
}

func inboxDetailHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Inbox == nil {
			writeError(w, http.StatusConflict, "not_configured", "inbox not configured")
			return
		}
		id, action := nestedResourcePath(r.URL.Path, "/api/v1/inbox/")
		if id == "" {
			writeError(w, http.StatusNotFound, "not_found", "inbox message not found")
			return
		}

		switch {
		case r.Method == http.MethodGet && action == "":
			principal, ok := requireAuthenticatedPrincipal(deps, w, r, "")
			if !ok {
				return
			}
			_ = principal
			msg, err := deps.Inbox.Get(r.Context(), id)
			if err != nil {
				writeError(w, http.StatusNotFound, "not_found", "inbox message not found")
				return
			}
			writeJSON(w, http.StatusOK, inboxToDTO(msg))

		case r.Method == http.MethodPost && action == "read":
			principal, ok := requireAuthenticatedPrincipal(deps, w, r, "")
			if !ok {
				return
			}
			_ = principal
			msg, err := deps.Inbox.MarkRead(r.Context(), id)
			if err != nil {
				writeError(w, http.StatusNotFound, "not_found", "inbox message not found")
				return
			}
			writeJSON(w, http.StatusOK, inboxToDTO(msg))

		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

// --- helpers ---

func inboxToDTO(m inbox.Message) dto.InboxMessage {
	return dto.InboxMessage{
		ID:        m.ID,
		TenantID:  m.TenantID,
		Subject:   m.Subject,
		Body:      m.Body,
		Channel:   m.Channel,
		RefType:   m.RefType,
		RefID:     m.RefID,
		Source:    m.Source,
		Actions:   inboxActionsDTO(m.Actions),
		IsRead:    m.IsRead,
		CreatedAt: m.CreatedAt,
		ReadAt:    m.ReadAt,
	}
}

func inboxActionsDTO(actions []inbox.Action) []dto.ChannelAction {
	if len(actions) == 0 {
		return nil
	}
	items := make([]dto.ChannelAction, 0, len(actions))
	for _, action := range actions {
		items = append(items, dto.ChannelAction{Label: action.Label, Value: action.Value})
	}
	return items
}

func principalTenantID(_ access.Principal) string {
	// MVP: single-tenant, always "default"
	return "default"
}
