// Package middleware 提供 HTTP 中间件：恢复、鉴权、角色校验、CORS、请求日志。
package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"runtime/debug"
	"strings"

	"projectManager/internal/errcode"
	pmlog "projectManager/internal/log"
	"projectManager/internal/router"
)

// maxLogBodyBytes 单次请求体最多打印的字节数，避免日志爆炸。
const maxLogBodyBytes = 4096

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

// AccessLog 请求日志：打印完整 URL、Header、Body，便于问题定位。
// 读取后会把 Body 重新塞回 r.Body，保证后续 handler 仍可正常解析。
func AccessLog(next router.HandlerFunc) router.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Header 拍平为单行字符串；敏感字段做脱敏。
		var hb strings.Builder
		first := true
		for k, vs := range r.Header {
			if !first {
				hb.WriteString("&")
			}
			first = false
			val := strings.Join(vs, ",")
			if isSensitiveHeader(k) {
				val = "***"
			}
			hb.WriteString(k)
			hb.WriteString("=")
			hb.WriteString(val)
		}

		// Body 读出来用于打印，再回填到 r.Body。
		bodyStr := ""
		if r.Body != nil && r.Body != http.NoBody {
			raw, err := io.ReadAll(r.Body)
			_ = r.Body.Close()
			if err != nil {
				pmlog.Errorf("HTTP read body err=%v method=%s url=%s", err, r.Method, r.URL.String())
			} else {
				r.Body = io.NopCloser(bytes.NewReader(raw))
				if len(raw) > maxLogBodyBytes {
					bodyStr = string(raw[:maxLogBodyBytes]) + "...(truncated)"
				} else {
					bodyStr = string(raw)
				}
			}
		}

		pmlog.Infof("HTTP %s url=%s remote=%s headers={%s} body=%s",
			r.Method, r.URL.String(), r.RemoteAddr, hb.String(), bodyStr)
		next(w, r)
	}
}

// isSensitiveHeader 判断是否敏感 header（不打印明文）。
func isSensitiveHeader(k string) bool {
	switch strings.ToLower(k) {
	case "authorization", "cookie", "set-cookie", "proxy-authorization":
		return true
	}
	return false
}

// decodeHeaderValue 兼容前端把含中文等非 ISO-8859-1 字符的 header 值做 URL 编码后传过来的情况。
// 当解码后字符串与原值不同（说明确实是被编码过），则使用解码结果；否则保留原值。
func decodeHeaderValue(s string) string {
	if s == "" {
		return s
	}
	if !strings.Contains(s, "%") {
		return s
	}
	if dec, err := url.QueryUnescape(s); err == nil && dec != "" {
		return dec
	}
	return s
}

// Auth 从 Header 解析当前用户，注入 ctx。未携带则视为匿名（部分接口需 RequireRoles 校验）。
func Auth(next router.HandlerFunc) router.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid := strings.TrimSpace(r.Header.Get("X-User-Id"))
		role := strings.TrimSpace(r.Header.Get("X-User-Role"))
		name := decodeHeaderValue(strings.TrimSpace(r.Header.Get("X-User-Name")))
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
