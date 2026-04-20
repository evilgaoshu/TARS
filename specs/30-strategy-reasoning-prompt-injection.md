# TARS Reasoning Prompt 注入

## 目标

TARS 平台本身不负责生成具体命令模板，命令候选由 LLM 生成。
平台负责的事情是：

- 结构化解析 `summary / execution_hint`
- 对明显不合规的命令形态做最小过滤
- 在执行前走授权匹配、审批和执行策略

这意味着提示词是命令生成质量的主要控制面。

## 当前运行时能力

- 支持通过 `TARS_REASONING_PROMPTS_CONFIG_PATH` 注入 prompt 配置文件
- 支持通过 `TARS_DESENSITIZATION_CONFIG_PATH` 注入脱敏配置文件
- 支持自定义：
  - `system_prompt`
  - `user_prompt_template`
- 默认关闭本地命令生成兜底：
  - `TARS_REASONING_LOCAL_COMMAND_FALLBACK_ENABLED=false`
- 如需回退旧逻辑，可显式打开本地兜底
- 脱敏策略会在发给 LLM 前处理敏感值：
  - `host / IP / path` 使用占位符
  - `password / token / secret / api key / bearer / basic auth` 统一替换为 `[REDACTED]`
- 脱敏规则当前已支持配置化：
  - secret key/query key
  - additional regex patterns
  - HOST/IP/PATH 占位策略
  - host/ip/path 回填策略
  - 预留 `local_llm_assist` 配置
- 敏感值不会回填到执行草稿：
  - `host / IP / path` 可在审批前按 `desense_map` 有限回填
  - `password / token / secret / api key` 永不回填
  - 若模型返回的 `execution_hint` 仍包含 `[REDACTED]` 或明显密钥片段，平台会直接丢弃该命令候选

## 配置样例

见 [reasoning_prompts.example.yaml](../configs/reasoning_prompts.example.yaml)。

脱敏规则样例见 [desensitization.example.yaml](../configs/desensitization.example.yaml)。

```yaml
reasoning:
  system_prompt: |
    You are TARS, an operations copilot.
    Return ONLY strict JSON with fields: summary, execution_hint.
    The platform will handle authorization, approval, and execution policy after generation.
    If a shell command can answer the operator request, provide it directly.
    If no shell command is appropriate, use an empty string.

  user_prompt_template: |
    session_id={{ .SessionID }}
    context={{ .ContextJSON }}
```

## 模板变量

`user_prompt_template` 当前支持：

- `{{ .SessionID }}`
- `{{ .Context }}`
- `{{ .ContextJSON }}`

建议优先使用 `ContextJSON`，这样不会和后续上下文字段扩展冲突。

## 推荐原则

- 把“生成命令”放进 system prompt 明确写死
- 对本地小模型（如 LM Studio 里跑的 3B/4B/7B 级模型）增加少量 few-shot examples，效果会明显稳定
- 把“平台后续会授权匹配，不必在模型里重复做审批判断”写清楚
- 要求输出单条 shell 命令，不要输出解释性 prose
- 把“合法的只读运维查询范围”写清楚，例如负载、磁盘、端口、服务状态、IP、MAC、网卡、路由，避免模型把系统信息查询误判成拒答场景
- 若不适合执行命令，要求返回空字符串，而不是自然语言建议
- 明确告诉模型不要在 `summary` 或 `execution_hint` 中复述明文密码、token、api key
- 明确告诉模型：
  - 如果 `host / instance` 已经在 context 中，不要生成 `ssh ...`
  - 不要生成 `sudo ...`
  - 优先返回平台容易授权的只读命令形态

推荐针对小模型补一组固定示例，例如：

```json
{"summary":"检查目标主机系统负载","execution_hint":"uptime && cat /proc/loadavg"}
{"summary":"检查目标主机磁盘使用情况","execution_hint":"df -h"}
{"summary":"检查 sshd 服务状态","execution_hint":"systemctl status sshd --no-pager --lines=20 || true"}
{"summary":"查询目标主机出口 IP","execution_hint":"curl -fsS https://api.ipify.org && echo"}
```

## 与授权策略的关系

- prompt 决定“模型会生成什么命令”
- [命令与能力授权策略](30-strategy-command-authorization.md) 决定“这个命令后续怎么处理”
  - `direct_execute`
  - `require_approval`
  - `suggest_only`
  - `deny`

两者不要混在一起。
prompt 负责提高命令生成质量，授权策略负责控制运行风险。

## 审计记录

每次调用模型前，平台会记录一条 `llm_request / chat_completions_send` 审计事件，包含：

- `user_prompt_raw`
- `user_prompt_sent`
- `request_raw`
- `request_sent`

其中：

- `raw` 表示平台渲染出的原始 prompt/request
- `sent` 表示实际发给 LLM 的脱敏版本

这两份都会进入审计，方便排查“模型为什么这么回答”和“模型实际看到了什么”。

## 本地 LLM 辅助脱敏

当前 `local_llm_assist` 已支持 `detect_only`：

- `enabled`
- `provider`
- `base_url`
- `model`
- `mode`

启用后，本地可信 LLM 会先接收原始上下文，返回 `secrets / hosts / ips / paths` 四类精确值，平台再统一执行替换。主链路仍以**规则式脱敏**为最终安全边界，本地 LLM 辅助脱敏不作为唯一控制面。详细设计见 [30-strategy-desensitization.md](30-strategy-desensitization.md)。

当前支持的 `provider`：

- `openai_compatible`
- `anthropic`
- `ollama`
- `lmstudio`
