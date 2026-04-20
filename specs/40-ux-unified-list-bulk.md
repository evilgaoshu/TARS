# TARS 统一列表与批量操作框架规范

> 状态：Phase 2 产品化基线  
> 日期：2026-03-13  
> 适用范围：`sessions / executions / outbox / audit / knowledge trace / tickets / connectors`

---

## 1. 目标

这份规范用于统一 TARS 中“多条记录”场景的查询和批量操作方式，避免每个页面、每个资源各自发明协议。

核心原则：

- **后端是能力底座**
  - 统一查询协议
  - 统一批量操作协议
  - 统一错误语义
  - 统一审计与权限判断
- **前端是交互底座**
  - 统一筛选栏
  - 统一分页控件
  - 统一批量选择
  - 统一批量动作条
  - 统一结果反馈

结论：

- 真正的“框架”首先落在后端协议和约束上
- 前端负责把同一套协议做成可复用交互壳

---

## 2. 当前与后续范围

### 2.1 第一阶段已落地

- `sessions`
- `executions`
- `outbox`

已统一能力：

- 分页
- 搜索
- limit
- 排序

已部分落地：

- `outbox` 批量选择
- `outbox` 批量 `replay`
- `outbox` 批量 `delete`

### 2.2 后续必须接入

- `knowledge trace`
- `tickets`
- `connector runs`
- `external source sync jobs`

### 2.3 后续批量能力优先级

| 资源 | 批量操作 |
|------|----------|
| `outbox` | `replay / delete / mark_handled` |
| `sessions` | `archive / export / retry_diagnosis` |
| `executions` | `export / reverify / mark_reviewed` |
| `audit` | `export` |
| `knowledge trace` | `reindex / mark_reviewed / export` |

当前状态补充：

- `audit` 已落地当前页选择 + `POST /api/v1/audit/bulk/export`
- `knowledge` 已落地当前页选择 + `POST /api/v1/knowledge/bulk/export`
- 导出接口统一复用 `BatchOperationRequest`，并返回 JSON attachment + 部分成功/失败明细

---

## 3. 统一查询协议

所有多记录只读接口必须优先支持以下通用参数：

| 参数 | 类型 | 说明 |
|------|------|------|
| `page` | `int` | 页码，从 `1` 开始 |
| `limit` | `int` | 每页条数，默认 `20`，最大 `100` |
| `q` | `string` | 通用关键字搜索 |
| `sort_by` | `string` | 排序字段 |
| `sort_order` | `string` | `asc / desc` |

可按资源追加结构化筛选：

| 参数模式 | 示例 |
|----------|------|
| 单字段筛选 | `status=failed` |
| 资源特定筛选 | `host=192.168.3.106` |
| 未来扩展筛选 | `service=sshd`、`aggregate_id=...` |

约束：

- `page <= 0` 时回退为 `1`
- `limit <= 0` 时回退为默认值
- `limit > 100` 时强制截断为 `100`
- `sort_order` 非法值统一回退为 `desc`
- `sort_by` 非法值不得直接拼接 SQL，只能命中白名单字段

---

## 4. 统一列表响应

所有多记录查询接口返回结构必须统一为：

```json
{
  "items": [],
  "page": 1,
  "limit": 20,
  "total": 135,
  "has_next": true,
  "q": "sshd",
  "sort_by": "updated_at",
  "sort_order": "desc"
}
```

字段定义：

| 字段 | 说明 |
|------|------|
| `items` | 当前页数据 |
| `page` | 当前页码 |
| `limit` | 当前页大小 |
| `total` | 当前筛选条件下的总记录数 |
| `has_next` | 是否还有下一页 |
| `q` | 服务端实际接收并生效的搜索关键字 |
| `sort_by` | 服务端实际生效的排序字段 |
| `sort_order` | 服务端实际生效的排序方向 |

要求：

- `total` 必须是筛选后的总数，而不是全表总数
- `has_next` 必须基于 `page / limit / total` 计算
- 前端不得自己猜测 `has_next`

---

## 5. 统一批量操作协议

### 5.1 请求结构

批量写操作统一使用：

```json
{
  "ids": ["id-1", "id-2"]
}
```

约束：

