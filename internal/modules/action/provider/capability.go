package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"tars/internal/contracts"
	"tars/internal/modules/connectors"
)

const capabilityDefaultTimeout = 15 * time.Second

type ObservabilityHTTPRuntime struct {
	client *http.Client
}

type DeliveryRuntime struct {
	client *http.Client
}

func NewObservabilityHTTPRuntime(client *http.Client) *ObservabilityHTTPRuntime {
	if client == nil {
		client = &http.Client{Timeout: capabilityDefaultTimeout}
	}
	return &ObservabilityHTTPRuntime{client: client}
}

func NewDeliveryRuntime(client *http.Client) *DeliveryRuntime {
	if client == nil {
		client = &http.Client{Timeout: capabilityDefaultTimeout}
	}
	return &DeliveryRuntime{client: client}
}

func (r *ObservabilityHTTPRuntime) Invoke(ctx context.Context, manifest connectors.Manifest, capabilityID string, params map[string]interface{}) (contracts.CapabilityResult, error) {
	capID := strings.ToLower(strings.TrimSpace(capabilityID))
	switch capID {
	case "", "query", "observability.query", "log.query", "trace.query":
	default:
		return contracts.CapabilityResult{
			Status: "failed",
			Error:  fmt.Sprintf("unsupported observability capability %q", capabilityID),
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", false, false, "",
				""),
		}, fmt.Errorf("unsupported observability capability %q", capabilityID)
	}

	baseURL := strings.TrimRight(strings.TrimSpace(manifest.Config.Values["base_url"]), "/")
	if strings.EqualFold(strings.TrimSpace(manifest.Spec.Protocol), "log_file") {
		return r.queryLogFile(manifest, capabilityID, params)
	}
	if baseURL == "" {
		return contracts.CapabilityResult{
			Status: "failed",
			Error:  "observability connector base_url is not configured",
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", false, false,
				"", ""),
		}, fmt.Errorf("observability connector base_url is not configured")
	}

	reqURL, err := buildObservabilityQueryURL(manifest, baseURL, params)
	if err != nil {
		return contracts.CapabilityResult{
			Status:  "failed",
			Error:   err.Error(),
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", false, false, "", ""),
		}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return contracts.CapabilityResult{
			Status:  "failed",
			Error:   err.Error(),
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", false, false, "", ""),
		}, err
	}
	req.Header.Set("Accept", "application/json")
	if token := strings.TrimSpace(manifest.Config.Values["bearer_token"]); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return contracts.CapabilityResult{
			Status: "failed",
			Error:  err.Error(),
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", false, false,
				"", ""),
		}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return contracts.CapabilityResult{
			Status: "failed",
			Error:  err.Error(),
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", false, false,
				"", ""),
		}, err
	}

	if resp.StatusCode >= http.StatusBadRequest {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("observability query failed with status %d", resp.StatusCode)
		}
		return contracts.CapabilityResult{
			Status: "failed",
			Error:  message,
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", false, false,
				"", ""),
			Metadata: map[string]interface{}{
				"http_status": resp.StatusCode,
			},
		}, fmt.Errorf("observability query failed: %s", message)
	}

	payload, summary, count, meta := parseObservabilityPayload(body)
	payload = filterObservabilityResults(payload, params)
	count = len(payload)
	if summary == "" || strings.HasPrefix(strings.ToLower(summary), "observability returned ") || strings.HasPrefix(strings.ToLower(summary), "returned ") {
		summary = summarizeObservabilityResults(payload, meta)
	}
	meta["http_status"] = resp.StatusCode
	meta["request_url"] = req.URL.String()
	artifacts := buildCapabilityArtifacts("observability", payload, summary, map[string]interface{}{
		"connector_id":  strings.TrimSpace(manifest.Metadata.ID),
		"capability_id": capabilityFirstNonEmpty(capabilityID, "observability.query"),
		"query_params":  cloneCapabilityParams(params),
		"metadata":      meta,
	})

	return contracts.CapabilityResult{
		Status: "completed",
		Output: map[string]interface{}{
			"source":          capabilityFirstNonEmpty(strings.TrimSpace(manifest.Spec.Protocol), "observability_http"),
			"capability_id":   capabilityFirstNonEmpty(capabilityID, "observability.query"),
			"connector_id":    strings.TrimSpace(manifest.Metadata.ID),
			"query_params":    cloneCapabilityParams(params),
			"result_count":    count,
			"summary":         summary,
			"results":         payload,
			"queried_at":      time.Now().UTC().Format(time.RFC3339),
			"runtime_state":   runtimeStateFromProtocol(manifest.Spec.Protocol),
			"capability_kind": "observability",
		},
		Artifacts: artifacts,
		Metadata:  meta,
		Runtime:   capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", true, false, "", "stub"),
	}, nil
}

