# 项目管理系统 需求文档（Requirement Spec）

> 文档用途：作为后端 / 前端模块设计与编码阶段的输入，约束数据模型、接口契约、状态机与权限矩阵。
> 来源依据：`docs/requirement_template.docx`（需求模板）+ `docs/workflow.md`（工作流）+ 本次补充需求。
> 关键补充（用户口述）：
> 1. 仅需"项目名称 + 业主单位基本信息"即可立项；立项后进入整体表格内容填写。
> 2. 表格内容存在多轮修改过程，需要体现各角色人员的修改记录。
> 3. 表格内容完全确定后才进入审批环节。

---

## 1. 名词与角色

### 1.1 名词
- **项目（Project）**：业务核心实体，贯穿"立项 → 内容填报 → 审批 → 归档"全生命周期。
- **业主单位（Owner）**：项目甲方/委托方，立项必需。
- **项目表格（ProjectForm）**：项目立项后的整体内容表，由若干字段分组（Section）组成，字段定义来自 `requirement_template.docx`。
- **修改记录（ChangeLog / Revision）**：对项目表格任一字段的变更记录，含修改人、角色、时间、前后值、备注。
- **审批流（ApprovalFlow）**：表格定稿后启动的多级审批流程。

### 1.2 角色（RBAC）
> 假设：以下角色基于 `workflow.md` 推断，若实际工作流不同请调整。

| 角色 Code      | 名称       | 主要职责                                                     |
| -------------- | ---------- | ------------------------------------------------------------ |
| `applicant`    | 申请人     | 立项、填写/修改项目表格、提交定稿                             |
| `editor`       | 协办人     | 协助修改项目表格的部分字段                                   |
| `reviewer`     | 审核人     | 表格定稿后第一级审批（合规/技术审查）                         |
| `approver`     | 审批人     | 表格定稿后终审                                               |
| `admin`        | 管理员     | 用户/角色管理、模板管理、流程配置                             |
| `viewer`       | 查看者     | 只读查看项目数据与历史                                       |

---

## 2. 业务流程总览

```
[草稿态]  -- 填写项目名称+业主单位 --> [立项成功 / 表格填报中]
   |                                          |
   |                                  多轮修改（产生 ChangeLog）
   |                                          |
   |                              [申请人提交定稿] --> [待审核]
   |                                                      |
   |                            审核人通过 --> [待审批] --> 审批人通过 --> [已归档]
   |                            审核人/审批人驳回 --> [表格填报中]（可继续修改）
```

### 2.1 状态机（Project.status）

| 状态 Code        | 中文       | 进入条件                                  | 可执行动作                                      |
| ---------------- | ---------- | ----------------------------------------- | ----------------------------------------------- |
| `DRAFT`          | 草稿       | 用户点击"新建项目"                        | 填写项目名称+业主单位 → `submitProposal`        |
| `FORM_EDITING`   | 表格填报中 | 立项成功                                  | 编辑任意字段（产 ChangeLog） / `submitForFinal` |
| `PENDING_REVIEW` | 待审核     | 申请人提交定稿                            | 审核人 `approveReview` / `rejectReview`         |
| `PENDING_APPROVE`| 待审批     | 审核通过                                  | 审批人 `approveFinal` / `rejectFinal`           |
| `APPROVED`       | 已审批     | 审批通过                                  | 只读，归档                                      |
| `REJECTED`       | 已驳回     | 审核或审批驳回                            | 退回 `FORM_EDITING`，可继续修改后重新提交       |

> 状态约束：`PENDING_REVIEW` / `PENDING_APPROVE` / `APPROVED` 三态下，**禁止**修改项目表格字段；如需修改，必须由审批人/审核人主动驳回回到 `FORM_EDITING`。

---

## 3. 功能需求（FR）

### FR-1 项目立项（最小信息立项）
- **FR-1.1** 用户在"新建项目"页面，仅需填写两类信息即可创建项目：
  - 项目基本信息：`projectName`（必填，<=100 字）、`projectCode`（可选，系统可自动生成）。
  - 业主单位基本信息：`ownerName`（必填）、`ownerContact`（可选：联系人/电话/邮箱/地址）、`ownerType`（可选：政府/企业/事业单位/其他）。
