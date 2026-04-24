package ssh

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	cryptossh "golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

var (
	ErrRemoteCommandFailed = errors.New("remote command failed")
	ErrCommandTimedOut     = errors.New("remote command timed out")
)

var privateKeyTempFileFactory = func(privateKey string) (string, func(), error) {
	file, err := os.CreateTemp("", "tars-ssh-key-*")
	if err != nil {
		return "", nil, err
	}
	path := file.Name()
	cleanup := func() {
		_ = os.Remove(path)
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, err
	}
	if _, err := file.WriteString(privateKey); err != nil {
		_ = file.Close()
		cleanup()
		return "", nil, err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", nil, err
	}
	return path, cleanup, nil
}

type Config struct {
	User                   string
	PrivateKeyPath         string
	KnownHostsPath         string
	ConnectTimeout         time.Duration
	CommandTimeout         time.Duration
	DisableHostKeyChecking bool
}

type CredentialConfig struct {
	User                   string
	Password               string
	PrivateKey             string
	Passphrase             string
	Port                   int
	KnownHostsPath         string
	DisableHostKeyChecking bool
}

type Result struct {
	ExitCode int
	Output   string
	TimedOut bool
}

type Executor struct {
	cfg Config
}

func NewExecutor(cfg Config) *Executor {
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = 10 * time.Second
	}
	if cfg.CommandTimeout <= 0 {
		cfg.CommandTimeout = 5 * time.Minute
	}
	return &Executor{cfg: cfg}
}

func (e *Executor) Run(ctx context.Context, targetHost string, command string) (Result, error) {
	if strings.TrimSpace(targetHost) == "" {
		return Result{}, fmt.Errorf("target host is empty")
	}
	if strings.TrimSpace(command) == "" {
		return Result{}, fmt.Errorf("command is empty")
	}

	timeoutSeconds := int(e.cfg.CommandTimeout.Seconds())
	if timeoutSeconds <= 0 {
		timeoutSeconds = 300
	}

	remoteCommand := fmt.Sprintf(
		"timeout -k 10s %ds sh -lc %s",
		timeoutSeconds,
		quoteForShell(command),
	)

	args := []string{
		"-o", "BatchMode=yes",
		"-o", fmt.Sprintf("ConnectTimeout=%d", int(e.cfg.ConnectTimeout.Seconds())),
		"-o", "LogLevel=ERROR",
	}
	if e.cfg.DisableHostKeyChecking {
		args = append(args,
			"-o", "StrictHostKeyChecking=no",
			"-o", "UserKnownHostsFile=/dev/null",
		)
	}
	if e.cfg.PrivateKeyPath != "" {
		args = append(args, "-i", e.cfg.PrivateKeyPath)
	}

	destination := targetHost
	if e.cfg.User != "" && !strings.Contains(targetHost, "@") {
		destination = e.cfg.User + "@" + targetHost
	}
	args = append(args, destination, remoteCommand)

	deadline := e.cfg.ConnectTimeout + e.cfg.CommandTimeout + 5*time.Second
	if deadline <= 0 {
		deadline = 6 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "ssh", args...)
	output, err := cmd.CombinedOutput()
	result := Result{
		Output: strings.TrimSpace(string(output)),
	}

	if runCtx.Err() == context.DeadlineExceeded {
		result.ExitCode = 124
		result.TimedOut = true
		return result, ErrCommandTimedOut
	}

	if err == nil {
		return result, nil
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		if result.ExitCode == 124 {
			result.TimedOut = true
			return result, ErrCommandTimedOut
		}
		return result, ErrRemoteCommandFailed
	}

	return result, err
}

