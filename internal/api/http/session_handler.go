package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"tars/internal/api/dto"
	"tars/internal/contracts"
	"tars/internal/foundation/audit"
)

func sessionsHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}

		query := parseListQuery(r)
		items, err := deps.Workflow.ListSessions(r.Context(), contracts.ListSessionsFilter{
			Status:    r.URL.Query().Get("status"),
			Host:      r.URL.Query().Get("host"),
			Query:     query.Query,
			SortBy:    query.SortBy,
			SortOrder: query.SortOrder,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		pageItems, meta := paginateItems(items, query)
		response := dto.SessionListResponse{
			Items:    make([]dto.SessionDetail, 0, len(pageItems)),
			ListPage: meta,
		}
		for _, item := range pageItems {
			response.Items = append(response.Items, sessionDTO(item))
		}
		auditOpsRead(r.Context(), deps, "session", "*", "list", map[string]any{
			"status_filter": r.URL.Query().Get("status"),
			"host_filter":   r.URL.Query().Get("host"),
			"query":         query.Query,
			"sort_by":       query.SortBy,
			"sort_order":    query.SortOrder,
			"page":          query.Page,
			"limit":         query.Limit,
			"item_count":    len(response.Items),
			"total":         meta.Total,
		})

		writeJSON(w, http.StatusOK, response)
	}
}

func sessionsBulkExportHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}

		var req struct {
			IDs            []string `json:"ids"`
			OperatorReason string   `json:"operator_reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeValidationError(w, "invalid request body")
			return
		}
		if strings.TrimSpace(req.OperatorReason) == "" {
			writeValidationError(w, "operator_reason is required")
			return
		}

		ids := uniqueNonEmptyIDs(req.IDs)
		if len(ids) == 0 {
			writeValidationError(w, "ids is required")
			return
		}
		if len(ids) > 100 {
			writeTooManyIDsError(w, 100)
			return
		}

		items := make([]dto.SessionDetail, 0, len(ids))
		failures := make([]dto.BatchOperationResult, 0)
		for _, id := range ids {
			if !isValidResourceID(id) {
				failures = append(failures, dto.BatchOperationResult{
					ID:      id,
					Success: false,
					Code:    "validation_failed",
					Message: "invalid uuid",
				})
				continue
			}
			sessionDetail, err := deps.Workflow.GetSession(r.Context(), id)
			if err != nil {
				failures = append(failures, dto.BatchOperationResult{
					ID:      id,
					Success: false,
					Code:    batchOperationErrorCode(err),
					Message: batchOperationErrorMessage(err),
				})
				continue
			}
			items = append(items, sessionDTO(sessionDetail))
		}

		exportedAt := time.Now().UTC()
		auditOpsWrite(r.Context(), deps, "session", "*", "bulk_export", map[string]any{
			"ids":             ids,
			"operator_reason": req.OperatorReason,
			"total":           len(ids),
			"exported":        len(items),
			"failed":          len(failures),
		})

		filename := fmt.Sprintf("tars-sessions-export-%s.json", exportedAt.Format("20060102T150405Z"))
		writeJSONAttachment(w, http.StatusOK, filename, dto.SessionExportResponse{
			ResourceType:   "session",
			ExportedAt:     exportedAt,
			OperatorReason: req.OperatorReason,
			TotalRequested: len(ids),
			ExportedCount:  len(items),
			FailedCount:    len(failures),
			Items:          items,
			Failures:       failures,
		})
	}
}

func sessionRouterHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/v1/sessions/")
		path = strings.TrimSuffix(path, "/")
		if path == "" {
			writeError(w, http.StatusNotFound, "not_found", "session not found")
			return
		}

		if strings.HasSuffix(path, "/trace") {
			sessionTraceHandler(deps, strings.TrimSuffix(path, "/trace"))(w, r)
			return
		}
		sessionDetailHandler(deps, path)(w, r)
	}
}

func sessionDetailHandler(deps Dependencies, sessionID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if strings.TrimSpace(sessionID) == "" {
			writeError(w, http.StatusNotFound, "not_found", "session not found")
			return
		}

		sessionDetail, err := deps.Workflow.GetSession(r.Context(), sessionID)
		if err != nil {
			if errors.Is(err, contracts.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "session not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		auditOpsRead(r.Context(), deps, "session", sessionID, "get", map[string]any{
			"status":          sessionDetail.Status,
			"execution_count": len(sessionDetail.Executions),
			"timeline_count":  len(sessionDetail.Timeline),
		})

		writeJSON(w, http.StatusOK, sessionDTO(sessionDetail))
	}
}

func sessionTraceHandler(deps Dependencies, sessionID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if strings.TrimSpace(sessionID) == "" {
			writeError(w, http.StatusNotFound, "not_found", "session not found")
			return
		}

		if _, err := deps.Workflow.GetSession(r.Context(), sessionID); err != nil {
			if errors.Is(err, contracts.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "session not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		auditRecords := []audit.Record{}
		if reader, ok := deps.Audit.(audit.SessionReader); ok {
			records, err := reader.ListBySession(r.Context(), sessionID, 100)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
				return
			}
			auditRecords = records
		}

		var knowledgeTrace *contracts.SessionKnowledgeTrace
		if reader, ok := deps.Knowledge.(contracts.SessionKnowledgeReader); ok {
			trace, err := reader.GetSessionKnowledge(r.Context(), sessionID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
				return
			}
			if trace.Available {
				knowledgeTrace = &trace
			}
		}

		auditOpsRead(r.Context(), deps, "session", sessionID, "trace_get", map[string]any{
			"audit_count":       len(auditRecords),
			"knowledge_present": knowledgeTrace != nil,
		})

		writeJSON(w, http.StatusOK, sessionTraceDTO(sessionID, auditRecords, knowledgeTrace))
	}
}