- **FR-1.2** 提交后，系统创建项目记录，状态置为 `FORM_EDITING`，并生成空的项目表格（按模板字段初始化）。
- **FR-1.3** 立项成功后，前端跳转至该项目的"表格内容"页面。
- **FR-1.4** `projectName + ownerName` 组合在"未归档项目"内应做软重复校验（提示但不强制阻断）。

### FR-2 项目表格内容填写
- **FR-2.1** 表格字段定义来自 `requirement_template.docx`（需将其结构化为 `form_template` 配置：分组 / 字段 / 类型 / 校验 / 是否必填）。
  > 假设：模板字段例如「项目概况、建设规模、投资估算、建设地点、建设周期、技术方案、附件…」等；具体字段以模板为准，模块设计需将模板抽象为可配置的 `FormTemplate` + `FormField`。
- **FR-2.2** 字段类型至少支持：`text` / `textarea` / `number` / `money` / `date` / `select` / `multiselect` / `file` / `table`（子表格）。
- **FR-2.3** 表格支持**分段保存**（按 Section 保存），每次保存只对**实际发生变化的字段**生成 ChangeLog（见 FR-3）。
- **FR-2.4** 支持草稿自动保存（前端节流，建议 30s 一次），自动保存不进入 ChangeLog 主流，仅写入 `auto_save` 临时区。
- **FR-2.5** 字段级权限：某些字段仅特定角色可编辑（由 `FormField.editableRoles` 配置）。

### FR-3 修改记录（ChangeLog） ★ 核心
- **FR-3.1** 任一字段值发生变更并提交保存时，必须生成一条 `ChangeLog` 记录，结构如下：

  | 字段           | 类型     | 说明                                            |
  | -------------- | -------- | ----------------------------------------------- |
  | `id`           | string   | 主键                                            |
  | `projectId`    | string   | 关联项目                                        |
  | `fieldKey`     | string   | 字段标识（如 `baseInfo.investAmount`）          |
  | `fieldLabel`   | string   | 字段显示名（冗余，便于历史展示）                 |
  | `oldValue`     | json     | 修改前值（可为空，表示首次填写）                 |
  | `newValue`     | json     | 修改后值                                        |
  | `operatorId`   | string   | 操作人 ID                                       |
  | `operatorName` | string   | 操作人姓名（冗余）                               |
  | `operatorRole` | string   | 操作人角色 Code（冗余，体现"角色人员"）          |
  | `operatedAt`   | datetime | 操作时间                                        |
  | `revision`     | int      | 项目级单调递增版本号（每次提交保存后 +1）        |
  | `remark`       | string   | 修改说明（可选，建议必填以便审批回溯）           |
  | `phase`        | enum     | 修改发生的阶段（`FORM_EDITING` / `REJECTED_REWORK`） |

- **FR-3.2** 一次提交保存（一个事务）内若同时变更 N 个字段，应共享同一个 `revision` 号，但生成 N 条 ChangeLog。
- **FR-3.3** 修改记录**只增不改不删**（审计要求）；管理员也不可物理删除，仅可标记 `hidden`（默认 false）。
- **FR-3.4** 提供"修改记录视图"：
  - **按项目** 时间轴视图：展示项目从立项到现在的所有变更，按 `revision` 倒序，可按角色 / 操作人 / 字段 过滤。
  - **按字段** diff 视图：选定某字段，展示其全部变更历史（旧值 → 新值）。
  - **版本对比**：选定两个 `revision`，展示整体表格差异（diff）。
- **FR-3.5** 审批人 / 审核人在审批界面必须能直接看到本轮提交相对上一定稿（或上一审批轮次）的差异。

### FR-4 提交定稿与审批
- **FR-4.1** 申请人在 `FORM_EDITING` 状态点击"提交定稿"：
  - 系统执行**完整性校验**：所有必填字段非空、字段格式合法、附件齐全。
  - 校验通过 → 状态转为 `PENDING_REVIEW`，记录一条 `ApprovalEvent(submit)`。
  - 校验失败 → 阻断提交，前端高亮缺失字段。
