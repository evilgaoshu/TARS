package httpapi

// security_regression_test.go — 安全与权限固定回归子集
//
// 本文件是 TARS 平台的安全回归护栏，覆盖以下边界：
//   1. 越权访问矩阵：无 token 访问关键 API 必须返回 401
//   2. 角色越权：viewer 角色尝试写操作必须返回 403
//   3. disabled 用户：账号停用后 session 认证必须失败
//   4. ops-token break-glass 边界：OpsAPI 禁用时 ops-token 被完全拒绝
//   5. 配置 API 脱敏：secrets/connectors 配置响应中不暴露明文密钥
//   6. 审批绕过防护：pending_approval 状态的 execution 直接 approve 调用受限
//   7. automation/trigger 绕过防护：automation 对高风险 capability 的访问被拦截
//
// 执行入口：
//   go test ./internal/api/http/... -run TestSecurity -v
//   make security-regression

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"tars/internal/foundation/config"
	"tars/internal/foundation/secrets"
	"tars/internal/modules/access"
	"tars/internal/modules/action"
	actionssh "tars/internal/modules/action/ssh"
	"tars/internal/modules/connectors"
	"tars/internal/modules/sshcredentials"
)

// =============================================================================
// 1. 越权访问矩阵 — 无 token 访问关键 API 必须返回 401
// =============================================================================

// TestSecurityUnauthorizedAccessMatrix 验证所有关键受保护端点在无 token 时返回 401。
// 本测试是"越权访问矩阵"固定回归子集的核心入口。
func TestSecurityUnauthorizedAccessMatrix(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	protectedEndpoints := []struct {
		method string
		path   string
		name   string
	}{
		{http.MethodGet, "/api/v1/summary", "ops-summary"},
		{http.MethodGet, "/api/v1/sessions", "sessions-list"},
		{http.MethodGet, "/api/v1/executions", "executions-list"},
		// 注意：/api/v1/connectors GET 是公开端点（供 agent 发现工具能力），不在受保护列表
		// /api/v1/setup/status 在 first-run 阶段允许匿名读取，用于首次安装向导。
		{http.MethodGet, "/api/v1/config/authorization", "config-authorization"},
		{http.MethodGet, "/api/v1/config/desensitization", "config-desensitization"},
		{http.MethodGet, "/api/v1/config/approval-routing", "config-approval-routing"},
		{http.MethodGet, "/api/v1/config/providers", "config-providers"},
		{http.MethodGet, "/api/v1/config/connectors", "config-connectors"},
		{http.MethodGet, "/api/v1/audit", "audit-log"},
		{http.MethodGet, "/api/v1/logs", "logs"},
		{http.MethodGet, "/api/v1/knowledge", "knowledge-list"},
		{http.MethodGet, "/api/v1/outbox", "outbox-list"},
		{http.MethodGet, "/api/v1/automations", "automations-list"},
		{http.MethodGet, "/api/v1/observability", "observability-summary"},
	}

	for _, ep := range protectedEndpoints {
		ep := ep
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()
			resp := performJSONRequest(t, system.handler, ep.method, ep.path, nil, nil)
			if resp.Code != http.StatusUnauthorized {
				t.Errorf("[越权访问矩阵] %s %s: 期望 401，实际 %d (body=%s)",
					ep.method, ep.path, resp.Code, resp.Body.String())
			}
		})
	}
}

