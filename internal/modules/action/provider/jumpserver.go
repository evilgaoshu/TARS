package provider

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"tars/internal/contracts"
	"tars/internal/foundation/secrets"
	"tars/internal/modules/action"
	"tars/internal/modules/connectors"
)

const (
	jumpServerDefaultTimeoutSeconds = 60
	jumpServerDefaultPollInterval   = 2 * time.Second
	jumpServerDefaultPollTimeout    = 2 * time.Minute
	jumpServerDefaultOrgID          = "00000000-0000-0000-0000-000000000002"
)

type JumpServerRuntime struct {
	Client *http.Client
}

type jumpServerConfig struct {
	BaseURL      string
	AccessKey    string
	SecretKey    string
	OrgID        string
	RunAs        string
	Module       string
	HostField    string
	HostSearch   string
	Timeout      int
	PollInterval time.Duration
	PollTimeout  time.Duration
}

type jumpServerExecutionResponse struct {
	ID     string `json:"id"`
	TaskID string `json:"task_id"`
}

type jumpServerTaskDetailResponse struct {
	Status struct {
		Value string `json:"value"`
		Label string `json:"label"`
	} `json:"status"`
	IsFinished bool                   `json:"is_finished"`
	IsSuccess  bool                   `json:"is_success"`
	Summary    map[string]interface{} `json:"summary"`
}

type jumpServerExecutionDetailResponse struct {
	ID      string                 `json:"id"`
	Result  interface{}            `json:"result"`
	Summary map[string]interface{} `json:"summary"`
}

type jumpServerLogResponse struct {
	Content string `json:"content"`
	Data    string `json:"data"`
	Log     string `json:"log"`
	Text    string `json:"text"`
}

type jumpServerAsset struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Name    string `json:"name"`
}

type jumpServerHostListResponse struct {
	Count   int               `json:"count"`
	Results []jumpServerAsset `json:"results"`
}

func (r *jumpServerHostListResponse) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*r = jumpServerHostListResponse{}
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var items []jumpServerAsset
		if err := json.Unmarshal(data, &items); err != nil {
			return err
		}
		r.Count = len(items)
		r.Results = items
		return nil
	}
	type alias jumpServerHostListResponse
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*r = jumpServerHostListResponse(decoded)
	if r.Count == 0 && len(r.Results) > 0 {
		r.Count = len(r.Results)
	}
	return nil
}

func NewJumpServerRuntime(client *http.Client) *JumpServerRuntime {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	return &JumpServerRuntime{Client: client}
}

func (r *JumpServerRuntime) Execute(ctx context.Context, manifest connectors.Manifest, req contracts.ApprovedExecutionRequest) (contracts.ExecutionResult, error) {
	resolved := secrets.ResolveValues(nil, manifest.Config.Values, manifest.Config.SecretRefs)
	cfg, err := loadJumpServerConfig(resolved)
	if err != nil {
		return contracts.ExecutionResult{}, err
	}

	asset, err := r.lookupAsset(ctx, cfg, req.TargetHost)
	if err != nil {
		return contracts.ExecutionResult{}, err
	}

	taskID, jobID, err := r.submitJob(ctx, cfg, asset, req)
	if err != nil {
		return contracts.ExecutionResult{}, err
	}

	statusDetail, err := r.pollTask(ctx, cfg, taskID)
	if err != nil {
		return contracts.ExecutionResult{}, err
	}

	executionDetail, execDetailErr := r.fetchExecutionDetail(ctx, cfg, taskID)
	logs, logErr := r.fetchExecutionLogs(ctx, cfg, taskID)
	combinedOutput := buildJumpServerOutput(logs, executionDetail, asset, req, taskID, jobID)
	status, exitCode := mapJumpServerExecutionStatus(statusDetail)
	if combinedOutput == "" {
		combinedOutput = fmt.Sprintf("jumpserver execution finished with status %s for %s", status, req.TargetHost)
	}

	result := contracts.ExecutionResult{
		ExecutionID:   req.ExecutionID,
		SessionID:     req.SessionID,
		Status:        status,
		ConnectorID:   manifest.Metadata.ID,
		Protocol:      manifest.Spec.Protocol,
		ExecutionMode: firstNonEmpty(req.ExecutionMode, "jumpserver_job"),
		Runtime: &contracts.RuntimeMetadata{
			Runtime:         "connector",
			Selection:       "explicit_connector",
			ConnectorID:     manifest.Metadata.ID,
			ConnectorType:   manifest.Spec.Type,
			ConnectorVendor: manifest.Metadata.Vendor,
			Protocol:        manifest.Spec.Protocol,
			ExecutionMode:   firstNonEmpty(req.ExecutionMode, "jumpserver_job"),
			FallbackEnabled: true,
			FallbackTarget:  "ssh",
		},
		ExitCode: exitCode,
		Output:   combinedOutput,
	}

	if execDetailErr == nil && executionDetail != nil {
		result.OutputPreview = compactJumpServerSummary(executionDetail.Summary)
	}
	if result.OutputPreview == "" {
		result.OutputPreview = compactJumpServerSummary(statusDetail.Summary)
	}
	if result.OutputPreview == "" {
		result.OutputPreview = firstLine(compactWhitespace(combinedOutput), 240)
	}
	if execDetailErr != nil {
		result.Output += fmt.Sprintf("\n\n[warn] fetch execution detail failed: %s", execDetailErr)
	}
	if logErr != nil {
		result.Output += fmt.Sprintf("\n\n[warn] fetch execution log failed: %s", logErr)
	}

	return result, nil
}

