package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"tars/internal/modules/connectors"
)

type staticStringer string

func (s staticStringer) String() string {
	return string(s)
}

func TestExtractObservabilityResultsAcrossPayloadShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload map[string]interface{}
		want    []interface{}
	}{
		{
			name: "data alerts",
			payload: map[string]interface{}{
				"data": map[string]interface{}{
					"alerts": []interface{}{
						map[string]interface{}{"name": "DiskWillFillSoon"},
					},
				},
			},
			want: []interface{}{
				map[string]interface{}{"name": "DiskWillFillSoon"},
			},
		},
		{
			name: "data groups",
			payload: map[string]interface{}{
				"data": map[string]interface{}{
					"groups": []interface{}{
						map[string]interface{}{"name": "api.rules"},
						map[string]interface{}{"name": "worker.rules"},
					},
				},
			},
			want: []interface{}{
				map[string]interface{}{"name": "api.rules"},
				map[string]interface{}{"name": "worker.rules"},
			},
		},
		{
			name: "top level logs",
			payload: map[string]interface{}{
				"logs": []interface{}{
					map[string]interface{}{"message": "request failed"},
				},
			},
			want: []interface{}{
				map[string]interface{}{"message": "request failed"},
			},
		},
		{
			name: "nested entries",
			payload: map[string]interface{}{
				"entries": map[string]interface{}{
					"entries": []interface{}{
						map[string]interface{}{"message": "entry one"},
					},
				},
			},
			want: []interface{}{
				map[string]interface{}{"message": "entry one"},
			},
		},
		{
			name: "falls back to original payload",
			payload: map[string]interface{}{
				"message": "no structured results",
			},
			want: []interface{}{
				map[string]interface{}{"message": "no structured results"},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			results := extractObservabilityResults(tc.payload)
			if !reflect.DeepEqual(results, tc.want) {
				t.Fatalf("expected results %#v, got %#v", tc.want, results)
			}
		})
	}
}

func TestSummarizeObservabilityResultsBranches(t *testing.T) {
	t.Parallel()

	t.Run("alerts include firing count", func(t *testing.T) {
		payload := map[string]interface{}{
			"data": map[string]interface{}{
				"alerts": []interface{}{
					map[string]interface{}{"state": "firing"},
					map[string]interface{}{"state": "inactive"},
					map[string]interface{}{"state": ""},
				},
			},
		}

		got := summarizeObservabilityResults(nil, payload)
		if got != "returned 3 alert(s), 2 firing" {
			t.Fatalf("unexpected alert summary %q", got)
		}
	})

	t.Run("groups report rule group count", func(t *testing.T) {
		payload := map[string]interface{}{
			"data": map[string]interface{}{
				"groups": []interface{}{
					map[string]interface{}{"name": "api"},
					map[string]interface{}{"name": "worker"},
				},
			},
		}

		got := summarizeObservabilityResults(nil, payload)
		if got != "returned 2 rule group(s)" {
			t.Fatalf("unexpected group summary %q", got)
		}
	})

	t.Run("payload level message wins", func(t *testing.T) {
		got := summarizeObservabilityResults(nil, map[string]interface{}{"message": "connector note"})
		if got != "connector note" {
			t.Fatalf("unexpected payload message summary %q", got)
		}
	})

	t.Run("first result supplies summary fields", func(t *testing.T) {
		results := []interface{}{
			map[string]interface{}{"event": "pod restarted"},
		}

		got := summarizeObservabilityResults(results, nil)
		if got != "pod restarted" {
			t.Fatalf("unexpected result summary %q", got)
		}
	})

	t.Run("empty results return no results summary", func(t *testing.T) {
		got := summarizeObservabilityResults(nil, nil)
		if got != "no observability results returned" {
			t.Fatalf("unexpected empty summary %q", got)
		}
	})
}

