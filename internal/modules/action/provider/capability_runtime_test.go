package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"tars/internal/modules/connectors"
)

func TestObservabilityLogFileRuntimeAndHealth(t *testing.T) {
	t.Parallel()

	logDir := t.TempDir()
	logPath := filepath.Join(logDir, "app.log")
	content := strings.Join([]string{
		`{"level":"INFO","service":"api","message":"startup complete"}`,
		`{"level":"ERROR","service":"worker","message":"worker failed"}`,
		`{"level":"ERROR","service":"api","message":"database timeout"}`,
	}, "\n")
	if err := os.WriteFile(logPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write log file: %v", err)
	}

	manifest := connectors.Manifest{}
	manifest.Metadata.ID = "obs-log"
	manifest.Spec.Protocol = "log_file"
	manifest.Config.Values = map[string]string{"file_path": logPath}

	runtime := NewObservabilityHTTPRuntime(nil)
	result, err := runtime.Invoke(context.Background(), manifest, "observability.query", map[string]interface{}{
		"service":    "api",
		"severity":   "error",
		"limit":      1,
		"tail_bytes": 32,
	})
	if err != nil {
		t.Fatalf("invoke log runtime: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed result, got %+v", result)
	}
	if got := result.Output["source"]; got != "observability_log_file" {
		t.Fatalf("expected log source, got %#v", got)
	}
	if got := result.Output["result_count"]; got != 1 {
		t.Fatalf("expected one filtered log line, got %#v", got)
	}
	if summary, _ := result.Output["summary"].(string); !strings.Contains(summary, "api error") {
		t.Fatalf("expected filtered summary, got %#v", result.Output["summary"])
	}
	if metadataPath := result.Metadata["log_path"]; metadataPath != logPath {
		t.Fatalf("expected metadata log path %q, got %#v", logPath, metadataPath)
	}
	status, summary, err := runtime.CheckHealth(context.Background(), manifest)
	if err != nil {
		t.Fatalf("check health: %v", err)
	}
	if status != "healthy" || !strings.Contains(summary, logPath) {
		t.Fatalf("unexpected health result: %q %q", status, summary)
	}
}

func TestObservabilityCheckHealthDegradedBranches(t *testing.T) {
	t.Parallel()

	runtime := NewObservabilityHTTPRuntime(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadGateway,
				Header:     make(http.Header),
				Body:       http.NoBody,
			}, nil
		}),
	})

	t.Run("missing log file path", func(t *testing.T) {
		manifest := connectors.Manifest{}
		manifest.Spec.Protocol = "log_file"
		status, summary, err := runtime.CheckHealth(context.Background(), manifest)
		if err == nil {
			t.Fatalf("expected missing path error")
		}
		if status != "unhealthy" || !strings.Contains(summary, "file_path is not configured") {
			t.Fatalf("unexpected result: %q %q", status, summary)
		}
	})

	t.Run("missing local file", func(t *testing.T) {
		manifest := connectors.Manifest{}
		manifest.Spec.Protocol = "log_file"
		manifest.Config.Values = map[string]string{"file_path": filepath.Join(t.TempDir(), "missing.log")}
		status, summary, err := runtime.CheckHealth(context.Background(), manifest)
		if err == nil {
			t.Fatalf("expected stat error")
		}
		if status != "unhealthy" || !strings.Contains(summary, "no such file") {
			t.Fatalf("unexpected result: %q %q", status, summary)
		}
	})

	t.Run("http health status fallback message", func(t *testing.T) {
		manifest := connectors.Manifest{}
		manifest.Config.Values = map[string]string{"base_url": "https://observability.example.test"}
		status, summary, err := runtime.CheckHealth(context.Background(), manifest)
		if err == nil {
			t.Fatalf("expected http health error")
		}
		if status != "unhealthy" || summary != "observability health check failed with status 502" {
			t.Fatalf("unexpected result: %q %q", status, summary)
		}
	})
}

func TestTailFileFirstMapAndParseGitHubRepository(t *testing.T) {
	t.Parallel()

	filePath := filepath.Join(t.TempDir(), "tail.log")
	if err := os.WriteFile(filePath, []byte("0123456789"), 0o644); err != nil {
		t.Fatalf("write tail file: %v", err)
	}

	tailed, err := tailFile(filePath, 4)
	if err != nil {
		t.Fatalf("tail file: %v", err)
	}
	if tailed != "6789" {
		t.Fatalf("expected tailed suffix, got %q", tailed)
	}

	if got, ok := firstMap([]interface{}{map[string]interface{}{"summary": "hello"}}); !ok || got["summary"] != "hello" {
		t.Fatalf("expected first map, got %+v %v", got, ok)
	}
	if got, ok := firstMap([]interface{}{"not-a-map"}); ok || got != nil {
		t.Fatalf("expected missing first map, got %+v %v", got, ok)
	}

	owner, repo, err := parseGitHubRepository("git@github.com:evilgaoshu/TARS.git")
	if err != nil {
		t.Fatalf("parse github repo: %v", err)
	}
	if owner != "evilgaoshu" || repo != "TARS" {
		t.Fatalf("unexpected owner/repo: %q/%q", owner, repo)
	}
	if _, _, err := parseGitHubRepository("https://gitlab.com/example/project.git"); err == nil {
		t.Fatalf("expected parse error for non-github repo")
	}
}

