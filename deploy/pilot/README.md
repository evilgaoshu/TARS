# TARS 试点交付包

这份目录是给试点环境直接落地用的最小交付包，目标是把“看文档自己拼配置”收敛成“拿样例改值就能跑”。

## 包含内容

- [pilot.env.example](pilot.env.example)：试点环境变量模板
- [providers.example.yaml](providers.example.yaml)：统一模型 Provider 样例
- [golden_cases.md](golden_cases.md)：建议在试点环境逐条回归的 golden cases
- [golden_path_alert_v1.json](golden_path_alert_v1.json)：官方黄金路径告警回放样本
- [golden_path_telegram_callback_v1.json](golden_path_telegram_callback_v1.json)：官方黄金路径审批回调样本

## 配套配置

这几个配置样例继续复用仓库根目录 `configs/`：

- 审批路由：[approvals.example.yaml](../../configs/approvals.example.yaml)
- Reasoning Prompt：[reasoning_prompts.example.yaml](../../configs/reasoning_prompts.example.yaml)
- 脱敏策略：[desensitization.example.yaml](../../configs/desensitization.example.yaml)
- 命令授权策略：[authorization_policy.vnext.example.yaml](../../configs/authorization_policy.vnext.example.yaml)

## 建议使用方式

1. 复制 [pilot.env.example](pilot.env.example) 到目标环境，补齐 secret 和地址。
2. 把 [providers.example.yaml](providers.example.yaml) 与 `configs/*.example.yaml` 放到 `/etc/tars/` 或等价目录。
3. 先按 [pilot_runbook.md](../../docs/operations/pilot_runbook.md) 做启动和 smoke。
4. 演示前先跑 [pilot_hygiene_check.sh](../../scripts/pilot_hygiene_check.sh) 做环境卫生检查。
5. 再按 [golden_cases.md](golden_cases.md) 逐条验收，或用 [demo_acceptance_script.md](../../docs/operations/demo_acceptance_script.md) 直接演示。
6. 如果需要本地/共享环境可重复回放的官方黄金路径，直接运行 [run_golden_path_replay.sh](../../scripts/run_golden_path_replay.sh)。

## 环境卫生解释

- `pilot_hygiene_check.sh` 优先以 Ops API 为准。
- `TARS_SERVER_BASE_URL` 对应的 `/healthz`、`/readyz` 检查是可选项；如果服务只在内网 listener 上暴露公开健康接口，脚本会给出 warning，但不会中断整份报告。
- 如果报告出现 `failed_outbox > 0`，要区分“历史失败样本”和“当前系统仍在失败”：
  - 历史 `session.closed` UTF-8 错误：来自旧版本知识沉淀实现，升级后新会话不应再出现。
  - 历史 `telegram.send chat not found`：通常是旧测试 chat_id 或失效路由留下的 outbox。
  - 正式演示前，建议先人工确认这些 failed outbox 是否为历史样本；必要时 replay 或清理后再开始对外演示。