func TestParseObservabilityPayloadBranches(t *testing.T) {
	t.Parallel()

	t.Run("invalid json becomes raw result", func(t *testing.T) {
		results, summary, count, metadata := parseObservabilityPayload([]byte("plain text response"))
		if count != 1 || len(results) != 1 {
			t.Fatalf("expected one raw result, got count=%d results=%#v", count, results)
		}
		rawItem, ok := results[0].(map[string]interface{})
		if !ok || rawItem["raw"] != "plain text response" {
			t.Fatalf("expected raw response item, got %#v", results[0])
		}
		if summary != "plain text response" {
			t.Fatalf("unexpected raw summary %q", summary)
		}
		if len(metadata) != 0 {
			t.Fatalf("expected empty metadata, got %#v", metadata)
		}
	})

	t.Run("arrays summarize by count", func(t *testing.T) {
		results, summary, count, _ := parseObservabilityPayload([]byte(`[{"message":"entry one"},{"message":"entry two"}]`))
		if count != 2 || len(results) != 2 {
			t.Fatalf("expected two array results, got count=%d results=%#v", count, results)
		}
		if summary != "entry one" {
			t.Fatalf("unexpected array summary %q", summary)
		}
	})

	t.Run("maps reuse structured summary branches", func(t *testing.T) {
		results, summary, count, metadata := parseObservabilityPayload([]byte(`{"data":{"groups":[{"name":"api.rules"}]}}`))
		if count != 1 || len(results) != 1 {
			t.Fatalf("expected one group result, got count=%d results=%#v", count, results)
		}
		if summary != "returned 1 rule group(s)" {
			t.Fatalf("unexpected map summary %q", summary)
		}
		if _, ok := metadata["data"]; !ok {
			t.Fatalf("expected original payload metadata, got %#v", metadata)
		}
	})
}

func TestBuildCapabilityArtifactsFallbacks(t *testing.T) {
	t.Parallel()

	artifacts := buildCapabilityArtifacts("", map[string]interface{}{"ok": true}, "", map[string]interface{}{"scope": "test"})
	if len(artifacts) != 1 {
		t.Fatalf("expected one artifact, got %#v", artifacts)
	}
	if artifacts[0].Name != "capability-result.json" {
		t.Fatalf("unexpected artifact name %q", artifacts[0].Name)
	}
	if artifacts[0].PreviewText != "capability capability result" {
		t.Fatalf("unexpected preview text %q", artifacts[0].PreviewText)
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(artifacts[0].Content), &payload); err != nil {
		t.Fatalf("unmarshal artifact content: %v", err)
	}
	if metadata, ok := payload["metadata"].(map[string]interface{}); !ok || metadata["scope"] != "test" {
		t.Fatalf("unexpected artifact metadata %#v", payload["metadata"])
	}

	if got := buildCapabilityArtifacts("delivery", map[string]interface{}{"bad": make(chan int)}, "summary", nil); got != nil {
		t.Fatalf("expected nil artifacts on marshal failure, got %#v", got)
	}
}

func TestResolveDeliveryRepoPathAndIntFromAny(t *testing.T) {
	t.Parallel()

	absDot, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("resolve dot: %v", err)
	}

	t.Run("defaults to current directory for local delivery", func(t *testing.T) {
		manifest := connectors.Manifest{}

		repoPath, err := resolveDeliveryRepoPath(manifest)
		if err != nil {
			t.Fatalf("resolve default repo path: %v", err)
		}
		if repoPath != absDot {
			t.Fatalf("expected default repo path %q, got %q", absDot, repoPath)
		}
	})

	t.Run("uses git_repo_path fallback", func(t *testing.T) {
		manifest := connectors.Manifest{}
		manifest.Config.Values = map[string]string{"git_repo_path": "."}

		repoPath, err := resolveDeliveryRepoPath(manifest)
		if err != nil {
			t.Fatalf("resolve git_repo_path: %v", err)
		}
		if repoPath != absDot {
			t.Fatalf("expected git_repo_path to resolve to %q, got %q", absDot, repoPath)
		}
	})

	t.Run("remote delivery requires explicit local repo path", func(t *testing.T) {
		manifest := connectors.Manifest{}
		manifest.Spec.Protocol = "delivery_http"

		_, err := resolveDeliveryRepoPath(manifest)
		if err == nil || !strings.Contains(err.Error(), "repo_path is not configured") {
			t.Fatalf("expected missing repo_path error, got %v", err)
		}
	})

	tests := []struct {
		name     string
		input    interface{}
		fallback int
		want     int
	}{
		{name: "int", input: 7, fallback: 1, want: 7},
		{name: "int64", input: int64(8), fallback: 1, want: 8},
		{name: "float64", input: 9.9, fallback: 1, want: 9},
		{name: "string", input: " 10 ", fallback: 1, want: 10},
		{name: "fallback", input: "bad", fallback: 11, want: 11},
	}

	for _, tc := range tests {
		tc := tc
		t.Run("intFromAny/"+tc.name, func(t *testing.T) {
			t.Parallel()

			if got := intFromAny(tc.input, tc.fallback); got != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, got)
			}
		})
	}
}