func TestDeliveryRuntimeLocalGitInvokeAndHealth(t *testing.T) {
	t.Parallel()

	repoPath := initTestGitRepo(t)

	runtime := NewDeliveryRuntime(nil)
	manifest := connectors.Manifest{}
	manifest.Metadata.ID = "delivery-local"
	manifest.Config.Values = map[string]string{"repo_path": repoPath}

	result, err := runtime.Invoke(context.Background(), manifest, "delivery.query", map[string]interface{}{
		"service": "api",
		"limit":   1,
	})
	if err != nil {
		t.Fatalf("invoke delivery git: %v", err)
	}
	if result.Status != "completed" {
		t.Fatalf("expected completed result, got %+v", result)
	}
	if got := result.Output["source"]; got != "delivery_git" {
		t.Fatalf("expected delivery_git source, got %#v", got)
	}
	if got := result.Output["result_count"]; got != 1 {
		t.Fatalf("expected one filtered commit, got %#v", got)
	}
	commits, ok := result.Output["commits"].([]map[string]interface{})
	if !ok || len(commits) != 1 {
		t.Fatalf("expected typed commit list, got %#v", result.Output["commits"])
	}
	if subject := commits[0]["subject"]; subject != "fix api timeout regression" {
		t.Fatalf("unexpected commit subject %#v", subject)
	}
	status, summary, err := runtime.CheckHealth(context.Background(), manifest)
	if err != nil {
		t.Fatalf("check local git health: %v", err)
	}
	if status != "healthy" || !strings.Contains(summary, repoPath) {
		t.Fatalf("unexpected health result: %q %q", status, summary)
	}

	notRepo := t.TempDir()
	manifest.Config.Values = map[string]string{"repo_path": notRepo}
	status, summary, err = runtime.CheckHealth(context.Background(), manifest)
	if err == nil {
		t.Fatalf("expected unhealthy health for non-git dir")
	}
	if status != "unhealthy" || !strings.Contains(summary, "not a git repository") {
		t.Fatalf("unexpected non-git health result: %q %q", status, summary)
	}
}

func TestDeliveryRuntimeCheckHealthRemoteProbe(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/evilgaoshu/TARS" {
			t.Fatalf("unexpected probe path %q", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":1}`))
	}))
	defer server.Close()

	client := server.Client()
	client.Transport = rewriteGitHubHostTransport{base: client.Transport, target: server.URL}
	runtime := NewDeliveryRuntime(client)

	manifest := connectors.Manifest{}
	manifest.Spec.Protocol = "delivery_github"
	manifest.Config.Values = map[string]string{"repo_url": "https://github.com/evilgaoshu/TARS.git"}

	status, summary, err := runtime.CheckHealth(context.Background(), manifest)
	if err != nil {
		t.Fatalf("check remote git health: %v", err)
	}
	if status != "healthy" || !strings.Contains(summary, "evilgaoshu/TARS") {
		t.Fatalf("unexpected remote health result: %q %q", status, summary)
	}
}