func (r *JumpServerRuntime) Verify(ctx context.Context, manifest connectors.Manifest, req contracts.VerificationRequest) (contracts.VerificationResult, error) {
	checkedAt := time.Now().UTC()
	resolved := secrets.ResolveValues(nil, manifest.Config.Values, manifest.Config.SecretRefs)
	cfg, err := loadJumpServerConfig(resolved)
	if err != nil {
		return contracts.VerificationResult{}, err
	}
	if strings.TrimSpace(req.TargetHost) == "" || strings.TrimSpace(req.Service) == "" {
		return contracts.VerificationResult{
			SessionID:   req.SessionID,
			ExecutionID: req.ExecutionID,
			Status:      "skipped",
			Summary:     fmt.Sprintf("verification skipped for connector %s", manifest.Metadata.ID),
			Runtime: &contracts.RuntimeMetadata{
				Runtime:         "connector",
				Selection:       "explicit_connector",
				ConnectorID:     manifest.Metadata.ID,
				ConnectorType:   manifest.Spec.Type,
				ConnectorVendor: manifest.Metadata.Vendor,
				Protocol:        manifest.Spec.Protocol,
				ExecutionMode:   firstNonEmpty(req.ExecutionMode, "jumpserver_job"),
				FallbackEnabled: true,
				FallbackTarget:  "ssh",
			},
			CheckedAt: checkedAt,
			Details: map[string]interface{}{
				"connector_id": manifest.Metadata.ID,
				"protocol":     manifest.Spec.Protocol,
				"mode":         firstNonEmpty(req.ExecutionMode, "jumpserver_job"),
			},
		}, nil
	}

	asset, err := r.lookupAsset(ctx, cfg, req.TargetHost)
	if err != nil {
		return contracts.VerificationResult{}, err
	}

	serviceCandidates := action.VerificationServiceCandidates(req.Service)
	verificationStatus := "failed"
	summary := fmt.Sprintf("verification failed: %s is not active", req.Service)
	var (
		statusDetail    *jumpServerTaskDetailResponse
		executionDetail *jumpServerExecutionDetailResponse
		combinedOutput  string
		matchedService  string
		lastTaskID      string
		lastJobID       string
		attempted       []string
	)
	for _, candidate := range serviceCandidates {
		verifyReq := contracts.ApprovedExecutionRequest{
			ExecutionID:     req.ExecutionID,
			SessionID:       req.SessionID,
			TargetHost:      req.TargetHost,
			Command:         fmt.Sprintf("systemctl is-active %s", candidate),
			Service:         candidate,
			ConnectorID:     req.ConnectorID,
			ConnectorVendor: manifest.Metadata.Vendor,
			Protocol:        manifest.Spec.Protocol,
			ExecutionMode:   firstNonEmpty(req.ExecutionMode, "jumpserver_verify"),
		}
		attempted = append(attempted, candidate)
		lastTaskID, lastJobID, err = r.submitJob(ctx, cfg, asset, verifyReq)
		if err != nil {
			return contracts.VerificationResult{}, err
		}
		statusDetail, err = r.pollTask(ctx, cfg, lastTaskID)
		if err != nil {
			return contracts.VerificationResult{}, err
		}
		executionDetail, _ = r.fetchExecutionDetail(ctx, cfg, lastTaskID)
		logs, _ := r.fetchExecutionLogs(ctx, cfg, lastTaskID)
		combinedOutput = buildJumpServerOutput(logs, executionDetail, asset, verifyReq, lastTaskID, lastJobID)
		status, _ := mapJumpServerExecutionStatus(statusDetail)
		if jumpServerOutputIndicatesActive(combinedOutput) && status == "completed" {
			verificationStatus = "success"
			summary = fmt.Sprintf("verification passed: %s is active", candidate)
			matchedService = candidate
			break
		}
	}

	details := map[string]interface{}{
		"connector_id":       manifest.Metadata.ID,
		"protocol":           manifest.Spec.Protocol,
		"mode":               firstNonEmpty(req.ExecutionMode, "jumpserver_job"),
		"target_host":        req.TargetHost,
		"service":            req.Service,
		"service_candidates": attempted,
		"output":             firstLine(compactWhitespace(combinedOutput), 240),
	}
	if statusDetail != nil {
		details["task_status"] = statusDetail.Status.Value
	}
	if matchedService != "" {
		details["matched_service"] = matchedService
	}
	if lastTaskID != "" {
		details["task_id"] = lastTaskID
	}
	if lastJobID != "" {
		details["job_id"] = lastJobID
	}
	return contracts.VerificationResult{
		SessionID:   req.SessionID,
		ExecutionID: req.ExecutionID,
		Status:      verificationStatus,
		Summary:     summary,
		Runtime: &contracts.RuntimeMetadata{
			Runtime:         "connector",
			Selection:       "explicit_connector",
			ConnectorID:     manifest.Metadata.ID,
			ConnectorType:   manifest.Spec.Type,
			ConnectorVendor: manifest.Metadata.Vendor,
			Protocol:        manifest.Spec.Protocol,
			ExecutionMode:   firstNonEmpty(req.ExecutionMode, "jumpserver_job"),
			FallbackEnabled: true,
			FallbackTarget:  "ssh",
		},
		CheckedAt: checkedAt,
		Details:   details,
	}, nil
}