func TestGitHelpersAndQueryDeliveryGitFallback(t *testing.T) {
	t.Parallel()

	repoPath := initTestGitRepo(t)
	branch, err := gitSingleLine(context.Background(), repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("discover branch: %v", err)
	}
	remoteRoot := t.TempDir()
	remotePath := filepath.Join(remoteRoot, "origin.git")
	runGit(t, remoteRoot, "init", "--bare", remotePath)
	runGit(t, repoPath, "remote", "set-url", "origin", remotePath)
	runGit(t, repoPath, "push", "-u", "origin", branch)
	writeRepoFile(t, repoPath, "README.md", "hello\nupdated locally\n")
	runGit(t, repoPath, "add", "README.md")
	runGit(t, repoPath, "commit", "-m", "chore: local ahead commit")

	filteredResult, err := queryDeliveryGit(context.Background(), repoPath, map[string]interface{}{
		"service": "api",
		"limit":   "1",
	})
	if err != nil {
		t.Fatalf("query delivery git with matching filter: %v", err)
	}
	if got := filteredResult.Commits[0]["subject"]; got != "fix api timeout regression" {
		t.Fatalf("expected filtered path to return api commit, got %#v", got)
	}

	result, err := queryDeliveryGit(context.Background(), repoPath, map[string]interface{}{
		"service": "missing-service",
		"limit":   "1",
	})
	if err != nil {
		t.Fatalf("query delivery git fallback: %v", err)
	}
	if result.Filter != "missing-service" {
		t.Fatalf("expected normalized filter, got %q", result.Filter)
	}
	if len(result.Commits) != 1 {
		t.Fatalf("expected fallback commit result, got %#v", result.Commits)
	}
	if got := result.Commits[0]["subject"]; got != "chore: local ahead commit" {
		t.Fatalf("expected fallback to latest commit, got %#v", got)
	}
	if !strings.Contains(result.Summary, `filtered by "missing-service"`) {
		t.Fatalf("expected filtered summary to survive fallback, got %q", result.Summary)
	}
	if dirty, ok := result.WorkingTree["dirty"].(bool); !ok || !dirty {
		t.Fatalf("expected dirty working tree, got %#v", result.WorkingTree)
	}
	if result.RemoteURL != remotePath {
		t.Fatalf("unexpected remote url %q", result.RemoteURL)
	}

	output, err := gitSingleLine(context.Background(), repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		t.Fatalf("git single line: %v", err)
	}
	if strings.Contains(output, "\n") || strings.TrimSpace(output) == "" {
		t.Fatalf("expected trimmed branch name, got %q", output)
	}

	_, err = gitMultiLine(context.Background(), repoPath, "not-a-real-git-command")
	if err == nil || !strings.Contains(err.Error(), "git not-a-real-git-command failed") {
		t.Fatalf("expected git command failure, got %v", err)
	}
}