- **FR-4.2** 审批环节为**两级**（可配置，默认为审核 → 审批）：
  > 假设：`workflow.md` 中具体级数若不同，在 `ApprovalFlow` 配置中调整即可。
  - **第一级 审核**：`reviewer` 在 `PENDING_REVIEW` 状态执行 `approve` / `reject`。
  - **第二级 审批**：`approver` 在 `PENDING_APPROVE` 状态执行 `approve` / `reject`。
- **FR-4.3** 任一级 `reject`：
  - 状态回退至 `FORM_EDITING`。
  - 必须填写驳回意见 `rejectReason`。
  - 后续修改产生的 ChangeLog `phase = REJECTED_REWORK`，便于区分。
- **FR-4.4** 审批通过：状态转为下一级或最终 `APPROVED`；`APPROVED` 后表格只读、不可再修改。
- **FR-4.5** 每个审批动作生成一条 `ApprovalEvent`：

  | 字段          | 说明                                              |
  | ------------- | ------------------------------------------------- |
  | `id`          | 主键                                              |
  | `projectId`   | 项目                                              |
  | `level`       | 审批级别（1=review, 2=approve）                   |
  | `action`      | submit / approve / reject / withdraw              |
  | `operatorId`  | 操作人                                            |
  | `operatorRole`| 角色                                              |
  | `comment`     | 审批意见（reject 必填）                           |
  | `snapshotId`  | 关联的表格快照 ID（见 FR-4.6）                    |
  | `createdAt`   | 时间                                              |

- **FR-4.6** 每次"提交定稿"应对当时整张表格生成**只读快照（FormSnapshot）**，便于审批人看到"你这次审的是什么版本"，且与未来修改解耦。

### FR-5 权限矩阵

| 动作                 | applicant | editor | reviewer | approver | admin | viewer |
| -------------------- | :-------: | :----: | :------: | :------: | :---: | :----: |
| 创建项目（立项）     |     ✔     |        |          |          |   ✔   |        |
| 编辑表格字段         |     ✔     |   ✔*   |          |          |   ✔   |        |
| 提交定稿             |     ✔     |        |          |          |   ✔   |        |
| 审核（一级）         |           |        |    ✔     |          |       |        |
| 审批（二级）         |           |        |          |    ✔     |       |        |
| 驳回                 |           |        |    ✔     |    ✔     |       |        |
| 查看项目 / 历史      |     ✔     |   ✔    |    ✔     |    ✔     |   ✔   |   ✔    |
| 配置模板 / 流程      |           |        |          |          |   ✔   |        |

> `editor*`：仅可编辑 `FormField.editableRoles` 包含 `editor` 的字段。

### FR-6 通知 / 待办（建议）
- **FR-6.1** 状态流转关键节点产生站内消息 / 邮件通知：提交定稿 → 通知审核人；审核通过 → 通知审批人；驳回 → 通知申请人。
- **FR-6.2** 各角色首页提供"待办列表"。

### FR-7 检索 / 列表
- **FR-7.1** 项目列表支持按状态、业主单位、申请人、时间范围、关键字检索。
- **FR-7.2** 列表展示：项目名称 / 业主单位 / 当前状态 / 当前 revision / 最近修改人+时间 / 当前处理人。

---

## 4. 数据模型（实体设计）

> 命名按后端常用风格（蛇形）给出，前端 TypeScript 类型可镜像生成。

### 4.1 `project`
| 字段           | 类型      | 说明                                       |
| -------------- | --------- | ------------------------------------------ |
| id             | bigint/uuid| 主键                                      |
| project_code   | varchar   | 项目编号（系统生成，唯一）                  |
| project_name   | varchar   | 项目名称（必填）                           |
| owner_id       | bigint    | 业主单位 ID（必填，关联 `owner`）           |
| status         | varchar   | 见状态机                                   |
| current_revision| int      | 当前表格版本号                             |
| applicant_id   | bigint    | 申请人                                     |
| created_at     | datetime  |                                            |
| updated_at     | datetime  |                                            |
| approved_at    | datetime  | 终审通过时间，可空                         |