func (r *JumpServerRuntime) CheckHealth(ctx context.Context, manifest connectors.Manifest) (string, string, error) {
	resolved := secrets.ResolveValues(nil, manifest.Config.Values, manifest.Config.SecretRefs)
	cfg, err := loadJumpServerConfig(resolved)
	if err != nil {
		return "unhealthy", "jumpserver connector is not fully configured: " + err.Error(), err
	}
	var assets jumpServerHostListResponse
	if err := r.getJSON(ctx, cfg, "/api/v1/assets/hosts/?limit=1", &assets); err != nil {
		return "unhealthy", "jumpserver API error: " + err.Error(), err
	}
	return "healthy", "jumpserver API probe succeeded", nil
}


func loadJumpServerConfig(values map[string]string) (jumpServerConfig, error) {
	cfg := jumpServerConfig{
		BaseURL:      strings.TrimRight(strings.TrimSpace(values["base_url"]), "/"),
		AccessKey:    strings.TrimSpace(values["access_key"]),
		SecretKey:    strings.TrimSpace(values["secret_key"]),
		OrgID:        firstNonEmpty(values["org_id"], jumpServerDefaultOrgID),
		RunAs:        firstNonEmpty(values["runas"], "root"),
		Module:       firstNonEmpty(values["module"], "shell"),
		HostField:    firstNonEmpty(values["host_lookup_field"], "address"),
		HostSearch:   firstNonEmpty(values["host_lookup_mode"], "exact"),
		Timeout:      parsePositiveInt(values["timeout_seconds"], jumpServerDefaultTimeoutSeconds),
		PollInterval: parseDurationSeconds(values["poll_interval_seconds"], jumpServerDefaultPollInterval),
		PollTimeout:  parseDurationSeconds(values["poll_timeout_seconds"], jumpServerDefaultPollTimeout),
	}
	missingFields := make([]string, 0, 3)
	if cfg.BaseURL == "" {
		missingFields = append(missingFields, "base_url")
	}
	if cfg.AccessKey == "" {
		missingFields = append(missingFields, "access_key")
	}
	if cfg.SecretKey == "" {
		missingFields = append(missingFields, "secret_key")
	}
	if len(missingFields) > 0 {
		return jumpServerConfig{}, fmt.Errorf("missing required fields: %s", strings.Join(missingFields, ", "))
	}
	return cfg, nil
}

