package ssh

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	cryptossh "golang.org/x/crypto/ssh"
)

type capturedKeyFile struct {
	path    string
	mode    os.FileMode
	content string
	exists  bool
}

const (
	fakeSSHModeEnv     = "TARS_FAKE_SSH_MODE"
	fakeSSHExitCodeEnv = "TARS_FAKE_SSH_EXIT_CODE"
	fakeSSHSleepEnv    = "TARS_FAKE_SSH_SLEEP_MS"
)

func TestMain(m *testing.M) {
	if os.Getenv(fakeSSHModeEnv) == "1" {
		os.Exit(runFakeSSH())
	}

	os.Exit(m.Run())
}

func runFakeSSH() int {
	if sleep := envInt(fakeSSHSleepEnv); sleep > 0 {
		time.Sleep(time.Duration(sleep) * time.Millisecond)
	}

	argsJSON, err := json.Marshal(os.Args[1:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal args: %v", err)
		return 2
	}
	if output := os.Getenv("TARS_FAKE_SSH_OUTPUT"); output != "" {
		fmt.Fprint(os.Stdout, output)
	} else {
		fmt.Fprint(os.Stdout, string(argsJSON))
	}

	if code := envInt(fakeSSHExitCodeEnv); code != 0 {
		return code
	}
	return 0
}

func envInt(name string) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return 0
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return n
}

func TestNewExecutorAppliesDefaultTimeouts(t *testing.T) {
	t.Parallel()

	exec := NewExecutor(Config{})
	if exec.cfg.ConnectTimeout != 10*time.Second {
		t.Fatalf("expected default connect timeout 10s, got %s", exec.cfg.ConnectTimeout)
	}
	if exec.cfg.CommandTimeout != 5*time.Minute {
		t.Fatalf("expected default command timeout 5m, got %s", exec.cfg.CommandTimeout)
	}
}

