# projectManager HTTP 接口文档

> Base path：`/api/v1`
> 所有请求/响应字段均为 **snake_case**（规则 R20）。
> 统一响应结构（规则 R06）：
>
> ```json
> { "code": 0, "message": "OK", "data": {} }
> ```
>
> 鉴权：所有需登录接口要求请求头：
> - `X-User-Id`：用户 ID（必填）
> - `X-User-Role`：用户角色 `applicant` / `editor` / `reviewer` / `approver` / `admin` / `viewer`
> - `X-User-Name`：用户姓名（可选，记录到 ChangeLog 操作人字段）

## 错误码（规则 R19）

| code | http | message            | 含义 |
|------|------|--------------------|------|
| 0    | 200  | OK                 | 成功 |
| 1001 | 400  | INVALID_PARAM      | 参数错误 |
| 1002 | 401  | UNAUTHORIZED       | 未登录 |
| 1003 | 403  | FORBIDDEN          | 无权限 |
| 1004 | 404  | NOT_FOUND          | 资源不存在 |
| 1005 | 409  | STATE_CONFLICT     | 状态机不允许 |
| 1006 | 422  | VALIDATION_FAILED  | 校验失败（含 missing_fields） |
| 2001 | 500  | INTERNAL_ERROR     | 内部错误 |
| 2002 | 500  | DB_ERROR           | DB 异常 |

---

## 1. 立项

### POST `/api/v1/projects`
角色：`applicant` / `admin`

请求：
```json
{
  "project_name": "示例项目",
  "owner": {
    "name": "示例业主单位",
    "owner_type": "企业",
    "contact_name": "张三",
    "contact_phone": "13800000000",
    "contact_email": "zs@example.com",
    "address": "深圳南山"
  }
}
```

成功响应：
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "id": "65f0c2a3a1b2c3d4e5f6a7b8",
    "project_code": "PJ20260526AB12CD",
    "project_name": "示例项目",
    "owner_id": "65f0c2a3a1b2c3d4e5f6a7c0",
    "owner_name": "示例业主单位",
    "status": "FORM_EDITING",
    "current_revision": 0,
    "applicant_id": "u-001",
    "applicant_name": "张三",
    "duplicate_active": false
  }
}
```

失败响应（缺少 `project_name`）：
```json
{
  "code": 1001,
  "message": "INVALID_PARAM",
  "data": { "detail": "invalid_param: project_name required" }
}
```

---

## 2. 项目列表

### GET `/api/v1/projects`
查询参数：`status` `keyword` `owner_id` `applicant_id` `page` `size`

响应：
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "total": 1,
    "page": 1,
    "size": 20,
    "items": [
      {
        "id": "65f0c2a3a1b2c3d4e5f6a7b8",
        "project_code": "PJ20260526AB12CD",
        "project_name": "示例项目",
        "owner_id": "65f0c2a3a1b2c3d4e5f6a7c0",
        "owner_name": "示例业主单位",
        "status": "FORM_EDITING",
        "current_revision": 1,
        "applicant_id": "u-001",
        "applicant_name": "张三",
        "last_phase": "FORM_EDITING",
        "created_at": "2026-05-26T10:00:00Z",
        "updated_at": "2026-05-26T10:30:00Z",
        "approved_at": null
      }
    ]
  }
}
```

---

## 3. 项目详情

### GET `/api/v1/projects/{id}`