func (r *JumpServerRuntime) lookupAsset(ctx context.Context, cfg jumpServerConfig, targetHost string) (jumpServerAsset, error) {
	targetHost = strings.TrimSpace(targetHost)
	if targetHost == "" {
		return jumpServerAsset{}, fmt.Errorf("target host is required")
	}

	params := url.Values{}
	switch strings.ToLower(cfg.HostSearch) {
	case "search", "fuzzy":
		params.Set("search", targetHost)
	default:
		params.Set(cfg.HostField, targetHost)
	}

	var list jumpServerHostListResponse
	if err := r.getJSON(ctx, cfg, "/api/v1/assets/hosts/?"+params.Encode(), &list); err != nil {
		return jumpServerAsset{}, fmt.Errorf("jumpserver asset lookup failed: %w", err)
	}
	if len(list.Results) == 0 && cfg.HostSearch != "search" {
		fallback := url.Values{}
		fallback.Set("search", targetHost)
		if err := r.getJSON(ctx, cfg, "/api/v1/assets/hosts/?"+fallback.Encode(), &list); err != nil {
			return jumpServerAsset{}, fmt.Errorf("jumpserver asset lookup failed: %w", err)
		}
	}
	for _, item := range list.Results {
		if strings.EqualFold(strings.TrimSpace(item.Address), targetHost) || strings.EqualFold(strings.TrimSpace(item.Name), targetHost) {
			return item, nil
		}
	}
	if len(list.Results) == 1 {
		return list.Results[0], nil
	}
	return jumpServerAsset{}, fmt.Errorf("jumpserver asset not found for host %s", targetHost)
}

func (r *JumpServerRuntime) submitJob(ctx context.Context, cfg jumpServerConfig, asset jumpServerAsset, req contracts.ApprovedExecutionRequest) (string, string, error) {
	payload := map[string]interface{}{
		"name":        fmt.Sprintf("tars-%s", firstNonEmpty(req.ExecutionID, "job")),
		"instant":     true,
		"is_periodic": false,
		"type":        "adhoc",
		"module":      cfg.Module,
		"args":        req.Command,
		"assets":      []string{asset.ID},
		"runas":       cfg.RunAs,
		"timeout":     cfg.Timeout,
		"comment":     fmt.Sprintf("TARS approved execution %s for session %s", req.ExecutionID, req.SessionID),
	}

	var response jumpServerExecutionResponse
	if err := r.postJSON(ctx, cfg, "/api/v1/ops/jobs/", payload, &response); err != nil {
		return "", "", fmt.Errorf("jumpserver job submission failed: %w", err)
	}
	taskID := firstNonEmpty(response.TaskID, response.ID)
	if taskID == "" {
		return "", "", fmt.Errorf("jumpserver job submission did not return task id")
	}
	return taskID, response.ID, nil
}

func (r *JumpServerRuntime) pollTask(ctx context.Context, cfg jumpServerConfig, taskID string) (*jumpServerTaskDetailResponse, error) {
	deadlineCtx, cancel := context.WithTimeout(ctx, cfg.PollTimeout)
	defer cancel()

	ticker := time.NewTicker(cfg.PollInterval)
	defer ticker.Stop()

	for {
		var detail jumpServerTaskDetailResponse
		if err := r.getJSON(deadlineCtx, cfg, "/api/v1/ops/job-execution/task-detail/"+taskID+"/", &detail); err != nil {
			return nil, err
		}
		if detail.IsFinished || jumpServerTaskDone(detail.Status.Value) {
			return &detail, nil
		}
		select {
		case <-deadlineCtx.Done():
			return nil, fmt.Errorf("jumpserver task polling timed out: %w", deadlineCtx.Err())
		case <-ticker.C:
		}
	}
}

func jumpServerTaskDone(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "failed", "timeout", "canceled", "cancelled", "error":
		return true
	default:
		return false
	}
}

func (r *JumpServerRuntime) fetchExecutionDetail(ctx context.Context, cfg jumpServerConfig, taskID string) (*jumpServerExecutionDetailResponse, error) {
	var detail jumpServerExecutionDetailResponse
	if err := r.getJSON(ctx, cfg, "/api/v1/ops/job-executions/"+taskID+"/", &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func (r *JumpServerRuntime) fetchExecutionLogs(ctx context.Context, cfg jumpServerConfig, taskID string) (string, error) {
	req, err := r.newRequest(ctx, cfg, http.MethodGet, "/api/v1/ops/ansible/job-execution/"+taskID+"/log/", nil)
	if err != nil {
		return "", err
	}
	resp, err := r.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("status=%d body=%s", resp.StatusCode, compactWhitespace(string(body)))
	}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "", nil
	}
	var payload jumpServerLogResponse
	if err := json.Unmarshal(body, &payload); err == nil {
		return firstNonEmpty(payload.Content, payload.Data, payload.Log, payload.Text), nil
	}
	return trimmed, nil
}