### 4.2 `owner`（业主单位）
| 字段          | 类型     | 说明                              |
| ------------- | -------- | --------------------------------- |
| id            | bigint   |                                   |
| name          | varchar  | 单位名称（必填）                  |
| owner_type    | varchar  | 政府 / 企业 / 事业 / 其他         |
| contact_name  | varchar  |                                   |
| contact_phone | varchar  |                                   |
| contact_email | varchar  |                                   |
| address       | varchar  |                                   |
| extra         | json     | 扩展字段                          |

### 4.3 `form_template` / `form_field`
- 把 `requirement_template.docx` 结构化为模板配置，支持后续模板版本演进（`template_version`）。
- `form_field` 关键属性：`field_key`, `label`, `type`, `required`, `validation`(json), `editable_roles`(json[]), `section`, `sort_order`, `template_version`.

### 4.4 `project_form_value`
扁平存储某项目当前每个字段的最新值。
| 字段        | 说明                              |
| ----------- | --------------------------------- |
| project_id  |                                   |
| field_key   |                                   |
| value       | json                              |
| updated_by  |                                   |
| updated_at  |                                   |
| revision    | 该字段最近一次变更所在的 revision |

> 主键：`(project_id, field_key)`。

### 4.5 `project_change_log` ★
按 FR-3.1 字段定义。索引：`(project_id, revision)`、`(project_id, field_key, operated_at)`、`(operator_id)`。

### 4.6 `project_form_snapshot`
| 字段          | 说明                                 |
| ------------- | ------------------------------------ |
| id            |                                      |
| project_id    |                                      |
| revision      | 提交定稿时的 revision                |
| content       | json，整张表格的全量值                |
| submitted_by  |                                      |
| submitted_at  |                                      |

### 4.7 `approval_event`
按 FR-4.5 字段定义。索引：`(project_id, created_at)`。

### 4.8 `user` / `role` / `user_role`
标准 RBAC，三表结构。

---

## 5. 接口契约（REST 草案）

> Base path：`/api/v1`

### 5.1 立项
- `POST /projects`
  - body: `{ projectName, owner: { name, type?, contactName?, contactPhone?, contactEmail?, address? } }`
  - 200: `{ id, projectCode, status: "FORM_EDITING" }`

### 5.2 表格读写
- `GET /projects/{id}/form` → 返回模板结构 + 当前字段值 + 当前 revision。
- `PATCH /projects/{id}/form`
  - body: `{ changes: [{ fieldKey, newValue, remark? }, ...] }`
  - 服务端：对每个 change 与当前值 diff，写 `project_form_value` 与 `project_change_log`，`current_revision += 1`。
  - 仅 `status = FORM_EDITING` 时可调。

### 5.3 修改记录
- `GET /projects/{id}/changelogs?fieldKey=&operatorId=&role=&from=&to=&page=&size=` → 时间轴。
- `GET /projects/{id}/changelogs/by-field/{fieldKey}` → 字段历史。
- `GET /projects/{id}/diff?from=R1&to=R2` → 版本差异。

### 5.4 提交与审批
- `POST /projects/{id}/submit` → 申请人提交定稿。校验通过后写 `FormSnapshot`，状态转 `PENDING_REVIEW`，写 `ApprovalEvent(submit)`。
- `POST /projects/{id}/approvals/review` body: `{ action: "approve"|"reject", comment? }` → 一级审核。
- `POST /projects/{id}/approvals/final`  body: `{ action: "approve"|"reject", comment? }` → 二级审批。
- `GET  /projects/{id}/approvals` → 审批事件列表。

### 5.5 列表 / 待办
- `GET /projects?status=&keyword=&ownerId=&applicantId=&page=&size=`
- `GET /me/todos` → 根据当前用户角色聚合：申请人查看被驳回项目；审核/审批人查看待我处理。

---

## 6. 校验与业务规则