func TestMetricsAndStubCapabilityRuntimes(t *testing.T) {
	t.Parallel()

	t.Run("metrics instant and range", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v1/query":
				_, _ = w.Write([]byte(`{
					"status":"success",
					"data":{"result":[{"metric":{"instance":"api-1"},"value":[1710000000,"1"]}]}
				}`))
			case "/api/v1/query_range":
				_, _ = w.Write([]byte(`{
					"status":"success",
					"data":{"result":[{"metric":{"instance":"api-1"},"values":[[1710000000,"1"]]}]}
				}`))
			default:
				t.Fatalf("unexpected metrics path %q", r.URL.Path)
			}
		}))
		defer server.Close()

		manifest := connectors.Manifest{}
		manifest.Metadata.ID = "metrics-main"
		manifest.Spec.Protocol = "prometheus_http"
		manifest.Config.Values = map[string]string{"base_url": server.URL}

		runtime := NewMetricsCapabilityRuntime(VictoriaMetricsConfig{})
		instantResult, err := runtime.Invoke(context.Background(), manifest, "query.instant", map[string]interface{}{
			"query": "up",
		})
		if err != nil {
			t.Fatalf("invoke instant metrics query: %v", err)
		}
		if got := instantResult.Output["result_count"]; got != 1 {
			t.Fatalf("expected one instant series, got %#v", got)
		}
		if mode := instantResult.Metadata["mode"]; mode != "instant" {
			t.Fatalf("expected instant mode metadata, got %#v", mode)
		}

		rangeResult, err := runtime.Invoke(context.Background(), manifest, "query.range", map[string]interface{}{
			"query":  "up",
			"window": "5m",
			"step":   "30s",
		})
		if err != nil {
			t.Fatalf("invoke range metrics query: %v", err)
		}
		if got := rangeResult.Output["result_count"]; got != 1 {
			t.Fatalf("expected one range series, got %#v", got)
		}
		if mode := rangeResult.Metadata["mode"]; mode != "range" {
			t.Fatalf("expected range mode metadata, got %#v", mode)
		}
	})

	t.Run("metrics failure without base url", func(t *testing.T) {
		runtime := NewMetricsCapabilityRuntime(VictoriaMetricsConfig{})
		manifest := connectors.Manifest{}
		manifest.Spec.Protocol = "prometheus_http"

		result, err := runtime.Invoke(context.Background(), manifest, "query.instant", map[string]interface{}{"query": "up"})
		if err == nil {
			t.Fatalf("expected missing base_url error")
		}
		if result.Status != "failed" || !strings.Contains(result.Error, "base_url is not configured") {
			t.Fatalf("unexpected failure result: %+v", result)
		}
	})

	t.Run("stub runtimes", func(t *testing.T) {
		manifest := connectors.Manifest{}
		manifest.Metadata.ID = "stub-connector"

		mcpRuntime := NewMCPStubRuntime()
		mcpResult, err := mcpRuntime.Invoke(context.Background(), manifest, "mcp.echo", map[string]interface{}{"message": "hello"})
		if err != nil {
			t.Fatalf("invoke mcp stub: %v", err)
		}
		if got := mcpResult.Output["source"]; got != "mcp_stub" {
			t.Fatalf("expected mcp_stub source, got %#v", got)
		}
		if note, _ := mcpResult.Output["note"].(string); !strings.Contains(note, "mcp.echo") {
			t.Fatalf("unexpected mcp note %#v", mcpResult.Output["note"])
		}

		skillRuntime := NewSkillStubRuntime()
		skillResult, err := skillRuntime.Invoke(context.Background(), manifest, "skill.echo", map[string]interface{}{"message": "hello"})
		if err != nil {
			t.Fatalf("invoke skill stub: %v", err)
		}
		if got := skillResult.Output["source"]; got != "skill_stub" {
			t.Fatalf("expected skill_stub source, got %#v", got)
		}
		if note, _ := skillResult.Output["note"].(string); !strings.Contains(note, "skill.echo") {
			t.Fatalf("unexpected skill note %#v", skillResult.Output["note"])
		}
	})
}

type rewriteGitHubHostTransport struct {
	base   http.RoundTripper
	target string
}

func (t rewriteGitHubHostTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	rewritten := req.Clone(req.Context())
	targetURL := strings.TrimRight(t.target, "/")
	rewritten.URL.Scheme = "http"
	rewritten.URL.Host = strings.TrimPrefix(targetURL, "http://")
	rewritten.Host = rewritten.URL.Host
	return base.RoundTrip(rewritten)
}

func initTestGitRepo(t *testing.T) string {
	t.Helper()

	repoPath := t.TempDir()
	runGit(t, repoPath, "init")
	runGit(t, repoPath, "config", "user.name", "TARS Test")
	runGit(t, repoPath, "config", "user.email", "tars@example.com")
	runGit(t, repoPath, "remote", "add", "origin", "https://github.com/evilgaoshu/TARS.git")

	writeRepoFile(t, repoPath, "README.md", "hello\n")
	runGit(t, repoPath, "add", "README.md")
	runGit(t, repoPath, "commit", "-m", "docs: add readme")

	writeRepoFile(t, repoPath, "api.txt", "timeout fix\n")
	runGit(t, repoPath, "add", "api.txt")
	runGit(t, repoPath, "commit", "-m", "fix api timeout regression")

	return repoPath
}

func writeRepoFile(t *testing.T, repoPath string, name string, content string) {
	t.Helper()

	fullPath := filepath.Join(repoPath, name)
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write repo file %s: %v", name, err)
	}
}

func runGit(t *testing.T, repoPath string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, fmt.Sprintf("%s", output))
	}
}