响应：
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "id": "65f0c2a3a1b2c3d4e5f6a7b8",
    "project_code": "PJ20260526AB12CD",
    "project_name": "示例项目",
    "owner_id": "65f0c2a3a1b2c3d4e5f6a7c0",
    "owner_name": "示例业主单位",
    "status": "FORM_EDITING",
    "current_revision": 1,
    "applicant_id": "u-001",
    "applicant_name": "张三",
    "last_phase": "FORM_EDITING",
    "created_at": "2026-05-26T10:00:00Z",
    "updated_at": "2026-05-26T10:30:00Z"
  }
}
```

---

## 4. 表格读写

### GET `/api/v1/projects/{id}/form`

响应：
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "project": {
      "id": "65f0c2a3a1b2c3d4e5f6a7b8",
      "project_code": "PJ20260526AB12CD",
      "project_name": "示例项目",
      "owner_id": "65f0c2a3a1b2c3d4e5f6a7c0",
      "owner_name": "示例业主单位",
      "status": "FORM_EDITING",
      "current_revision": 1,
      "applicant_id": "u-001",
      "applicant_name": "张三",
      "last_phase": "FORM_EDITING",
      "created_at": "2026-05-26T10:00:00Z",
      "updated_at": "2026-05-26T10:30:00Z"
    },
    "template": {
      "id": "65f0c2a3a1b2c3d4e5f6a7d0",
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
              "editable_roles": ["applicant", "editor", "admin"]
            }
          ]
        }
      ],
      "created_at": "2026-05-26T09:00:00Z"
    },
    "values": {
      "base_info.project_overview": "项目目的与范围…",
      "base_info.build_location": "深圳南山",
      "base_info.build_period": "12个月"
    }
  }
}
```

### PATCH `/api/v1/projects/{id}/form`
角色：`applicant` / `editor` / `admin`

请求：
```json
{
  "changes": [
    { "field_key": "base_info.build_location", "new_value": "深圳福田", "remark": "调整片区" },
    { "field_key": "investment.total_amount", "new_value": 1000000 }
  ]
}
```

成功响应（共享同一 revision）：
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "revision": 2,
    "applied_count": 2,
    "skipped_count": 0
  }
}
```

未发生实际变更：
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "revision": 1,
    "applied_count": 0,
    "skipped_count": 1
  }
}
```

非 `FORM_EDITING` 状态：
```json
{ "code": 1005, "message": "STATE_CONFLICT" }
```

---

## 5. 修改记录

### GET `/api/v1/projects/{id}/changelogs`
查询参数：`field_key` `operator_id` `role` `from`(RFC3339) `to`(RFC3339) `page` `size`

响应：
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "total": 2,
    "page": 1,
    "size": 50,
    "items": [
      {
        "id": "65f0c2a3a1b2c3d4e5f6a7e0",
        "project_id": "65f0c2a3a1b2c3d4e5f6a7b8",
        "field_key": "base_info.build_location",
        "field_label": "建设地点",
        "old_value": "深圳南山",
        "new_value": "深圳福田",
        "operator_id": "u-001",
        "operator_name": "张三",
        "operator_role": "applicant",
        "operated_at": "2026-05-26T10:30:00Z",
        "revision": 2,
        "remark": "调整片区",
        "phase": "FORM_EDITING"
      }
    ]
  }
}
```

### GET `/api/v1/projects/{id}/changelogs/by-field/{field_key}`

响应：
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "field_key": "base_info.build_location",
    "items": [
      {
        "id": "65f0c2a3a1b2c3d4e5f6a7e0",
        "project_id": "65f0c2a3a1b2c3d4e5f6a7b8",
        "field_key": "base_info.build_location",
        "field_label": "建设地点",
        "old_value": "深圳南山",
        "new_value": "深圳福田",
        "operator_id": "u-001",
        "operator_name": "张三",
        "operator_role": "applicant",
        "operated_at": "2026-05-26T10:30:00Z",
        "revision": 2,
        "remark": "调整片区",
        "phase": "FORM_EDITING"
      }
    ]
  }
}
```

### GET `/api/v1/projects/{id}/diff?from=R1&to=R2`

响应：
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "from": 1,
    "to": 2,
    "items": [
      {
        "id": "65f0c2a3a1b2c3d4e5f6a7e0",
        "project_id": "65f0c2a3a1b2c3d4e5f6a7b8",
        "field_key": "base_info.build_location",
        "field_label": "建设地点",
        "old_value": "深圳南山",
        "new_value": "深圳福田",
        "operator_id": "u-001",
        "operator_name": "张三",
        "operator_role": "applicant",
        "operated_at": "2026-05-26T10:30:00Z",
        "revision": 2,
        "remark": "",
        "phase": "FORM_EDITING"
      }
    ]
  }
}
```

---

## 6. 提交定稿与审批

### POST `/api/v1/projects/{id}/submit`
角色：`applicant` / `admin`

成功响应：
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "status": "PENDING_REVIEW",
    "snapshot_id": "65f0c2a3a1b2c3d4e5f6a8a0"
  }
}
```

