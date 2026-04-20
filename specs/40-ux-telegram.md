# TARS — Telegram 交互设计规范 (UX Specs)

> **基准版本**: MVP Phase 1  
> **关联任务**: FE-1, FE-2, FE-3  
> **日期**: 2026-03-11

---

## 1. 诊断消息 (Diagnosis Message)

当告警经过 Reasoning Service 分析完成并建议执行操作时，或直接给出诊断建议时下发本消息。

### 1.1 页面数据结构 (ViewModel)

| 字段名 | 来源 DTO 映射 | 说明 |
|--------|---------------|------|
| `AlertIcon` | `severity` 映射 | 🔴 critical, 🟠 warning, ℹ️ info |
| `AlertName` | `alert.labels.alertname` | 告警名称 |
| `TargetContext` | `alert.labels.service` / `alert.labels.instance` | 服务或机器上下文 |
| `Summary` | `diagnosis_summary` | AI 生成的段落级诊断摘要 |
| `Citations` | 检索得到的引用数组 | (可选) 展示引用的 Runbook 链接或来源 |
| `Recommendation` | `executions[0].command` (如果存在) | AI 建议的一键执行命令描述 |

### 1.2 Telegram 消息模版范例 (HTML/Markdown 格式化)

```text
[TARS] 诊断
告警: HighCPUUsage @ prod-web-01
服务 web · 级别 critical
结论: CPU 持续高于 90%，Nginx worker 异常且连接数暴增，优先确认流量异常与服务存活状态。
下一步: systemctl status nginx
参考: 1 条知识
会话: ses_xxx
```

*(如果带有执行建议，诊断消息下方会附带下方的审批消息，或直接在包含执行时呈现为一条组合消息)*

### 1.2.1 消息模板自定义（后续阶段）

后续渠道平台不应把 Telegram 消息模板长期写死在代码中。

至少应支持：

- 诊断消息模板自定义
- 审批消息模板自定义
- 执行结果模板自定义
- 中英文模板变体
- 渠道 render profile 与兼容性覆写
- 团队/场景级覆盖

模板治理要求：

- 模板变量必须有固定白名单
- 模板修改必须有预览
- 模板应支持测试发送
- 模板变更必须有审计

**当前实现状态（2026-03-22）：**  
Web Console `/msg-templates` 路由已落地前端管理台（FE-25），但产品心智统一按 `Notification Templates` 表达。  
- 三类模板（diagnosis / approval / execution_result）× 两种语言（zh-CN / en-US）
- 变量白名单说明内置于编辑器
- 基于示例数据的预览渲染已可用
- 测试发送为前端占位，待后端通知模板测试发送接口上线后开放
- 当前数据存于 localStorage（MVP 阶段）；后端接口就绪后自动对接远端持久化

### 1.3 对话请求诊断模版

当用户直接在 Telegram 私聊 / 群里发自然语言请求，例如“看系统负载”“查看磁盘使用情况”“看一下 sshd 状态”“看一下你的出口IP是多少”，TARS 会把它转成一条 `telegram_chat` 会话，先返回即时确认，再下发诊断与审批。

```text
[TARS] 诊断
请求: 看系统负载
目标: 192.168.3.106
结论: 当前负载有上升趋势，建议先看最近一小时时序，再决定是否上机排查。
下一步: uptime && cat /proc/loadavg
会话: ses_xxx
```

说明：

- 单一白名单主机场景下，机器人会自动使用默认主机
- 多主机场景下，如果用户消息里没有带 `host=` 或明确主机名，机器人只返回引导信息，不会创建执行请求
- 该路径默认走 `chat_request:direct` 审批路由，审批消息直接回到原始聊天窗口
- “出口 IP / 公网 IP / egress IP” 这类请求会生成只读命令 `curl -fsS https://api.ipify.org && echo`，执行结果回到原始聊天窗口

---

## 2. 审批消息与动作 (Approval Message & Callbacks)

这是 Workflow Core 请求人工审批命令时下发的消息，附带 Inline Keyboard 按钮。

### 2.1 页面数据结构 (ViewModel)

