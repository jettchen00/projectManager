package httpapi

import (
	"net/http"
	"os"
	"path/filepath"
	pmlog "projectManager/internal/log"
	"strings"

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
		mux.Handle("/", staticFileGuard(webDir, fs))
	}
	return mux
}

// staticFileGuard 在交给 http.FileServer 之前，先校验请求的静态文件是否存在；
// 不存在则直接返回 404，避免 FileServer 对目录返回索引页或对缺失文件回落到其它行为。
// 同时清理路径，防止通过 ".." 跳出 webDir。
func staticFileGuard(webDir string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// 规范化 URL 路径，"/" 视为请求 index.html。
		urlPath := req.URL.Path
		if urlPath == "" || urlPath == "/" {
			urlPath = "/index.html"
		}
		// 拼接到磁盘路径并清理，确保仍在 webDir 内。
		cleaned := filepath.Clean("/" + strings.TrimPrefix(urlPath, "/"))
		absRoot, err := filepath.Abs(webDir)
		if err != nil {
			pmlog.Errorf("static file path invalid, err=%v urlPath=%s", err, urlPath)
			http.Error(w, "static file path invalid", http.StatusInternalServerError)
			return
		}
		fullPath := filepath.Join(absRoot, cleaned)
		if !strings.HasPrefix(fullPath, absRoot+string(filepath.Separator)) && fullPath != absRoot {
			pmlog.Errorf("fullPath != absRoot, fullPath=%s absRoot=%s", fullPath, absRoot)
			http.NotFound(w, req)
			return
		}
		// 检查存在性：文件不存在 / 是目录 都视为 404。
		info, err := os.Stat(fullPath)
		if err != nil || info.IsDir() {
			pmlog.Errorf("fullPath not exist, err=%v, fullPath=%s", err, fullPath)
			http.NotFound(w, req)
			return
		}
		next.ServeHTTP(w, req)
	})
}
