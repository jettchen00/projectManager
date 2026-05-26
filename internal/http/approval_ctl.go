package httpapi

import (
	"errors"
	"net/http"

	"projectManager/internal/errcode"
	pmlog "projectManager/internal/log"
	"projectManager/internal/middleware"
	"projectManager/internal/modules/approval"
	"projectManager/internal/router"
)

// ApprovalController 审批控制器。
type ApprovalController struct {
	svc *approval.Service
}

// NewApprovalController 构造。
func NewApprovalController(s *approval.Service) *ApprovalController {
	return &ApprovalController{svc: s}
}

// DecideRequest 审核 / 审批请求。
type DecideRequest struct {
	Action  string `json:"action"`
	Comment string `json:"comment"`
}

// Submit 提交定稿。
func (c *ApprovalController) Submit(w http.ResponseWriter, r *http.Request) {
	id := router.Param(r, "id")
	u := middleware.CurrentUser(r)
	if u == nil {
		errcode.WriteErr(w, errcode.CodeUnauthorized, nil)
		return
	}
	res, err := c.svc.Submit(r.Context(), id, u.ID, u.Name, u.Role)
	if err != nil {
		switch {
		case errors.Is(err, approval.ErrStateConflict):
			errcode.WriteErr(w, errcode.CodeStateConflict, nil)
		case errors.Is(err, approval.ErrValidation):
			errcode.WriteErr(w, errcode.CodeValidationFail, map[string]interface{}{
				"missing_fields": res.MissingFields,
			})
		default:
			pmlog.Errorf("approval Submit err=%v project_id=%s", err, id)
			errcode.WriteErr(w, errcode.CodeDBError, nil)
		}
		return
	}
	errcode.WriteOK(w, res)
}

// Review 一级审核。
func (c *ApprovalController) Review(w http.ResponseWriter, r *http.Request) {
	c.decide(w, r, approval.LevelReview)
}

// Final 二级审批。
func (c *ApprovalController) Final(w http.ResponseWriter, r *http.Request) {
	c.decide(w, r, approval.LevelFinal)
}

func (c *ApprovalController) decide(w http.ResponseWriter, r *http.Request, level int32) {
	id := router.Param(r, "id")
	u := middleware.CurrentUser(r)
	if u == nil {
		errcode.WriteErr(w, errcode.CodeUnauthorized, nil)
		return
	}
	var req DecideRequest
	if err := decodeJSON(r, &req); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	to, err := c.svc.Decide(r.Context(), id, level, req.Action, req.Comment, u.ID, u.Name, u.Role)
	if err != nil {
		switch {
		case errors.Is(err, approval.ErrStateConflict):
			errcode.WriteErr(w, errcode.CodeStateConflict, nil)
		case errors.Is(err, approval.ErrCommentEmpty):
			errcode.WriteErr(w, errcode.CodeInvalidParam, map[string]string{"detail": "comment required on reject"})
		case errors.Is(err, approval.ErrInvalidParam):
			writeBadRequest(w, "invalid action")
		default:
			pmlog.Errorf("approval Decide err=%v project_id=%s level=%d", err, id, level)
			errcode.WriteErr(w, errcode.CodeDBError, nil)
		}
		return
	}
	errcode.WriteOK(w, map[string]interface{}{
		"status": to,
	})
}

// List 审批事件。
func (c *ApprovalController) List(w http.ResponseWriter, r *http.Request) {
	id := router.Param(r, "id")
	list, err := c.svc.List(r.Context(), id)
	if err != nil {
		errcode.WriteErr(w, errcode.CodeDBError, nil)
		return
	}
	items := make([]map[string]interface{}, 0, len(list))
	for _, e := range list {
		item := map[string]interface{}{
			"id":            e.ID.Hex(),
			"project_id":    e.ProjectID.Hex(),
			"level":         e.Level,
			"action":        e.Action,
			"operator_id":   e.OperatorID,
			"operator_name": e.OperatorName,
			"operator_role": e.OperatorRole,
			"comment":       e.Comment,
			"created_at":    e.CreatedAt,
		}
		if e.SnapshotID != nil {
			item["snapshot_id"] = e.SnapshotID.Hex()
		}
		items = append(items, item)
	}
	errcode.WriteOK(w, map[string]interface{}{"items": items})
}
