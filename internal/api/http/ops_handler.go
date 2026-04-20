package httpapi

import (
	"context"
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

func executionRouterHandler(deps Dependencies) http.HandlerFunc {
	detailHandler := executionHandler(deps)
	outputHandler := executionOutputHandler(deps)
	approveHandler := executionActionHandler(deps, "approve")
	rejectHandler := executionActionHandler(deps, "reject")
	requestContextHandler := executionActionHandler(deps, "request_context")
	modifyApproveHandler := executionActionHandler(deps, "modify_approve")
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/output"):
			outputHandler(w, r)
		case strings.HasSuffix(r.URL.Path, "/approve"):
			approveHandler(w, r)
		case strings.HasSuffix(r.URL.Path, "/reject"):
			rejectHandler(w, r)
		case strings.HasSuffix(r.URL.Path, "/request-context"):
			requestContextHandler(w, r)
		case strings.HasSuffix(r.URL.Path, "/modify-approve"):
			modifyApproveHandler(w, r)
		default:
			detailHandler(w, r)
		}
	}
}

func executionsHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}

		query := parseListQuery(r)
		items, err := deps.Workflow.ListExecutions(r.Context(), contracts.ListExecutionsFilter{
			Status:    r.URL.Query().Get("status"),
			Query:     query.Query,
			SortBy:    query.SortBy,
			SortOrder: query.SortOrder,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		pageItems, meta := paginateItems(items, query)
		response := dto.ExecutionListResponse{
			Items:    make([]dto.ExecutionDetail, 0, len(pageItems)),
			ListPage: meta,
		}
		for _, item := range pageItems {
			response.Items = append(response.Items, executionDTO(item))
		}

		auditOpsRead(r.Context(), deps, "execution", "*", "list", map[string]any{
			"status_filter": r.URL.Query().Get("status"),
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

func executionsBulkExportHandler(deps Dependencies) http.HandlerFunc {
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

		items := make([]dto.ExecutionDetail, 0, len(ids))
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
			executionDetail, err := deps.Workflow.GetExecution(r.Context(), id)
			if err != nil {
				failures = append(failures, dto.BatchOperationResult{
					ID:      id,
					Success: false,
					Code:    batchOperationErrorCode(err),
					Message: batchOperationErrorMessage(err),
				})
				continue
			}
			items = append(items, executionDTO(executionDetail))
		}

		exportedAt := time.Now().UTC()
		auditOpsWrite(r.Context(), deps, "execution", "*", "bulk_export", map[string]any{
			"ids":             ids,
			"operator_reason": req.OperatorReason,
			"total":           len(ids),
			"exported":        len(items),
			"failed":          len(failures),
		})

		filename := fmt.Sprintf("tars-executions-export-%s.json", exportedAt.Format("20060102T150405Z"))
		writeJSONAttachment(w, http.StatusOK, filename, dto.ExecutionExportResponse{
			ResourceType:   "execution",
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

func opsSummaryHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}

		sessions, err := deps.Workflow.ListSessions(r.Context(), contracts.ListSessionsFilter{})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		outbox, err := deps.Workflow.ListOutbox(r.Context(), contracts.ListOutboxFilter{})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		var activeSessions int
		var pendingApprovals int
		var executionsTotal int
		var executionsCompleted int
		for _, session := range sessions {
			if session.Status != "resolved" && session.Status != "failed" {
				activeSessions++
			}
			if session.Status == "pending_approval" {
				pendingApprovals++
			}
			for _, execution := range session.Executions {
				executionsTotal++
				if execution.Status == "completed" {
					executionsCompleted++
				}
			}
		}

		var blockedOutbox int
		var failedOutbox int
		for _, event := range outbox {
			switch event.Status {
			case "blocked":
				blockedOutbox++
			case "failed":
				failedOutbox++
			}
		}

		successRate := 0
		if executionsTotal > 0 {
			successRate = int(float64(executionsCompleted) / float64(executionsTotal) * 100)
		}

		auditOpsRead(r.Context(), deps, "ops_dashboard", "summary", "get", map[string]any{
			"session_count":     len(sessions),
			"outbox_count":      len(outbox),
			"active_sessions":   activeSessions,
			"pending_approvals": pendingApprovals,
			"provider_count":    len(providerHealthCards(deps)),
			"connector_count":   len(connectorHealthCards(deps)),
		})

		health := buildDashboardHealthResponse(deps)
		writeJSON(w, http.StatusOK, dto.OpsSummaryResponse{
			ActiveSessions:       activeSessions,
			PendingApprovals:     pendingApprovals,
			ExecutionsTotal:      executionsTotal,
			ExecutionsCompleted:  executionsCompleted,
			ExecutionSuccessRate: successRate,
			BlockedOutbox:        blockedOutbox,
			FailedOutbox:         failedOutbox,
			VisibleOutbox:        len(outbox),
			HealthyConnectors:    health.Summary.HealthyConnectors,
			DegradedConnectors:   health.Summary.DegradedConnectors,
			ConfiguredSecrets:    health.Summary.ConfiguredSecrets,
			MissingSecrets:       health.Summary.MissingSecrets,
			ProviderFailures:     health.Summary.ProviderFailures,
			ActiveAlerts:         health.Summary.ActiveAlerts,
		})
	}
}

func executionHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}

		executionID := strings.TrimPrefix(r.URL.Path, "/api/v1/executions/")
		if executionID == "" {
			writeError(w, http.StatusNotFound, "not_found", "execution not found")
			return
		}

		executionDetail, err := deps.Workflow.GetExecution(r.Context(), executionID)
		if err != nil {
			if errors.Is(err, contracts.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "execution not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		auditOpsRead(r.Context(), deps, "execution", executionID, "get", map[string]any{
			"status":           executionDetail.Status,
			"approval_group":   executionDetail.ApprovalGroup,
			"output_truncated": executionDetail.OutputTruncated,
		})

		writeJSON(w, http.StatusOK, executionDTO(executionDetail))
	}
}

func executionOutputHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}

		executionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/executions/"), "/output")
		if executionID == "" {
			writeError(w, http.StatusNotFound, "not_found", "execution not found")
			return
		}

		chunks, err := deps.Workflow.GetExecutionOutput(r.Context(), executionID)
		if err != nil {
			if errors.Is(err, contracts.ErrNotFound) {
				writeError(w, http.StatusNotFound, "not_found", "execution not found")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		auditOpsRead(r.Context(), deps, "execution", executionID, "get_output", map[string]any{
			"chunk_count": len(chunks),
		})

		writeJSON(w, http.StatusOK, executionOutputDTO(executionID, chunks))
	}
}

func outboxListHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}

		query := parseListQuery(r)
		items, err := deps.Workflow.ListOutbox(r.Context(), contracts.ListOutboxFilter{
			Status:    r.URL.Query().Get("status"),
			Query:     query.Query,
			SortBy:    query.SortBy,
			SortOrder: query.SortOrder,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		pageItems, meta := paginateItems(items, query)
		response := dto.OutboxListResponse{
			Items:    make([]dto.OutboxEvent, 0, len(pageItems)),
			ListPage: meta,
		}
		for _, item := range pageItems {
			response.Items = append(response.Items, outboxDTO(item))
		}
		auditOpsRead(r.Context(), deps, "outbox_event", "*", "list", map[string]any{
			"status_filter": r.URL.Query().Get("status"),
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

func outboxItemRouterHandler(deps Dependencies) http.HandlerFunc {
	replayHandler := outboxReplayHandler(deps)
	deleteHandler := outboxDeleteHandler(deps)
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/replay"):
			replayHandler(w, r)
		case r.Method == http.MethodDelete:
			deleteHandler(w, r)
		default:
			writeError(w, http.StatusNotFound, "not_found", "endpoint not found")
		}
	}
}

func outboxBulkReplayHandler(deps Dependencies) http.HandlerFunc {
	return outboxBulkActionHandler(deps, "replay")
}

func outboxBulkDeleteHandler(deps Dependencies) http.HandlerFunc {
	return outboxBulkActionHandler(deps, "delete")
}

func outboxBulkActionHandler(deps Dependencies, action string) http.HandlerFunc {
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
			writeError(w, http.StatusBadRequest, "validation_failed", "invalid request body")
			return
		}
		if strings.TrimSpace(req.OperatorReason) == "" {
			writeError(w, http.StatusBadRequest, "validation_failed", "operator_reason is required")
			return
		}

		ids := uniqueNonEmptyIDs(req.IDs)
		if len(ids) == 0 {
			writeError(w, http.StatusBadRequest, "validation_failed", "ids is required")
			return
		}
		if len(ids) > 100 {
			writeError(w, http.StatusBadRequest, "validation_failed", "ids exceeds maximum batch size of 100")
			return
		}

		results := make([]dto.BatchOperationResult, 0, len(ids))
		succeeded := 0
		for _, id := range ids {
			err := runOutboxBatchAction(r.Context(), deps, action, id, req.OperatorReason)
			if err == nil {
				succeeded++
				results = append(results, dto.BatchOperationResult{
					ID:      id,
					Success: true,
					Message: "accepted",
				})
				continue
			}
			results = append(results, dto.BatchOperationResult{
				ID:      id,
				Success: false,
				Code:    batchOperationErrorCode(err),
				Message: batchOperationErrorMessage(err),
			})
		}

		if deps.Metrics != nil {
			deps.Metrics.IncOutbox("unknown", "bulk_"+action)
		}
		auditOpsWrite(r.Context(), deps, "outbox_event", "*", "bulk_"+action, map[string]any{
			"ids":             ids,
			"operator_reason": req.OperatorReason,
			"total":           len(ids),
			"succeeded":       succeeded,
			"failed":          len(ids) - succeeded,
		})

		writeJSON(w, http.StatusOK, dto.BatchOperationResponse{
			Operation:    action,
			ResourceType: "outbox_event",
			Total:        len(ids),
			Succeeded:    succeeded,
			Failed:       len(ids) - succeeded,
			Results:      results,
		})
	}
}

func outboxReplayHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}

		if !strings.HasSuffix(r.URL.Path, "/replay") {
			writeError(w, http.StatusNotFound, "not_found", "endpoint not found")
			return
		}

		var req struct {
			OperatorReason string `json:"operator_reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OperatorReason == "" {
			writeError(w, http.StatusBadRequest, "validation_failed", "operator_reason is required")
			return
		}

		eventID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/outbox/"), "/replay")
		if eventID == "" {
			writeError(w, http.StatusNotFound, "not_found", "outbox event not found")
			return
		}

		err := deps.Workflow.ReplayOutbox(r.Context(), eventID, req.OperatorReason)
		if err != nil {
			switch {
			case errors.Is(err, contracts.ErrNotFound):
				writeError(w, http.StatusNotFound, "not_found", "outbox event not found")
			case errors.Is(err, contracts.ErrBlockedByFeatureFlag):
				if deps.Metrics != nil {
					deps.Metrics.IncOutbox("unknown", "replay_blocked")
				}
				writeError(w, http.StatusConflict, "blocked_by_feature_flag", "outbox event is still blocked by feature flag")
			default:
				writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			}
			return
		}

		auditOpsWrite(r.Context(), deps, "outbox_event", eventID, "replay", map[string]any{
			"operator_reason": req.OperatorReason,
		})
		if deps.Metrics != nil {
			deps.Metrics.IncOutbox("unknown", "replayed")
		}

		writeJSON(w, http.StatusOK, dto.AcceptedResponse{Accepted: true})
	}
}

func outboxDeleteHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if strings.TrimPrefix(r.URL.Path, "/api/v1/outbox/") == "" || strings.Contains(strings.TrimPrefix(r.URL.Path, "/api/v1/outbox/"), "/") {
			writeError(w, http.StatusNotFound, "not_found", "outbox event not found")
			return
		}

		var req struct {
			OperatorReason string `json:"operator_reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OperatorReason == "" {
			writeError(w, http.StatusBadRequest, "validation_failed", "operator_reason is required")
			return
		}

		eventID := strings.TrimPrefix(r.URL.Path, "/api/v1/outbox/")
		err := deps.Workflow.DeleteOutbox(r.Context(), eventID, req.OperatorReason)
		if err != nil {
			switch {
			case errors.Is(err, contracts.ErrNotFound):
				writeError(w, http.StatusNotFound, "not_found", "outbox event not found")
			case errors.Is(err, contracts.ErrInvalidState):
				writeError(w, http.StatusConflict, "invalid_state", "only failed or blocked outbox events can be deleted")
			default:
				writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			}
			return
		}

		auditOpsWrite(r.Context(), deps, "outbox_event", eventID, "delete", map[string]any{
			"operator_reason": req.OperatorReason,
		})
		if deps.Metrics != nil {
			deps.Metrics.IncOutbox("unknown", "deleted")
		}

		writeJSON(w, http.StatusOK, dto.AcceptedResponse{Accepted: true})
	}
}

func uniqueNonEmptyIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func runOutboxBatchAction(ctx context.Context, deps Dependencies, action string, id string, operatorReason string) error {
	switch action {
	case "replay":
		return deps.Workflow.ReplayOutbox(ctx, id, operatorReason)
	case "delete":
		return deps.Workflow.DeleteOutbox(ctx, id, operatorReason)
	default:
		return errors.New("unsupported batch action")
	}
}

func batchOperationErrorCode(err error) string {
	switch {
	case strings.Contains(err.Error(), "invalid input syntax for type uuid"):
		return "validation_failed"
	case errors.Is(err, contracts.ErrNotFound):
		return "not_found"
	case errors.Is(err, contracts.ErrInvalidState):
		return "invalid_state"
	case errors.Is(err, contracts.ErrBlockedByFeatureFlag):
		return "blocked_by_feature_flag"
	default:
		return "internal_error"
	}
}

func batchOperationErrorMessage(err error) string {
	switch {
	case strings.Contains(err.Error(), "invalid input syntax for type uuid"):
		return "invalid uuid"
	case errors.Is(err, contracts.ErrNotFound):
		return "resource not found"
	case errors.Is(err, contracts.ErrInvalidState):
		return "resource is not in a valid state for this action"
	case errors.Is(err, contracts.ErrBlockedByFeatureFlag):
		return "resource is still blocked by feature flag"
	default:
		return err.Error()
	}
}

func reindexHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}

		var req struct {
			OperatorReason string `json:"operator_reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OperatorReason == "" {
			writeError(w, http.StatusBadRequest, "validation_failed", "operator_reason is required")
			return
		}

		if deps.Knowledge == nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "knowledge service is not configured")
			return
		}
		if err := deps.Knowledge.ReindexDocuments(r.Context(), req.OperatorReason); err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		deps.Audit.Log(r.Context(), audit.Entry{
			ResourceType: "knowledge_index",
			ResourceID:   "documents",
			Action:       "reindex",
			Actor:        "ops_api",
			TraceID:      "knowledge-reindex",
			Metadata: map[string]any{
				"operator_reason": req.OperatorReason,
			},
		})

		writeJSON(w, http.StatusOK, dto.AcceptedResponse{Accepted: true})
	}
}
