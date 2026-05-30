# 项目管理系统 方案设计文档（Design Spec）

> 输入依据：`docs/requirement.md`、`docs/workflow.md`、`docs/rule.md`
> 本文档面向后端实现与前端对接，约束目录结构、模块边界、数据模型（MongoDB 集合）与 HTTP 契约。

---

## 1. 技术选型

| 维度       | 选择                                          | 说明 |
|------------|-----------------------------------------------|------|
| 后端语言   | Go 1.20                                       | 见 `go.mod` |
| HTTP 框架  | `net/http` + 轻量自研路由（`internal/router`） | 减少外部依赖；规则 R12（最简化） |
| 存储       | MongoDB（官方驱动 `go.mongodb.org/mongo-driver`） | 规则 R05 |
| 鉴权       | HTTP Header `X-User-Id`、`X-User-Role`（演示用），中间件二次校验 | 规则 R08：敏感信息不硬编码 |
| 配置       | 配置文件 `etc/config.yaml` 加载核心项；敏感字段（`MONGO_URI`/`MONGO_DB`/`HTTP_ADDR`/`WEB_DIR`/`CONFIG_FILE`/`LOG_DIR`/`LOG_MAX_SIZE_MB`/`LOG_MAX_BACKUPS`）支持环境变量覆盖 | 规则 R08 |
| 命名       | HTTP 字段统一 `snake_case`                    | 规则 R20 |
| 响应       | 统一 `{ code, message, data }`                | 规则 R06 |
| 日志       | `go.uber.org/zap` + `lumberjack`（按大小滚动），关键流程 INFO，异常 ERROR；门面 `internal/log` 保持 `Infof/Errorf/KV` 不变 | 规则 R10/R18 |
| 测试       | `go test`                                     | 规则要求覆盖核心用例 |

---

## 2. 分层架构（强制单向依赖，规则 R04）

```
cmd/server          ── main 入口，装配依赖
internal/
  config/           ── 环境变量加载
  errcode/          ── 错误码与统一响应（规则 R19、R06）
  log/              ── 日志封装
  router/           ── 极简 HTTP 路由
  middleware/       ── 鉴权 / 角色校验 / panic 恢复
  store/mongo/      ── MongoDB 客户端、集合常量
  modules/
    rbac/           ── 用户/角色（最小实现，鉴权由 header 注入）
    owner/          ── 业主单位
    project/        ── 项目主体（立项/列表/状态机）
    formtemplate/   ── 模板字段配置
    formvalue/      ── 表格值读写
    changelog/      ── 修改记录 ★
    snapshot/       ── 提交定稿快照
    approval/       ── 审核 + 审批
web/                ── 前端静态资源（HTML+原生 JS）
```

依赖方向（自上而下，禁止反向）：
```
router → middleware → modules.{xxx}.controller → modules.{xxx}.service → store/mongo
```

跨模块仅允许 `service` 之间组合调用，且组合方向遵循：
```
project → formvalue → changelog
project → approval → snapshot
所有写操作 ← middleware.rbac
```

---

## 3. MongoDB 集合设计

> 约定：所有 `_id` 使用 `ObjectID`；时间字段 `created_at` / `updated_at` 使用 `time.Time`，BSON 默认即可。
> 所有读写均使用 BSON 参数化查询（规则 R05），禁止字符串拼接。

### 3.1 `projects`
| 字段              | BSON 类型     | 说明 |
|-------------------|---------------|------|
| `_id`             | ObjectID      | 主键 |
| `project_code`    | string        | 系统生成（`PJ` + yyyyMMdd + 6 位随机），唯一索引 |
| `project_name`    | string        | 必填 |
| `owner_id`        | ObjectID      | 关联 `owners._id` |
| `owner_name`      | string        | 冗余，便于列表显示 |
| `status`          | string        | DRAFT / FORM_EDITING / PENDING_REVIEW / PENDING_APPROVE / APPROVED / REJECTED |
| `current_revision`| int32         | 表格版本号，单调递增 |
| `applicant_id`    | string        | 申请人（来自 header） |
| `applicant_name`  | string        |  |
| `created_at`      | datetime      |  |
| `updated_at`      | datetime      |  |
| `approved_at`     | datetime      | 可空 |

索引：
- `{ project_code: 1 }` unique
- `{ status: 1, updated_at: -1 }`
- `{ owner_id: 1 }`

### 3.2 `owners`
| 字段            | 说明 |
|-----------------|------|
| `_id`           |  |
| `name`          | 唯一（业务唯一，不强制 unique 索引以容错） |
| `owner_type`    | 政府/企业/事业/其他 |
| `contact_name`  |  |
| `contact_phone` |  |
| `contact_email` |  |
| `address`       |  |
| `created_at`    |  |

