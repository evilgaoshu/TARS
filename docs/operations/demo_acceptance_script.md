# TARS 正式验收演示脚本

这份脚本面向两种场景：

- 对外演示前，先快速确认环境没有明显脏状态
- 演示时，一键触发 smoke，然后跟踪到 `resolved / failed`

## 1. 演示前环境卫生检查

运行：

```sh
TARS_SERVER_BASE_URL=http://<tars-host>:8081 \
TARS_OPS_BASE_URL=http://<tars-host>:8081 \
TARS_OPS_API_TOKEN=<ops-api-token> \
./scripts/pilot_hygiene_check.sh
```

脚本会检查：

- `/healthz`
- `/readyz`
- `/api/v1/summary`
- `/api/v1/setup/status`
- `failed / blocked outbox`

如果输出 `result=attention_needed`，优先先处理历史失败 outbox，再开始正式演示。

补充说明：

- 如果脚本提示 `/healthz` 或 `/readyz` 失败，但 `/api/v1/summary` 和 `/api/v1/setup/status` 正常，这通常表示反向代理没有放行公共探针路径，而不是系统完全不可用。
- 历史试点环境 `192.168.3.106` 的基线报告里，`failed_outbox=6` 属于历史样本；当前本地开发测试机默认切到 `192.168.3.100` 后，演示前需要重新确认是否保留或清理。

## 2. 一键触发正式演示

运行：

```sh
TARS_OPS_BASE_URL=http://<tars-host>:8081 \
TARS_OPS_API_TOKEN=<ops-api-token> \
TARS_DEMO_ALERTNAME=TarsDemoManual \
TARS_DEMO_SERVICE=sshd \
TARS_DEMO_HOST=192.168.3.100 \
./scripts/run_demo_smoke.sh
```

脚本会：

1. 调用 `/api/v1/smoke/alerts`
2. 打印 `session_id`
3. 输出对应的 Web Session URL
4. 轮询 session 状态，直到：
   - `resolved`
   - `failed`
   - 或超时

## 2A. 官方黄金路径回放

如果目标不是临时 smoke，而是正式验收“官方黄金路径 v1”，优先运行：

```sh
TARS_OPS_BASE_URL=http://<tars-host>:8081 \
TARS_SERVER_BASE_URL=http://<tars-host>:8081 \
TARS_OPS_API_TOKEN=<ops-api-token> \
TARS_VMALERT_WEBHOOK_SECRET=<vmalert-secret> \
bash scripts/run_golden_path_replay.sh
```

如果希望在本地/共享环境一键自动推进审批闭环，可额外设置：

```sh
TARS_GOLDEN_AUTO_APPROVE=1
```

这条脚本会固定输出：

- `session_id`
- `headline`
- `conclusion`
- `next_action`
- `snapshot=status|execution_status|verification_status|notification_headline|execution_headline|notification_count`

适合做：

- 演示前快速确认黄金链路没有跑偏
- 共享环境回归
- 文档化验收
- 对比不同版本的值班体验是否退化

## 3. 推荐演示话术顺序

1. 打开 `/login`
2. 登录后切到 `/setup`
3. 先跑 `pilot_hygiene_check.sh`，说明环境健康
4. 再执行 `run_demo_smoke.sh`
5. 让观众同时看到：
   - Web Setup / Session Detail
   - Telegram diagnosis / approval / result
6. 在 Telegram 点击批准
7. 回到 Web 展示：
    - timeline
    - execution output
    - trace / audit / knowledge
    - golden path snapshot / notification reasons

补充说明：

- 如果当前授权策略把演示命令命中为 `direct_execute`，正式 smoke 会直接从 diagnosis 进入执行，不会停在 Telegram 审批。
- 如果演示目标是“必须展示审批按钮”，请选一条按当前策略会命中 `require_approval` 的样本，或临时把对应规则调成审批执行。
- 2026-03-19 已完成一条正式 JumpServer 审批样本，可作为参考基线：
  - Session: `6e190d44-d6c2-496a-bb26-8999a3cfda97`
  - Execution: `4d95b605-9003-4da4-80e1-3c78521d2965`
  - Connector: `jumpserver-main`
  - 最终结果：`resolved / verification=success`

## 4. 建议保留的演示样本

- `Setup -> Runtime Checks` 直通
- Telegram 对话：`host=192.168.3.100 看系统负载`
- Telegram 对话：`host=192.168.3.100 重启 sshd`
- provider failover 只做内部演示，不建议第一次对外演示就展示

其中 `host=192.168.3.100 重启 sshd` 最适合演示 “必须审批 + 走 JumpServer 官方执行连接器” 这条路径。
