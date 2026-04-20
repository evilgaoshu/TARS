# TARS 脱敏策略与本地 LLM 辅助设计

## 目标

TARS 在调用模型前，需要同时满足两件事：

- 不把密码、token、API key 这类敏感值直接暴露给外部模型
- 保留足够的主机、IP、路径语义，支持诊断、审批、执行和审计排查

当前运行时采用：

- **规则式脱敏作为强制主链路**
- **有限回填用于可读性**
- **本地 LLM 辅助脱敏作为增强层，当前已支持 `detect_only`**

## 当前运行时能力

当前已支持通过 `TARS_DESENSITIZATION_CONFIG_PATH` 加载并热更新脱敏配置文件。

配置能力包括：

- `secrets`
  - `key_names`
  - `query_key_names`
  - `additional_patterns`
  - `redact_bearer`
  - `redact_basic_auth_url`
  - `redact_sk_tokens`
- `placeholders`
  - `host_key_fragments`
  - `path_key_fragments`
  - `replace_inline_ip`
  - `replace_inline_host`
  - `replace_inline_path`
- `rehydration`
  - `host`
  - `ip`
  - `path`
- `local_llm_assist`
  - `enabled`
  - `provider`
  - `base_url`
  - `model`
  - `mode`

## 默认行为

### 1. Secret 类

以下内容会在发给模型前替换为 `[REDACTED]`：

- `password=...`
- `token=...`
- `secret=...`
- `api_key=...`
- URL query 中的 `token / secret / api_key`
- `Authorization: Bearer ...`
- `https://user:password@example.com`
- `sk-...`

这些值**永不回填**。

### 2. 定位信息类

以下内容会转换为稳定占位符：

- 主机 / 域名 -> `[HOST_n]`
- IP -> `[IP_n]`
- 路径 / 文件 -> `[PATH_n]`

这些值是否回填，由 `rehydration` 控制。

## 有限回填规则

当前推荐默认值：

```yaml
rehydration:
  host: true
  ip: true
  path: true
```

用途是让：

- `AI Diagnosis Summary`
- 审批消息
- 执行草稿

保留足够的业务可读性。

如果你希望更严格，可以关闭其中任一项，例如：

```yaml
rehydration:
  host: true
  ip: true
  path: false
```

这样模型返回中的 `[PATH_1]` 不会在摘要和执行草稿里恢复成真实路径。

## 本地 LLM 辅助脱敏

### 原则

本地 LLM **不应替代规则式脱敏**。

推荐方式是：

1. 规则式脱敏先跑，作为安全底线
2. 本地 LLM 只做补充识别
3. 最终替换和是否回填，仍由平台规则控制

### 当前状态

`local_llm_assist` 当前已支持 `detect_only`：

- 可以配置并热加载
- 会出现在 `/setup` 与 `/ops`
- 启用后会在调用主模型前，先把原始上下文发送到本地可信 LLM 做“精确值识别”
- 当前支持的接入协议是 `openai_compatible`、`anthropic`、`ollama`、`lmstudio`
- 平台会根据识别结果做统一替换
- 当前主链路**不会把它作为唯一脱敏器**
- 只有在 `local_llm_assist.enabled=true` 时才会启用；单独配置 Provider Registry 的 `assist` 绑定不会自动开启 raw-context 检测

### 当前能力

当前按 `detect_only` 模式工作：

- 输入：原始上下文
- 输出：`secrets / hosts / ips / paths` 四类**精确字符串值**
- 平台：
  - 先跑规则式脱敏
  - 再合并本地 LLM 的补充识别结果
  - 统一执行替换
  - 继续保留 `request_raw / request_sent` 双轨审计

失败时：

- 本地 LLM 不可达、返回非法 JSON、配置缺失、模式不支持
- 都会直接回退到纯规则式脱敏
- 不会阻断主诊断链路

## 审计要求

平台当前已记录：

- `request_raw`
- `request_sent`
- `user_prompt_raw`
- `user_prompt_sent`
- `local_llm_desensitization_detect_send`
- `local_llm_desensitization_detect_result`

所以可以同时回答两类问题：

- 平台原始收到的是什么
- 模型实际看到的是什么

这两份都需要保留，便于审计和排障。

## Web Console

当前 `/ops` 已支持：

- 引导式表单编辑脱敏规则
- Advanced YAML 直接编辑
- 热加载

当前 `/setup` 已支持：

- 查看脱敏配置是否已加载
- 查看是否启用 `local_llm_assist`
- 查看 `base_url / model / mode`

## 样例

见 [desensitization.example.yaml](../configs/desensitization.example.yaml)。

---

## 安全回归测试覆盖（已落地，2026-03-27）

脱敏策略的关键边界已有固定自动化验证：

**测试文件**：`internal/api/http/security_regression_test.go`

**运行入口**：`make security-regression`

**覆盖的脱敏边界**：

| 边界 | 测试 | 说明 |
|------|------|------|
| 配置 API 不暴露明文 secret | `TestSecurityConfigAPIDoesNotExposeSecrets` | 配置 API 响应中不含 `Bearer `、`token=`、`password=`、`secret=`、`api_key=` 前缀字符串 |

**说明**：

- `TestSecurityConfigAPIDoesNotExposeSecrets` 通过 `/api/v1/configs` GET 端点验证，响应 body 中不含 5 种敏感字符串前缀
- 这是最小可验证的脱敏回归：保证平台配置接口不因重构或新增字段而意外返回明文凭据
- 更细粒度的 LLM 请求/响应脱敏验证依赖 live 环境，属于 L3/L4 测试范畴
