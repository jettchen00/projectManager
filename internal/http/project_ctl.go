package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"projectManager/internal/errcode"
	pmlog "projectManager/internal/log"
	"projectManager/internal/middleware"
	"projectManager/internal/modules/project"
	"projectManager/internal/router"
)

// ProjectController 项目相关控制器。
type ProjectController struct {
	svc *project.Service
}

// NewProjectController 构造。
func NewProjectController(s *project.Service) *ProjectController {
	return &ProjectController{svc: s}
}

// CreateRequest 立项请求体（蛇形命名，规则 R20）。
type CreateRequest struct {
	ProjectName string `json:"project_name"`
	Owner       struct {
		Name         string `json:"name"`
		OwnerType    string `json:"owner_type"`
		ContactName  string `json:"contact_name"`
		ContactPhone string `json:"contact_phone"`
		ContactEmail string `json:"contact_email"`
		Address      string `json:"address"`
	} `json:"owner"`
}

// Create 立项。
func (c *ProjectController) Create(w http.ResponseWriter, r *http.Request) {
	u := middleware.CurrentUser(r)
	if u == nil {
		errcode.WriteErr(w, errcode.CodeUnauthorized, nil)
		return
	}
	var req CreateRequest
	if err := decodeJSON(r, &req); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	in := &project.CreateInput{
		ProjectName:   strings.TrimSpace(req.ProjectName),
		OwnerName:     strings.TrimSpace(req.Owner.Name),
		OwnerType:     req.Owner.OwnerType,
		ContactName:   req.Owner.ContactName,
		ContactPhone:  req.Owner.ContactPhone,
		ContactEmail:  req.Owner.ContactEmail,
		Address:       req.Owner.Address,
		ApplicantID:   u.ID,
		ApplicantName: u.Name,
	}
	p, dup, err := c.svc.Create(r.Context(), in)
	if err != nil {
		if errors.Is(err, project.ErrInvalidParam) {
			writeBadRequest(w, err.Error())
			return
		}
		pmlog.Errorf("project Create err=%v", err)
		errcode.WriteErr(w, errcode.CodeDBError, nil)
		return
	}
	errcode.WriteOK(w, map[string]interface{}{
		"id":               p.ID.Hex(),
		"project_code":     p.ProjectCode,
		"project_name":     p.ProjectName,
		"owner_id":         p.OwnerID.Hex(),
		"owner_name":       p.OwnerName,
		"status":           p.Status,
		"current_revision": p.CurrentRevision,
		"applicant_id":     p.ApplicantID,
		"applicant_name":   p.ApplicantName,
		"duplicate_active": dup,
	})
}

// List 列表。
func (c *ProjectController) List(w http.ResponseWriter, r *http.Request) {
	q := project.ListQuery{
		Status:      r.URL.Query().Get("status"),
		Keyword:     r.URL.Query().Get("keyword"),
		OwnerID:     r.URL.Query().Get("owner_id"),
		ApplicantID: r.URL.Query().Get("applicant_id"),
		Page:        queryInt(r, "page", 1),
		Size:        queryInt(r, "size", 20),
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
		"total": total,
		"page":  q.Page,
		"size":  q.Size,
		"items": items,
	})
}

// Detail 详情。
func (c *ProjectController) Detail(w http.ResponseWriter, r *http.Request) {
	id := router.Param(r, "id")
	p, err := c.svc.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			errcode.WriteErr(w, errcode.CodeNotFound, nil)
			return
		}
		errcode.WriteErr(w, errcode.CodeDBError, nil)
		return
	}
	errcode.WriteOK(w, projectBrief(p))
}

// projectBrief 标准化输出。
func projectBrief(p *project.Project) map[string]interface{} {
	out := map[string]interface{}{
		"id":               p.ID.Hex(),
		"project_code":     p.ProjectCode,
		"project_name":     p.ProjectName,
		"owner_id":         p.OwnerID.Hex(),
		"owner_name":       p.OwnerName,
		"status":           p.Status,
		"current_revision": p.CurrentRevision,
		"applicant_id":     p.ApplicantID,
		"applicant_name":   p.ApplicantName,
		"last_phase":       p.LastPhase,
		"created_at":       p.CreatedAt,
		"updated_at":       p.UpdatedAt,
	}
	if p.ApprovedAt != nil {
		out["approved_at"] = *p.ApprovedAt
	}
	return out
}
