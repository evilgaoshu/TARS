package action

import (
	"context"
	"errors"
	"strings"
	"testing"

	"tars/internal/contracts"
	sshclient "tars/internal/modules/action/ssh"
	"tars/internal/modules/connectors"
	"tars/internal/modules/sshcredentials"
)

type fakeCredentialExecutor struct {
	credential sshclient.CredentialConfig
	host       string
	command    string
	result     sshclient.Result
	err        error
}

func (f *fakeCredentialExecutor) RunWithCredential(ctx context.Context, targetHost string, command string, credential sshclient.CredentialConfig) (sshclient.Result, error) {
	f.host = targetHost
	f.command = command
	f.credential = credential
	if f.result.Output == "" && f.result.ExitCode == 0 && !f.result.TimedOut {
		f.result.Output = "ok"
	}
	return f.result, f.err
}

func TestSSHNativeRuntimeResolvesEncryptedCredentialMaterial(t *testing.T) {
	repo := sshcredentials.NewMemoryRepository()
	vault := sshcredentials.NewMemorySecretBackend()
	manager := sshcredentials.NewManager(repo, vault)
	if _, err := manager.Create(context.Background(), sshcredentials.CreateInput{
		CredentialID:   "ops-key",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       sshcredentials.AuthTypePassword,
		Password:       "local-only-password",
		HostScope:      "192.168.3.100",
		OperatorReason: "test",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	executor := &fakeCredentialExecutor{}
	runtime := NewSSHNativeRuntime(executor, manager)
	result, err := runtime.Execute(context.Background(), connectors.Manifest{
		Metadata: connectors.Metadata{ID: "ssh-main"},
		Spec:     connectors.Spec{Type: "execution", Protocol: "ssh_native"},
		Config: connectors.RuntimeConfig{Values: map[string]string{
			"host":          "192.168.3.100",
			"port":          "22",
			"username":      "root",
			"credential_id": "ops-key",
		}},
	}, contracts.ApprovedExecutionRequest{
		ExecutionID: "exec-1",
		SessionID:   "session-1",
		TargetHost:  "192.168.3.100",
		Command:     "uptime",
		ConnectorID: "ssh-main",
		Protocol:    "ssh_native",
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.Status != "completed" || executor.credential.Password != "local-only-password" {
		t.Fatalf("unexpected execution result=%#v credential=%#v", result, executor.credential)
	}
}

func TestSSHNativeRuntimeFailsClosedWithoutCredentialBackend(t *testing.T) {
	runtime := NewSSHNativeRuntime(&fakeCredentialExecutor{}, nil)
	_, err := runtime.Execute(context.Background(), connectors.Manifest{
		Metadata: connectors.Metadata{ID: "ssh-main"},
		Spec:     connectors.Spec{Type: "execution", Protocol: "ssh_native"},
		Config: connectors.RuntimeConfig{Values: map[string]string{
			"host":          "192.168.3.100",
			"username":      "root",
			"credential_id": "ops-key",
		}},
	}, contracts.ApprovedExecutionRequest{ExecutionID: "exec-1", SessionID: "session-1", TargetHost: "192.168.3.100", Command: "uptime"})
	if err == nil {
		t.Fatalf("expected missing credential backend to fail closed")
	}
}

func TestSSHNativeRuntimeNormalizesRemoteFailureIntoExecutionStatus(t *testing.T) {
	manager := sshcredentials.NewManager(sshcredentials.NewMemoryRepository(), sshcredentials.NewMemorySecretBackend())
	if _, err := manager.Create(context.Background(), sshcredentials.CreateInput{
		CredentialID:   "ops-key",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       sshcredentials.AuthTypePassword,
		Password:       "pw",
		HostScope:      "192.168.3.100",
		OperatorReason: "test",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	executor := &fakeCredentialExecutor{result: sshclient.Result{Output: "permission denied", ExitCode: 17}, err: sshclient.ErrRemoteCommandFailed}
	runtime := NewSSHNativeRuntime(executor, manager)
	result, err := runtime.Execute(context.Background(), connectors.Manifest{
		Metadata: connectors.Metadata{ID: "ssh-main"},
		Spec:     connectors.Spec{Type: "execution", Protocol: "ssh_native"},
		Config:   connectors.RuntimeConfig{Values: map[string]string{"credential_id": "ops-key", "username": "root"}},
	}, contracts.ApprovedExecutionRequest{ExecutionID: "exec-1", SessionID: "session-1", TargetHost: "192.168.3.100", Command: "uptime", ConnectorID: "ssh-main"})
	if err != nil {
		t.Fatalf("expected remote command failure to become status, got err=%v", err)
	}
	if result.Status != "failed" || result.ExitCode != 17 {
		t.Fatalf("unexpected failed result: %#v", result)
	}
}

func TestSSHNativeRuntimeVerifyBranches(t *testing.T) {
	manager := sshcredentials.NewManager(sshcredentials.NewMemoryRepository(), sshcredentials.NewMemorySecretBackend())
	if _, err := manager.Create(context.Background(), sshcredentials.CreateInput{
		CredentialID:   "ops-key",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       sshcredentials.AuthTypePassword,
		Password:       "pw",
		HostScope:      "192.168.3.100",
		OperatorReason: "test",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	manifest := connectors.Manifest{
		Metadata: connectors.Metadata{ID: "ssh-main"},
		Spec:     connectors.Spec{Type: "execution", Protocol: "ssh_native"},
		Config: connectors.RuntimeConfig{Values: map[string]string{
			"credential_id": "ops-key",
			"username":      "root",
		}},
	}

	runtime := NewSSHNativeRuntime(&fakeCredentialExecutor{}, manager)
	skipped, err := runtime.Verify(context.Background(), manifest, contracts.VerificationRequest{SessionID: "session-1", ExecutionID: "exec-1"})
	if err != nil || skipped.Status != "skipped" {
		t.Fatalf("expected skipped verify, got status=%s err=%v", skipped.Status, err)
	}

	successRuntime := NewSSHNativeRuntime(&fakeCredentialExecutor{result: sshclient.Result{Output: "active"}}, manager)
	success, err := successRuntime.Verify(context.Background(), manifest, contracts.VerificationRequest{
		SessionID: "session-1", ExecutionID: "exec-2", TargetHost: "192.168.3.100", Service: "sshd", ConnectorID: "ssh-main",
	})
	if err != nil || success.Status != "success" {
		t.Fatalf("expected success verify, got status=%s err=%v", success.Status, err)
	}

	failRuntime := NewSSHNativeRuntime(&fakeCredentialExecutor{err: errors.New("dial failed")}, manager)
	failed, err := failRuntime.Verify(context.Background(), manifest, contracts.VerificationRequest{
		SessionID: "session-1", ExecutionID: "exec-3", TargetHost: "192.168.3.100", Service: "sshd", ConnectorID: "ssh-main",
	})
	if err != nil || failed.Status != "failed" {
		t.Fatalf("expected failed verify, got status=%s err=%v", failed.Status, err)
	}
}

func TestSSHNativeRuntimeFailsClosedWhenRotationRequired(t *testing.T) {
	manager := sshcredentials.NewManager(sshcredentials.NewMemoryRepository(), sshcredentials.NewMemorySecretBackend())
	if _, err := manager.Create(context.Background(), sshcredentials.CreateInput{
		CredentialID:   "ops-key",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       sshcredentials.AuthTypePassword,
		Password:       "pw",
		HostScope:      "192.168.3.100",
		OperatorReason: "test",
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := manager.SetStatus(context.Background(), "ops-key", sshcredentials.StatusRotationRequired, "admin", "rotate now"); err != nil {
		t.Fatalf("SetStatus() error = %v", err)
	}
	runtime := NewSSHNativeRuntime(&fakeCredentialExecutor{}, manager)
	_, err := runtime.Execute(context.Background(), connectors.Manifest{
		Metadata: connectors.Metadata{ID: "ssh-main"},
		Spec:     connectors.Spec{Type: "execution", Protocol: "ssh_native"},
		Config: connectors.RuntimeConfig{Values: map[string]string{
			"host":          "192.168.3.100",
			"port":          "22",
			"username":      "root",
			"credential_id": "ops-key",
		}},
	}, contracts.ApprovedExecutionRequest{
		ExecutionID: "exec-1",
		SessionID:   "session-1",
		TargetHost:  "192.168.3.100",
		Command:     "uptime",
		ConnectorID: "ssh-main",
		Protocol:    "ssh_native",
	})
	if err == nil || !strings.Contains(err.Error(), sshcredentials.StatusRotationRequired) {
		t.Fatalf("expected rotation_required credential to fail closed, got %v", err)
	}
}