func (r *ObservabilityHTTPRuntime) queryLogFile(manifest connectors.Manifest, capabilityID string, params map[string]interface{}) (contracts.CapabilityResult, error) {
	logPath := strings.TrimSpace(manifest.Config.Values["file_path"])
	if logPath == "" {
		logPath = strings.TrimSpace(manifest.Config.Values["log_path"])
	}
	if logPath == "" {
		return contracts.CapabilityResult{
			Status:  "failed",
			Error:   "observability log_file connector file_path is not configured",
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", true, false, "", ""),
		}, fmt.Errorf("observability log_file connector file_path is not configured")
	}

	content, err := tailFile(logPath, int64(clampInt(intFromAny(params["tail_bytes"], 256*1024), 4*1024, 2*1024*1024)))
	if err != nil {
		return contracts.CapabilityResult{
			Status:  "failed",
			Error:   err.Error(),
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", true, false, "", ""),
		}, err
	}

	filterTerms := buildObservabilityFilterTerms(params)
	limit := clampInt(intFromAny(params["limit"], 20), 1, 200)
	results := filterLogLines(content, filterTerms, limit)
	summary := fmt.Sprintf("matched %d log line(s) from %s", len(results), logPath)
	if len(filterTerms) > 0 {
		summary = fmt.Sprintf("matched %d log line(s) for %q", len(results), strings.Join(filterTerms, " "))
	}
	outputResults := make([]interface{}, 0, len(results))
	for _, line := range results {
		outputResults = append(outputResults, map[string]interface{}{"line": line})
	}

	return contracts.CapabilityResult{
		Status: "completed",
		Output: map[string]interface{}{
			"source":          "observability_log_file",
			"capability_id":   capabilityFirstNonEmpty(capabilityID, "observability.query"),
			"connector_id":    strings.TrimSpace(manifest.Metadata.ID),
			"query_params":    cloneCapabilityParams(params),
			"result_count":    len(outputResults),
			"summary":         summary,
			"results":         outputResults,
			"queried_at":      time.Now().UTC().Format(time.RFC3339),
			"runtime_state":   runtimeStateFromProtocol(manifest.Spec.Protocol),
			"capability_kind": "observability",
			"log_path":        logPath,
		},
		Artifacts: buildCapabilityArtifacts("observability", outputResults, summary, map[string]interface{}{
			"connector_id":  strings.TrimSpace(manifest.Metadata.ID),
			"capability_id": capabilityFirstNonEmpty(capabilityID, "observability.query"),
			"query_params":  cloneCapabilityParams(params),
			"log_path":      logPath,
		}),
		Metadata: map[string]interface{}{
			"log_path": logPath,
			"filters":  filterTerms,
		},
		Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", true, false, "", "stub"),
	}, nil
}

func (r *ObservabilityHTTPRuntime) CheckHealth(ctx context.Context, manifest connectors.Manifest) (string, string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(manifest.Config.Values["base_url"]), "/")
	if strings.EqualFold(strings.TrimSpace(manifest.Spec.Protocol), "log_file") {
		logPath := strings.TrimSpace(manifest.Config.Values["file_path"])
		if logPath == "" {
			logPath = strings.TrimSpace(manifest.Config.Values["log_path"])
		}
		if logPath == "" {
			return "unhealthy", "observability log_file connector file_path is not configured", fmt.Errorf("observability log_file connector file_path is not configured")
		}
		if _, err := os.Stat(logPath); err != nil {
			return "unhealthy", "log file unreachable: " + err.Error(), err
		}
		return "healthy", fmt.Sprintf("observability connector tails %s", logPath), nil
	}
	if baseURL == "" {
		return "unhealthy", "observability connector base_url is not configured", fmt.Errorf("observability connector base_url is not configured")
	}
	healthPath := strings.TrimSpace(manifest.Config.Values["health_path"])
	if healthPath == "" {
		healthPath = "/health"
	}
	if !strings.HasPrefix(healthPath, "/") {
		healthPath = "/" + healthPath
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+healthPath, nil)
	if err != nil {
		return "unhealthy", "failed to create health request: " + err.Error(), err
	}
	req.Header.Set("Accept", "application/json")
	if token := strings.TrimSpace(manifest.Config.Values["bearer_token"]); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := r.client.Do(req)
	if err != nil {
		if strings.Contains(err.Error(), "connection refused") || strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "no such host") {
			return "unhealthy", "observability API unreachable: " + err.Error(), err
		}
		return "unhealthy", "observability API probe failed: " + err.Error(), err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		summary := strings.TrimSpace(string(body))
		if summary == "" {
			summary = fmt.Sprintf("observability health check failed with status %d", resp.StatusCode)
		}
		return "unhealthy", summary, fmt.Errorf("%s", summary)
	}
	return "healthy", "observability connector health check succeeded", nil
}