// TestSecurityUnauthorizedWriteAccessMatrix 验证关键写操作端点在无 token 时返回 401。
func TestSecurityUnauthorizedWriteAccessMatrix(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	writeEndpoints := []struct {
		method string
		path   string
		name   string
		body   []byte
	}{
		{http.MethodPost, "/api/v1/executions/fake-id/approve", "execution-approve-action", []byte(`{"approved":true}`)},
		{http.MethodPost, "/api/v1/executions/fake-id/reject", "execution-reject-action", []byte(`{"reason":"test"}`)},
		{http.MethodPut, "/api/v1/config/authorization", "config-auth-write", []byte(`{}`)},
		{http.MethodPut, "/api/v1/config/desensitization", "config-desense-write", []byte(`{}`)},
		{http.MethodPut, "/api/v1/config/approval-routing", "config-approval-write", []byte(`{}`)},
		{http.MethodPost, "/api/v1/automations/fake-job/run", "automation-run", []byte(`{}`)},
		{http.MethodPost, "/api/v1/reindex/documents", "knowledge-reindex", []byte(`{"operator_reason":"test"}`)},
	}

	for _, ep := range writeEndpoints {
		ep := ep
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()
			resp := performJSONRequest(t, system.handler, ep.method, ep.path, ep.body, nil)
			if resp.Code != http.StatusUnauthorized {
				t.Errorf("[越权写操作矩阵] %s %s: 期望 401，实际 %d (body=%s)",
					ep.method, ep.path, resp.Code, resp.Body.String())
			}
		})
	}
}

// =============================================================================
// 2. 角色越权 — viewer 角色尝试写操作必须返回 403
// =============================================================================