func (e *Executor) RunWithCredential(ctx context.Context, targetHost string, command string, credential CredentialConfig) (Result, error) {
	if strings.TrimSpace(targetHost) == "" {
		return Result{}, fmt.Errorf("target host is empty")
	}
	if strings.TrimSpace(command) == "" {
		return Result{}, fmt.Errorf("command is empty")
	}
	userName, host, port := destinationParts(targetHost, e.cfg.User, credential.User, credential.Port)
	if userName == "" {
		return Result{}, fmt.Errorf("ssh username is required")
	}
	if port <= 0 {
		port = 22
	}
	privateKeyPath := strings.TrimSpace(e.cfg.PrivateKeyPath)
	if strings.TrimSpace(credential.Password) != "" && strings.TrimSpace(credential.PrivateKey) == "" {
		privateKeyPath = ""
	}
	cleanup := func() {}
	if strings.TrimSpace(credential.PrivateKey) != "" {
		tempPath, release, err := privateKeyTempFileFactory(credential.PrivateKey)
		if err != nil {
			return Result{}, err
		}
		cleanup = release
		privateKeyPath = tempPath
		credential.PrivateKey = ""
	}
	defer cleanup()

	authMethods, err := credentialAuthMethods(privateKeyPath, credential)
	if err != nil {
		return Result{}, err
	}
	if len(authMethods) == 0 {
		return Result{}, fmt.Errorf("ssh credential material is required")
	}
	hostKeyCallback, err := e.hostKeyCallback(credential)
	if err != nil {
		return Result{}, err
	}

	timeoutSeconds := int(e.cfg.CommandTimeout.Seconds())
	if timeoutSeconds <= 0 {
		timeoutSeconds = 300
	}
	remoteCommand := fmt.Sprintf(
		"timeout -k 10s %ds sh -lc %s",
		timeoutSeconds,
		quoteForShell(command),
	)
	deadline := e.cfg.ConnectTimeout + e.cfg.CommandTimeout + 5*time.Second
	if deadline <= 0 {
		deadline = 6 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, deadline)
	defer cancel()

	clientConfig := &cryptossh.ClientConfig{
		User:            userName,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         e.cfg.ConnectTimeout,
	}
	address := net.JoinHostPort(host, strconv.Itoa(port))
	type runResult struct {
		result Result
		err    error
	}
	done := make(chan runResult, 1)
	go func() {
		client, err := cryptossh.Dial("tcp", address, clientConfig)
		if err != nil {
			done <- runResult{err: err}
			return
		}
		defer client.Close()
		session, err := client.NewSession()
		if err != nil {
			done <- runResult{err: err}
			return
		}
		defer session.Close()
		output, err := session.CombinedOutput(remoteCommand)
		result := Result{Output: strings.TrimSpace(string(output))}
		if err == nil {
			done <- runResult{result: result}
			return
		}
		var exitErr *cryptossh.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitStatus()
			if result.ExitCode == 124 {
				result.TimedOut = true
				done <- runResult{result: result, err: ErrCommandTimedOut}
				return
			}
			done <- runResult{result: result, err: ErrRemoteCommandFailed}
			return
		}
		done <- runResult{result: result, err: err}
	}()
	select {
	case <-runCtx.Done():
		return Result{ExitCode: 124, TimedOut: true}, ErrCommandTimedOut
	case out := <-done:
		return out.result, out.err
	}
}

func (e *Executor) hostKeyCallback(credential CredentialConfig) (cryptossh.HostKeyCallback, error) {
	if credential.DisableHostKeyChecking || e.cfg.DisableHostKeyChecking {
		return cryptossh.InsecureIgnoreHostKey(), nil
	}
	path := strings.TrimSpace(credential.KnownHostsPath)
	if path == "" {
		path = strings.TrimSpace(e.cfg.KnownHostsPath)
	}
	if path == "" {
		currentUser, err := user.Current()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(currentUser.HomeDir, ".ssh", "known_hosts")
	}
	return knownhosts.New(path)
}

func credentialAuthMethods(privateKeyPath string, credential CredentialConfig) ([]cryptossh.AuthMethod, error) {
	var methods []cryptossh.AuthMethod
	if strings.TrimSpace(privateKeyPath) != "" {
		privateKey, err := os.ReadFile(privateKeyPath)
		if err != nil {
			return nil, err
		}
		var signer cryptossh.Signer
		if strings.TrimSpace(credential.Passphrase) != "" {
			signer, err = cryptossh.ParsePrivateKeyWithPassphrase(privateKey, []byte(credential.Passphrase))
		} else {
			signer, err = cryptossh.ParsePrivateKey(privateKey)
		}
		if err != nil {
			return nil, err
		}
		methods = append(methods, cryptossh.PublicKeys(signer))
	} else if strings.TrimSpace(credential.PrivateKey) != "" {
		return nil, fmt.Errorf("private key must be materialized to a temp file before authentication")
	}
	if strings.TrimSpace(credential.Password) != "" {
		methods = append(methods, cryptossh.Password(credential.Password))
	}
	return methods, nil
}

func destinationParts(targetHost string, defaultUser string, credentialUser string, credentialPort int) (string, string, int) {
	targetHost = strings.TrimSpace(targetHost)
	userName := strings.TrimSpace(credentialUser)
	if userName == "" {
		userName = strings.TrimSpace(defaultUser)
	}
	if at := strings.LastIndex(targetHost, "@"); at >= 0 && at < len(targetHost)-1 {
		if prefix := strings.TrimSpace(targetHost[:at]); prefix != "" {
			userName = prefix
		}
		targetHost = targetHost[at+1:]
	}
	port := credentialPort
	if host, parsedPort, err := net.SplitHostPort(targetHost); err == nil {
		targetHost = strings.Trim(host, "[]")
		if parsed, err := strconv.Atoi(parsedPort); err == nil {
			port = parsed
		}
	}
	return userName, strings.Trim(targetHost, "[]"), port
}

func quoteForShell(command string) string {
	return "'" + strings.ReplaceAll(command, "'", `'"'"'`) + "'"
}