func (r *DeliveryRuntime) Invoke(ctx context.Context, manifest connectors.Manifest, capabilityID string, params map[string]interface{}) (contracts.CapabilityResult, error) {
	capID := strings.ToLower(strings.TrimSpace(capabilityID))
	switch capID {
	case "", "query", "delivery.query", "status.query":
	default:
		return contracts.CapabilityResult{
			Status:  "failed",
			Error:   fmt.Sprintf("unsupported delivery capability %q", capabilityID),
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", true, false, "", ""),
		}, fmt.Errorf("unsupported delivery capability %q", capabilityID)
	}

	if repoURL := strings.TrimSpace(manifest.Config.Values["repo_url"]); repoURL != "" && prefersRemoteDelivery(manifest) {
		result, err := r.queryRemoteGit(ctx, manifest, capabilityID, repoURL, params)
		if err == nil {
			return result, nil
		}
		return result, err
	}

	repoPath, err := resolveDeliveryRepoPath(manifest)
	if err != nil {
		if repoURL := strings.TrimSpace(manifest.Config.Values["repo_url"]); repoURL != "" {
			result, remoteErr := r.queryRemoteGit(ctx, manifest, capabilityID, repoURL, params)
			if remoteErr == nil {
				return result, nil
			}
			return result, remoteErr
		}
		return contracts.CapabilityResult{
			Status:  "failed",
			Error:   err.Error(),
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", true, false, "", ""),
		}, err
	}

	gitResult, err := queryDeliveryGit(ctx, repoPath, params)
	if err != nil {
		return contracts.CapabilityResult{
			Status:  "failed",
			Error:   err.Error(),
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", true, false, "", ""),
			Metadata: map[string]interface{}{
				"repo_path": repoPath,
			},
		}, err
	}

	output := map[string]interface{}{
		"source":          "delivery_git",
		"capability_id":   capabilityFirstNonEmpty(capabilityID, "delivery.query"),
		"connector_id":    strings.TrimSpace(manifest.Metadata.ID),
		"repo_path":       repoPath,
		"query_params":    cloneCapabilityParams(params),
		"result_count":    len(gitResult.Commits),
		"summary":         gitResult.Summary,
		"branch":          gitResult.Branch,
		"head":            gitResult.Head,
		"commits":         gitResult.Commits,
		"working_tree":    gitResult.WorkingTree,
		"runtime_state":   runtimeStateFromProtocol(manifest.Spec.Protocol),
		"capability_kind": "delivery",
		"queried_at":      time.Now().UTC().Format(time.RFC3339),
	}

	if gitResult.RemoteURL != "" {
		output["remote_url"] = gitResult.RemoteURL
	}

	metadata := map[string]interface{}{
		"repo_path": repoPath,
	}
	if gitResult.Filter != "" {
		metadata["filter"] = gitResult.Filter
	}

	return contracts.CapabilityResult{
		Status: "completed",
		Output: output,
		Artifacts: buildCapabilityArtifacts("delivery", gitResult.Commits, gitResult.Summary, map[string]interface{}{
			"connector_id":  strings.TrimSpace(manifest.Metadata.ID),
			"capability_id": capabilityFirstNonEmpty(capabilityID, "delivery.query"),
			"repo_path":     repoPath,
			"branch":        gitResult.Branch,
			"head":          gitResult.Head,
			"query_params":  cloneCapabilityParams(params),
		}),
		Metadata: metadata,
		Runtime:  capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", true, false, "", "stub"),
	}, nil
}

func (r *DeliveryRuntime) CheckHealth(ctx context.Context, manifest connectors.Manifest) (string, string, error) {
	repoPath, err := resolveDeliveryRepoPath(manifest)
	if err != nil {
		if repoURL := strings.TrimSpace(manifest.Config.Values["repo_url"]); repoURL != "" {
			owner, repo, parseErr := parseGitHubRepository(repoURL)
			if parseErr != nil {
				// URL is not a valid GitHub repo - return the original local config error
				return "unhealthy", "delivery connector not configured: " + err.Error(), err
			}
			req, reqErr := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo), nil)
			if reqErr != nil {
				return "unhealthy", "failed to create github probe request: " + reqErr.Error(), reqErr
			}
			resp, doErr := r.client.Do(req)
			if doErr != nil {
				if strings.Contains(doErr.Error(), "connection refused") || strings.Contains(doErr.Error(), "timeout") || strings.Contains(doErr.Error(), "no such host") {
					return "unhealthy", "github API unreachable: " + doErr.Error(), doErr
				}
				return "unhealthy", "github API probe failed: " + doErr.Error(), doErr
			}
			defer resp.Body.Close()
			if resp.StatusCode >= http.StatusBadRequest {
				return "unhealthy", fmt.Sprintf("github repo probe failed with status %d", resp.StatusCode), fmt.Errorf("github repo probe failed with status %d", resp.StatusCode)
			}
			return "healthy", fmt.Sprintf("delivery connector points to github repo %s/%s", owner, repo), nil
		}
		return "unhealthy", "delivery connector not configured: " + err.Error(), err
	}
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "rev-parse", "--is-inside-work-tree")
	output, err := cmd.CombinedOutput()
	if err != nil {
		summary := strings.TrimSpace(string(output))
		if summary == "" {
			summary = err.Error()
		}
		return "unhealthy", "local git repo probe failed: " + summary, fmt.Errorf("delivery git health check failed: %s", summary)
	}
	if strings.TrimSpace(string(output)) != "true" {
		return "unhealthy", "configured repo_path is not a git repository", fmt.Errorf("configured repo_path is not a git repository")
	}
	return "healthy", fmt.Sprintf("delivery connector points to git repo %s", repoPath), nil
}