### 3.3 `form_templates`
单文档存放当前生效模板（v1）：
```json
{
  "_id": ObjectID,
  "version": 1,
  "active": true,
  "sections": [
    {
      "section_key": "base_info",
      "section_label": "项目基本信息",
      "fields": [
        {
          "field_key": "base_info.project_overview",
          "label": "项目概况",
          "type": "textarea",
          "required": true,
          "editable_roles": ["applicant","editor","admin"]
        }
      ]
    }
  ]
}
```
> 字段类型枚举：`text` / `textarea` / `number` / `money` / `date` / `select` / `multiselect` / `file` / `table`。

启动时若集合为空，写入内置默认模板（来源：`requirement_template.docx` 抽象）。

### 3.4 `project_form_values`
扁平存储项目当前每个字段的最新值。
| 字段        | 说明 |
|-------------|------|
| `_id`       |  |
| `project_id`| ObjectID |
| `field_key` | string |
| `value`     | any（BSON 任意类型） |
| `updated_by`| string |
| `updated_at`| datetime |
| `revision`  | int32 |

唯一索引：`{ project_id: 1, field_key: 1 }`

### 3.5 `project_change_logs` ★
按需求 FR-3.1。
| 字段           | 说明 |
|----------------|------|
| `_id`          |  |
| `project_id`   |  |
| `field_key`    |  |
| `field_label`  | 冗余 |
| `old_value`    | any |
| `new_value`    | any |
| `operator_id`  |  |
| `operator_name`|  |
| `operator_role`|  |
| `operated_at`  |  |
| `revision`     |  |
| `remark`       |  |
| `phase`        | FORM_EDITING / REJECTED_REWORK |
| `hidden`       | bool 默认 false |

索引：
- `{ project_id: 1, revision: -1 }`
- `{ project_id: 1, field_key: 1, operated_at: -1 }`
- `{ operator_id: 1 }`

### 3.6 `project_form_snapshots`
| 字段          | 说明 |
|---------------|------|
| `_id`         |  |
| `project_id`  |  |
| `revision`    |  |
| `content`     | object：`{ field_key: value, ... }` |
| `submitted_by`|  |
| `submitted_at`|  |

索引：`{ project_id: 1, revision: -1 }`

### 3.7 `approval_events`
| 字段           | 说明 |
|----------------|------|
| `_id`          |  |
| `project_id`   |  |
| `level`        | 1=review, 2=final |
| `action`       | submit / approve / reject |
| `operator_id`  |  |
| `operator_role`|  |
| `comment`      |  |
| `snapshot_id`  | ObjectID（submit 时写入） |
| `created_at`   |  |

索引：`{ project_id: 1, created_at: -1 }`

---

## 4. 状态机实现要点

`project.status` 仅允许如下迁移（在 `project/service.go` 中以白名单矩阵实现）：

| from \ action          | submit_proposal | edit_form | submit_final | review_approve | review_reject | final_approve | final_reject |
|------------------------|-----------------|-----------|--------------|----------------|---------------|---------------|--------------|
| DRAFT                  | FORM_EDITING    |           |              |                |               |               |              |
| FORM_EDITING           |                 | FORM_EDITING | PENDING_REVIEW |              |               |               |              |
| PENDING_REVIEW         |                 |           |              | PENDING_APPROVE| FORM_EDITING (REJECTED) |               |              |
| PENDING_APPROVE        |                 |           |              |                |               | APPROVED      | FORM_EDITING (REJECTED) |
| APPROVED               |   只读                                                                                                       |

> 实际实现中合并 `DRAFT` 与立项动作：立项即创建 `FORM_EDITING`，简化前端流程（FR-1.2）。

---

## 5. 修改记录核心算法（FR-3 ★）

`PATCH /projects/{id}/form` 业务流程：
1. 通过 `project_id` 取项目，状态必须为 `FORM_EDITING`，否则 409。
2. 取项目当前所有 `project_form_values` 形成 `currentMap`。
3. 服务端分配 `nextRevision = current_revision + 1`（单文档原子 `$inc`）。
4. 遍历 `changes[]`：
   - 若 `newValue` 与 `currentMap[field_key]` 深比较相等 → 跳过（不写日志）。
   - 否则 upsert `project_form_values`（`value = newValue, revision = nextRevision`），并 append 一条 `project_change_logs`（共享 `revision`）。
   - `phase` 来源：项目最近一次审批事件若为 `reject`，则 `REJECTED_REWORK`，否则 `FORM_EDITING`。
5. 全部完成后无任何实际变更则回滚 revision（`$inc -1`）。
6. 由于 MongoDB 单机模式不一定支持事务，本设计采用「以 `project.current_revision` 的原子自增作为版本号分配点 + 失败时显式回滚 + 只追加日志」的策略；`project_form_values` upsert 与 `project_change_logs` 写入为顺序操作，任一失败记录 ERROR 日志（规则 R18）。

