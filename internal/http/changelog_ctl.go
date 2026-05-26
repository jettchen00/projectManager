package httpapi

import (
	"net/http"
	"time"

	"projectManager/internal/errcode"
	"projectManager/internal/modules/changelog"
	"projectManager/internal/router"
)

// ChangeLogController 修改记录控制器。
type ChangeLogController struct {
	svc *changelog.Service
}

// NewChangeLogController 构造。
func NewChangeLogController(s *changelog.Service) *ChangeLogController {
	return &ChangeLogController{svc: s}
}

// Timeline 时间轴。
func (c *ChangeLogController) Timeline(w http.ResponseWriter, r *http.Request) {
	id := router.Param(r, "id")
	q := changelog.Query{
		ProjectID:    id,
		FieldKey:     r.URL.Query().Get("field_key"),
		OperatorID:   r.URL.Query().Get("operator_id"),
		OperatorRole: r.URL.Query().Get("role"),
		Page:         queryInt(r, "page", 1),
		Size:         queryInt(r, "size", 50),
	}
	if v := r.URL.Query().Get("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q.From = &t
		}
	}
	if v := r.URL.Query().Get("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			q.To = &t
		}
	}
	list, total, err := c.svc.Timeline(r.Context(), q)
	if err != nil {
		errcode.WriteErr(w, errcode.CodeDBError, nil)
		return
	}
	items := make([]map[string]interface{}, 0, len(list))
	for _, l := range list {
		items = append(items, logBrief(l))
	}
	errcode.WriteOK(w, map[string]interface{}{
		"total": total,
		"page":  q.Page,
		"size":  q.Size,
		"items": items,
	})
}

// ByField 字段历史。
func (c *ChangeLogController) ByField(w http.ResponseWriter, r *http.Request) {
	id := router.Param(r, "id")
	field := router.Param(r, "field_key")
	if field == "" {
		field = r.URL.Query().Get("field_key")
	}
	if field == "" {
		writeBadRequest(w, "field_key required")
		return
	}
	list, err := c.svc.ByField(r.Context(), id, field)
	if err != nil {
		errcode.WriteErr(w, errcode.CodeDBError, nil)
		return
	}
	items := make([]map[string]interface{}, 0, len(list))
	for _, l := range list {
		items = append(items, logBrief(l))
	}
	errcode.WriteOK(w, map[string]interface{}{
		"field_key": field,
		"items":     items,
	})
}

// Diff 版本对比。
func (c *ChangeLogController) Diff(w http.ResponseWriter, r *http.Request) {
	id := router.Param(r, "id")
	from := int32(queryInt(r, "from", 0))
	to := int32(queryInt(r, "to", 0))
	if to <= 0 {
		writeBadRequest(w, "to required")
		return
	}
	list, err := c.svc.Diff(r.Context(), id, from, to)
	if err != nil {
		errcode.WriteErr(w, errcode.CodeDBError, nil)
		return
	}
	items := make([]map[string]interface{}, 0, len(list))
	for _, l := range list {
		items = append(items, logBrief(l))
	}
	errcode.WriteOK(w, map[string]interface{}{
		"from":  from,
		"to":    to,
		"items": items,
	})
}

func logBrief(l *changelog.ChangeLog) map[string]interface{} {
	return map[string]interface{}{
		"id":            l.ID.Hex(),
		"project_id":    l.ProjectID.Hex(),
		"field_key":     l.FieldKey,
		"field_label":   l.FieldLabel,
		"old_value":     l.OldValue,
		"new_value":     l.NewValue,
		"operator_id":   l.OperatorID,
		"operator_name": l.OperatorName,
		"operator_role": l.OperatorRole,
		"operated_at":   l.OperatedAt,
		"revision":      l.Revision,
		"remark":        l.Remark,
		"phase":         l.Phase,
	}
}
