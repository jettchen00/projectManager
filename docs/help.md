# 项目管理系统 使用帮助手册（User Help）

> 适用版本：v1
> 阅读对象：申请人 / 协办人 / 审核人 / 审批人 / 管理员 / 查看者
> 配套文档：`requirement.md`（需求）、`design.md`（设计）、`http_api.md`（接口）、`workflow.md`（开发流程）

---

## 目录

1. [系统简介](#1-系统简介)
2. [快速开始](#2-快速开始)
3. [角色与权限](#3-角色与权限)
4. [项目生命周期与状态](#4-项目生命周期与状态)
5. [Web 操作指南](#5-web-操作指南)
   - 5.1 设置我的身份
   - 5.2 新建立项
   - 5.3 项目列表与检索
   - 5.4 我的待办
   - 5.5 项目详情：表格内容
   - 5.6 项目详情：修改记录
   - 5.7 项目详情：审批事件
   - 5.8 提交定稿与审批
6. [接口直接调用（高级用户）](#6-接口直接调用高级用户)
7. [常见问题（FAQ）](#7-常见问题-faq)
8. [错误码速查](#8-错误码速查)

---

## 1. 系统简介

本系统用于管理项目从 **立项 → 表格填报 → 审核 → 审批 → 归档** 的全生命周期，重点能力：

- **最小信息立项**：仅需"项目名称 + 业主单位名称"即可创建项目，立即进入表格填报。
- **多轮修改记录**：项目表格的每一次字段变更都会生成不可篡改的 ChangeLog（含操作人、角色、旧值、新值、时间、版本号）。
- **两级审批**：申请人提交定稿后，依次由审核人（一级）和审批人（二级）审批；任一级可驳回回到表格填报。
- **快照与版本对比**：每次提交定稿都会生成只读快照；可在两个版本号（revision）之间查看差异。
- **角色权限矩阵**：服务端基于角色二次校验，前端按钮按角色显示/隐藏。

---

## 2. 快速开始

### 2.1 启动后端

```powershell
# 在仓库根目录
$env:MONGO_URI="mongodb://127.0.0.1:27017"
$env:MONGO_DB="project_manager"
$env:HTTP_ADDR=":8080"
go run ./cmd/server
```

启动成功后会监听 `http://localhost:8080`。
> 首次启动时会自动写入内置默认表格模板（`form_templates` 集合为空时）。

### 2.2 打开 Web 后台

浏览器访问：`http://localhost:8080/`

页面顶部右侧填写"用户ID / 姓名 / 角色"后点击 **保存身份**，所有后续接口请求都会自动携带：

- `X-User-Id`
- `X-User-Name`
- `X-User-Role`

> 提示：身份保存在浏览器 LocalStorage，刷新后保留。切换角色后请重新点击 **保存身份**。

---

## 3. 角色与权限

| 角色 Code   | 名称   | 主要可做                                           |
| ----------- | ------ | -------------------------------------------------- |
| `applicant` | 申请人 | 立项、填写/修改表格、提交定稿                      |
| `editor`    | 协办人 | 协助修改表格中"开放给协办人"的字段                 |
| `reviewer`  | 审核人 | 一级审批：审核通过 / 审核驳回                      |
| `approver`  | 审批人 | 二级审批：终审通过 / 终审驳回                      |
| `admin`     | 管理员 | 等同申请人 + 模板/流程管理                         |
| `viewer`    | 查看者 | 只读查看项目、修改记录、审批事件                   |

权限矩阵速查：

| 动作                | applicant | editor | reviewer | approver | admin | viewer |
| ------------------- | :-------: | :----: | :------: | :------: | :---: | :----: |
| 新建立项            |     ✔     |        |          |          |   ✔   |        |
| 编辑表格字段        |     ✔     |   ✔*   |          |          |   ✔   |        |
| 提交定稿            |     ✔     |        |          |          |   ✔   |        |
| 审核（一级）        |           |        |    ✔     |          |       |        |
| 审批（二级）        |           |        |          |    ✔     |       |        |
| 驳回                |           |        |    ✔     |    ✔     |       |        |
| 查看项目 / 历史     |     ✔     |   ✔    |    ✔     |    ✔     |   ✔   |   ✔    |

> `editor*`：仅能编辑模板中 `editable_roles` 包含 `editor` 的字段。

---

## 4. 项目生命周期与状态

```
[新建] -- 立项 --> FORM_EDITING（表格填报中）
                     │
                     │ 多轮修改（每次保存生成 ChangeLog，revision +1）
                     ▼
                提交定稿 -> PENDING_REVIEW（待审核）
                     │
       审核通过 ──┐  │  ┌── 审核驳回 → 回到 FORM_EDITING（phase=REJECTED_REWORK）
                 ▼  ▼
              PENDING_APPROVE（待审批）
                     │
       终审通过 ──┐     ┌── 终审驳回 → 回到 FORM_EDITING
                 ▼
               APPROVED（已审批，只读归档）
```

| 状态               | 中文显示   | 说明                                                       |
| ------------------ | ---------- | ---------------------------------------------------------- |
| `FORM_EDITING`     | 表格填报中 | 可编辑、可保存、可提交定稿。被驳回也回到此态。             |
| `PENDING_REVIEW`   | 待审核     | 仅审核人可操作；表格只读。                                 |
| `PENDING_APPROVE`  | 待审批     | 仅审批人可操作；表格只读。                                 |
| `APPROVED`         | 已审批     | 已归档、整张表只读，所有写接口返回 409。                   |

> 关键约束：**只有在 `FORM_EDITING` 状态下才能修改表格**。其他状态请由审核人/审批人主动驳回后再修改。

---

## 5. Web 操作指南

### 5.1 设置我的身份

1. 打开页面顶部 **用户ID / 姓名 / 角色** 三个输入框。
2. 用户ID 自定义即可（演示版未做用户体系），如 `u-001`。
3. 选择当前你要扮演的角色（决定能看到哪些按钮、能调用哪些接口）。
4. 点击 **保存身份**。

> 切换角色不需要刷新页面，但**必须重新点击"保存身份"**。

### 5.2 新建立项（applicant / admin）

1. 顶部导航点击 **新建立项** 标签。
2. 填写：
   - **项目名称**（必填，<=100 字）
   - **业主单位名称**（必填）
   - 业主类型 / 联系人 / 电话 / 邮箱 / 地址（可选）
3. 点击 **立项**。
4. 立项成功页面会提示项目编号（形如 `PJ20260526AB12CD`），状态自动为 `FORM_EDITING`，并生成空的项目表格。
5. 如果系统提示 `duplicate_active=true`，说明已有相同 **项目名+业主单位名** 的未归档项目，**只是软提示**，仍可继续创建。

> 业主单位若名称已存在，系统会自动复用，不会重复建档。

### 5.3 项目列表与检索

顶部导航点击 **项目列表**，支持：

- **关键字**：模糊匹配项目名称 / 项目编号 / 业主单位名。
- **状态筛选**：表格填报中 / 待审核 / 待审批 / 已审批。
- **分页**：通过 URL/接口参数 `page`、`size` 控制（默认 page=1, size=20）。

每行的 **进入** 按钮可打开项目详情页。

### 5.4 我的待办

顶部导航点击 **我的待办**：

- 申请人：自己创建、且当前处于 `FORM_EDITING` 的项目（包含被驳回返工）。
- 审核人：所有 `PENDING_REVIEW` 项目。
- 审批人：所有 `PENDING_APPROVE` 项目。
- 其他角色：最近更新的最多 50 条项目。

### 5.5 项目详情 → 表格内容

进入项目详情后默认显示 **表格内容** 子页：

- 模板按 Section（分组）展开，每个字段会渲染为对应控件：
  - `text` / `textarea` 文本
  - `number` / `money` 数字
  - `date` 日期
  - `select` / `multiselect` 下拉
  - `file` 文件（上传 URL）
  - `table` 子表格
- 字段右上角带 `*` 表示必填。
- 字段下方可填写 **本次修改说明**（remark），强烈建议填写，方便审批人回溯原因。
- 点击 **保存** 后：
  - 仅对**实际发生变化的字段**生成 ChangeLog；未变化字段被忽略。
  - 一次保存中所有变化字段共享同一个 `revision` 号。
  - 全部字段无变化时，`revision` 不会增长。
- 字段是否可编辑取决于：
  1. 当前状态必须为 `FORM_EDITING`；
  2. 当前用户角色必须命中字段的 `editable_roles`。

### 5.6 项目详情 → 修改记录

子页 **修改记录** 提供三类视图：

1. **时间轴**：默认按 `revision` 倒序，展示每条变更。可通过：
   - `field_key` 过滤特定字段
   - `operator_id` 过滤操作人
   - 时间范围（接口参数 `from` / `to`，RFC3339）
2. **按字段历史**：在字段输入框中填入完整 `field_key`（如 `base_info.build_location`）后点击过滤，可只看该字段的历史。
3. **版本对比**：在 `from` / `to` 中填入两个 revision 号，点击 **查看差异** 即可。

每条记录展示：版本 R、字段、旧值、新值、操作人、角色、时间、阶段（`FORM_EDITING` 或 `REJECTED_REWORK`）、说明。

> 修改记录**只增不删**：管理员也无法物理删除，仅可逻辑隐藏（`hidden=true`）。

### 5.7 项目详情 → 审批事件

子页 **审批事件** 按时间倒序列出所有审批动作：

- 级别 1 = 审核（reviewer），级别 2 = 审批（approver）。
- 动作：`submit` 提交 / `approve` 通过 / `reject` 驳回。
- `submit` 事件会关联当时的 **快照ID**，可用来重看那一版的整张表格。

### 5.8 提交定稿与审批

#### 5.8.1 申请人：提交定稿

1. 在 `FORM_EDITING` 状态下点击 **提交定稿**。
2. 系统执行**完整性校验**：所有 `required=true` 的字段必须非空。
3. 校验失败 → 返回 422，前端高亮缺失字段（接口 `data.missing_fields` 给出具体 `field_key` 列表）。
4. 校验通过 → 状态变为 `PENDING_REVIEW`，并生成只读 **FormSnapshot**；同时写入一条 `submit` 审批事件。

#### 5.8.2 审核人：一级审核

1. 进入 **我的待办** 或 **项目列表**，状态为 `PENDING_REVIEW` 的项目。
2. 进入项目详情，可在 **表格内容** 子页查看本轮提交的内容；在 **修改记录** 子页通过版本对比查看本轮相对上一轮的差异。
3. 决策：
   - 点击 **审核通过** → 状态转 `PENDING_APPROVE`。
   - 点击 **审核驳回** → 弹窗输入驳回意见（必填），状态回到 `FORM_EDITING`。
4. 后续申请人再修改产生的 ChangeLog `phase = REJECTED_REWORK`，便于区分本轮返工。

#### 5.8.3 审批人：二级审批（终审）

1. 状态 `PENDING_APPROVE` 的项目可见于待办。
2. 决策：
   - 点击 **终审通过** → 状态变为 `APPROVED`，并写入 `approved_at`；表格此后**只读**。
   - 点击 **终审驳回** → 输入驳回意见后回到 `FORM_EDITING`。

#### 5.8.4 驳回与返工的最佳实践

- 驳回时必填 `comment`，说明问题点。
- 申请人按驳回意见修改字段，在保存时建议在 **本次修改说明** 中引用驳回意见关键词。
- 修改完成后再次 **提交定稿** 即可，新的快照与旧快照相互独立、可对比。

---

## 6. 接口直接调用（高级用户）

通用：

- Base URL：`http://<host>:<port>/api/v1`
- 必带请求头：`X-User-Id`、`X-User-Role`，可选 `X-User-Name`。
- 字段全部 `snake_case`；统一响应 `{ "code": 0, "message": "OK", "data": {...} }`。

最常用的几条 cURL 示例：

```bash
# 立项
curl -X POST http://localhost:8080/api/v1/projects \
  -H "Content-Type: application/json" \
  -H "X-User-Id: u-001" -H "X-User-Role: applicant" -H "X-User-Name: 张三" \
  -d '{"project_name":"示例项目","owner":{"name":"示例业主单位","owner_type":"企业"}}'

# 保存表格（产生 ChangeLog）
curl -X PATCH http://localhost:8080/api/v1/projects/<id>/form \
  -H "Content-Type: application/json" \
  -H "X-User-Id: u-001" -H "X-User-Role: applicant" \
  -d '{"changes":[{"field_key":"base_info.build_location","new_value":"深圳福田","remark":"调整片区"}]}'

# 提交定稿
curl -X POST http://localhost:8080/api/v1/projects/<id>/submit \
  -H "X-User-Id: u-001" -H "X-User-Role: applicant"

# 审核通过
curl -X POST http://localhost:8080/api/v1/projects/<id>/approvals/review \
  -H "Content-Type: application/json" \
  -H "X-User-Id: u-002" -H "X-User-Role: reviewer" \
  -d '{"action":"approve"}'

# 终审驳回
curl -X POST http://localhost:8080/api/v1/projects/<id>/approvals/final \
  -H "Content-Type: application/json" \
  -H "X-User-Id: u-003" -H "X-User-Role: approver" \
  -d '{"action":"reject","comment":"投资估算明显不足"}'

# 查询修改记录
curl "http://localhost:8080/api/v1/projects/<id>/changelogs?page=1&size=50" \
  -H "X-User-Id: u-001" -H "X-User-Role: applicant"

# 版本对比
curl "http://localhost:8080/api/v1/projects/<id>/diff?from=1&to=3" \
  -H "X-User-Id: u-001" -H "X-User-Role: applicant"
```

完整接口与字段定义详见 [`docs/http_api.md`](./http_api.md)。

---

## 7. 常见问题（FAQ）

**Q1：保存表格后没有产生修改记录？**
A：服务端会对每个字段做深比较，**值未变化的字段不写日志**。如果整次提交全部字段都没变化，`revision` 也不会增长。

**Q2：明明是我提交的项目，为什么"我的待办"没显示？**
A：申请人的待办仅显示当前处于 `FORM_EDITING` 的项目；项目处于审批中（`PENDING_REVIEW` / `PENDING_APPROVE`）时不在待办，可在 **项目列表** 中查看。

**Q3：点击保存提示 `STATE_CONFLICT (409)`？**
A：项目当前不在 `FORM_EDITING` 状态。如确需修改，请联系审核人/审批人**主动驳回**项目后再编辑。

**Q4：提交定稿提示 `VALIDATION_FAILED (422)`？**
A：必填字段未填齐。响应 `data.missing_fields` 列出了缺失的 `field_key` 列表，前端会高亮。

**Q5：驳回时提示 `comment required on reject (1001)`？**
A：审核驳回 / 终审驳回时**必须**填写意见，请在弹窗中输入驳回原因后再提交。

**Q6：怎么看"本轮提交相对上一版改了什么"？**
A：进入项目详情 → 修改记录 → 版本对比 → 在 from / to 中填入要比较的两个 revision，例如对比上一次提交时的 revision 与最新 revision。

**Q7：项目已审批通过，发现还要改怎么办？**
A：本期 `APPROVED` 后表格强制只读，**不允许再修改**。如需变更，由管理员评估流程后另行处理（v1 不开放反向回退）。

**Q8：管理员能删除一个项目吗？**
A：v1 暂不开放删除接口。修改记录、审批事件按合规要求保留至少 5 年。

---

## 8. 错误码速查

| code | http | message            | 何时出现 / 处理建议                                |
| ---- | ---- | ------------------ | -------------------------------------------------- |
| 0    | 200  | OK                 | 正常                                               |
| 1001 | 400  | INVALID_PARAM      | 参数缺失或格式错误，按 `data.detail` 修正          |
| 1002 | 401  | UNAUTHORIZED       | 未携带 `X-User-Id`，请先在顶部"保存身份"           |
| 1003 | 403  | FORBIDDEN          | 当前角色无权限，切换正确角色后再试                 |
| 1004 | 404  | NOT_FOUND          | 项目/字段不存在，确认路径参数                      |
| 1005 | 409  | STATE_CONFLICT     | 当前状态不允许该动作（如非 FORM_EDITING 时改表格） |
| 1006 | 422  | VALIDATION_FAILED  | 必填字段缺失，见 `data.missing_fields`             |
| 2001 | 500  | INTERNAL_ERROR     | 内部错误，查看后端日志                             |
| 2002 | 500  | DB_ERROR           | DB 异常，检查 MongoDB 连接                         |

---

> 如需进一步了解系统设计细节，请阅读 `docs/design.md`；
> 如需了解所有接口字段及完整 JSON 结构，请阅读 `docs/http_api.md`。
