package httpapi

import (
	"net/http"

	"projectManager/internal/middleware"
	"projectManager/internal/router"
)

// Deps 控制器依赖集合。
type Deps struct {
	Project   *ProjectController
	Form      *FormController
	ChangeLog *ChangeLogController
	Approval  *ApprovalController
	Owner     *OwnerController
	Todo      *TodoController
}

// BuildRouter 装配路由。
func BuildRouter(d *Deps, webDir string) http.Handler {
	r := router.New()
	r.Use(middleware.Recover)
	r.Use(middleware.CORS)
	r.Use(middleware.AccessLog)
	r.Use(middleware.Auth)

	// 业主单位
	r.GET("/api/v1/owners", d.Owner.List)

	// 模板
	r.GET("/api/v1/form-template", d.Form.Template)

	// 项目
	r.POST("/api/v1/projects", middleware.RequireRoles("applicant", "admin")(d.Project.Create))
	r.GET("/api/v1/projects", d.Project.List)
	r.GET("/api/v1/projects/{id}", d.Project.Detail)

	// 表格
	r.GET("/api/v1/projects/{id}/form", d.Form.Get)
	r.PATCH("/api/v1/projects/{id}/form",
		middleware.RequireRoles("applicant", "editor", "admin")(d.Form.Patch))

	// 修改记录
	r.GET("/api/v1/projects/{id}/changelogs", d.ChangeLog.Timeline)
	r.GET("/api/v1/projects/{id}/changelogs/by-field/{field_key}", d.ChangeLog.ByField)
	r.GET("/api/v1/projects/{id}/diff", d.ChangeLog.Diff)

	// 审批
	r.POST("/api/v1/projects/{id}/submit",
		middleware.RequireRoles("applicant", "admin")(d.Approval.Submit))
	r.POST("/api/v1/projects/{id}/approvals/review",
		middleware.RequireRoles("reviewer")(d.Approval.Review))
	r.POST("/api/v1/projects/{id}/approvals/final",
		middleware.RequireRoles("approver")(d.Approval.Final))
	r.GET("/api/v1/projects/{id}/approvals", d.Approval.List)

	// 待办
	r.GET("/api/v1/me/todos", d.Todo.Mine)

	// 静态资源（web）。
	mux := http.NewServeMux()
	mux.Handle("/api/", r)
	if webDir != "" {
		fs := http.FileServer(http.Dir(webDir))
		mux.Handle("/", fs)
	}
	return mux
}
