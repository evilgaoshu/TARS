# TARS 命令与能力授权策略

> **状态**: 部分已落地，继续演进中  
> **当前运行时现状**: 运行时已支持 `ssh_command` 的授权策略文件、glob 匹配和 `direct_execute / require_approval / suggest_only / deny` 四种动作；MCP skill 与插件动作仍是后续阶段  
> **配套样例**: [authorization_policy.vnext.example.yaml](../configs/authorization_policy.vnext.example.yaml)

## 1. 目标

这份文档定义 TARS 后续统一的“命令与能力授权策略”，覆盖：

- SSH 命令
- 后续 MCP skill / tool 调用
- 再往后的插件动作授权

目标不是把所有安全判断都交给 LLM，而是给系统一层稳定、可配置、可审计的授权决策：

- 白名单：直接执行，不需要审批
- 黑名单：默认只展示建议，提醒人工处理
- 其他：进入审批后再执行

同时保留更细的覆盖能力：

- 某些黑名单项可以改成“允许审批后执行”
- 不同服务、主机、渠道、skill 可以使用不同策略
- 所有决策都要可审计、可回放

## 2. 当前 MVP 已实现的行为

当前运行时已经具备以下基础控制：

1. 主机白名单：`TARS_SSH_ALLOWED_HOSTS`
2. 全局命令前缀 allowlist：`TARS_SSH_ALLOWED_COMMAND_PREFIXES`
3. 全局危险片段 blocklist：`TARS_SSH_BLOCKED_COMMAND_FRAGMENTS`
4. 服务级命令 allowlist：`approvals.yaml -> approval.execution.command_allowlist.<service>`
5. 审批路由：`approvals.yaml -> approval.routing.*`
6. 新的授权策略文件：`TARS_AUTHORIZATION_CONFIG_PATH`

当前限制也要说清楚：

- `ssh_command` 已支持通配符规则引擎，但目前只在 SSH 命令路径生效
- 还没有把 SSH 命令和 MCP skill 全部放进同一套运行时模型
- 现有 env allowlist/blocklist 仍保留，主要作为兼容配置和底层 hard guardrails

所以这份文档里既包含**当前已落地能力**，也包含**下一步演进方向**。

## 3. 分层原则

后续建议把授权分成两层，而不是只保留一个“黑白名单”概念。

### 3.1 Hard Guardrails

这层是底线保护，不建议给普通运维配置页开放覆盖：

- 非授权主机永远不能执行
- 明显破坏性命令可以直接 `deny`
- 不允许通过策略绕过 SSH 身份、审计、输出留存

适用示例：

- `rm -rf /`
- `mkfs*`
- `dd if=/dev/zero*`

### 3.2 Authorization Policy

这层决定“用户体验和授权路径”：

- 直接执行
- 进入审批
- 只给建议，提醒人工执行
- 直接拒绝

这一层才是后续后台配置、MCP skill 授权和策略调整的主体。

## 4. 统一动作模型

建议统一成以下四种动作。

| 动作 | 含义 | 典型场景 |
|------|------|----------|
| `direct_execute` | 直接执行，无需审批；仍保留审计与输出 | 只读安全查询 |
| `require_approval` | 生成执行请求，审批通过后执行 | 重启服务、变更类动作 |
| `suggest_only` | 只展示命令或 skill 建议，不自动执行 | 高风险但允许人工线下操作 |
| `deny` | 直接拒绝，不给执行入口 | 极高风险或越权操作 |

## 5. 默认策略

推荐默认值就是你提出的这套：

- 白名单 -> `direct_execute`
- 黑名单 -> `suggest_only`
- 其他 -> `require_approval`

同时增加两个补充规则：

1. 很小一组 `hard_deny` 继续保留，不能被普通规则覆盖
2. 黑名单项允许通过更具体的覆盖规则改成 `require_approval`

换句话说：