func TestObservabilityHTTPRuntimeInvokeBranches(t *testing.T) {
	t.Parallel()

	t.Run("reports unsupported capability", func(t *testing.T) {
		runtime := NewObservabilityHTTPRuntime(nil)

		result, err := runtime.Invoke(context.Background(), connectors.Manifest{}, "observability.delete", nil)
		if err == nil {
			t.Fatalf("expected unsupported capability error")
		}
		if result.Status != "failed" || !strings.Contains(result.Error, "unsupported observability capability") {
			t.Fatalf("unexpected result %+v", result)
		}
	})

	t.Run("fails when base url is missing", func(t *testing.T) {
		runtime := NewObservabilityHTTPRuntime(nil)

		result, err := runtime.Invoke(context.Background(), connectors.Manifest{}, "observability.query", nil)
		if err == nil {
			t.Fatalf("expected missing base_url error")
		}
		if result.Status != "failed" || !strings.Contains(result.Error, "base_url is not configured") {
			t.Fatalf("unexpected result %+v", result)
		}
	})

	t.Run("uses custom query params and preserves raw text results", func(t *testing.T) {
		var capturedURL *url.URL
		var capturedAuth string
		runtime := NewObservabilityHTTPRuntime(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				capturedURL = req.URL
				capturedAuth = req.Header.Get("Authorization")
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(strings.NewReader("api backend timeout")),
				}, nil
			}),
		})

		manifest := connectors.Manifest{}
		manifest.Metadata.ID = "obs-http"
		manifest.Spec.Protocol = "observability_http"
		manifest.Config.Values = map[string]string{
			"base_url":         "https://observability.example.test/base",
			"query_path":       "custom/query",
			"query_param_name": "expr",
			"bearer_token":     "secret-token",
		}

		result, err := runtime.Invoke(context.Background(), manifest, "observability.query", map[string]interface{}{
			"query":    staticStringer("api backend"),
			"service":  "payments",
			"host":     "node-1",
			"severity": "critical",
			"trace_id": "trace-123",
			"limit":    999,
		})
		if err != nil {
			t.Fatalf("invoke raw observability response: %v", err)
		}
		if capturedAuth != "Bearer secret-token" {
			t.Fatalf("unexpected auth header %q", capturedAuth)
		}
		if capturedURL.Path != "/base/custom/query" {
			t.Fatalf("unexpected request path %q", capturedURL.Path)
		}
		if capturedURL.Query().Get("expr") != "api backend" {
			t.Fatalf("expected custom query param, got %q", capturedURL.RawQuery)
		}
		if capturedURL.Query().Get("service") != "payments" || capturedURL.Query().Get("limit") != "200" {
			t.Fatalf("unexpected encoded query params %q", capturedURL.RawQuery)
		}
		if result.Output["summary"] != "api backend timeout" {
			t.Fatalf("unexpected summary %#v", result.Output["summary"])
		}
		results, ok := result.Output["results"].([]interface{})
		if !ok || len(results) != 1 {
			t.Fatalf("expected one raw result, got %#v", result.Output["results"])
		}
		rawItem, ok := results[0].(map[string]interface{})
		if !ok || rawItem["raw"] != "api backend timeout" {
			t.Fatalf("expected raw text result, got %#v", results[0])
		}
		if result.Metadata["http_status"] != http.StatusOK {
			t.Fatalf("expected http status metadata, got %#v", result.Metadata)
		}
	})

	t.Run("uses fallback error message for empty http body", func(t *testing.T) {
		runtime := NewObservabilityHTTPRuntime(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Header:     make(http.Header),
					Body:       http.NoBody,
				}, nil
			}),
		})

		manifest := connectors.Manifest{}
		manifest.Spec.Protocol = "observability_http"
		manifest.Config.Values = map[string]string{"base_url": "https://observability.example.test"}

		result, err := runtime.Invoke(context.Background(), manifest, "observability.query", nil)
		if err == nil {
			t.Fatalf("expected observability http error")
		}
		if result.Error != "observability query failed with status 502" {
			t.Fatalf("unexpected error %#v", result.Error)
		}
		if result.Metadata["http_status"] != http.StatusBadGateway {
			t.Fatalf("expected http status metadata, got %#v", result.Metadata)
		}
	})
}