- **R-1** 立项时 `projectName` 与 `ownerName` 都为非空字符串，trim 后长度 > 0。
- **R-2** 业主单位若已存在（按 `name` 唯一匹配），复用其 `owner_id`，否则新建。
- **R-3** 仅 `FORM_EDITING` 状态允许写表格；其他状态调用写接口返回 `409 Conflict`。
- **R-4** `submit` 时若有必填字段缺失，返回 `422 Unprocessable Entity` 并列出缺失字段。
- **R-5** 审批 `reject` 时 `comment` 必填。
- **R-6** ChangeLog 写入与 `project_form_value` 更新必须在同一事务中。
- **R-7** `current_revision` 单调递增，单事务内多个字段变更共享同一 revision。
- **R-8** 任一接口调用都要鉴权 + 角色校验（FR-5 矩阵）。

---

## 7. 非功能性需求（NFR）

- **NFR-1 审计**：ChangeLog / ApprovalEvent 永久保留，至少保存 5 年。
- **NFR-2 性能**：单项目历史 ≥ 10000 条变更时，时间轴接口分页查询 P95 < 500ms。
- **NFR-3 安全**：字段级权限校验在服务端二次校验（不能仅依赖前端隐藏）。
- **NFR-4 可扩展**：审批级数、字段模板均可配置；新增一级审批不应改动核心代码，仅扩展 `ApprovalFlow` 配置。
- **NFR-5 可观测**：关键动作（立项/保存/提交/审批）打印结构化日志，含 `projectId / userId / role / action / revision`。
- **NFR-6 国际化**：字段 `label` 支持多语言（v2 可选）。

---

## 8. 模块划分建议（与编码对齐）

```
projectManager/
├── modules/
│   ├── project/          # 项目主体：立项、状态机、列表
│   ├── owner/            # 业主单位
│   ├── form-template/    # 模板配置（解析自 docx）
│   ├── form-value/       # 表格值读写
│   ├── changelog/        # 修改记录核心 ★
│   ├── snapshot/         # 提交定稿快照
│   ├── approval/         # 审核 + 审批 + 事件
│   ├── rbac/             # 用户/角色/权限
│   └── notification/     # 通知 / 待办
└── docs/
    ├── requirement.md          # 本文档
    ├── requirement_template.docx
    └── workflow.md
```

各模块对外暴露 `service` 与 `controller`；模块间依赖严格单向：
`project → form-value → changelog`、`project → approval → snapshot`、所有写操作经 `rbac` 拦截。

---

## 9. 待澄清问题（TBD）

1. **审批级数**：是固定两级（审核 + 审批）还是可配置 N 级？是否存在并签 / 或签？
2. **修改记录是否需要"撤回上一次修改"功能**？（目前设计为只追加历史，不支持撤回，仅可再次修改覆盖）
3. **附件版本化**：附件被替换时，是否在 ChangeLog 中保留旧附件下载入口？
4. **业主单位主数据**是否需要独立维护页面，还是仅在立项时按需创建？
5. **驳回后修改**是否要求"必须填写本次修改原因"才能再次提交？
6. `requirement_template.docx` 中具体字段清单需另行落地为 `form-template` 配置，本次只定义结构。

---

## 10. 验收标准（节选）

- [ ] 仅填写"项目名称 + 业主单位名称"即可成功立项，状态为 `FORM_EDITING`。
- [ ] 在 `FORM_EDITING` 状态下，对 N 个字段一次性保存，产生 N 条 ChangeLog 且共享同一 revision。
- [ ] 时间轴可查询到每条修改的：操作人姓名、角色、字段、旧值、新值、时间。
- [ ] 字段未填齐时点击"提交定稿"被阻断且提示具体缺失字段。
- [ ] 审核人 / 审批人页面能看到本轮提交相对上一轮的差异。
- [ ] 驳回后状态回到 `FORM_EDITING`，可继续修改，再次提交后审批人看到的是新快照。
- [ ] `APPROVED` 后表格只读，所有写接口返回 409。
- [ ] 角色无对应权限调用接口时返回 403。

