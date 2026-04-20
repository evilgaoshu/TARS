package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"tars/internal/api/dto"
	"tars/internal/contracts"
	"tars/internal/foundation/audit"
)

func auditListHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}

		query := parseListQuery(r)
		reader, ok := deps.Audit.(audit.ListReader)
		if !ok {
			writeJSON(w, http.StatusOK, dto.AuditListResponse{Items: []dto.AuditRecord{}, ListPage: dto.ListPage{Page: query.Page, Limit: query.Limit}})
			return
		}

		items, err := reader.List(r.Context(), audit.ListFilter{
			Query:        query.Query,
			ResourceType: r.URL.Query().Get("resource_type"),
			Action:       r.URL.Query().Get("action"),
			Actor:        r.URL.Query().Get("actor"),
			SortBy:       query.SortBy,
			SortOrder:    query.SortOrder,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		pageItems, meta := paginateItems(items, query)
		response := dto.AuditListResponse{
			Items:    make([]dto.AuditRecord, 0, len(pageItems)),
			ListPage: meta,
		}
		for _, item := range pageItems {
			response.Items = append(response.Items, auditRecordDTO(item))
		}

		auditOpsRead(r.Context(), deps, "audit_log", "*", "list", map[string]any{
			"resource_type_filter": r.URL.Query().Get("resource_type"),
			"action_filter":        r.URL.Query().Get("action"),
			"actor_filter":         r.URL.Query().Get("actor"),
			"query":                query.Query,
			"sort_by":              query.SortBy,
			"sort_order":           query.SortOrder,
			"page":                 query.Page,
			"limit":                query.Limit,
			"item_count":           len(response.Items),
			"total":                meta.Total,
		})

		writeJSON(w, http.StatusOK, response)
	}
}

func knowledgeListHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}

		query := parseListQuery(r)
		reader, ok := deps.Knowledge.(contracts.KnowledgeListReader)
		if !ok {
			writeJSON(w, http.StatusOK, dto.KnowledgeListResponse{Items: []dto.KnowledgeRecord{}, ListPage: dto.ListPage{Page: query.Page, Limit: query.Limit}})
			return
		}

		items, err := reader.ListKnowledgeRecords(r.Context(), contracts.ListKnowledgeFilter{
			Query:     query.Query,
			SortBy:    query.SortBy,
			SortOrder: query.SortOrder,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}

		pageItems, meta := paginateItems(items, query)
		response := dto.KnowledgeListResponse{
			Items:    make([]dto.KnowledgeRecord, 0, len(pageItems)),
			ListPage: meta,
		}
		for _, item := range pageItems {
			response.Items = append(response.Items, knowledgeRecordDTO(item))
		}

		auditOpsRead(r.Context(), deps, "knowledge_record", "*", "list", map[string]any{
			"query":      query.Query,
			"sort_by":    query.SortBy,
			"sort_order": query.SortOrder,
			"page":       query.Page,
			"limit":      query.Limit,
			"item_count": len(response.Items),
			"total":      meta.Total,
		})

		writeJSON(w, http.StatusOK, response)
	}
}

func auditBulkExportHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}
		reader, ok := deps.Audit.(audit.BulkReader)
		if !ok {
			writeError(w, http.StatusConflict, "not_configured", "audit bulk export is not available")
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

		records, err := reader.ListByIDs(r.Context(), ids)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		recordMap := make(map[string]audit.Record, len(records))
		for _, record := range records {
			recordMap[record.ID] = record
		}
		items := make([]dto.AuditRecord, 0, len(ids))
		failures := make([]dto.BatchOperationResult, 0)
		for _, id := range ids {
			record, ok := recordMap[id]
			if !ok {
				failures = append(failures, dto.BatchOperationResult{ID: id, Success: false, Code: "not_found", Message: "resource not found"})
				continue
			}
			items = append(items, auditRecordDTO(record))
		}
		exportedAt := time.Now().UTC()
		auditOpsWrite(r.Context(), deps, "audit_log", "*", "bulk_export", map[string]any{
			"ids":             ids,
			"operator_reason": req.OperatorReason,
			"total":           len(ids),
			"exported":        len(items),
			"failed":          len(failures),
		})
		filename := fmt.Sprintf("tars-audit-export-%s.json", exportedAt.Format("20060102T150405Z"))
		writeJSONAttachment(w, http.StatusOK, filename, dto.AuditExportResponse{
			ResourceType:   "audit_log",
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

func knowledgeBulkExportHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		if !requireOpsAccess(deps, w, r) {
			return
		}
		reader, ok := deps.Knowledge.(contracts.KnowledgeBulkReader)
		if !ok {
			writeError(w, http.StatusConflict, "not_configured", "knowledge bulk export is not available")
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

		records, err := reader.ListKnowledgeRecordsByIDs(r.Context(), ids)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", err.Error())
			return
		}
		recordMap := make(map[string]contracts.KnowledgeRecordDetail, len(records))
		for _, record := range records {
			recordMap[record.DocumentID] = record
		}
		items := make([]dto.KnowledgeRecord, 0, len(ids))
		failures := make([]dto.BatchOperationResult, 0)
		for _, id := range ids {
			record, ok := recordMap[id]
			if !ok {
				failures = append(failures, dto.BatchOperationResult{ID: id, Success: false, Code: "not_found", Message: "resource not found"})
				continue
			}
			items = append(items, knowledgeRecordDTO(record))
		}
		exportedAt := time.Now().UTC()
		auditOpsWrite(r.Context(), deps, "knowledge_record", "*", "bulk_export", map[string]any{
			"ids":             ids,
			"operator_reason": req.OperatorReason,
			"total":           len(ids),
			"exported":        len(items),
			"failed":          len(failures),
		})
		filename := fmt.Sprintf("tars-knowledge-export-%s.json", exportedAt.Format("20060102T150405Z"))
		writeJSONAttachment(w, http.StatusOK, filename, dto.KnowledgeExportResponse{
			ResourceType:   "knowledge_record",
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