func TestDeliveryRuntimeCheckHealthAndRemoteGitBranches(t *testing.T) {
	t.Parallel()

	t.Run("remote health handles probe transport error", func(t *testing.T) {
		runtime := NewDeliveryRuntime(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, io.ErrUnexpectedEOF
			}),
		})

		manifest := connectors.Manifest{}
		manifest.Spec.Protocol = "delivery_github"
		manifest.Config.Values = map[string]string{"repo_url": "https://github.com/evilgaoshu/TARS.git"}

		status, summary, err := runtime.CheckHealth(context.Background(), manifest)
		if err == nil {
			t.Fatalf("expected probe transport error")
		}
		if status != "unhealthy" || !strings.Contains(summary, "unexpected EOF") {
			t.Fatalf("unexpected health result %q %q", status, summary)
		}
	})

	t.Run("remote health reports probe status failures", func(t *testing.T) {
		runtime := NewDeliveryRuntime(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadGateway,
					Header:     make(http.Header),
					Body:       http.NoBody,
				}, nil
			}),
		})

		manifest := connectors.Manifest{}
		manifest.Spec.Protocol = "delivery_github"
		manifest.Config.Values = map[string]string{"repo_url": "https://github.com/evilgaoshu/TARS.git"}

		status, summary, err := runtime.CheckHealth(context.Background(), manifest)
		if err == nil {
			t.Fatalf("expected probe status error")
		}
		if status != "unhealthy" || summary != "github repo probe failed with status 502" {
			t.Fatalf("unexpected health result %q %q", status, summary)
		}
	})

	t.Run("remote health keeps local resolution error when repo url is unsupported", func(t *testing.T) {
		runtime := NewDeliveryRuntime(nil)

		manifest := connectors.Manifest{}
		manifest.Spec.Protocol = "delivery_github"
		manifest.Config.Values = map[string]string{"repo_url": "https://gitlab.com/example/project.git"}

		status, summary, err := runtime.CheckHealth(context.Background(), manifest)
		if err == nil {
			t.Fatalf("expected unsupported repo health error")
		}
		if status != "unhealthy" || !strings.Contains(summary, "repo_path is not configured for local delivery mode") {
			t.Fatalf("unexpected health result %q %q", status, summary)
		}
	})

	t.Run("remote git trims multiline commit subjects and reports empty-body failures", func(t *testing.T) {
		longFirstLine := strings.Repeat("a", 170) + "\\nwith body text"
		runtime := NewDeliveryRuntime(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.URL.Query().Get("per_page") == "1" {
					return &http.Response{
						StatusCode: http.StatusOK,
						Header:     make(http.Header),
						Body: io.NopCloser(strings.NewReader(`[
							{
								"sha":"abc123",
								"html_url":"https://github.com/evilgaoshu/TARS/commit/abc123",
								"commit":{"message":"` + longFirstLine + `","author":{"name":"dev","date":"2026-03-20T12:00:00Z"}}
							}
						]`)),
					}, nil
				}
				return &http.Response{
					StatusCode: http.StatusInternalServerError,
					Header:     make(http.Header),
					Body:       http.NoBody,
				}, nil
			}),
		})

		manifest := connectors.Manifest{}
		manifest.Metadata.ID = "delivery-remote"
		manifest.Spec.Protocol = "delivery_github"

		result, err := runtime.queryRemoteGit(context.Background(), manifest, "delivery.query", "https://github.com/evilgaoshu/TARS.git", map[string]interface{}{"limit": 1})
		if err != nil {
			t.Fatalf("query remote git: %v", err)
		}
		commits, ok := result.Output["commits"].([]map[string]interface{})
		if !ok || len(commits) != 1 {
			t.Fatalf("expected one remote commit, got %#v", result.Output["commits"])
		}
		subject, _ := commits[0]["subject"].(string)
		if strings.Contains(subject, "\n") || len(subject) != 160 || !strings.HasSuffix(subject, "...") {
			t.Fatalf("expected first-line truncated subject, got %q", subject)
		}

		failedResult, err := runtime.queryRemoteGit(context.Background(), manifest, "delivery.query", "https://github.com/evilgaoshu/TARS.git", map[string]interface{}{"query": "api", "limit": 2})
		if err == nil {
			t.Fatalf("expected empty-body github error")
		}
		if failedResult.Error != "github commits query failed with status 500" {
			t.Fatalf("unexpected remote error %#v", failedResult.Error)
		}
	})
}

