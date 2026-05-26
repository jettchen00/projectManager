package httpapi

import (
	"errors"
	"net/http"

	"projectManager/internal/errcode"
	pmlog "projectManager/internal/log"
	"projectManager/internal/middleware"
	"projectManager/internal/modules/formtemplate"
	"projectManager/internal/modules/formvalue"
	"projectManager/internal/modules/project"
	"projectManager/internal/router"
)

// FormController 表格控制器。
type FormController struct {
	formSvc *formvalue.Service
	tmplSvc *formtemplate.Service
}

// NewFormController 构造。
func NewFormController(fs *formvalue.Service, ts *formtemplate.Service) *FormController {
	return &FormController{formSvc: fs, tmplSvc: ts}
}

// Get 取项目当前表格（模板 + 值）。
func (c *FormController) Get(w http.ResponseWriter, r *http.Request) {
	id := router.Param(r, "id")
	p, tmpl, vmap, err := c.formSvc.GetForm(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			errcode.WriteErr(w, errcode.CodeNotFound, nil)
			return
		}
		errcode.WriteErr(w, errcode.CodeDBError, nil)
		return
	}
	errcode.WriteOK(w, map[string]interface{}{
		"project":  projectBrief(p),
		"template": tmpl,
		"values":   vmap,
	})
}

// SaveRequest 保存请求体。
type SaveRequest struct {
	Changes []formvalue.Change `json:"changes"`
}

// Patch 保存表格字段（产生 ChangeLog）。
func (c *FormController) Patch(w http.ResponseWriter, r *http.Request) {
	id := router.Param(r, "id")
	u := middleware.CurrentUser(r)
	if u == nil {
		errcode.WriteErr(w, errcode.CodeUnauthorized, nil)
		return
	}
	var req SaveRequest
	if err := decodeJSON(r, &req); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if len(req.Changes) == 0 {
		writeBadRequest(w, "changes empty")
		return
	}
	// 规则 R17：参数校验。
	for _, ch := range req.Changes {
		if ch.FieldKey == "" {
			writeBadRequest(w, "field_key empty")
			return
		}
	}
	res, err := c.formSvc.Save(r.Context(), &formvalue.SaveInput{
		ProjectID:    id,
		OperatorID:   u.ID,
		OperatorName: u.Name,
		OperatorRole: u.Role,
		Changes:      req.Changes,
	})
	if err != nil {
		switch {
		case errors.Is(err, formvalue.ErrStateConflict):
			errcode.WriteErr(w, errcode.CodeStateConflict, nil)
		case errors.Is(err, formvalue.ErrFieldUnknown):
			errcode.WriteErr(w, errcode.CodeInvalidParam, map[string]string{"detail": "unknown field_key"})
		case errors.Is(err, formvalue.ErrFieldForbid):
			errcode.WriteErr(w, errcode.CodeForbidden, map[string]string{"detail": "field not editable for role"})
		case errors.Is(err, project.ErrNotFound):
			errcode.WriteErr(w, errcode.CodeNotFound, nil)
		default:
			pmlog.Errorf("form Save err=%v", err)
			errcode.WriteErr(w, errcode.CodeDBError, nil)
		}
		return
	}
	errcode.WriteOK(w, res)
}

// Template 取当前模板。
func (c *FormController) Template(w http.ResponseWriter, r *http.Request) {
	t, err := c.tmplSvc.GetActive(r.Context())
	if err != nil {
		errcode.WriteErr(w, errcode.CodeDBError, nil)
		return
	}
	if t == nil {
		errcode.WriteErr(w, errcode.CodeNotFound, nil)
		return
	}
	errcode.WriteOK(w, t)
}
