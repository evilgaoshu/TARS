# TARS 故障注入演练记录

日期：2026-03-12  
环境：`root@192.168.3.106`  
目标版本：包含 `telegram.send retry/outbox`、向量检索与知识幂等修复的最新 `v7` 二进制

## 1. 演练目标

验证 MVP 后端在以下故障场景下的降级和幂等行为：

- 模型网关超时或不可达
- VictoriaMetrics 查询不可达
- SSH 执行超时
- Telegram callback replay

## 2. 演练结果

### 2.1 模型超时 / 不可达

- 临时配置：`TARS_MODEL_BASE_URL=http://127.0.0.1:9`
- 其余处理：Telegram 发送改为 stub，避免对真实聊天目标产生噪音
- 样本 session：`d50818b7-1b52-491f-9f42-1cccf403cfe1`
- 结果：
  - session 最终进入 `open`
  - `diagnosis_summary` 回退为本地 deterministic diagnosis
  - 结果判定：通过

摘录：

```text
model_timeout_status=open
model_timeout_fallback=yes
model_timeout_summary=diagnosis ready: TarsModelTimeoutDrill2 on 192.168.3.106 (severity=warning)
```

### 2.2 VictoriaMetrics 不可达

- 临时配置：`TARS_VM_BASE_URL=http://127.0.0.1:9`
- 同时配置：`TARS_MODEL_BASE_URL=`，确保 diagnosis 由本地 fallback 生成
- 样本 session：`8f20883c-ed99-44dd-a4ac-06526eead025`
- 结果：
  - session 最终进入 `open`
  - `diagnosis_summary` 仍然成功生成
  - 结果判定：通过  
    说明：当前远端演练未直接抓到 provider error 指标增量，但在现有实现里 `dispatcher -> QueryMetrics` 为固定前置路径，运行结果表明 VM 不可达不会阻断 diagnosis 主链路

摘录：

```text
vm_timeout_status=open
vm_timeout_summary_ok=yes
```

### 2.3 SSH 执行超时

- 临时配置：`TARS_SSH_COMMAND_TIMEOUT=1s`
- 触发方式：对 pending execution 发送 `modify_approve`，命令改为 `hostname && sleep 2`
- 样本 session：`c8ceb7c9-07e7-42b6-9aab-3a6d34995896`
- 样本 execution：`6ed99bf8-f4eb-4ba7-8af9-c4a841ff6cac`
- 结果：
  - callback 返回 `200 {"accepted":true}`
  - session 最终进入 `failed`
  - execution 最终进入 `timeout`
  - 结果判定：通过

摘录：

```text
failed
timeout
hostname && sleep 2
```

### 2.4 Telegram callback replay

- 触发方式：对同一 execution 重放相同 `update_id=9003` 的 callback
- 样本 session：`c8ceb7c9-07e7-42b6-9aab-3a6d34995896`
- 样本 execution：`6ed99bf8-f4eb-4ba7-8af9-c4a841ff6cac`
- 结果：
  - 首次 callback 返回 `200`
  - 重放 callback 再次返回 `200`
  - execution 状态保持 `timeout`，未发生二次推进
  - 结果判定：通过

摘录：

```text
HTTP/1.1 200 OK
{"accepted":true}

failed
timeout
```

## 3. 结论

- 模型、VM、SSH、Telegram replay 4 类故障场景下，MVP 主链路都能保持“不中断、可降级或可幂等重放”的目标行为
- 当前剩余工作已不再是主链路缺失，而是非阻塞完善：
  - 更完整的边界单测
  - 可直接导入的 dashboard 产物
  - 试点期缓冲与观察