type deliveryGitResult struct {
	Summary     string
	Filter      string
	Branch      string
	Head        string
	RemoteURL   string
	WorkingTree map[string]interface{}
	Commits     []map[string]interface{}
}

func queryDeliveryGit(ctx context.Context, repoPath string, params map[string]interface{}) (deliveryGitResult, error) {
	branch, err := gitSingleLine(ctx, repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return deliveryGitResult{}, err
	}
	head, err := gitSingleLine(ctx, repoPath, "rev-parse", "HEAD")
	if err != nil {
		return deliveryGitResult{}, err
	}
	remoteURL, _ := gitSingleLine(ctx, repoPath, "remote", "get-url", "origin")
	statusText, err := gitSingleLine(ctx, repoPath, "status", "--short", "--branch")
	if err != nil {
		return deliveryGitResult{}, err
	}

	filter := strings.ToLower(strings.TrimSpace(capabilityFirstNonEmpty(
		interfaceString(params["service"]),
		interfaceString(params["query"]),
		interfaceString(params["summary"]),
	)))
	commitLimit := clampInt(intFromAny(params["limit"], 8), 1, 20)
	logOutput, err := gitMultiLine(ctx, repoPath, "log", fmt.Sprintf("-%d", commitLimit*3), "--date=iso-strict", "--pretty=format:%H%x1f%an%x1f%ad%x1f%s")
	if err != nil {
		return deliveryGitResult{}, err
	}
	commits := parseDeliveryCommits(logOutput, filter, commitLimit)
	if len(commits) == 0 {
		fallbackOutput, fallbackErr := gitMultiLine(ctx, repoPath, "log", fmt.Sprintf("-%d", commitLimit), "--date=iso-strict", "--pretty=format:%H%x1f%an%x1f%ad%x1f%s")
		if fallbackErr != nil {
			return deliveryGitResult{}, fallbackErr
		}
		commits = parseDeliveryCommits(fallbackOutput, "", commitLimit)
	}

	summary := fmt.Sprintf("delivery facts from %s on branch %s", filepath.Base(repoPath), branch)
	if filter != "" {
		summary = fmt.Sprintf("delivery facts filtered by %q on branch %s", filter, branch)
	}

	return deliveryGitResult{
		Summary:   summary,
		Filter:    filter,
		Branch:    branch,
		Head:      head,
		RemoteURL: remoteURL,
		WorkingTree: map[string]interface{}{
			"status": statusText,
			"dirty":  strings.Contains(statusText, "ahead") || strings.Contains(statusText, "behind") || strings.Count(statusText, "\n") > 0,
		},
		Commits: commits,
	}, nil
}

func parseDeliveryCommits(output string, filter string, limit int) []map[string]interface{} {
	if limit <= 0 {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	commits := make([]map[string]interface{}, 0, minInt(len(lines), limit))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\x1f")
		if len(parts) < 4 {
			continue
		}
		subject := strings.TrimSpace(parts[3])
		if filter != "" && !strings.Contains(strings.ToLower(subject), filter) {
			continue
		}
		commits = append(commits, map[string]interface{}{
			"sha":          strings.TrimSpace(parts[0]),
			"author":       strings.TrimSpace(parts[1]),
			"committed_at": strings.TrimSpace(parts[2]),
			"subject":      subject,
		})
		if len(commits) >= limit {
			break
		}
	}
	return commits
}

func resolveDeliveryRepoPath(manifest connectors.Manifest) (string, error) {
	repoPath := strings.TrimSpace(manifest.Config.Values["repo_path"])
	if repoPath == "" {
		repoPath = strings.TrimSpace(manifest.Config.Values["git_repo_path"])
	}
	if repoPath == "" {
		if prefersRemoteDelivery(manifest) {
			return "", fmt.Errorf("repo_path is not configured for local delivery mode")
		}
		repoPath = "."
	}
	if !filepath.IsAbs(repoPath) {
		abs, err := filepath.Abs(repoPath)
		if err != nil {
			return "", fmt.Errorf("resolve repo_path: %w", err)
		}
		repoPath = abs
	}
	return repoPath, nil
}

func prefersRemoteDelivery(manifest connectors.Manifest) bool {
	protocol := strings.ToLower(strings.TrimSpace(manifest.Spec.Protocol))
	switch protocol {
	case "delivery_github", "delivery_http":
		return true
	default:
		return false
	}
}

func gitSingleLine(ctx context.Context, repoPath string, args ...string) (string, error) {
	output, err := gitMultiLine(ctx, repoPath, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.Split(strings.TrimSpace(output), "\n")[0]), nil
}

func gitMultiLine(ctx context.Context, repoPath string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repoPath}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), message)
	}
	return string(output), nil
}