| 字段名 | 来源 DTO 映射 | 说明 |
|--------|---------------|------|
| `ExecutionID` | `executions[0].execution_id` | 用于回调操作 |
| `TargetHost` | `executions[0].target_host` | 目标执行主机 |
| `RiskLevel` | `executions[0].risk_level` | `critical` / `warning` / `info` |
| `Command` | `executions[0].command` | 具体的 Bash 命令 |
| `ApprovalSource`| `executions[0].approval_group` 或后端直接通知提供 | (如 `service_owner` 或 `oncall`) |
| `Timeout` | `executions[0].timeout_seconds` | 剩余审批时效 (分钟/小时提示) |

### 2.2 Telegram 消息模版范例

```html
<b>⚠️ 待审批执行请求</b>
<b>目标主机:</b> prod-web-01
<b>风险等级:</b> 🟠 WARNING
<b>审批来源:</b> service owner (web)
<b>时限:</b> 15 分钟

<b>执行命令:</b>
<code>systemctl restart nginx && systemctl status nginx</code>
```

### 2.2.1 对话请求审批消息

```text
[TARS] 待审批
请求: 看系统负载
目标: 192.168.3.106
风险: info
命令: uptime && cat /proc/loadavg
原因: 当前需要主机侧补充证据确认负载来源。
时限: 15m
会话: ses_xxx
```

出口 IP 请求示例：

```text
[TARS] 待审批
请求: 看一下你的出口IP是多少
目标: 192.168.3.106
风险: info
命令: curl -fsS https://api.ipify.org && echo
原因: 需要主机侧执行只读命令获取出口地址。
时限: 15m
会话: ses_xxx
```

### 2.3 按钮语义与回调结构 (Inline Keyboard)

每个审批动作都是一次 Telegram webhook 回调，我们在业务侧统一定义如下的 JSON Data Payload。

| 按钮文案 | 预期动作行为 | Callback Data Payload 样例 | 预期后置表现 |
|---------|--------------|-----------------------------|--------------|
| `✅ 批准执行` | 同意按原命令执行 | `{"action":"approve","exec_id":"exe_123"}` | 消息刷新为“已批准，执行中...” |
| `❌ 拒绝` | 不同意执行 | `{"action":"reject","exec_id":"exe_123"}` | 消息刷新为“已拒绝” |
| `✏️ 修改后批准` | 修改命令后执行 | `{"action":"edit","exec_id":"exe_123"}` | 唤起输入强制交互模式，回敲后执行 |
| `🔄 转交他人` | 转交给其他管理员 | `{"action":"reassign","exec_id":"exe_123"}` | (唤起选择人菜单) |
| `❓ 需补充信息` | 打回要求更多上下文 | `{"action":"request_context","exec_id":"exe_123"}` | 消息刷新为“等待补充上下文” |

*注：受限于 Telegram `callback_data` 64字节限制，实际发往下游的标识可能会进一步压缩(如用短 hash 替代 UUID 或前缀映射)，本处仅为逻辑结构。*

---

## 3. 执行结果消息 (Execution Result Message)

命令执行结束或超时后，通知运维人员最终状态。

### 3.1 页面数据结构 (ViewModel)

| 字段名 | 来源 API | 说明 |
|--------|----------|------|
| `ExecutionStatus` | `status` | `completed`, `failed`, `timeout` |
| `OutputPreview` | 命令标准输出的前 5 行或末尾 5 行 | 可通过后台截断 |
| `TruncationFlag`| `output_truncated` | 是否因为太长被截断 |
| `ActionTip` | AI 针对结果补充的建议 | (如检查是否存活) |

### 3.2 Telegram 消息模版范例

**成功场景：**
```text
[TARS] 执行结果
主机: prod-web-01
状态: completed
执行链: jumpserver-main · jumpserver_job
校验: success
校验说明: verification passed: nginx is active
日志: /var/lib/tars/execution_output/exe_xxx.log
输出:
● nginx.service - A high performance web server
  Loaded: loaded
  Active: active (running) since Wed 2026-03-11 08:00...
会话: ses_xxx
```

**失败场景：**
```text
[TARS] 执行结果
主机: prod-web-01
状态: failed
执行链: jumpserver-main · jumpserver_job
退出码: 1
校验: skipped
校验说明: verification skipped: no service hint available
日志: /var/lib/tars/execution_output/exe_xxx.log
输出:
nginx: [emerg] open() "/etc/nginx/nginx.conf" failed (2: No such file or directory)
会话: ses_xxx
```
