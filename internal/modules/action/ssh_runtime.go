package action

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"tars/internal/contracts"
	sshclient "tars/internal/modules/action/ssh"
	"tars/internal/modules/connectors"
	"tars/internal/modules/sshcredentials"
)

type credentialExecutor interface {
	RunWithCredential(ctx context.Context, targetHost string, command string, credential sshclient.CredentialConfig) (sshclient.Result, error)
}

type SSHNativeRuntime struct {
	executor    credentialExecutor
	credentials *sshcredentials.Manager
}

func NewSSHNativeRuntime(executor credentialExecutor, credentials *sshcredentials.Manager) *SSHNativeRuntime {
	return &SSHNativeRuntime{executor: executor, credentials: credentials}
}

func (r *SSHNativeRuntime) Execute(ctx context.Context, manifest connectors.Manifest, req contracts.ApprovedExecutionRequest) (contracts.ExecutionResult, error) {
	if r == nil || r.executor == nil || r.credentials == nil || !r.credentials.Configured() {
		return contracts.ExecutionResult{}, sshcredentials.ErrNotConfigured
	}
	targetHost := firstNonEmpty(req.TargetHost, manifest.Config.Values["host"])
	command := strings.TrimSpace(req.Command)
	if targetHost == "" || command == "" {
		return contracts.ExecutionResult{}, fmt.Errorf("target_host and command are required")
	}
	credentialID := strings.TrimSpace(manifest.Config.Values["credential_id"])
	if credentialID == "" {
		return contracts.ExecutionResult{}, fmt.Errorf("ssh credential_id is required")
	}
	resolved, err := r.credentials.Resolve(ctx, credentialID, targetHost)
	if err != nil {
		return contracts.ExecutionResult{}, err
	}
	result, err := r.executor.RunWithCredential(ctx, targetHost, command, sshclient.CredentialConfig{
		User:       firstNonEmpty(manifest.Config.Values["username"], resolved.Username),
		Password:   resolved.Password,
		PrivateKey: resolved.PrivateKey,
		Passphrase: resolved.Passphrase,
		Port:       parseSSHPort(manifest.Config.Values["port"]),
	})
	if err != nil && !errors.Is(err, sshclient.ErrRemoteCommandFailed) && !errors.Is(err, sshclient.ErrCommandTimedOut) {
		return contracts.ExecutionResult{}, err
	}
	status := "completed"
	switch {
	case errors.Is(err, sshclient.ErrCommandTimedOut) || result.TimedOut:
		status = "timeout"
	case result.ExitCode != 0:
		status = "failed"
	}
	runtimeErr := err
	if errors.Is(err, sshclient.ErrRemoteCommandFailed) || errors.Is(err, sshclient.ErrCommandTimedOut) {
		runtimeErr = nil
	}
	return contracts.ExecutionResult{
		ExecutionID:   req.ExecutionID,
		SessionID:     req.SessionID,
		Status:        status,
		ConnectorID:   manifest.Metadata.ID,
		Protocol:      manifest.Spec.Protocol,
		ExecutionMode: firstNonEmpty(req.ExecutionMode, connectors.DefaultExecutionMode(manifest.Spec.Protocol)),
		ExitCode:      result.ExitCode,
		Output:        result.Output,
		OutputPreview: result.Output,
		Runtime: runtimeMetadataForConnectorManifest(
			"connector",
			runtimeSelectionMode(req.ConnectorID),
			manifest,
			firstNonEmpty(req.ExecutionMode, connectors.DefaultExecutionMode(manifest.Spec.Protocol)),
			true,
			false,
			"",
			"",
		),
	}, runtimeErr
}

func (r *SSHNativeRuntime) Verify(ctx context.Context, manifest connectors.Manifest, req contracts.VerificationRequest) (contracts.VerificationResult, error) {
	service := strings.TrimSpace(req.Service)
	if service == "" {
		return contracts.VerificationResult{SessionID: req.SessionID, ExecutionID: req.ExecutionID, Status: "skipped", Summary: "verification skipped: no service hint available"}, nil
	}
	execReq := contracts.ApprovedExecutionRequest{
		ExecutionID:   req.ExecutionID,
		SessionID:     req.SessionID,
		TargetHost:    req.TargetHost,
		Command:       fmt.Sprintf("systemctl is-active %s", service),
		Service:       service,
		ConnectorID:   req.ConnectorID,
		Protocol:      req.Protocol,
		ExecutionMode: req.ExecutionMode,
	}
	result, err := r.Execute(ctx, manifest, execReq)
	if err != nil {
		return contracts.VerificationResult{SessionID: req.SessionID, ExecutionID: req.ExecutionID, Status: "failed", Summary: err.Error(), Runtime: result.Runtime}, nil
	}
	status := "failed"
	summary := fmt.Sprintf("verification failed: %s is not active", service)
	if strings.HasPrefix(strings.TrimSpace(result.OutputPreview), "active") {
		status = "success"
		summary = fmt.Sprintf("verification passed: %s is active", service)
	}
	return contracts.VerificationResult{
		SessionID:   req.SessionID,
		ExecutionID: req.ExecutionID,
		Status:      status,
		Summary:     summary,
		Runtime:     result.Runtime,
		Details: map[string]interface{}{
			"command":   execReq.Command,
			"exit_code": result.ExitCode,
			"output":    result.OutputPreview,
		},
	}, nil
}

func (r *SSHNativeRuntime) CheckHealth(ctx context.Context, manifest connectors.Manifest) (string, string, error) {
	targetHost := strings.TrimSpace(manifest.Config.Values["host"])
	if targetHost == "" {
		return "degraded", "ssh host is not configured", fmt.Errorf("ssh host is not configured")
	}
	execReq := contracts.ApprovedExecutionRequest{
		ExecutionID: "health-probe",
		SessionID:   "health-probe",
		TargetHost:  targetHost,
		Command:     "true",
	}
	_, err := r.Execute(ctx, manifest, execReq)
	if err != nil {
		return "unhealthy", "ssh connector health probe failed: " + err.Error(), err
	}
	return "healthy", "ssh connector health probe succeeded (connectivity and authentication verified)", nil
}

func parseSSHPort(value string) int {
	port, _ := strconv.Atoi(strings.TrimSpace(value))
	return port
}
