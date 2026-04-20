# TARS — Extensions 规范

> **状态**: 设计基线
> **适用范围**: extension candidate intake、validate、review、import
> **结构**: 本规范遵循四段式模板：`功能 Spec + UX Spec + 内容 Spec + 验收清单`
> **关联**: [00-spec-four-part-template.md](./00-spec-four-part-template.md)、[20-component-skills.md](./20-component-skills.md)、[20-component-connectors.md](./20-component-connectors.md)

---

## 1. 功能 Spec

### 1.1 对象定义

#### Extensions 是什么

`Extensions` 是 **候选扩展与受治理 bundle intake 对象域**。

它管理进入正式 registry 之前的候选包，而不是最终的 skill / connector 对象本身。

#### Extensions 不是什么

- 不是正式安装后的 Skills 列表
- 不是 Connectors 的配置目录
- 不是原始 bundle 文件浏览器而已

### 1.2 用户目标与关键场景

#### 高频任务

- 生成新的 extension candidate
- 预览 validate 结果
- 审批、退回或拒绝候选
- 导入到正式 skill registry

#### 关键场景

- 在进入正式 registry 前，对 bundle 做结构与治理检查
- 明确一个候选包是否已通过 review、能否 import
- 区分“候选扩展治理”与“正式 skill 运营”两条路径

### 1.3 状态模型

- `generated`
- `validated`
- `invalid`
- `approved`
- `changes_requested`
- `rejected`
- `imported`

#### 展示优先级

1. 当前 candidate 是否可继续推进
2. validate 结果如何
3. review 结论如何
4. 是否已经 import 到正式 registry

### 1.4 核心字段与层级

#### L1 默认字段

- `id`
- `status`
- `review_state`
- `display_name`

#### L2 条件字段

- `docs`
- `tests`
- `governance`
- `validation_summary`

#### L3 高级字段

- `review_history`
- `generated_by`
- `source`

#### L4 系统隐藏字段

- raw bundle payload

#### L5 运行诊断字段

- validation errors
- validation warnings

### 1.5 关键规则与约束

- `/extensions` 承接 candidate lifecycle
- `/skills` 承接正式安装后的对象
- 未通过 approved 的 candidate 不允许进入 import 主路径
- Extensions 应始终讲“候选治理”，不应与正式 registry 混叠

---

## 2. UX Spec

### 2.1 用户任务

#### 高频任务

- 新建或导入一个候选扩展
- 查看 validate 和 review 结果
- 决定批准、退回还是拒绝
- 将通过审核的 candidate 导入正式 registry

#### 首屏必须回答的 3 个问题

1. 当前有哪些 candidate 在等待处理
2. 它们是否通过 validate，review 状态如何
3. 哪些已经可以 import，哪些还需要修复

### 2.2 入口与页面归属

#### `/extensions`

承接：

- candidate lifecycle
- validate
- review
- import

#### `/skills`

承接：

- 正式安装后的 skill registry
- 启停、导出与日常管理

Extensions 与 Skills 的边界必须保持清晰。

### 2.3 页面结构

推荐结构：

1. Summary stats
2. Candidate registry sidebar
3. Candidate detail
4. Composer drawer / panel

页面默认先回答“这个 candidate 当前到哪一步了、能不能继续推进”，而不是先暴露 raw bundle。

### 2.4 CTA 与操作层级

#### 主动作

- `创建 Candidate`
- `验证`
- `批准`
- `退回`
- `拒绝`
- `导入`

#### 次级动作

- `查看文档`
- `查看测试`
- `查看 Review 历史`

#### 高级动作

- `查看 Raw Bundle`
- `重新验证`

### 2.5 页面字段裁剪规则

- 默认区只显示 L1 / L2
- L3 放在 review / source / history 区
- L4/L5 不应默认占据首屏
- raw bundle、完整 validate errors 进入高级抽屉或补充区块

---

## 3. 内容 Spec

### 3.1 命名

#### 默认命名

- 模块名：`Extensions`
- 对象名：`Extension Candidate`

#### 页面叙事

- 页面讲“候选扩展治理”
- 不讲“正式 skill registry”
- 不把 Extensions 讲成 bundle 文件夹

### 3.2 页面标题与副标题

#### 列表 / 主页

- 标题：`Extensions`
- 副标题应表达：验证、审核并导入候选扩展

### 3.3 CTA 文案

主路径默认使用：

- `创建 Candidate`
- `验证`
- `批准`
- `退回`
- `拒绝`
- `导入`

次级路径默认使用：

- `查看文档`
- `查看测试`
- `查看 Review 历史`

高级区允许：

- `查看 Raw Bundle`
- `重新验证`

### 3.4 状态文案

#### 无 candidate

- 结论：`当前还没有候选扩展`
- 细节：可以先创建一个 candidate，或导入现有 bundle
- 动作：`创建 Candidate`

#### validate 失败

- 结论：`当前候选未通过验证`
- 细节：请先修复 errors / warnings，再进入 review 或 import
- 动作：`查看验证结果`

#### 未 approved 即尝试 import

- 结论：`当前候选还不能导入`
- 细节：必须先通过审核后才能进入正式 registry
- 动作：`发起审核`

### 3.5 术语黑名单

以下词不应默认出现在主路径：

- raw bundle payload
- internal import job naming
- 把 `Extensions` 和 `Skills` 说成同一个对象

这些内容可留在高级区，不应主导 Extensions 默认体验。

---

## 4. 验收清单

### 4.1 页面级验收

- `/extensions` 已清晰表达为 candidate intake / review / import 工作台
- 页面默认先给 validate / review / import 进度，而不是 raw bundle
- `Extensions` 与 `Skills` 的对象边界清晰

### 4.2 交互级验收

- 用户能完成创建、验证、批准/退回/拒绝、导入的主路径
- 未 approved 的 candidate 无法直接走 import 主路径
- 候选修复与正式 skill 管理不再混在一个页面

### 4.3 展示级验收

- 候选列表至少展示 id、status、review_state、display_name、validation_summary
- 空态、validate 失败、未 approved 即尝试 import 等状态有清晰文案
- L4/L5 字段不再默认上浮

### 4.4 测试与验证要求

- 需要页面级测试覆盖 validate、review、import 以及关键状态文案
- 需要浏览器或截图验收确认 Extensions 默认叙事已经从“bundle 清单”收口为“候选治理台”
- 若后端尚未提供更完整 review history 或 source 细节，前端不应伪装成已齐备

### 4.5 剩余限制说明

- 更复杂的 candidate composer 与 diff 视图可作为下一阶段增强项
- raw import/export 与底层兼容修复继续保留在高级区或 `Ops`
