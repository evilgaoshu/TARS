# TARS — 企业级平台下一阶段主题

> **状态**: Next Phase Baseline
> **用途**: 取代旧的 gap 表述，作为企业级平台下一阶段主题与规划入口

---

## 1. 说明

当前 TARS 已完成 MVP 主链路与若干平台化能力，但如果目标是企业级长期运行平台，仍需要继续规划一批治理、组织、合规与运维主题。

这里不再用“gap 清单”表达，而统一按“下一阶段主题”维护。

---

## 2. 主题分组

### 2.1 组织与多租户

- Organization / Workspace / Project 模型
- Tenant Admin
- 跨组织隔离边界
- 资产租户归属与默认策略

### 2.2 高可用与恢复

- 明确 `RPO / RTO`
- 多副本 / 切换拓扑
- 灾备策略
- 恢复演练机制

### 2.3 运行时隔离

- connector / skill / MCP runtime 隔离等级
- 资源限制模型
- 沙箱 / 容器 / 子进程安全边界

### 2.4 企业身份生命周期

- SCIM
- 目录同步
- JIT provision
- offboarding / deprovision
- group sync

### 2.5 访问加固

- session lifecycle
- revoke / idle timeout
- device inventory
- step-up auth
- impersonation / delegated troubleshooting

### 2.6 Secret / KMS / Encryption Governance

- Vault / KMS 集成
- BYOK
- key hierarchy
- rotation
- access audit

### 2.7 认证增强

- local password
- challenge / verification
- 2FA / MFA
- step-up auth
- break-glass 治理

### 2.8 审计与合规

- SIEM 导出
- 不可篡改审计
- retention policy
- legal hold
- audit archival

### 2.9 供应链与包信任

- package signing
- provenance
- source allowlist
- trust policy
- 第三方扩展审核链

### 2.10 配额、成本与使用治理

- tenant quota
- runtime limits
- model budget
- usage analytics
- cost attribution

### 2.11 运维治理窗口

- maintenance window
- silence / mute
- change window
- alert suppression

### 2.12 配置治理与漂移控制

- drift detection
- config diff
- rollback governance
- approval-aware change flow

---

## 3. 使用方式

当某个主题准备进入正式设计时，应新增独立 spec，而不是继续把所有后续工作堆在一份“大 gap 文档”里。
