// Package router 是基于 net/http 的极简路径参数路由。
// 规则 R12：保持最简实现，避免引入大型框架。
//
// 仅支持类似 /api/v1/projects/{id}/form 的单段参数；不支持通配/正则/优先级匹配。
package router

import (
	"context"
	"net/http"
	"strings"
)

type ctxKey string

const paramsKey ctxKey = "router.params"

// HandlerFunc 与标准库一致。
type HandlerFunc func(http.ResponseWriter, *http.Request)

type route struct {
	method  string
	parts   []string // 模式分段；如 "{id}" 表示参数
	keys    []string // 参数 key
	handler HandlerFunc
}

// Router 简易路由器。
type Router struct {
	routes      []route
	middlewares []func(HandlerFunc) HandlerFunc
}

// New 创建路由器。
func New() *Router { return &Router{} }

// Use 注册全局中间件，按注册顺序执行（先注册先包裹外层）。
func (r *Router) Use(mw func(HandlerFunc) HandlerFunc) {
	r.middlewares = append(r.middlewares, mw)
}

// Handle 注册路由。
func (r *Router) Handle(method, pattern string, h HandlerFunc) {
	parts := splitPath(pattern)
	keys := make([]string, len(parts))
	for i, p := range parts {
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			keys[i] = p[1 : len(p)-1]
		}
	}
	r.routes = append(r.routes, route{method: method, parts: parts, keys: keys, handler: h})
}

// GET / POST / PATCH 快捷方法。
func (r *Router) GET(p string, h HandlerFunc)   { r.Handle(http.MethodGet, p, h) }
func (r *Router) POST(p string, h HandlerFunc)  { r.Handle(http.MethodPost, p, h) }
func (r *Router) PATCH(p string, h HandlerFunc) { r.Handle(http.MethodPatch, p, h) }

// ServeHTTP 实现 http.Handler。
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	parts := splitPath(req.URL.Path)
	for _, rt := range r.routes {
		if rt.method != req.Method {
			continue
		}
		if len(rt.parts) != len(parts) {
			continue
		}
		params := map[string]string{}
		matched := true
		for i, p := range rt.parts {
			if rt.keys[i] != "" {
				params[rt.keys[i]] = parts[i]
				continue
			}
			if p != parts[i] {
				matched = false
				break
			}
		}
		if !matched {
			continue
		}
		ctx := context.WithValue(req.Context(), paramsKey, params)
		req = req.WithContext(ctx)
		// 应用中间件（后注册的更靠近 handler）
		h := rt.handler
		for i := len(r.middlewares) - 1; i >= 0; i-- {
			h = r.middlewares[i](h)
		}
		h(w, req)
		return
	}
	http.NotFound(w, req)
}

// Param 从 ctx 中取出路径参数。
func Param(req *http.Request, key string) string {
	v, _ := req.Context().Value(paramsKey).(map[string]string)
	return v[key]
}

func splitPath(p string) []string {
	p = strings.Trim(p, "/")
	if p == "" {
		return nil
	}
	return strings.Split(p, "/")
}