func TestCapabilityHelpersAndFilteringBranches(t *testing.T) {
	t.Parallel()

	t.Run("buildObservabilityQueryURL validates and encodes params", func(t *testing.T) {
		manifest := connectors.Manifest{}
		manifest.Config.Values = map[string]string{
			"alerts_path":      "/api/v1/alerts",
			"query_param_name": "expr",
		}

		built, err := buildObservabilityQueryURL(manifest, "://bad-url", nil)
		if err == nil || !strings.Contains(err.Error(), "parse observability base_url") {
			t.Fatalf("expected base_url parse error, got %v", err)
		}

		built, err = buildObservabilityQueryURL(manifest, "https://obs.example.test/root", map[string]interface{}{
			"mode":  "alerts",
			"query": staticStringer("rate(http_requests_total[5m])"),
			"limit": 0,
		})
		if err != nil {
			t.Fatalf("build query url: %v", err)
		}
		parsed, err := url.Parse(built)
		if err != nil {
			t.Fatalf("parse built url: %v", err)
		}
		if parsed.Path != "/root/api/v1/alerts" || parsed.Query().Get("expr") != "rate(http_requests_total[5m])" {
			t.Fatalf("unexpected query url %q", built)
		}
		if parsed.Query().Get("limit") != "1" {
			t.Fatalf("expected clamped limit, got %q", parsed.RawQuery)
		}
	})

	t.Run("filterObservabilityResults preserves marshal failures and no-match fallback", func(t *testing.T) {
		results := []interface{}{
			map[string]interface{}{"message": "api timeout"},
			map[string]interface{}{"message": "worker timeout"},
			map[string]interface{}{"bad": make(chan int)},
		}

		filtered := filterObservabilityResults(results, map[string]interface{}{"service": "api"})
		if len(filtered) != 2 {
			t.Fatalf("expected api match plus marshal failure fallback, got %#v", filtered)
		}
		if message := filtered[0].(map[string]interface{})["message"]; message != "api timeout" {
			t.Fatalf("unexpected filtered message %#v", message)
		}

		fallback := filterObservabilityResults(results[:2], map[string]interface{}{"service": "missing"})
		if !reflect.DeepEqual(fallback, results[:2]) {
			t.Fatalf("expected original results on no match, got %#v", fallback)
		}
	})

	t.Run("helper functions cover nil and boundary cases", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "tail.log")
		if err := os.WriteFile(filePath, []byte("0123456789"), 0o644); err != nil {
			t.Fatalf("write helper tail file: %v", err)
		}
		tailed, err := tailFile(filePath, 32)
		if err != nil {
			t.Fatalf("tail full file: %v", err)
		}
		if tailed != "0123456789" {
			t.Fatalf("expected full file tail, got %q", tailed)
		}
		if got := stringMapValue(nil, "key"); got != "" {
			t.Fatalf("expected empty stringMapValue for nil map, got %q", got)
		}
		if got := stringMapValue(map[string]interface{}{"key": staticStringer("value")}, "key"); got != "value" {
			t.Fatalf("unexpected stringMapValue %q", got)
		}
		if got := runtimeStateFromProtocol("stub"); got != "stub" {
			t.Fatalf("expected stub runtime state, got %q", got)
		}
		if got := runtimeStateFromProtocol("observability_http"); got != "real" {
			t.Fatalf("expected real runtime state, got %q", got)
		}
		if got := interfaceString(staticStringer("stringer")); got != "stringer" {
			t.Fatalf("unexpected interfaceString %q", got)
		}
		if got := capabilityFirstNonEmpty("", " ", "value"); got != "value" {
			t.Fatalf("unexpected capabilityFirstNonEmpty %q", got)
		}
		if got := capabilityFirstNonEmpty("", " "); got != "" {
			t.Fatalf("expected empty capabilityFirstNonEmpty, got %q", got)
		}
		if got := clampInt(0, 1, 5); got != 1 {
			t.Fatalf("expected low clamp, got %d", got)
		}
		if got := clampInt(8, 1, 5); got != 5 {
			t.Fatalf("expected high clamp, got %d", got)
		}
		if got, ok := firstMap(nil); ok || got != nil {
			t.Fatalf("expected empty firstMap result, got %#v %v", got, ok)
		}
	})
}

func TestDeliveryRuntimeInvokeFailureBranches(t *testing.T) {
	t.Parallel()

	runtime := NewDeliveryRuntime(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`[
					{
						"sha":"abc123",
						"html_url":"https://github.com/evilgaoshu/TARS/commit/abc123",
						"commit":{"message":"release api","author":{"name":"dev","date":"2026-03-20T12:00:00Z"}}
					}
				]`)),
			}, nil
		}),
	})

	t.Run("rejects unsupported capability", func(t *testing.T) {
		result, err := runtime.Invoke(context.Background(), connectors.Manifest{}, "delivery.delete", nil)
		if err == nil {
			t.Fatalf("expected unsupported capability error")
		}
		if result.Status != "failed" || !strings.Contains(result.Error, "unsupported delivery capability") {
			t.Fatalf("unexpected result %+v", result)
		}
	})

	t.Run("fails when remote mode has no repo path and no repo url", func(t *testing.T) {
		manifest := connectors.Manifest{}
		manifest.Spec.Protocol = "delivery_http"

		result, err := runtime.Invoke(context.Background(), manifest, "delivery.query", nil)
		if err == nil {
			t.Fatalf("expected missing repo path error")
		}
		if result.Status != "failed" || !strings.Contains(result.Error, "repo_path is not configured") {
			t.Fatalf("unexpected result %+v", result)
		}
	})

	t.Run("returns failed result when local path is not a git repo", func(t *testing.T) {
		manifest := connectors.Manifest{}
		manifest.Config.Values = map[string]string{"repo_path": t.TempDir()}

		result, err := runtime.Invoke(context.Background(), manifest, "delivery.query", map[string]interface{}{"service": "api"})
		if err == nil {
			t.Fatalf("expected local git failure")
		}
		if result.Status != "failed" {
			t.Fatalf("expected failed result, got %+v", result)
		}
		if _, ok := result.Metadata["repo_path"]; !ok {
			t.Fatalf("expected repo_path metadata, got %#v", result.Metadata)
		}
	})
}
