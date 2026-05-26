// Package middleware 提供 HTTP 中间件：恢复、鉴权、角色校验、CORS、请求日志。
package middleware

import (
	"context"
	"net/http"
	"runtime/debug"
	"strings"

	"projectManager/internal/errcode"
	pmlog "projectManager/internal/log"
	"projectManager/internal/router"
)

type ctxKey string

const userCtxKey ctxKey = "auth.user"

// User 当前请求用户。
type User struct {
	ID   string
	Name string
	Role string
}

// CurrentUser 从 ctx 取当前用户。
func CurrentUser(r *http.Request) *User {
	u, _ := r.Context().Value(userCtxKey).(*User)
	return u
}

// Recover panic 恢复中间件。
func Recover(next router.HandlerFunc) router.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if e := recover(); e != nil {
				pmlog.Errorf("panic recovered err=%v stack=%s", e, debug.Stack())
				errcode.WriteErr(w, errcode.CodeInternalError, nil)
			}
		}()
		next(w, r)
	}
}

// CORS 简易跨域支持，便于 web 静态页本地联调。
func CORS(next router.HandlerFunc) router.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,X-User-Id,X-User-Name,X-User-Role")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next(w, r)
	}
}

// AccessLog 请求日志。
func AccessLog(next router.HandlerFunc) router.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pmlog.Infof("HTTP %s %s remote=%s", r.Method, r.URL.Path, r.RemoteAddr)
		next(w, r)
	}
}

// Auth 从 Header 解析当前用户，注入 ctx。未携带则视为匿名（部分接口需 RequireRoles 校验）。
func Auth(next router.HandlerFunc) router.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := strings.TrimSpace(r.Header.Get("X-User-Id"))
		role := strings.TrimSpace(r.Header.Get("X-User-Role"))
		name := strings.TrimSpace(r.Header.Get("X-User-Name"))
		if uid != "" && role != "" {
			u := &User{ID: uid, Name: name, Role: role}
			ctx := context.WithValue(r.Context(), userCtxKey, u)
			r = r.WithContext(ctx)
		}
		next(w, r)
	}
}

// RequireRoles 包装 handler，要求当前用户角色在白名单内。
func RequireRoles(roles ...string) func(router.HandlerFunc) router.HandlerFunc {
	allow := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allow[r] = struct{}{}
	}
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			u := CurrentUser(r)
			if u == nil {
				errcode.WriteErr(w, errcode.CodeUnauthorized, nil)
				return
			}
			if _, ok := allow[u.Role]; !ok {
				pmlog.Errorf("forbidden user_id=%s role=%s path=%s", u.ID, u.Role, r.URL.Path)
				errcode.WriteErr(w, errcode.CodeForbidden, nil)
				return
			}
			next(w, r)
		}
	}
}