- “黑名单”在策略层默认代表“不要自动执行”
- 但不是所有黑名单都必须永远禁止
- 真正永远禁止的，单独放在 `hard_deny`

## 6. 匹配模型与通配符

为了让配置足够方便，推荐使用 **glob**，不要求写正则。

### 6.1 规范化

命令匹配前先做最小规范化：

- 去掉首尾空白
- 连续空格折叠成一个空格
- 服务名、主机名、渠道名统一转小写

### 6.2 通配符规则

建议支持：

- `*`：匹配任意长度字符，允许跨空格
- `?`：匹配单个字符

示例：

- `systemctl status *`
- `journalctl -u *`
- `df -h*`
- `curl -fsS https://api.ipify.org*`
- `victoriametrics.query_*`
- `github.merge_*`

### 6.3 匹配顺序

为了保持可理解，规则按顺序评估，第一条命中即生效：

1. `hard_deny`
2. 更具体的 `overrides`
3. `whitelist`
4. `blacklist`
5. `unmatched_action`

这样配置直观，也方便后续在后台按顺序展示。

## 7. 推荐配置模型

为了“方便配置”，不建议一上来做特别抽象的策略 DSL。推荐采用：

- 一组默认动作
- 每类能力一个简单的 `whitelist / blacklist / overrides`
- `overrides` 支持按服务、主机、渠道、skill 进一步收口

推荐顶层结构：

```yaml
authorization:
  defaults:
    whitelist_action: direct_execute
    blacklist_action: suggest_only
    unmatched_action: require_approval
  hard_deny:
    ssh_command: [...]
    mcp_skill: [...]
  ssh_command:
    whitelist: [...]
    blacklist: [...]
    overrides: [...]
  mcp_skill:
    whitelist: [...]
    blacklist: [...]
    overrides: [...]
```

## 8. SSH 命令策略设计

### 8.1 推荐默认白名单

适合 `direct_execute` 的通常是只读命令：

- `hostname`
- `uptime`
- `cat /proc/loadavg`
- `df -h*`
- `free -m*`
- `systemctl status *`
- `systemctl is-active *`
- `journalctl -u *`
- `curl -fsS https://api.ipify.org*`

### 8.2 推荐默认黑名单

默认进入 `suggest_only`：

- `systemctl restart *`
- `systemctl stop *`
- `reboot*`
- `shutdown*`
- `iptables *`

说明：

- 这类命令默认不自动执行
- 但可以通过更具体的规则改成 `require_approval`

### 8.3 典型覆盖规则

示例：

- `systemctl restart sshd` 在 `sshd` 服务上允许“审批后执行”
- `systemctl restart nginx` 在 `web*` 服务上允许“审批后执行”
- 某个维护窗口里允许 `reboot*` 走审批，但平时仍然只建议人工操作

## 9. 当前落地范围

当前运行时已经落地的范围：

- `ssh_command`
- glob 匹配
- `direct_execute / require_approval / suggest_only / deny`
- 与现有 `Workflow Core` 状态机联动
- 与 `Action Gateway` 最终执行校验联动

当前还没有落地的范围：

- `mcp_skill`
- 插件动作统一授权
- MCP skill 外部源导入

## 10. MCP Skill 授权设计

后续 MCP skill 建议复用同一套动作模型，而不是另起一套授权语言。

### 9.1 MCP Skill 建议的匹配维度

- `server_globs`
- `skill_globs`
- `tool_globs`
- `services`
- `hosts`
- `channels`

### 9.2 MCP Skill 默认策略

建议默认保持和 SSH 命令一致：

- 白名单 skill -> `direct_execute`
- 黑名单 skill -> `suggest_only`
- 未命中 -> `require_approval`

### 9.3 MCP Skill 示例

适合白名单直执行的：

- `victoriametrics.query_*`
- `grafana.read_*`
- `kubernetes.get_*`

适合黑名单或审批的：

- `terraform.destroy*`
- `github.delete_*`
- `shell.exec_*`
- `github.merge_*`