- `ids` 必填
- 空字符串 ID 要先清理
- 重复 ID 要自动去重
- 单次批量最大 `100`
- 后台必须自动记录批量动作审计；前端不展示 `operator_reason` 输入框

### 5.2 响应结构

批量结果必须允许部分成功，不得强制全有或全无：

```json
{
  "operation": "delete",
  "resource_type": "outbox_event",
  "total": 3,
  "succeeded": 2,
  "failed": 1,
  "results": [
    {
      "id": "id-1",
      "success": true,
      "message": "accepted"
    },
    {
      "id": "id-2",
      "success": false,
      "code": "invalid_state",
      "message": "resource is not in a valid state for this action"
    }
  ]
}
```

### 5.3 错误语义

单条失败时优先落到 `results[]`，而不是让整个 HTTP 请求失败。

推荐映射：

| `code` | 含义 |
|--------|------|
| `validation_failed` | 非法 ID、参数格式错误 |
| `not_found` | 目标不存在 |
| `invalid_state` | 当前状态不允许该动作 |
| `blocked_by_feature_flag` | 当前动作被开关拦截 |
| `internal_error` | 其他未分类错误 |

只有以下情况应直接返回整请求错误：

- body 无法解析
- `ids` 为空
- 超过最大批量上限
- 无权限

---

## 6. 前端交互规范

### 6.1 统一列表页面结构

所有多记录页面优先复用同一结构：

1. 标题与说明
2. 筛选栏
3. 批量动作条
4. 表格
5. 分页控件

### 6.2 批量选择规则

当前阶段统一采用：

- 当前页逐条勾选
- 当前页全选
- 清空选择

当前**不做**：

- 跨页“全量选中”
- 基于查询结果全集的服务端选择集

原因：

- 先把当前页选择语义收稳
- 避免在没有服务端 selection token 的前提下误导用户

后续如需“对当前筛选结果全量操作”，必须引入：

- `selection_token`
- 后端持久化或可重建选择条件

### 6.3 批量操作交互

统一要求：

- 批量操作必须在动作前确认
- 不展示 `operator_reason` 输入框
- 操作完成后，统一提示：
  - 成功多少
  - 失败多少
- 前端后续应支持展开失败详情

---

## 7. 后端实现约束

### 7.1 查询层

- 所有 `sort_by` 必须走字段白名单映射
- 不允许直接将前端字段拼进 SQL
- 搜索字段可以不同资源不同实现，但参数名必须统一为 `q`

### 7.2 批量层

- 批量动作本质上是“循环执行单条动作 + 聚合结果”
- Phase 2 不要求事务级全成功
- 优先保证：
  - 可审计
  - 可解释
  - 可部分成功

### 7.3 审计层

批量动作必须记录：

- `resource_type`
- `action`
- `ids`
- `actor`
- `timestamp`
- `result summary`
- `total / succeeded / failed`

### 7.4 指标建议

后续建议补：

- `tars_bulk_operations_total{resource,action}`
- `tars_bulk_operation_failures_total{resource,action,code}`

---

## 8. 资源能力矩阵

| 资源 | 查询协议 | 单条操作 | 批量操作 | 当前状态 |
|------|----------|----------|----------|----------|
| `sessions` | 统一 | `get` | 后续 | 已统一查询 |
| `executions` | 统一 | `get / get_output` | 后续 | 已统一查询 |
| `outbox` | 统一 | `replay / delete` | `bulk replay / bulk delete` | 已落地 |
| `audit` | 后续统一 | `trace get` | 后续 | 待接入 |
| `knowledge trace` | 后续统一 | `trace get` | 后续 | 待接入 |

---

## 9. 分阶段落地顺序

### Phase A

- 统一查询协议
- `sessions / executions / outbox` 分页、搜索、limit、排序

### Phase B

- 统一批量结果协议
- `outbox` 批量 `replay / delete`

### Phase C

- `sessions / executions` 批量框架
- `audit / knowledge trace` 接入统一列表协议

### Phase D

- 选择集跨页能力
- 服务端 selection token
- 批量导出、批量重建、批量标记处理

---

## 10. 当前结论

这套规范里：

- **后端协议是主底座**
- **前端组件是复用壳**
- **当前页选择是当前阶段唯一允许的批量选择语义**
- **批量接口必须支持部分成功**
- **后续所有多记录资源都必须接入这套协议，而不是单页特例实现**