func (r *JumpServerRuntime) getJSON(ctx context.Context, cfg jumpServerConfig, path string, target interface{}) error {
	req, err := r.newRequest(ctx, cfg, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	resp, err := r.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, compactWhitespace(string(body)))
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (r *JumpServerRuntime) postJSON(ctx context.Context, cfg jumpServerConfig, path string, payload interface{}, target interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := r.newRequest(ctx, cfg, http.MethodPost, path, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := r.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("status=%d body=%s", resp.StatusCode, compactWhitespace(string(respBody)))
	}
	if target == nil {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func (r *JumpServerRuntime) newRequest(ctx context.Context, cfg jumpServerConfig, method string, path string, body io.Reader) (*http.Request, error) {
	fullURL := cfg.BaseURL + path
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Set("X-JMS-ORG", cfg.OrgID)
	req.Header.Set("Authorization", jumpServerSignatureHeader(cfg.AccessKey, cfg.SecretKey, method, req.URL.RequestURI(), req.Header.Get("Accept"), req.Header.Get("Date")))
	return req, nil
}

func jumpServerSignatureHeader(accessKey string, secretKey string, method string, requestURI string, accept string, date string) string {
	canonical := strings.Join([]string{
		fmt.Sprintf("(request-target): %s %s", strings.ToLower(method), requestURI),
		fmt.Sprintf("accept: %s", accept),
		fmt.Sprintf("date: %s", date),
	}, "\n")
	mac := hmac.New(sha256.New, []byte(secretKey))
	_, _ = mac.Write([]byte(canonical))
	signature := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf(`Signature keyId=%q,algorithm=%q,headers=%q,signature=%q`, accessKey, "hmac-sha256", "(request-target) accept date", signature)
}

func buildJumpServerOutput(logs string, detail *jumpServerExecutionDetailResponse, asset jumpServerAsset, req contracts.ApprovedExecutionRequest, taskID string, jobID string) string {
	sections := make([]string, 0, 6)
	sections = append(sections,
		fmt.Sprintf("[jumpserver] host=%s asset_id=%s task_id=%s job_id=%s", req.TargetHost, asset.ID, taskID, jobID),
		fmt.Sprintf("[command] %s", req.Command),
	)
	if strings.TrimSpace(logs) != "" {
		sections = append(sections, "[logs]\n"+strings.TrimSpace(logs))
	}
	if detail != nil {
		if result := stringifyJumpServerPayload(detail.Result); result != "" {
			sections = append(sections, "[result]\n"+result)
		}
		if summary := compactJumpServerSummary(detail.Summary); summary != "" {
			sections = append(sections, "[summary]\n"+summary)
		}
	}
	return strings.TrimSpace(strings.Join(sections, "\n\n"))
}

func mapJumpServerExecutionStatus(detail *jumpServerTaskDetailResponse) (string, int) {
	if detail == nil {
		return "failed", 1
	}
	switch strings.ToLower(strings.TrimSpace(detail.Status.Value)) {
	case "success":
		return "completed", 0
	case "timeout":
		return "timeout", 124
	case "canceled", "cancelled":
		return "failed", 130
	default:
		if detail.IsSuccess {
			return "completed", 0
		}
		return "failed", 1
	}
}

func compactJumpServerSummary(summary map[string]interface{}) string {
	if len(summary) == 0 {
		return ""
	}
	keys := make([]string, 0, len(summary))
	for key := range summary {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		value := strings.TrimSpace(stringifyJumpServerPayload(summary[key]))
		if value == "" {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(parts, "; ")
}

func stringifyJumpServerPayload(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return compactWhitespace(typed)
	case fmt.Stringer:
		return compactWhitespace(typed.String())
	default:
		bytes, err := json.MarshalIndent(typed, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(bytes)
	}
}

func compactWhitespace(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	lines := strings.Split(value, "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, strings.Join(strings.Fields(trimmed), " "))
	}
	return strings.TrimSpace(strings.Join(normalized, "\n"))
}

func firstLine(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if idx := strings.IndexByte(value, '\n'); idx >= 0 {
		value = value[:idx]
	}
	if maxLen > 0 && len(value) > maxLen {
		return value[:maxLen]
	}
	return value
}

func jumpServerOutputIndicatesActive(value string) bool {
	normalized := strings.ToLower(stripANSIEscapeCodes(strings.ReplaceAll(value, "\x00", "")))
	for _, line := range strings.Split(normalized, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "active" || strings.HasSuffix(trimmed, " active") {
			return true
		}
	}
	return false
}

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

func stripANSIEscapeCodes(value string) string {
	return ansiEscapePattern.ReplaceAllString(value, "")
}

func parsePositiveInt(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func parseDurationSeconds(raw string, fallback time.Duration) time.Duration {
	seconds := parsePositiveInt(raw, int(fallback/time.Second))
	return time.Duration(seconds) * time.Second
}

var _ action.ExecutionRuntime = (*JumpServerRuntime)(nil)
