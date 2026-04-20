package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"tars/internal/api/dto"
	"tars/internal/modules/automations"
)

type automationFilter struct {
	Type    string
	Status  string
	Enabled *bool
	Query   string
	SortBy  string
	Order   string
}

func automationsListHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		if deps.Automations == nil {
			writeError(w, http.StatusConflict, "not_configured", "automation manager is not configured")
			return
		}
		switch r.Method {
		case http.MethodGet:
			query := parseListQuery(r)
			items := listAutomations(deps.Automations.Snapshot(), automationFilter{
				Type:    strings.TrimSpace(r.URL.Query().Get("type")),
				Status:  strings.TrimSpace(r.URL.Query().Get("status")),
				Enabled: parseOptionalBool(r.URL.Query().Get("enabled")),
				Query:   query.Query,
				SortBy:  query.SortBy,
				Order:   query.SortOrder,
			})
			pageItems, meta := paginateItems(items, query)
			resp := dto.AutomationListResponse{Items: make([]dto.AutomationJob, 0, len(pageItems)), ListPage: meta}
			for _, item := range pageItems {
				resp.Items = append(resp.Items, automationJobDTO(item.Job, item.State))
			}
			auditOpsRead(r.Context(), deps, "automation_job", "", "automations_listed", map[string]any{"count": len(resp.Items)})
			writeJSON(w, http.StatusOK, resp)
		case http.MethodPost:
			var req struct {
				Job automations.Job `json:"job"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			job, state, err := deps.Automations.Upsert(req.Job)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_automation", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "automation_job", job.ID, "automation_created", map[string]any{"type": job.Type, "target_ref": job.TargetRef})
			writeJSON(w, http.StatusCreated, automationJobDTO(job, state))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func automationRouterHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requireOpsAccess(deps, w, r) {
			return
		}
		path := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/automations/"), "/")
		if path == "" {
			writeError(w, http.StatusNotFound, "not_found", "automation not found")
			return
		}
		switch {
		case strings.HasSuffix(path, "/enable"):
			automationEnableHandler(deps, strings.TrimSuffix(path, "/enable"), true)(w, r)
		case strings.HasSuffix(path, "/disable"):
			automationEnableHandler(deps, strings.TrimSuffix(path, "/disable"), false)(w, r)
		case strings.HasSuffix(path, "/run"):
			automationRunHandler(deps, strings.TrimSuffix(path, "/run"))(w, r)
		default:
			automationDetailHandler(deps, path)(w, r)
		}
	}
}

func automationDetailHandler(deps Dependencies, jobID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if deps.Automations == nil {
			writeError(w, http.StatusConflict, "not_configured", "automation manager is not configured")
			return
		}
		switch r.Method {
		case http.MethodGet:
			job, state, ok := deps.Automations.Get(strings.TrimSpace(jobID))
			if !ok {
				writeError(w, http.StatusNotFound, "not_found", "automation not found")
				return
			}
			auditOpsRead(r.Context(), deps, "automation_job", job.ID, "automation_viewed", nil)
			writeJSON(w, http.StatusOK, automationJobDTO(job, state))
		case http.MethodPut:
			var req struct {
				Job automations.Job `json:"job"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeValidationError(w, "invalid request body")
				return
			}
			if strings.TrimSpace(req.Job.ID) != strings.TrimSpace(jobID) {
				writeValidationError(w, "job.id must match automation id")
				return
			}
			job, state, err := deps.Automations.Upsert(req.Job)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_automation", err.Error())
				return
			}
			auditOpsWrite(r.Context(), deps, "automation_job", job.ID, "automation_updated", map[string]any{"type": job.Type, "target_ref": job.TargetRef})
			writeJSON(w, http.StatusOK, automationJobDTO(job, state))
		default:
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}
}

func automationEnableHandler(deps Dependencies, jobID string, enabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		job, state, err := deps.Automations.SetEnabled(strings.TrimSpace(jobID), enabled)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, automations.ErrJobNotFound) {
				status = http.StatusNotFound
			}
			writeError(w, status, "invalid_automation", err.Error())
			return
		}
		auditOpsWrite(r.Context(), deps, "automation_job", job.ID, map[bool]string{true: "automation_enabled", false: "automation_disabled"}[enabled], nil)
		writeJSON(w, http.StatusOK, automationJobDTO(job, state))
	}
}

func automationRunHandler(deps Dependencies, jobID string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		principal, ok := authenticatedPrincipal(deps, r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "invalid bearer token")
			return
		}
		run, err := deps.Automations.RunNow(r.Context(), strings.TrimSpace(jobID), automations.RunRequest{Trigger: "manual", TriggeredBy: principal.Source})
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, automations.ErrJobNotFound) {
				status = http.StatusNotFound
			}
			writeError(w, status, "automation_run_failed", err.Error())
			return
		}
		job, state, ok := deps.Automations.Get(strings.TrimSpace(jobID))
		if !ok {
			writeError(w, http.StatusNotFound, "not_found", "automation not found")
			return
		}
		response := automationJobDTO(job, state)
		lastRun := automationRunDTO(run)
		response.LastRun = &lastRun
		auditOpsWrite(r.Context(), deps, "automation_job", job.ID, "automation_run_now", map[string]any{"run_id": run.RunID, "status": run.Status})
		writeJSON(w, http.StatusOK, response)
	}
}

type automationListItem struct {
	Job   automations.Job
	State automations.JobState
}

func listAutomations(snapshot automations.Snapshot, filter automationFilter) []automationListItem {
	items := make([]automationListItem, 0, len(snapshot.Config.Jobs))
	query := strings.ToLower(strings.TrimSpace(filter.Query))
	for _, job := range snapshot.Config.Jobs {
		state := snapshot.State[job.ID]
		if filter.Type != "" && !strings.EqualFold(job.Type, filter.Type) {
			continue
		}
		if filter.Enabled != nil && job.Enabled != *filter.Enabled {
			continue
		}
		if filter.Status != "" && !strings.EqualFold(state.Status, filter.Status) {
			continue
		}
		if query != "" {
			haystacks := []string{job.ID, job.DisplayName, job.Description, job.Type, job.TargetRef, state.LastOutcome, state.LastError}
			matched := false
			for _, haystack := range haystacks {
				if strings.Contains(strings.ToLower(haystack), query) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		items = append(items, automationListItem{Job: job, State: state})
	}
	sort.SliceStable(items, func(i, j int) bool {
		sortBy := strings.ToLower(strings.TrimSpace(filter.SortBy))
		desc := strings.EqualFold(filter.Order, "desc")
		switch sortBy {
		case "next_run_at":
			return compareTime(items[i].State.NextRunAt, items[j].State.NextRunAt, desc)
		case "last_run_at":
			return compareTime(items[i].State.LastRunAt, items[j].State.LastRunAt, desc)
		case "status":
			return compareString(items[i].State.Status, items[j].State.Status, desc)
		case "type":
			return compareString(items[i].Job.Type, items[j].Job.Type, desc)
		default:
			return compareString(items[i].Job.DisplayName, items[j].Job.DisplayName, desc)
		}
	})
	return items
}

func compareString(left string, right string, desc bool) bool {
	l := strings.ToLower(strings.TrimSpace(left))
	r := strings.ToLower(strings.TrimSpace(right))
	if l == r {
		return false
	}
	if desc {
		return l > r
	}
	return l < r
}

func compareTime(left time.Time, right time.Time, desc bool) bool {
	if left.Equal(right) {
		return false
	}
	if left.IsZero() {
		return false
	}
	if right.IsZero() {
		return true
	}
	if desc {
		return left.After(right)
	}
	return left.Before(right)
}