func buildObservabilityQueryURL(manifest connectors.Manifest, baseURL string, params map[string]interface{}) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse observability base_url: %w", err)
	}
	endpoint := strings.TrimSpace(interfaceString(params["endpoint"]))
	if endpoint == "" {
		mode := strings.ToLower(strings.TrimSpace(capabilityFirstNonEmpty(interfaceString(params["mode"]), interfaceString(params["kind"]))))
		switch mode {
		case "alerts", "alert":
			endpoint = strings.TrimSpace(manifest.Config.Values["alerts_path"])
		case "rules", "rule":
			endpoint = strings.TrimSpace(manifest.Config.Values["rules_path"])
		}
	}
	if endpoint == "" {
		endpoint = strings.TrimSpace(manifest.Config.Values["query_path"])
	}
	if endpoint == "" {
		endpoint = "/query"
	}
	if !strings.HasPrefix(endpoint, "/") {
		endpoint = "/" + endpoint
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + endpoint

	queryValues := parsed.Query()
	queryParamName := capabilityFirstNonEmpty(strings.TrimSpace(manifest.Config.Values["query_param_name"]), "query")
	for _, key := range []string{"query", "service", "host", "severity", "kind", "trace_id", "span_id"} {
		if value := strings.TrimSpace(interfaceString(params[key])); value != "" {
			targetKey := key
			if key == "query" {
				targetKey = queryParamName
			}
			queryValues.Set(targetKey, value)
		}
	}
	if limit := clampInt(intFromAny(params["limit"], 20), 1, 200); limit > 0 {
		queryValues.Set("limit", strconv.Itoa(limit))
	}
	parsed.RawQuery = queryValues.Encode()
	return parsed.String(), nil
}

func parseObservabilityPayload(body []byte) ([]interface{}, string, int, map[string]interface{}) {
	meta := map[string]interface{}{}
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return nil, "no observability results returned", 0, meta
	}

	var generic interface{}
	if err := json.Unmarshal(body, &generic); err != nil {
		return []interface{}{map[string]interface{}{"raw": trimmed}}, capabilityFirstNonEmpty(trimmed, "observability response received"), 1, meta
	}

	switch typed := generic.(type) {
	case []interface{}:
		return typed, summarizeObservabilityResults(typed, nil), len(typed), meta
	case map[string]interface{}:
		meta = typed
		results := extractObservabilityResults(typed)
		summary := stringMapValue(typed, "summary")
		if summary == "" {
			summary = summarizeObservabilityResults(results, typed)
		}
		return results, summary, len(results), meta
	default:
		return []interface{}{typed}, fmt.Sprintf("observability response type %T", typed), 1, meta
	}
}

func tailFile(path string, maxBytes int64) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return "", err
	}
	start := int64(0)
	if stat.Size() > maxBytes {
		start = stat.Size() - maxBytes
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		return "", err
	}
	body, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func buildObservabilityFilterTerms(params map[string]interface{}) []string {
	raw := []string{
		interfaceString(params["query"]),
		interfaceString(params["service"]),
		interfaceString(params["host"]),
		interfaceString(params["severity"]),
	}
	terms := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, item := range raw {
		for _, token := range strings.Fields(strings.ToLower(strings.TrimSpace(item))) {
			if len(token) < 2 {
				continue
			}
			if _, ok := seen[token]; ok {
				continue
			}
			seen[token] = struct{}{}
			terms = append(terms, token)
		}
	}
	return terms
}

func filterLogLines(content string, filters []string, limit int) []string {
	lines := strings.Split(content, "\n")
	preferred := make([]string, 0, minInt(len(lines), limit))
	fallback := make([]string, 0, minInt(len(lines), limit))
	auditRequested := filtersExplicitlyRequestAudit(filters)
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		matched := len(filters) == 0
		for _, filter := range filters {
			if strings.Contains(lower, filter) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if !auditRequested && isLikelyAuditLogLine(lower) {
			if len(fallback) < limit {
				fallback = append(fallback, line)
			}
			continue
		}
		preferred = append(preferred, line)
		if len(preferred) >= limit {
			break
		}
	}
	results := preferred
	if len(results) == 0 {
		results = fallback
	}
	for left, right := 0, len(results)-1; left < right; left, right = left+1, right-1 {
		results[left], results[right] = results[right], results[left]
	}
	return results
}

func filtersExplicitlyRequestAudit(filters []string) bool {
	for _, filter := range filters {
		switch strings.TrimSpace(strings.ToLower(filter)) {
		case "audit", "审计", "审计日志":
			return true
		}
	}
	return false
}