### 10.4 MCP Skill 外部源

后续 MCP skill 不应只支持本地静态注册，还需要支持“配置外部源地址后导入”的形式。建议把它设计成统一的 source registry，而不是一次性脚本：

- source 类型：
  - `http_index`
  - `git_repo`
  - 后续可扩展到 `yum_like_repo`
- source 最小字段：
  - `source_id`
  - `source_type`
  - `base_url`
  - `auth_ref`
  - `enabled`
  - `last_synced_at`
- 导入结果：
  - 形成可审计的 skill 元数据
  - 再进入统一的 MCP skill 授权层

也就是说，“从哪里导入 skill” 和 “导入后是否允许执行” 是两层事：

- source registry 负责发现、同步、版本跟踪
- authorization policy 负责 `direct_execute / require_approval / suggest_only / deny`

## 11. 审计与知识沉淀要求

不管最终动作是什么，都应该保留统一留痕：

- 原始用户请求
- 规范化后的命令或 skill 标识
- 命中的规则 ID
- 最终动作（`direct_execute / require_approval / suggest_only / deny`）
- 审批路由与审批人
- 执行输出与 verifier 结果

这有两个目的：

1. 审计：回答“为什么这次能直接执行 / 为什么被拦住”
2. 学习改进：后续能基于真实对话、命令、结果沉淀知识或优化策略

## 12. 与当前架构的衔接

推荐这样演进，改动最小：

1. 保留当前 `host allowlist` 作为外层硬边界
2. 把现有 env allowlist/blocklist 抽到统一 `authorization` 解析层
3. 在 `Workflow Core` 决定 `ExecutionRequest` 前先跑一次策略决策
4. `Action Gateway` 继续做最后一道执行侧安全校验
5. MCP skill 接入时复用同一策略接口，而不是各写一套审批判断

## 13. 落地顺序建议

为了降低风险，建议分三步做：

1. 先支持 `ssh_command` 的 `whitelist / blacklist / overrides + glob`
2. 再把当前服务级 `command_allowlist` 迁到统一策略配置
3. 最后扩展到 `mcp_skill` 和其他插件能力

## 14. 结论

后续授权策略建议统一成：

- 当前运行时：已经支持 `ssh_command` 的统一 `authorization` 配置
- 当前运行时：已支持通过 Web Console `/ops` 页面以引导式表单或 Advanced YAML 读取、编辑并热加载 `authorization`
- 下一步演进：把同一策略语言扩展到 MCP skill 和插件能力
- 统一默认语义：
  - 白名单：直接执行
  - 黑名单：默认只建议人工执行
  - 其他：审批执行
- 对少量场景允许更细粒度覆盖：
  - 黑名单也可配置为审批执行
  - MCP skill 与 SSH 命令共用同一套动作模型
  - MCP skill 后续支持从外部源地址导入，再统一进入授权层

---

## 15. 安全回归测试覆盖（已落地，2026-03-27）

命令授权策略的关键边界已有固定自动化验证：

**测试文件**：`internal/api/http/security_regression_test.go`

**运行入口**：`make security-regression`

**覆盖的命令授权边界**：

| 策略层 | 测试 | 说明 |
|--------|------|------|
| Automation trigger 需要认证 | `TestSecurityAutomationRunRequiresAuth` | 无 token 触发自动化 → 401 |
| Viewer 不能触发 automation | `TestSecurityViewerCannotRunAutomations` | viewer trigger → 403（无 automations.write） |
| Webhook secret 校验 | `TestSecurityWebhookRequiresValidSecretWhenConfigured` | 配置 secret 后，无效签名 → 401 |
| Approval 端点保护 | `TestSecurityApprovalEndpointRequiresAuth`、`TestSecurityViewerCannotApproveExecution` | 审批端点需要认证，viewer 无权批准 |

这些测试保证命令授权策略不会因代码变更而静默退化：无论 automation、webhook trigger，还是 approval，都必须经过同一套认证和权限判断。