func TestQuoteForShellEscapesSingleQuotes(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"simple":      "'simple'",
		"a'b'c":       "'a'\"'\"'b'\"'\"'c'",
		"quote's end": "'quote'\"'\"'s end'",
	}

	for input, want := range cases {
		if got := quoteForShell(input); got != want {
			t.Fatalf("quoteForShell(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestRunRejectsEmptyHostAndCommand(t *testing.T) {
	t.Parallel()

	exec := NewExecutor(Config{})

	if _, err := exec.Run(context.Background(), "   ", "echo ok"); err == nil || !strings.Contains(err.Error(), "target host is empty") {
		t.Fatalf("expected empty host error, got %v", err)
	}
	if _, err := exec.Run(context.Background(), "host.example", " \t "); err == nil || !strings.Contains(err.Error(), "command is empty") {
		t.Fatalf("expected empty command error, got %v", err)
	}
}

func TestRunSucceedsWithFakeSSHAndBuildsExpectedArgs(t *testing.T) {
	exec := NewExecutor(Config{
		User:                   "alice",
		PrivateKeyPath:         "/tmp/test-key",
		ConnectTimeout:         2 * time.Second,
		CommandTimeout:         500 * time.Millisecond,
		DisableHostKeyChecking: true,
	})

	result, err := runWithFakeSSH(t, exec, context.Background(), "db.example.internal", "printf 'hello world'", 0, 0)
	if err != nil {
		t.Fatalf("run fake ssh: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %+v", result)
	}
	if result.TimedOut {
		t.Fatalf("expected non-timeout result, got %+v", result)
	}

	args := mustDecodeArgs(t, result.Output)
	wantContains := []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=2",
		"-o", "LogLevel=ERROR",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-i", "/tmp/test-key",
		"alice@db.example.internal",
		fmt.Sprintf("timeout -k 10s 300s sh -lc %s", quoteForShell("printf 'hello world'")),
	}
	for _, want := range wantContains {
		if !contains(args, want) {
			t.Fatalf("expected args to contain %q, got %#v", want, args)
		}
	}
}

func TestRunReturnsRemoteCommandFailedForNonZeroExit(t *testing.T) {
	exec := NewExecutor(Config{
		ConnectTimeout: 1 * time.Second,
		CommandTimeout: 500 * time.Millisecond,
	})

	result, err := runWithFakeSSH(t, exec, context.Background(), "db.example.internal", "false", 17, 0)
	if !errors.Is(err, ErrRemoteCommandFailed) {
		t.Fatalf("expected ErrRemoteCommandFailed, got %v", err)
	}
	if result.ExitCode != 17 {
		t.Fatalf("expected fake ssh exit code 17, got %+v", result)
	}
	if result.TimedOut {
		t.Fatalf("expected non-timeout result, got %+v", result)
	}
}

func TestRunReturnsTimedOutWhenContextExpires(t *testing.T) {
	exec := NewExecutor(Config{
		ConnectTimeout: 1 * time.Second,
		CommandTimeout: 1 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	result, err := runWithFakeSSH(t, exec, ctx, "db.example.internal", "sleep 10", 0, 5_000)
	if !errors.Is(err, ErrCommandTimedOut) {
		t.Fatalf("expected ErrCommandTimedOut, got %v", err)
	}
	if !result.TimedOut {
		t.Fatalf("expected timeout result, got %+v", result)
	}
	if result.ExitCode != 124 {
		t.Fatalf("expected exit code 124, got %+v", result)
	}
}

func TestRunReturnsTimedOutForRemoteExit124(t *testing.T) {
	exec := NewExecutor(Config{
		ConnectTimeout: 1 * time.Second,
		CommandTimeout: 1 * time.Second,
	})

	result, err := runWithFakeSSH(t, exec, context.Background(), "db.example.internal", "sleep 10", 124, 0)
	if !errors.Is(err, ErrCommandTimedOut) {
		t.Fatalf("expected ErrCommandTimedOut, got %v", err)
	}
	if !result.TimedOut {
		t.Fatalf("expected timeout result, got %+v", result)
	}
	if result.ExitCode != 124 {
		t.Fatalf("expected exit code 124, got %+v", result)
	}
}

func TestRunWithCredentialRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	exec := NewExecutor(Config{DisableHostKeyChecking: true})
	if _, err := exec.RunWithCredential(context.Background(), "", "uptime", CredentialConfig{User: "root", Password: "pw"}); err == nil || !strings.Contains(err.Error(), "target host is empty") {
		t.Fatalf("expected empty host error, got %v", err)
	}
	if _, err := exec.RunWithCredential(context.Background(), "127.0.0.1", "", CredentialConfig{User: "root", Password: "pw"}); err == nil || !strings.Contains(err.Error(), "command is empty") {
		t.Fatalf("expected empty command error, got %v", err)
	}
	if _, err := exec.RunWithCredential(context.Background(), "127.0.0.1", "uptime", CredentialConfig{Password: "pw"}); err == nil || !strings.Contains(err.Error(), "ssh username is required") {
		t.Fatalf("expected username error, got %v", err)
	}
	if _, err := exec.RunWithCredential(context.Background(), "127.0.0.1", "uptime", CredentialConfig{User: "root"}); err == nil || !strings.Contains(err.Error(), "ssh credential material is required") {
		t.Fatalf("expected credential material error, got %v", err)
	}
}

func TestRunWithCredentialRunsAgainstInProcessSSHServer(t *testing.T) {
	address := startTestSSHServer(t, "root", "pw", "native-ok", 0)
	exec := NewExecutor(Config{ConnectTimeout: time.Second, CommandTimeout: time.Second, DisableHostKeyChecking: true})

	result, err := exec.RunWithCredential(context.Background(), address, "printf ok", CredentialConfig{User: "root", Password: "pw"})
	if err != nil {
		t.Fatalf("RunWithCredential() error = %v", err)
	}
	if result.Output != "native-ok" || result.ExitCode != 0 || result.TimedOut {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRunWithCredentialMapsRemoteExitStatus(t *testing.T) {
	address := startTestSSHServer(t, "root", "pw", "failed-output", 17)
	exec := NewExecutor(Config{ConnectTimeout: time.Second, CommandTimeout: time.Second, DisableHostKeyChecking: true})

	result, err := exec.RunWithCredential(context.Background(), address, "false", CredentialConfig{User: "root", Password: "pw"})
	if !errors.Is(err, ErrRemoteCommandFailed) {
		t.Fatalf("expected ErrRemoteCommandFailed, got %v", err)
	}
	if result.ExitCode != 17 || result.Output != "failed-output" {
		t.Fatalf("unexpected failed result: %#v", result)
	}
}

func TestRunWithCredentialMapsRemoteExit124ToTimeout(t *testing.T) {
	address := startTestSSHServer(t, "root", "pw", "timeout-output", 124)
	exec := NewExecutor(Config{ConnectTimeout: time.Second, CommandTimeout: time.Second, DisableHostKeyChecking: true})

	result, err := exec.RunWithCredential(context.Background(), address, "sleep 10", CredentialConfig{User: "root", Password: "pw"})
	if !errors.Is(err, ErrCommandTimedOut) {
		t.Fatalf("expected ErrCommandTimedOut, got %v", err)
	}
	if !result.TimedOut || result.ExitCode != 124 {
		t.Fatalf("unexpected timeout result: %#v", result)
	}
}

func TestRunWithCredentialReturnsAuthenticationError(t *testing.T) {
	address := startTestSSHServer(t, "root", "pw", "never", 0)
	exec := NewExecutor(Config{ConnectTimeout: time.Second, CommandTimeout: time.Second, DisableHostKeyChecking: true})

	if _, err := exec.RunWithCredential(context.Background(), address, "uptime", CredentialConfig{User: "root", Password: "wrong"}); err == nil {
		t.Fatalf("expected authentication error")
	}
}

func TestRunWithCredentialRequiresKnownHostsWhenHostKeyCheckingEnabled(t *testing.T) {
	t.Parallel()

	exec := NewExecutor(Config{ConnectTimeout: time.Second, CommandTimeout: time.Second, DisableHostKeyChecking: false})
	_, err := exec.RunWithCredential(context.Background(), "127.0.0.1:22", "uptime", CredentialConfig{
		User:           "root",
		Password:       "pw",
		KnownHostsPath: filepath.Join(t.TempDir(), "missing_known_hosts"),
	})
	if err == nil {
		t.Fatalf("expected missing known_hosts to fail closed")
	}
}

func TestRunWithCredentialRejectsInvalidPrivateKeyMaterial(t *testing.T) {
	t.Parallel()

	exec := NewExecutor(Config{DisableHostKeyChecking: true})
	_, err := exec.RunWithCredential(context.Background(), "127.0.0.1:22", "uptime", CredentialConfig{
		User:       "root",
		PrivateKey: "not-a-private-key",
	})
	if err == nil {
		t.Fatalf("expected invalid private key to fail before dialing")
	}
}

func TestRunWithCredentialWritesPrivateKeyTo0600TempFileAndCleansUp(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block, err := cryptossh.MarshalPrivateKey(privateKey, "unit-test")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}

	var captured capturedKeyFile
	previousFactory := privateKeyTempFileFactory
	t.Cleanup(func() { privateKeyTempFileFactory = previousFactory })
	privateKeyTempFileFactory = func(privateKey string) (string, func(), error) {
		dir := t.TempDir()
		path := filepath.Join(dir, "id_ssh_custody")
		if err := os.WriteFile(path, []byte(privateKey), 0o600); err != nil {
			return "", nil, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return "", nil, err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return "", nil, err
		}
		captured = capturedKeyFile{path: path, mode: info.Mode().Perm(), content: string(content), exists: true}
		return path, func() {
			_ = os.Remove(path)
			_, statErr := os.Stat(path)
			captured.exists = !errors.Is(statErr, os.ErrNotExist)
		}, nil
	}

	exec := NewExecutor(Config{DisableHostKeyChecking: true})
	_, err = exec.RunWithCredential(context.Background(), "127.0.0.1:22", "uptime", CredentialConfig{
		User:       "root",
		PrivateKey: string(pem.EncodeToMemory(block)),
	})
	if err == nil {
		t.Fatalf("expected dialing fake target to fail after preparing credential")
	}
	if captured.path == "" {
		t.Fatalf("expected private key temp file to be created")
	}
	if captured.mode != 0o600 {
		t.Fatalf("expected temp key file mode 0600, got %o", captured.mode)
	}
	if !strings.Contains(captured.content, "BEGIN OPENSSH PRIVATE KEY") {
		t.Fatalf("expected temp file to contain private key material")
	}
	if captured.exists {
		t.Fatalf("expected temp key file to be cleaned up")
	}
}

func TestCredentialAuthMethodsAcceptPrivateKeysAndPassphrases(t *testing.T) {
	t.Parallel()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	block, err := cryptossh.MarshalPrivateKey(privateKey, "unit-test")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	keyPath, cleanup, err := privateKeyTempFileFactory(string(pem.EncodeToMemory(block)))
	if err != nil {
		t.Fatalf("privateKeyTempFileFactory(private key) error = %v", err)
	}
	defer cleanup()
	methods, err := credentialAuthMethods(keyPath, CredentialConfig{})
	if err != nil {
		t.Fatalf("credentialAuthMethods(private key) error = %v", err)
	}
	if len(methods) != 1 {
		t.Fatalf("expected one public key auth method, got %d", len(methods))
	}
	encrypted, err := cryptossh.MarshalPrivateKeyWithPassphrase(privateKey, "unit-test", []byte("passphrase"))
	if err != nil {
		t.Fatalf("marshal encrypted key: %v", err)
	}
	encryptedPath, encryptedCleanup, err := privateKeyTempFileFactory(string(pem.EncodeToMemory(encrypted)))
	if err != nil {
		t.Fatalf("privateKeyTempFileFactory(encrypted key) error = %v", err)
	}
	defer encryptedCleanup()
	methods, err = credentialAuthMethods(encryptedPath, CredentialConfig{Passphrase: "passphrase", Password: "pw"})
	if err != nil {
		t.Fatalf("credentialAuthMethods(encrypted key) error = %v", err)
	}
	if len(methods) != 2 {
		t.Fatalf("expected public key and password methods, got %d", len(methods))
	}
}

func TestHostKeyCallbackBranches(t *testing.T) {
	t.Parallel()

	exec := NewExecutor(Config{DisableHostKeyChecking: true})
	if callback, err := exec.hostKeyCallback(CredentialConfig{}); err != nil || callback == nil {
		t.Fatalf("expected insecure host key callback, got callback=%v err=%v", callback, err)
	}
	knownHosts := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(knownHosts, []byte(""), 0o600); err != nil {
		t.Fatalf("write known_hosts: %v", err)
	}
	exec = NewExecutor(Config{DisableHostKeyChecking: false})
	if callback, err := exec.hostKeyCallback(CredentialConfig{KnownHostsPath: knownHosts}); err != nil || callback == nil {
		t.Fatalf("expected known_hosts callback, got callback=%v err=%v", callback, err)
	}
	exec = NewExecutor(Config{DisableHostKeyChecking: false, KnownHostsPath: knownHosts})
	if callback, err := exec.hostKeyCallback(CredentialConfig{}); err != nil || callback == nil {
		t.Fatalf("expected config known_hosts callback, got callback=%v err=%v", callback, err)
	}
	_, _ = NewExecutor(Config{DisableHostKeyChecking: false}).hostKeyCallback(CredentialConfig{})
}

func TestDestinationPartsParsesUserHostAndPort(t *testing.T) {
	t.Parallel()

	userName, host, port := destinationParts("alice@[::1]:2200", "default", "credential", 22)
	if userName != "alice" || host != "::1" || port != 2200 {
		t.Fatalf("unexpected parsed destination: user=%q host=%q port=%d", userName, host, port)
	}
	userName, host, port = destinationParts("db.example", "default", "", 0)
	if userName != "default" || host != "db.example" || port != 0 {
		t.Fatalf("unexpected default destination: user=%q host=%q port=%d", userName, host, port)
	}
}

func runWithFakeSSH(t *testing.T, exec *Executor, ctx context.Context, targetHost, command string, exitCode, sleepMS int) (Result, error) {
	t.Helper()

	sshPath := installFakeSSH(t)
	t.Setenv(fakeSSHModeEnv, "1")
	t.Setenv(fakeSSHExitCodeEnv, strconv.Itoa(exitCode))
	t.Setenv(fakeSSHSleepEnv, strconv.Itoa(sleepMS))
	t.Setenv("PATH", filepath.Dir(sshPath)+string(os.PathListSeparator)+os.Getenv("PATH"))

	return exec.Run(ctx, targetHost, command)
}

func startTestSSHServer(t *testing.T, username string, password string, output string, exitStatus uint32) string {
	t.Helper()

	_, hostPrivateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	signer, err := cryptossh.NewSignerFromSigner(hostPrivateKey)
	if err != nil {
		t.Fatalf("host signer: %v", err)
	}
	config := &cryptossh.ServerConfig{
		PasswordCallback: func(conn cryptossh.ConnMetadata, pass []byte) (*cryptossh.Permissions, error) {
			if conn.User() == username && string(pass) == password {
				return nil, nil
			}
			return nil, fmt.Errorf("invalid password")
		},
	}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen ssh server: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleTestSSHConn(conn, config, output, exitStatus)
		}
	}()
	return listener.Addr().String()
}

func handleTestSSHConn(conn net.Conn, config *cryptossh.ServerConfig, output string, exitStatus uint32) {
	sshConn, channels, requests, err := cryptossh.NewServerConn(conn, config)
	if err != nil {
		_ = conn.Close()
		return
	}
	defer sshConn.Close()
	go cryptossh.DiscardRequests(requests)
	for channel := range channels {
		if channel.ChannelType() != "session" {
			_ = channel.Reject(cryptossh.UnknownChannelType, "session only")
			continue
		}
		stream, requests, err := channel.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer stream.Close()
			for req := range requests {
				switch req.Type {
				case "exec":
					_ = req.Reply(true, nil)
					_, _ = io.WriteString(stream, output)
					payload := make([]byte, 4)
					binary.BigEndian.PutUint32(payload, exitStatus)
					_, _ = stream.SendRequest("exit-status", false, payload)
					return
				default:
					_ = req.Reply(false, nil)
				}
			}
		}()
	}
}

func installFakeSSH(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os executable: %v", err)
	}
	sshPath := filepath.Join(dir, "ssh")
	if err := os.Symlink(exe, sshPath); err != nil {
		t.Fatalf("create fake ssh symlink: %v", err)
	}
	return sshPath
}

func mustDecodeArgs(t *testing.T, output string) []string {
	t.Helper()

	var args []string
	if err := json.Unmarshal([]byte(output), &args); err != nil {
		t.Fatalf("decode helper output %q: %v", output, err)
	}
	return args
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
