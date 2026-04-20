# TARS — 平台配置导入导出与 Bundle 规范

> 关联总览：[30-strategy-platform-config-and-automation.md](./30-strategy-platform-config-and-automation.md)

## 1. 目标

定义平台配置如何以 bundle / module 级别导出导入，同时保持 secret、安全边界和对象依赖可控。

## 2. 两层能力

### 2.1 全量 Bundle Export / Import

- 面向环境迁移、交付打包、试点初始化。
- 输出应覆盖核心平台对象及其依赖关系。
- 不能把 secret 明文直接打进 bundle。

### 2.2 模块级 Export / Import

- 面向对象级迁移、模板复用、团队共享。
- 更适合 Connectors、Skills、Templates、Automations 等单域对象。
- 允许对象页提供次级导出入口，但不应抢主操作位。

## 3. Bundle 结构建议

- `metadata`
- `components`
- `references`
- `compatibility`
- `secrets` 只保留引用与占位，不保留明文

## 4. Secret 处理原则

- 用户侧输入真实凭据，系统后台统一转 secret store。
- Bundle 只能保留 secret 缺口与引用语义。
- 导入时应明确提示哪些 secret 需要补齐，不能假装导入成功。

## 5. Import 约束

- 导入必须校验对象类型、版本兼容、依赖顺序和高风险覆盖行为。
- 对已有对象的覆盖需要显式确认或审批。
- 模块导入失败时，错误应解释为“哪里不兼容、缺什么、如何继续”。

## 6. UI / UX 原则

- Bundle/import/export 属于高级动作，不应成为对象页默认心智。
- 对象页可以保留“导出”入口，但应放在次级操作中。
- `Ops` 可保留原始 bundle 视图、批量导入和修复工具。