校验失败：
```json
{
  "code": 1006,
  "message": "VALIDATION_FAILED",
  "data": {
    "missing_fields": [
      "base_info.project_overview",
      "investment.total_amount"
    ]
  }
}
```

### POST `/api/v1/projects/{id}/approvals/review`
角色：`reviewer`

请求：
```json
{ "action": "approve", "comment": "" }
```
或
```json
{ "action": "reject", "comment": "投资估算明显不足" }
```

成功响应：
```json
{
  "code": 0,
  "message": "OK",
  "data": { "status": "PENDING_APPROVE" }
}
```

驳回成功响应：
```json
{
  "code": 0,
  "message": "OK",
  "data": { "status": "FORM_EDITING" }
}
```

reject 时未填 comment：
```json
{
  "code": 1001,
  "message": "INVALID_PARAM",
  "data": { "detail": "comment required on reject" }
}
```

### POST `/api/v1/projects/{id}/approvals/final`
角色：`approver`

请求与响应结构同 `review`，approve 后状态为 `APPROVED`。

### GET `/api/v1/projects/{id}/approvals`

响应：
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "items": [
      {
        "id": "65f0c2a3a1b2c3d4e5f6a900",
        "project_id": "65f0c2a3a1b2c3d4e5f6a7b8",
        "level": 2,
        "action": "approve",
        "operator_id": "u-003",
        "operator_name": "李四",
        "operator_role": "approver",
        "comment": "",
        "created_at": "2026-05-26T11:00:00Z"
      },
      {
        "id": "65f0c2a3a1b2c3d4e5f6a8f0",
        "project_id": "65f0c2a3a1b2c3d4e5f6a7b8",
        "level": 1,
        "action": "submit",
        "operator_id": "u-001",
        "operator_name": "张三",
        "operator_role": "applicant",
        "comment": "",
        "snapshot_id": "65f0c2a3a1b2c3d4e5f6a8a0",
        "created_at": "2026-05-26T10:50:00Z"
      }
    ]
  }
}
```

---

## 7. 模板与业主单位

### GET `/api/v1/form-template`

响应：
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "id": "65f0c2a3a1b2c3d4e5f6a7d0",
    "version": 1,
    "active": true,
    "sections": [
      {
        "section_key": "base_info",
        "section_label": "项目基本信息",
        "fields": [
          { "field_key": "base_info.project_overview", "label": "项目概况", "type": "textarea", "required": true, "editable_roles": ["applicant","editor","admin"] }
        ]
      }
    ],
    "created_at": "2026-05-26T09:00:00Z"
  }
}
```

### GET `/api/v1/owners?keyword=&limit=`

响应：
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "items": [
      {
        "id": "65f0c2a3a1b2c3d4e5f6a7c0",
        "name": "示例业主单位",
        "owner_type": "企业",
        "contact_name": "张三",
        "contact_phone": "13800000000",
        "contact_email": "zs@example.com",
        "address": "深圳南山",
        "created_at": "2026-05-20T08:00:00Z"
      }
    ]
  }
}
```

---

## 8. 当前用户待办

### GET `/api/v1/me/todos`

响应（不同角色 items 含义不同）：
```json
{
  "code": 0,
  "message": "OK",
  "data": {
    "role": "reviewer",
    "total": 1,
    "items": [
      {
        "id": "65f0c2a3a1b2c3d4e5f6a7b8",
        "project_code": "PJ20260526AB12CD",
        "project_name": "示例项目",
        "owner_id": "65f0c2a3a1b2c3d4e5f6a7c0",
        "owner_name": "示例业主单位",
        "status": "PENDING_REVIEW",
        "current_revision": 1,
        "applicant_id": "u-001",
        "applicant_name": "张三",
        "last_phase": "FORM_EDITING",
        "created_at": "2026-05-26T10:00:00Z",
        "updated_at": "2026-05-26T10:50:00Z"
      }
    ]
  }
}
```

聚合规则：
- `applicant`：自己创建的、当前处于 `FORM_EDITING` 的项目（含被驳回返工）。
- `reviewer`：所有 `PENDING_REVIEW` 项目。
- `approver`：所有 `PENDING_APPROVE` 项目。
- 其他角色：返回最近更新的项目（最多 50 条）。