---

## 6. 提交定稿与审批

### 6.1 提交定稿（applicant）
1. 状态须为 `FORM_EDITING`。
2. 加载模板与当前所有 `project_form_values`，校验所有 `required = true` 的字段均非空。
3. 校验通过：
   - 写一份 `FormSnapshot`（content = 当前全量字段值）。
   - 写 `ApprovalEvent(level=1, action=submit, snapshot_id=<id>)`。
   - 项目 `status = PENDING_REVIEW`。
4. 校验失败：返回 422，`data.missing_fields = [field_key,...]`。

### 6.2 审核（reviewer）
- 入参 `{ action, comment }`；`reject` 时 `comment` 必填。
- approve → `status = PENDING_APPROVE`，写 `ApprovalEvent(level=1, action=approve)`。
- reject → `status = FORM_EDITING`（标记为驳回返工），写 `ApprovalEvent(level=1, action=reject, comment)`。

### 6.3 终审（approver）
- 类同 6.2，`level=2`；approve → `APPROVED` 并写 `approved_at`；之后所有写接口拒绝。

---

## 7. 鉴权与权限矩阵

中间件读取 Header：
- `X-User-Id`：用户 id（字符串，演示用，生产应解析 JWT）。
- `X-User-Name`：可选，操作人姓名（用于冗余写入）。
- `X-User-Role`：单一主角色，枚举 `applicant/editor/reviewer/approver/admin/viewer`。

各 Controller 入口用 `middleware.RequireRoles("applicant","admin")` 等装饰器声明所需角色（FR-5 矩阵）。

---

## 8. 错误码（规则 R19，统一收拢于 `internal/errcode/codes.go`）

| Code | HTTP | Message Key             | 场景 |
|------|------|-------------------------|------|
| 0    | 200  | OK                      | 成功 |
| 1001 | 400  | INVALID_PARAM           | 参数缺失 / 类型错误 / 蛇形命名校验失败 |
| 1002 | 401  | UNAUTHORIZED            | 缺失或无效用户 |
| 1003 | 403  | FORBIDDEN               | 角色无权限 |
| 1004 | 404  | NOT_FOUND               | 资源不存在 |
| 1005 | 409  | STATE_CONFLICT          | 状态机不允许该动作 |
| 1006 | 422  | VALIDATION_FAILED       | 必填字段缺失（含 `missing_fields`） |
| 2001 | 500  | INTERNAL_ERROR          | 内部错误 |
| 2002 | 500  | DB_ERROR                | DB 异常 |

统一响应：
```json
{ "code": 0, "message": "OK", "data": {...} }
```

---

## 9. HTTP 路由汇总（详细见 `http_api.md`）

```
POST   /api/v1/projects                              立项
GET    /api/v1/projects                              列表
GET    /api/v1/projects/{id}                         详情
GET    /api/v1/projects/{id}/form                    表格
PATCH  /api/v1/projects/{id}/form                    保存（产生 ChangeLog）
GET    /api/v1/projects/{id}/changelogs              修改记录时间轴
GET    /api/v1/projects/{id}/changelogs/by-field     字段历史
GET    /api/v1/projects/{id}/diff                    版本对比
POST   /api/v1/projects/{id}/submit                  提交定稿
POST   /api/v1/projects/{id}/approvals/review        一级审核
POST   /api/v1/projects/{id}/approvals/final         二级审批
GET    /api/v1/projects/{id}/approvals               审批事件
GET    /api/v1/form-template                         当前模板
GET    /api/v1/owners                                业主单位列表
GET    /api/v1/me/todos                              待办
```

---

## 10. 测试策略

- 在每个核心 service 的同包下编写 `*_test.go`。
- DB 依赖通过定义 `Repo` 接口 + 内存伪实现进行单测；ChangeLog 与状态机为重点用例。
- 用例覆盖：
  1. 立项最小信息成功，状态为 `FORM_EDITING`；
  2. 一次保存 N 字段产生 N 条 ChangeLog 且共享同一 revision；
  3. 字段值未变化不产生 ChangeLog；
  4. 必填缺失提交定稿被拒并返回缺失列表；
  5. 审核驳回回到 `FORM_EDITING`，后续修改 `phase = REJECTED_REWORK`；
  6. `APPROVED` 后写接口拒绝；
  7. 状态机非法迁移返回 STATE_CONFLICT。

---

## 11. 模块清理与资源管理（规则 R15/R16）

- 当前不引入内存缓存层，无 key 上限问题；如未来引入 `lru`，必须设定容量与 TTL。
- 项目删除（仅 admin，本期不开放接口）若开放，需级联删除 `project_form_values` / `project_change_logs` / `project_form_snapshots` / `approval_events`。