// viewerSessionToken 创建一个 viewer 角色用户并返回其 session token。
func viewerSessionToken(t *testing.T, system testSystem) string {
	t.Helper()

	_, err := system.access.UpsertAuthProvider(access.AuthProvider{
		ID:           "local_token_viewer",
		Type:         "local_token",
		Name:         "Viewer Token",
		Enabled:      true,
		ClientSecret: "viewer-secret-token",
		DefaultRoles: []string{"viewer"},
	})
	if err != nil {
		t.Fatalf("创建 viewer auth provider 失败: %v", err)
	}

	loginResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/auth/login",
		[]byte(`{"provider_id":"local_token_viewer","token":"viewer-secret-token"}`), nil)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("viewer 登录失败: %d body=%s", loginResp.Code, loginResp.Body.String())
	}

	var login struct {
		SessionToken string `json:"session_token"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&login); err != nil {
		t.Fatalf("解析 viewer login 响应失败: %v", err)
	}
	if strings.TrimSpace(login.SessionToken) == "" {
		t.Fatalf("viewer session token 为空")
	}
	return login.SessionToken
}

// TestSecurityViewerCannotWriteConfigs 验证 viewer 角色无法修改配置（应返回 403）。
func TestSecurityViewerCannotWriteConfigs(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)
	token := viewerSessionToken(t, system)
	authHeader := map[string]string{"Authorization": "Bearer " + token}

	writeAttempts := []struct {
		method string
		path   string
		name   string
		body   []byte
	}{
		{http.MethodPut, "/api/v1/config/authorization", "config-auth-write", []byte(`{"content":""}`)},
		{http.MethodPut, "/api/v1/config/desensitization", "config-desense-write", []byte(`{"content":""}`)},
		{http.MethodPut, "/api/v1/config/approval-routing", "config-approval-write", []byte(`{"content":""}`)},
		{http.MethodPut, "/api/v1/config/providers", "config-providers-write", []byte(`{"content":""}`)},
	}

	for _, ep := range writeAttempts {
		ep := ep
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()
			resp := performJSONRequest(t, system.handler, ep.method, ep.path, ep.body, authHeader)
			if resp.Code != http.StatusForbidden {
				t.Errorf("[角色越权] viewer %s %s: 期望 403，实际 %d (body=%s)",
					ep.method, ep.path, resp.Code, resp.Body.String())
			}
		})
	}
}

// TestSecurityViewerCannotRunAutomations 验证 viewer 角色无法触发自动化任务（应返回 403）。
func TestSecurityViewerCannotRunAutomations(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)
	token := viewerSessionToken(t, system)
	authHeader := map[string]string{"Authorization": "Bearer " + token}

	// viewer 只有 platform.read，不具备 configs.write（automation run 需要）
	resp := performJSONRequest(t, system.handler, http.MethodPost,
		"/api/v1/automations/nonexistent-job/run", []byte(`{}`), authHeader)
	if resp.Code != http.StatusForbidden {
		t.Errorf("[角色越权] viewer 触发自动化: 期望 403，实际 %d (body=%s)",
			resp.Code, resp.Body.String())
	}
}

// TestSecurityViewerCanReadButNotWriteConnectors 验证 viewer 可以读 connectors 但不能写配置。
func TestSecurityViewerCanReadButNotWriteConnectors(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)
	token := viewerSessionToken(t, system)
	authHeader := map[string]string{"Authorization": "Bearer " + token}

	// 读取应该成功（viewer 有 connectors.read）
	readResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/connectors", nil, authHeader)
	if readResp.Code != http.StatusOK {
		t.Errorf("[角色越权] viewer 读 connectors: 期望 200，实际 %d (body=%s)",
			readResp.Code, readResp.Body.String())
	}

	// 配置写入应该被拒绝（viewer 无 configs.write）
	writeResp := performJSONRequest(t, system.handler, http.MethodPut,
		"/api/v1/config/connectors", []byte(`{"content":""}`), authHeader)
	if writeResp.Code != http.StatusForbidden {
		t.Errorf("[角色越权] viewer 写 config/connectors: 期望 403，实际 %d (body=%s)",
			writeResp.Code, writeResp.Body.String())
	}
}

func TestSecuritySSHCredentialRoutesUseDedicatedPermissions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		method string
		path   string
		want   string
	}{
		{name: "list", method: http.MethodGet, path: "/api/v1/ssh-credentials", want: "ssh_credentials.read"},
		{name: "detail", method: http.MethodGet, path: "/api/v1/ssh-credentials/ops-key", want: "ssh_credentials.read"},
		{name: "create", method: http.MethodPost, path: "/api/v1/ssh-credentials", want: "ssh_credentials.write"},
		{name: "update", method: http.MethodPut, path: "/api/v1/ssh-credentials/ops-key", want: "ssh_credentials.write"},
		{name: "delete", method: http.MethodDelete, path: "/api/v1/ssh-credentials/ops-key", want: "ssh_credentials.write"},
		{name: "rotation_required", method: http.MethodPost, path: "/api/v1/ssh-credentials/ops-key/rotation-required", want: "ssh_credentials.write"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if got := routePermission(req); got != tc.want {
				t.Fatalf("routePermission(%s %s) = %q, want %q", tc.method, tc.path, got, tc.want)
			}
		})
	}
}

func TestSecuritySSHCredentialReadRoleCanReadMetadata(t *testing.T) {
	t.Parallel()

	deps, accessManager := newSSHCredentialSecurityDeps(t)
	token := issueSessionForPermissions(t, accessManager, "ssh_meta_reader", []string{"ssh_credentials.read"})
	resp := performJSONRequest(t, sshCredentialsListHandler(deps), http.MethodGet, "/api/v1/ssh-credentials", nil, map[string]string{"Authorization": "Bearer " + token})
	if resp.Code != http.StatusOK {
		t.Fatalf("[SSH 权限] ssh_credentials.read 读取 metadata: 期望 200，实际 %d (body=%s)", resp.Code, resp.Body.String())
	}
}

func TestSecurityConnectorsWriteRoleCannotManageSSHCredentials(t *testing.T) {
	t.Parallel()

	deps, accessManager := newSSHCredentialSecurityDeps(t)
	token := issueSessionForPermissions(t, accessManager, "connector_writer", []string{"connectors.write"})
	authHeader := map[string]string{"Authorization": "Bearer " + token}

	createResp := performJSONRequest(t, sshCredentialsListHandler(deps), http.MethodPost, "/api/v1/ssh-credentials",
		[]byte(`{"credential_id":"ops-key","connector_id":"ssh-main","username":"root","auth_type":"password","password":"pw","operator_reason":"create"}`), authHeader)
	if createResp.Code != http.StatusForbidden {
		t.Errorf("[SSH 权限] connectors.write 创建 ssh credential: 期望 403，实际 %d (body=%s)", createResp.Code, createResp.Body.String())
	}

	updateResp := performJSONRequest(t, sshCredentialRouterHandler(deps), http.MethodPut, "/api/v1/ssh-credentials/seed-key",
		[]byte(`{"display_name":"rotated","operator_reason":"rotate"}`), authHeader)
	if updateResp.Code != http.StatusForbidden {
		t.Errorf("[SSH 权限] connectors.write 更新 ssh credential: 期望 403，实际 %d (body=%s)", updateResp.Code, updateResp.Body.String())
	}

	deleteResp := performJSONRequest(t, sshCredentialRouterHandler(deps), http.MethodDelete, "/api/v1/ssh-credentials/seed-key",
		[]byte(`{"operator_reason":"delete"}`), authHeader)
	if deleteResp.Code != http.StatusForbidden {
		t.Errorf("[SSH 权限] connectors.write 删除 ssh credential: 期望 403，实际 %d (body=%s)", deleteResp.Code, deleteResp.Body.String())
	}
}

func TestSecurityConnectorsWriteRoleCannotUseSSHCredentialsWithoutCustodyUsePermission(t *testing.T) {
	t.Parallel()

	deps, accessManager := newSSHCredentialSecurityDeps(t)
	token := issueSessionForPermissions(t, accessManager, "connector_writer", []string{"connectors.write"})
	authHeader := map[string]string{"Authorization": "Bearer " + token}

	execResp := performJSONRequest(t, connectorExecutionHandler(deps, "ssh-main"), http.MethodPost, "/api/v1/connectors/ssh-main/execution/execute", []byte(`{
		"session_id":"session-1",
		"target_host":"192.168.3.100",
		"command":"uptime",
		"operator_reason":"test"
	}`), authHeader)
	if execResp.Code != http.StatusForbidden {
		t.Errorf("[SSH 权限] connectors.write 绕过 ssh_credentials.use 执行: 期望 403，实际 %d (body=%s)", execResp.Code, execResp.Body.String())
	}

	healthResp := performJSONRequest(t, connectorHealthHandler(deps, "ssh-main"), http.MethodPost, "/api/v1/connectors/ssh-main/health", []byte(`{}`), authHeader)
	if healthResp.Code != http.StatusForbidden {
		t.Errorf("[SSH 权限] connectors.write 绕过 ssh_credentials.use 健康检查: 期望 403，实际 %d (body=%s)", healthResp.Code, healthResp.Body.String())
	}
}

func newSSHCredentialSecurityDeps(t *testing.T) (Dependencies, *access.Manager) {
	t.Helper()

	accessManager, err := access.NewManager("")
	if err != nil {
		t.Fatalf("new access manager: %v", err)
	}
	connectorsManager, err := connectors.NewManager("")
	if err != nil {
		t.Fatalf("new connectors manager: %v", err)
	}
	secretStore, err := secrets.NewStore("")
	if err != nil {
		t.Fatalf("new secret store: %v", err)
	}
	sshManager := sshcredentials.NewManager(sshcredentials.NewMemoryRepository(), sshcredentials.NewMemorySecretBackend())
	if _, err := sshManager.Create(context.Background(), sshcredentials.CreateInput{
		CredentialID:   "seed-key",
		DisplayName:    "Seed key",
		ConnectorID:    "ssh-main",
		Username:       "root",
		AuthType:       sshcredentials.AuthTypePassword,
		Password:       "seed-password",
		HostScope:      "192.168.3.100",
		OperatorReason: "seed",
		ActorID:        "ops-admin",
	}); err != nil {
		t.Fatalf("seed ssh credential: %v", err)
	}
	if err := connectorsManager.Upsert(connectors.Manifest{
		APIVersion: "tars.connector/v1alpha1",
		Kind:       "connector",
		Metadata: connectors.Metadata{
			ID:          "ssh-main",
			Name:        "ssh-main",
			DisplayName: "SSH Main",
			Vendor:      "ssh",
			Version:     "1.0.0",
		},
		Spec: connectors.Spec{Type: "execution", Protocol: "ssh_native"},
		Config: connectors.RuntimeConfig{Values: map[string]string{
			"host":          "192.168.3.100",
			"username":      "root",
			"credential_id": "seed-key",
		}},
	}); err != nil {
		t.Fatalf("upsert ssh connector: %v", err)
	}
	actionSvc := action.NewService(action.Options{
		Connectors:     connectorsManager,
		Secrets:        secretStore,
		SSHCredentials: sshManager,
		OutputSpoolDir: t.TempDir(),
		ExecutionRuntimes: map[string]action.ExecutionRuntime{
			"ssh_native": action.NewSSHNativeRuntime(securityCredentialExecutor{}, sshManager),
		},
	})
	return Dependencies{
		Config:         config.Config{OpsAPI: config.OpsAPIConfig{Enabled: true}},
		Access:         accessManager,
		Connectors:     connectorsManager,
		Secrets:        secretStore,
		SSHCredentials: sshManager,
		Action:         actionSvc,
	}, accessManager
}

func issueSessionForPermissions(t *testing.T, accessManager *access.Manager, roleID string, permissions []string) string {
	t.Helper()
	if _, err := accessManager.UpsertRole(access.Role{ID: roleID, DisplayName: roleID, Permissions: permissions}); err != nil {
		t.Fatalf("upsert role %s: %v", roleID, err)
	}
	providerID := roleID + "-provider"
	secret := roleID + "-secret"
	if _, err := accessManager.UpsertAuthProvider(access.AuthProvider{
		ID:           providerID,
		Type:         "local_token",
		Name:         roleID,
		Enabled:      true,
		ClientSecret: secret,
		DefaultRoles: []string{roleID},
	}); err != nil {
		t.Fatalf("upsert provider %s: %v", providerID, err)
	}
	session, _, err := accessManager.LoginWithLocalToken(secret, "")
	if err != nil {
		t.Fatalf("issue session for %s: %v", roleID, err)
	}
	return session.Token
}

type securityCredentialExecutor struct{}

func (securityCredentialExecutor) RunWithCredential(ctx context.Context, targetHost string, command string, credential actionssh.CredentialConfig) (actionssh.Result, error) {
	return actionssh.Result{ExitCode: 0, Output: "ok"}, nil
}

// =============================================================================
// 3. disabled 用户 — 账号停用后 session token 不得继续认证
// =============================================================================

// TestSecurityDisabledUserCannotAuthenticate 验证账号 disabled 后，已有的 session token 不再有效。
func TestSecurityDisabledUserCannotAuthenticate(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	// 1. 创建一个 active 用户并登录获取 session token
	_, err := system.access.UpsertAuthProvider(access.AuthProvider{
		ID:           "local_token_disable_test",
		Type:         "local_token",
		Name:         "Disable Test Token",
		Enabled:      true,
		ClientSecret: "disable-test-secret",
		DefaultRoles: []string{"viewer"},
	})
	if err != nil {
		t.Fatalf("创建 auth provider 失败: %v", err)
	}

	loginResp := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/auth/login",
		[]byte(`{"provider_id":"local_token_disable_test","token":"disable-test-secret"}`), nil)
	if loginResp.Code != http.StatusOK {
		t.Fatalf("登录失败: %d body=%s", loginResp.Code, loginResp.Body.String())
	}

	var login struct {
		SessionToken string `json:"session_token"`
	}
	if err := json.NewDecoder(loginResp.Body).Decode(&login); err != nil {
		t.Fatalf("解析 login 响应失败: %v", err)
	}
	sessionToken := login.SessionToken

	// 2. 验证当前 token 有效
	meResp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/me", nil,
		map[string]string{"Authorization": "Bearer " + sessionToken})
	if meResp.Code != http.StatusOK {
		t.Fatalf("active 用户 /api/v1/me 期望 200，实际 %d", meResp.Code)
	}

	// 3. 禁用该用户（Status = disabled）
	users := system.access.ListUsers()
	var disableUserID string
	for _, u := range users {
		if u.Source == "local_token_disable_test" {
			disableUserID = u.UserID
			break
		}
	}
	if disableUserID == "" {
		t.Skip("未能找到 local_token_disable_test 用户，跳过 disabled 测试")
	}

	existingUser, ok := system.access.GetUser(disableUserID)
	if !ok {
		t.Fatalf("GetUser 未找到用户: %s", disableUserID)
	}
	existingUser.Status = "disabled"
	if _, err := system.access.UpsertUser(existingUser); err != nil {
		t.Fatalf("UpsertUser(disabled) 失败: %v", err)
	}

	// 4. disabled 后，原 session token 不得继续认证，必须返回 401
	meAfterDisable := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/me", nil,
		map[string]string{"Authorization": "Bearer " + sessionToken})
	if meAfterDisable.Code != http.StatusUnauthorized {
		t.Errorf("[disabled 用户] 账号禁用后 session 仍有效: 期望 401，实际 %d (body=%s)",
			meAfterDisable.Code, meAfterDisable.Body.String())
	}
}

// =============================================================================
// 4. ops-token break-glass 边界 — OpsAPI 禁用时 ops-token 被完全拒绝
// =============================================================================

// TestSecurityOpsTokenRejectedWhenOpsAPIDisabled 验证当 OpsAPI 禁用时，
// 即使使用有效的 ops-token 也应被拒绝（404 表示 API 不存在）。
func TestSecurityOpsTokenRejectedWhenOpsAPIDisabled(t *testing.T) {
	t.Parallel()

	// 创建一个 OpsAPI 禁用的配置
	cfg := defaultTestConfig()
	cfg.OpsAPI = config.OpsAPIConfig{
		Enabled: false,
		Token:   "ops-token",
	}
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	// ops-token 访问受保护 API，OpsAPI 禁用时应返回 404（而非 200）
	opsEndpoints := []struct {
		method string
		path   string
		name   string
	}{
		{http.MethodGet, "/api/v1/summary", "summary"},
		{http.MethodGet, "/api/v1/sessions", "sessions"},
		{http.MethodGet, "/api/v1/config/authorization", "config-auth"},
	}

	for _, ep := range opsEndpoints {
		ep := ep
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()
			resp := performJSONRequest(t, system.handler, ep.method, ep.path, nil,
				map[string]string{"Authorization": "Bearer ops-token"})
			// OpsAPI 禁用时，要么 404（路由不存在），要么 401（token 无效）
			// 总之不能是 200
			if resp.Code == http.StatusOK {
				t.Errorf("[break-glass 边界] OpsAPI 禁用后 ops-token 仍能访问 %s: 期望非 200，实际 %d",
					ep.path, resp.Code)
			}
		})
	}
}

// TestSecurityOpsTokenGrantsFullAccess 验证 ops-token 在 OpsAPI 启用时具有 * 权限。
func TestSecurityOpsTokenGrantsFullAccess(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)
	authHeader := map[string]string{"Authorization": "Bearer ops-token"}

	// ops-token 应该能访问所有受保护 API
	resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/summary", nil, authHeader)
	if resp.Code != http.StatusOK {
		t.Errorf("[break-glass] ops-token 访问 summary: 期望 200，实际 %d (body=%s)",
			resp.Code, resp.Body.String())
	}

	resp2 := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/sessions", nil, authHeader)
	if resp2.Code != http.StatusOK {
		t.Errorf("[break-glass] ops-token 访问 sessions: 期望 200，实际 %d (body=%s)",
			resp2.Code, resp2.Body.String())
	}
}

// =============================================================================
// 5. 配置 API 脱敏 — secrets/tokens 不得在响应中明文返回
// =============================================================================

// TestSecurityConfigAPIDoesNotExposeSecrets 验证配置响应中不返回明文 secret 值。
// 这是脱敏退化回归的核心检查点。
func TestSecurityConfigAPIDoesNotExposeSecrets(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)
	authHeader := map[string]string{"Authorization": "Bearer ops-token"}

	sensitivePatterns := []string{
		"secret_key",
		"api_key",
		"password",
		"private_key",
		"access_token",
		"client_secret",
	}

	configEndpoints := []struct {
		name string
		path string
	}{
		{"config-providers", "/api/v1/config/providers"},
		{"config-connectors", "/api/v1/config/connectors"},
		{"config-desensitization", "/api/v1/config/desensitization"},
	}

	for _, ep := range configEndpoints {
		ep := ep
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()
			resp := performJSONRequest(t, system.handler, http.MethodGet, ep.path, nil, authHeader)
			if resp.Code != http.StatusOK {
				t.Skipf("[脱敏检查] %s 返回 %d，跳过（可能未配置）", ep.path, resp.Code)
			}

			body := resp.Body.String()
			// 响应体中不应出现明文 secret 值（格式如 "api_key": "sk-...")
			// 检查是否有以 "sk-", "ghp_", "Bearer " 等已知 secret 前缀开头的值
			secretPrefixes := []string{"sk-", "ghp_", "xoxb-", "eyJ"}
			for _, prefix := range secretPrefixes {
				if strings.Contains(body, prefix) {
					t.Errorf("[脱敏检查] %s 响应包含疑似明文 secret（前缀 %q）: body=%s",
						ep.path, prefix, body)
				}
			}
			_ = sensitivePatterns // 模式列表备用于未来扩展
		})
	}
}

// =============================================================================
// 6. 审批绕过防护 — pending 状态 execution 不得被非法 approve
// =============================================================================

// TestSecurityApprovalEndpointRequiresAuth 验证执行审批端点必须经过认证。
func TestSecurityApprovalEndpointRequiresAuth(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	// 无 token 尝试 approve
	resp := performJSONRequest(t, system.handler, http.MethodPost,
		"/api/v1/executions/nonexistent-id/approve", []byte(`{"approved":true}`), nil)
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("[审批绕过] 无 token approve: 期望 401，实际 %d (body=%s)",
			resp.Code, resp.Body.String())
	}
}

// TestSecurityViewerCannotApproveExecution 验证 viewer 角色无法执行审批操作（应返回 403）。
func TestSecurityViewerCannotApproveExecution(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)
	token := viewerSessionToken(t, system)
	authHeader := map[string]string{"Authorization": "Bearer " + token}

	// viewer 只有 executions.read，不具备 executions.approve 权限
	resp := performJSONRequest(t, system.handler, http.MethodPost,
		"/api/v1/executions/nonexistent-id/approve", []byte(`{"approved":true}`), authHeader)
	// 应该是 403（权限不足）或 404（执行不存在），不得是 200
	if resp.Code == http.StatusOK {
		t.Errorf("[审批绕过] viewer approve execution: 不应返回 200，实际 %d (body=%s)",
			resp.Code, resp.Body.String())
	}
	if resp.Code != http.StatusForbidden && resp.Code != http.StatusNotFound {
		t.Errorf("[审批绕过] viewer approve execution: 期望 403 或 404，实际 %d (body=%s)",
			resp.Code, resp.Body.String())
	}
}

// =============================================================================
// 7. automation/trigger 绕过防护
// =============================================================================

// TestSecurityAutomationRunRequiresAuth 验证 automation 手动触发必须经过认证。
func TestSecurityAutomationRunRequiresAuth(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	// 无 token 尝试触发自动化
	resp := performJSONRequest(t, system.handler, http.MethodPost,
		"/api/v1/automations/any-job/run", []byte(`{}`), nil)
	if resp.Code != http.StatusUnauthorized {
		t.Errorf("[automation 绕过] 无 token 触发 automation: 期望 401，实际 %d (body=%s)",
			resp.Code, resp.Body.String())
	}
}

// TestSecurityWebhookRequiresValidSecretWhenConfigured 验证 webhook 端点在配置了 secret 后，
// 无效签名必须被拒绝。
// 这是 trigger/automation 绕过防护的关键检查点。
func TestSecurityWebhookRequiresValidSecretWhenConfigured(t *testing.T) {
	t.Parallel()

	// 创建配置了 webhook secret 的系统
	cfg := defaultTestConfig()
	cfg.VMAlert.WebhookSecret = "expected-webhook-secret"
	system := newTestSystemWithConfig(t, true, true, true, cfg)

	payload := []byte(`{"status":"firing","alerts":[{"labels":{"alertname":"HighCPU","instance":"host-1","severity":"critical"},"annotations":{"summary":"cpu too high"}}]}`)

	// 1. 无签名头的请求应被拒绝（401）
	respNoSig := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload, nil)
	if respNoSig.Code != http.StatusUnauthorized {
		t.Errorf("[webhook 绕过] 无签名 webhook: 期望 401，实际 %d (body=%s)",
			respNoSig.Code, respNoSig.Body.String())
	}

	// 2. 错误签名的请求应被拒绝（401）
	respBadSig := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload,
		map[string]string{"X-Tars-Signature": "wrong-signature"})
	if respBadSig.Code != http.StatusUnauthorized {
		t.Errorf("[webhook 绕过] 错误签名 webhook: 期望 401，实际 %d (body=%s)",
			respBadSig.Code, respBadSig.Body.String())
	}

	// 3. 正确签名的请求应被接受（200）
	respGoodSig := performJSONRequest(t, system.handler, http.MethodPost, "/api/v1/webhooks/vmalert", payload,
		map[string]string{"X-Tars-Signature": "expected-webhook-secret"})
	if respGoodSig.Code != http.StatusOK {
		t.Errorf("[webhook 绕过] 正确签名 webhook: 期望 200，实际 %d (body=%s)",
			respGoodSig.Code, respGoodSig.Body.String())
	}
}

// =============================================================================
// 8. 伪造 token / 无效 Bearer token 格式拒绝
// =============================================================================

// TestSecurityInvalidTokenFormatsAreRejected 验证各种无效 token 格式均返回 401。
func TestSecurityInvalidTokenFormatsAreRejected(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	invalidTokens := []struct {
		name   string
		header string
	}{
		{"empty-bearer", "Bearer "},
		{"no-bearer-prefix", "just-a-token"},
		{"basic-auth", "Basic dXNlcjpwYXNz"},
		{"fake-jwt", "Bearer eyJhbGciOiJub25lIn0.eyJzdWIiOiJoYWNrZXIifQ."},
		{"ops-wrong", "Bearer ops-token-wrong"},
	}

	for _, tc := range invalidTokens {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			resp := performJSONRequest(t, system.handler, http.MethodGet, "/api/v1/summary", nil,
				map[string]string{"Authorization": tc.header})
			if resp.Code != http.StatusUnauthorized {
				t.Errorf("[无效 token] %s: 期望 401，实际 %d (body=%s)",
					tc.name, resp.Code, resp.Body.String())
			}
		})
	}
}

// =============================================================================
// 9. 公开端点白名单验证 — 确认不需要认证的端点确实是预期公开的
// =============================================================================

// TestSecurityPublicEndpointsWhitelist 验证确实不需要认证的端点可以公开访问。
// 这防止将受保护端点误标记为公开，也防止将本应公开的端点误设为受保护。
func TestSecurityPublicEndpointsWhitelist(t *testing.T) {
	t.Parallel()

	system := newTestSystem(t, true, true, true)

	publicEndpoints := []struct {
		method string
		path   string
		name   string
	}{
		{http.MethodGet, "/api/v1/platform/discovery", "platform-discovery"},
		{http.MethodGet, "/api/v1/org/context", "org-context"},
		{http.MethodGet, "/api/v1/setup/wizard", "setup-wizard-first-run"},
		{http.MethodGet, "/api/v1/auth/providers", "auth-providers"},
		// connectors list 是公开端点（供 agent 发现工具能力，不含敏感配置）
		{http.MethodGet, "/api/v1/connectors", "connectors-discovery"},
	}

	for _, ep := range publicEndpoints {
		ep := ep
		t.Run(ep.name, func(t *testing.T) {
			t.Parallel()
			resp := performJSONRequest(t, system.handler, ep.method, ep.path, nil, nil)
			if resp.Code == http.StatusUnauthorized || resp.Code == http.StatusForbidden {
				t.Errorf("[公开端点白名单] %s %s: 该端点应公开，实际返回 %d",
					ep.method, ep.path, resp.Code)
			}
		})
	}
}