func isLikelyAuditLogLine(lower string) bool {
	for _, marker := range []string{
		`"msg":"audit event"`,
		`"component":"audit"`,
		`"resource_type":"audit"`,
		" audit event",
		"component\":\"audit",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func extractObservabilityResults(payload map[string]interface{}) []interface{} {
	if data, ok := payload["data"].(map[string]interface{}); ok {
		if alerts, ok := data["alerts"].([]interface{}); ok {
			return alerts
		}
		if groups, ok := data["groups"].([]interface{}); ok {
			return groups
		}
	}
	for _, key := range []string{"results", "items", "logs", "entries", "data"} {
		if values, ok := payload[key].([]interface{}); ok {
			return values
		}
		if nested, ok := payload[key].(map[string]interface{}); ok {
			for _, nestedKey := range []string{"results", "alerts", "groups", "items", "entries"} {
				if values, ok := nested[nestedKey].([]interface{}); ok {
					return values
				}
			}
			if values, ok := nested["results"].([]interface{}); ok {
				return values
			}
		}
	}
	return []interface{}{payload}
}

func summarizeObservabilityResults(results []interface{}, payload map[string]interface{}) string {
	if payload != nil {
		if data, ok := payload["data"].(map[string]interface{}); ok {
			if alerts, ok := data["alerts"].([]interface{}); ok {
				firing := 0
				for _, item := range alerts {
					alert, ok := item.(map[string]interface{})
					if !ok {
						continue
					}
					state := strings.ToLower(strings.TrimSpace(interfaceString(alert["state"])))
					if state == "firing" || state == "pending" || state == "" {
						firing++
					}
				}
				if firing > 0 {
					return fmt.Sprintf("returned %d alert(s), %d firing", len(alerts), firing)
				}
				return fmt.Sprintf("returned %d alert(s)", len(alerts))
			}
			if groups, ok := data["groups"].([]interface{}); ok {
				return fmt.Sprintf("returned %d rule group(s)", len(groups))
			}
		}
		for _, key := range []string{"summary", "message", "note"} {
			if value := strings.TrimSpace(interfaceString(payload[key])); value != "" {
				return value
			}
		}
	}
	if len(results) == 0 {
		return "no observability results returned"
	}
	if first, ok := firstMap(results); ok {
		for _, key := range []string{"summary", "message", "log", "event", "title"} {
			if value := strings.TrimSpace(interfaceString(first[key])); value != "" {
				return value
			}
		}
	}
	return fmt.Sprintf("observability returned %d result(s)", len(results))
}

func filterObservabilityResults(results []interface{}, params map[string]interface{}) []interface{} {
	if len(results) == 0 {
		return results
	}
	terms := buildObservabilityFilterTerms(params)
	if len(terms) == 0 {
		return results
	}
	filtered := make([]interface{}, 0, len(results))
	for _, item := range results {
		serialized, err := json.Marshal(item)
		if err != nil {
			filtered = append(filtered, item)
			continue
		}
		lower := strings.ToLower(string(serialized))
		matched := false
		for _, term := range terms {
			if strings.Contains(lower, term) {
				matched = true
				break
			}
		}
		if matched {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == 0 {
		return results
	}
	return filtered
}

func firstMap(items []interface{}) (map[string]interface{}, bool) {
	if len(items) == 0 {
		return nil, false
	}
	value, ok := items[0].(map[string]interface{})
	return value, ok
}

func stringMapValue(item map[string]interface{}, key string) string {
	if item == nil {
		return ""
	}
	return strings.TrimSpace(interfaceString(item[key]))
}

func runtimeStateFromProtocol(protocol string) string {
	if strings.EqualFold(strings.TrimSpace(protocol), "stub") {
		return "stub"
	}
	return "real"
}

func capabilityRuntimeMetadataWithFallback(manifest connectors.Manifest, runtimeName string, fallbackEnabled bool, fallbackUsed bool, fallbackReason string, fallbackTarget string) *contracts.RuntimeMetadata {
	return &contracts.RuntimeMetadata{
		Runtime:         runtimeName,
		Selection:       "explicit_connector",
		ConnectorID:     strings.TrimSpace(manifest.Metadata.ID),
		ConnectorType:   strings.TrimSpace(manifest.Spec.Type),
		ConnectorVendor: strings.TrimSpace(manifest.Metadata.Vendor),
		Protocol:        strings.TrimSpace(manifest.Spec.Protocol),
		ExecutionMode:   connectors.DefaultExecutionMode(manifest.Spec.Protocol),
		FallbackEnabled: fallbackEnabled,
		FallbackUsed:    fallbackUsed,
		FallbackReason:  strings.TrimSpace(fallbackReason),
		FallbackTarget:  strings.TrimSpace(fallbackTarget),
	}
}

func cloneCapabilityParams(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func interfaceString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}

func intFromAny(value interface{}, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		parsed, err := strconv.Atoi(strings.TrimSpace(typed))
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func clampInt(value int, minValue int, maxValue int) int {
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func capabilityFirstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}

func (r *DeliveryRuntime) queryRemoteGit(ctx context.Context, manifest connectors.Manifest, capabilityID string, repoURL string, params map[string]interface{}) (contracts.CapabilityResult, error) {
	owner, repo, err := parseGitHubRepository(repoURL)
	if err != nil {
		return contracts.CapabilityResult{
			Status:  "failed",
			Error:   err.Error(),
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", true, false, "", ""),
		}, err
	}
	limit := clampInt(intFromAny(params["limit"], 8), 1, 20)
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits?per_page=%d", owner, repo, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return contracts.CapabilityResult{Status: "failed", Error: err.Error(), Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", true, false, "", "")}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := r.client.Do(req)
	if err != nil {
		return contracts.CapabilityResult{Status: "failed", Error: err.Error(), Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", true, false, "", "")}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return contracts.CapabilityResult{Status: "failed", Error: err.Error(), Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", true, false, "", "")}, err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		message := strings.TrimSpace(string(body))
		if message == "" {
			message = fmt.Sprintf("github commits query failed with status %d", resp.StatusCode)
		}
		return contracts.CapabilityResult{Status: "failed", Error: message, Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", true, false, "", "")}, fmt.Errorf("%s", message)
	}
	var payload []map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return contracts.CapabilityResult{Status: "failed", Error: err.Error(), Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", true, false, "", "")}, err
	}
	commits := make([]map[string]interface{}, 0, len(payload))
	filter := strings.ToLower(strings.TrimSpace(capabilityFirstNonEmpty(interfaceString(params["service"]), interfaceString(params["query"]))))
	for _, item := range payload {
		commitNode, _ := item["commit"].(map[string]interface{})
		message := ""
		author := ""
		committedAt := ""
		if commitNode != nil {
			message = interfaceString(commitNode["message"])
			if authorNode, ok := commitNode["author"].(map[string]interface{}); ok {
				author = interfaceString(authorNode["name"])
				committedAt = interfaceString(authorNode["date"])
			}
		}
		if filter != "" && !strings.Contains(strings.ToLower(message), filter) {
			continue
		}
		commits = append(commits, map[string]interface{}{
			"sha":          interfaceString(item["sha"]),
			"author":       author,
			"committed_at": committedAt,
			"subject":      capabilityFirstLine(strings.TrimSpace(message), 160),
			"html_url":     interfaceString(item["html_url"]),
		})
	}
	usedFallbackCommits := false
	if len(commits) == 0 && filter != "" {
		usedFallbackCommits = true
		for _, item := range payload {
			commitNode, _ := item["commit"].(map[string]interface{})
			message := ""
			author := ""
			committedAt := ""
			if commitNode != nil {
				message = interfaceString(commitNode["message"])
				if authorNode, ok := commitNode["author"].(map[string]interface{}); ok {
					author = interfaceString(authorNode["name"])
					committedAt = interfaceString(authorNode["date"])
				}
			}
			commits = append(commits, map[string]interface{}{
				"sha":          interfaceString(item["sha"]),
				"author":       author,
				"committed_at": committedAt,
				"subject":      capabilityFirstLine(strings.TrimSpace(message), 160),
				"html_url":     interfaceString(item["html_url"]),
			})
			if len(commits) >= limit {
				break
			}
		}
	}
	summary := fmt.Sprintf("delivery facts from github repo %s/%s", owner, repo)
	if usedFallbackCommits && len(commits) > 0 {
		summary = fmt.Sprintf("delivery facts from github repo %s/%s (fallback to latest commits after no exact filter match for %q)", owner, repo, filter)
	}
	return contracts.CapabilityResult{
		Status: "completed",
		Output: map[string]interface{}{
			"source":          "delivery_github",
			"capability_id":   capabilityFirstNonEmpty(capabilityID, "delivery.query"),
			"connector_id":    strings.TrimSpace(manifest.Metadata.ID),
			"repo_url":        repoURL,
			"query_params":    cloneCapabilityParams(params),
			"result_count":    len(commits),
			"summary":         summary,
			"branch":          strings.TrimSpace(manifest.Config.Values["branch"]),
			"commits":         commits,
			"runtime_state":   runtimeStateFromProtocol(manifest.Spec.Protocol),
			"capability_kind": "delivery",
			"queried_at":      time.Now().UTC().Format(time.RFC3339),
		},
		Artifacts: buildCapabilityArtifacts("delivery", commits, summary, map[string]interface{}{
			"connector_id":  strings.TrimSpace(manifest.Metadata.ID),
			"capability_id": capabilityFirstNonEmpty(capabilityID, "delivery.query"),
			"repo_url":      repoURL,
			"owner":         owner,
			"repo":          repo,
			"query_params":  cloneCapabilityParams(params),
		}),
		Metadata: map[string]interface{}{
			"repo_url": repoURL,
			"owner":    owner,
			"repo":     repo,
		},
		Runtime: capabilityRuntimeMetadataWithFallback(manifest, "connector_capability", true, false, "", "stub"),
	}, nil
}

func buildCapabilityArtifacts(prefix string, payload interface{}, summary string, metadata map[string]interface{}) []contracts.MessageAttachment {
	content, err := json.MarshalIndent(map[string]interface{}{
		"summary":  strings.TrimSpace(summary),
		"payload":  payload,
		"metadata": metadata,
	}, "", "  ")
	if err != nil {
		return nil
	}
	name := strings.TrimSpace(prefix)
	if name == "" {
		name = "capability"
	}
	preview := strings.TrimSpace(summary)
	if preview == "" {
		preview = fmt.Sprintf("%s capability result", name)
	}
	return []contracts.MessageAttachment{{
		Type:        "file",
		Name:        fmt.Sprintf("%s-result.json", name),
		MimeType:    "application/json",
		Content:     string(content),
		PreviewText: preview,
		Metadata: map[string]interface{}{
			"source": name,
		},
	}}
}

func parseGitHubRepository(repoURL string) (string, string, error) {
	trimmed := strings.TrimSpace(strings.TrimSuffix(repoURL, ".git"))
	trimmed = strings.TrimPrefix(trimmed, "git@github.com:")
	trimmed = strings.TrimPrefix(trimmed, "https://github.com/")
	trimmed = strings.TrimPrefix(trimmed, "http://github.com/")
	parts := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(parts) < 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return "", "", fmt.Errorf("unsupported repo_url %q: only github repos are supported for remote delivery mode", repoURL)
	}
	return parts[0], parts[1], nil
}

func capabilityFirstLine(value string, maxLen int) string {
	line := strings.TrimSpace(strings.Split(value, "\n")[0])
	if maxLen > 0 && len(line) > maxLen {
		return line[:maxLen-3] + "..."
	}
	return line
}

type MCPStubRuntime struct{}

func NewMCPStubRuntime() *MCPStubRuntime {
	return &MCPStubRuntime{}
}

// MetricsCapabilityRuntime implements CapabilityRuntime for metrics connectors
// (victoriametrics_http, prometheus_http). It wraps MetricsConnectorRuntime so
// that connector_capability automation jobs can invoke metrics queries.
type MetricsCapabilityRuntime struct {
	inner *MetricsConnectorRuntime
}

func NewMetricsCapabilityRuntime(cfg VictoriaMetricsConfig) *MetricsCapabilityRuntime {
	return &MetricsCapabilityRuntime{inner: NewMetricsConnectorRuntime(cfg)}
}

func (r *MetricsCapabilityRuntime) Invoke(ctx context.Context, manifest connectors.Manifest, capabilityID string, params map[string]interface{}) (contracts.CapabilityResult, error) {
	capID := strings.ToLower(strings.TrimSpace(capabilityID))
	mode := "instant"
	if capID == "query.range" {
		mode = "range"
	}
	query := contracts.MetricsQuery{
		Query:   interfaceString(params["query"]),
		Service: interfaceString(params["service"]),
		Host:    interfaceString(params["host"]),
		Mode:    mode,
		Step:    interfaceString(params["step"]),
		Window:  interfaceString(params["window"]),
	}
	result, err := r.inner.Query(ctx, manifest, query)
	if err != nil {
		return contracts.CapabilityResult{
			Status:  "failed",
			Error:   err.Error(),
			Runtime: capabilityRuntimeMetadataWithFallback(manifest, "metrics_connector", false, false, "", ""),
		}, err
	}
	summary := fmt.Sprintf("metrics query returned %d series", len(result.Series))
	return contracts.CapabilityResult{
		Status: "completed",
		Output: map[string]interface{}{
			"source":          capabilityFirstNonEmpty(strings.TrimSpace(manifest.Spec.Protocol), "metrics"),
			"capability_id":   capabilityFirstNonEmpty(capabilityID, "query.instant"),
			"connector_id":    strings.TrimSpace(manifest.Metadata.ID),
			"query_params":    cloneCapabilityParams(params),
			"result_count":    len(result.Series),
			"summary":         summary,
			"series":          result.Series,
			"queried_at":      time.Now().UTC().Format(time.RFC3339),
			"runtime_state":   runtimeStateFromProtocol(manifest.Spec.Protocol),
			"capability_kind": "metrics",
		},
		Metadata: map[string]interface{}{
			"series_count": len(result.Series),
			"mode":         mode,
		},
		Runtime: capabilityRuntimeMetadataWithFallback(manifest, "metrics_connector", true, false, "", ""),
	}, nil
}

func (r *MCPStubRuntime) Invoke(ctx context.Context, manifest connectors.Manifest, capabilityID string, params map[string]interface{}) (contracts.CapabilityResult, error) {
	now := time.Now().UTC()
	return contracts.CapabilityResult{
		Status: "completed",
		Output: map[string]interface{}{
			"source":        "mcp_stub",
			"capability_id": capabilityID,
			"connector_id":  manifest.Metadata.ID,
			"params":        params,
			"note":          fmt.Sprintf("stub MCP runtime: capability %q executed as passthrough", capabilityID),
			"invoked_at":    now.Format(time.RFC3339),
		},
		Runtime: capabilityRuntimeMetadataWithFallback(manifest, "mcp_stub", false, false, "", ""),
	}, nil
}

type SkillStubRuntime struct{}

func NewSkillStubRuntime() *SkillStubRuntime {
	return &SkillStubRuntime{}
}

func (r *SkillStubRuntime) Invoke(ctx context.Context, manifest connectors.Manifest, capabilityID string, params map[string]interface{}) (contracts.CapabilityResult, error) {
	now := time.Now().UTC()
	return contracts.CapabilityResult{
		Status: "completed",
		Output: map[string]interface{}{
			"source":        "skill_stub",
			"capability_id": capabilityID,
			"connector_id":  manifest.Metadata.ID,
			"params":        params,
			"note":          fmt.Sprintf("stub skill runtime: capability %q executed as passthrough", capabilityID),
			"invoked_at":    now.Format(time.RFC3339),
		},
		Runtime: capabilityRuntimeMetadataWithFallback(manifest, "skill_stub", false, false, "", ""),
	}, nil
}
