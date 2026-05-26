package httpapi

import (
	"net/http"

	"projectManager/internal/errcode"
	"projectManager/internal/middleware"
	"projectManager/internal/modules/project"
)

// TodoController 待办（按当前用户角色聚合）。
type TodoController struct {
	svc *project.Service
}

// NewTodoController 构造。
func NewTodoController(s *project.Service) *TodoController { return &TodoController{svc: s} }

// Mine 当前用户待办。
func (c *TodoController) Mine(w http.ResponseWriter, r *http.Request) {
	u := middleware.CurrentUser(r)
	if u == nil {
		errcode.WriteErr(w, errcode.CodeUnauthorized, nil)
		return
	}
	q := project.ListQuery{Page: 1, Size: 50}
	switch u.Role {
	case "applicant":
		// 申请人关心：被驳回（FORM_EDITING + last_phase=REJECTED_REWORK）以及自己创建的草稿态。
		q.ApplicantID = u.ID
		q.Status = project.StatusFormEditing
	case "reviewer":
		q.Status = project.StatusPendingReview
	case "approver":
		q.Status = project.StatusPendingApprove
	default:
		// admin / viewer / editor 默认返回最近 N 个项目。
	}
	list, total, err := c.svc.List(r.Context(), q)
	if err != nil {
		errcode.WriteErr(w, errcode.CodeDBError, nil)
		return
	}
	items := make([]map[string]interface{}, 0, len(list))
	for _, p := range list {
		items = append(items, projectBrief(p))
	}
	errcode.WriteOK(w, map[string]interface{}{
		"role":  u.Role,
		"total": total,
		"items": items,
	})
}
