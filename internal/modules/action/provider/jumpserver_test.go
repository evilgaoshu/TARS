package provider

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"tars/internal/contracts"
	"tars/internal/modules/connectors"
)

func TestJumpServerVerifyFallsBackToServiceAlias(t *testing.T) {
	t.Parallel()

	var submittedArgs []string
	runtime := NewJumpServerRuntime(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && strings.HasPrefix(req.URL.Path, "/api/v1/assets/hosts/"):
				return jsonResponse(http.StatusOK, `[{"id":"asset-1","address":"192.168.3.106","name":"node-1"}]`), nil
			case req.Method == http.MethodPost && req.URL.Path == "/api/v1/ops/jobs/":
				body, err := io.ReadAll(req.Body)
				if err != nil {
					return nil, err
				}
				submittedArgs = append(submittedArgs, string(body))
				if strings.Contains(string(body), `"args":"systemctl is-active ssh"`) {
					return jsonResponse(http.StatusCreated, `{"id":"job-ssh","task_id":"task-ssh"}`), nil
				}
				if strings.Contains(string(body), `"args":"systemctl is-active sshd"`) {
					return jsonResponse(http.StatusCreated, `{"id":"job-sshd","task_id":"task-sshd"}`), nil
				}
				return jsonResponse(http.StatusBadRequest, `{"detail":"unexpected args"}`), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/ops/job-execution/task-detail/task-ssh/":
				return jsonResponse(http.StatusOK, `{"status":{"value":"success"},"is_finished":true,"is_success":true,"summary":{}}`), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/ops/job-executions/task-ssh/":
				return jsonResponse(http.StatusOK, `{"id":"task-ssh","result":"Unit ssh.service could not be found","summary":{}}`), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/ops/ansible/job-execution/task-ssh/log/":
				return textResponse(http.StatusOK, "Unit ssh.service could not be found\n"), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/ops/job-execution/task-detail/task-sshd/":
				return jsonResponse(http.StatusOK, `{"status":{"value":"success"},"is_finished":true,"is_success":true,"summary":{}}`), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/ops/job-executions/task-sshd/":
				return jsonResponse(http.StatusOK, `{"id":"task-sshd","result":"active","summary":{}}`), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/ops/ansible/job-execution/task-sshd/log/":
				return textResponse(http.StatusOK, "active\n"), nil
			default:
				return jsonResponse(http.StatusNotFound, `{"detail":"not found"}`), nil
			}
		}),
	})

	result, err := runtime.Verify(context.Background(), jumpServerManifest(), contracts.VerificationRequest{
		SessionID:   "ses-1",
		ExecutionID: "exe-1",
		TargetHost:  "192.168.3.106",
		Service:     "ssh",
		ConnectorID: "jumpserver-main",
	})
	if err != nil {
		t.Fatalf("verify execution: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("expected success, got %+v", result)
	}
	if !strings.Contains(result.Summary, "sshd is active") {
		t.Fatalf("unexpected summary: %s", result.Summary)
	}
	if matched, _ := result.Details["matched_service"].(string); matched != "sshd" {
		t.Fatalf("expected matched service sshd, got %+v", result.Details)
	}
	if len(submittedArgs) != 2 {
		t.Fatalf("expected 2 verification attempts, got %d", len(submittedArgs))
	}
}

func TestJumpServerOutputIndicatesActiveWithANSILogs(t *testing.T) {
	t.Parallel()

	value := "[logs]\n2026-03-19 14:07:42 \x1b[0;33mdev | CHANGED | rc=0 >>\x1b[0m\n\x1b[0;33mactive\x1b[0m\n"
	if !jumpServerOutputIndicatesActive(value) {
		t.Fatalf("expected active output to be detected")
	}
}

func jumpServerManifest() connectors.Manifest {
	return connectors.Manifest{
		Metadata: connectors.Metadata{
			ID:     "jumpserver-main",
			Vendor: "jumpserver",
		},
		Spec: connectors.Spec{
			Protocol: "jumpserver_api",
		},
		Config: connectors.RuntimeConfig{
			Values: map[string]string{
				"base_url":   "https://jumpserver.example.test",
				"access_key": "ak-test",
				"secret_key": "sk-test",
			},
		},
	}
}

func jsonResponse(status int, body string) *http.Response {
	resp := textResponse(status, body)
	resp.Header.Set("Content-Type", "application/json")
	return resp
}

func textResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
