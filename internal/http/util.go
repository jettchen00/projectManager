// Package httpapi HTTP 控制器层。
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"projectManager/internal/errcode"
	pmlog "projectManager/internal/log"
)

var snakeCaseRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// decodeJSON 解析 body；body 过大时返回 InvalidParam。
func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1MB
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("empty body")
		}
		return err
	}
	return nil
}

// queryInt 解析 query int。
func queryInt(r *http.Request, key string, def int64) int64 {
	v := r.URL.Query().Get(key)
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

// validateSnakeKeys 规则 R20：HTTP 字段命名为 snake_case。
// 仅校验顶层 map[string]interface{} 的键名，深层 JSON 由各业务模块自行约束。
func validateSnakeKeys(v map[string]interface{}) bool {
	for k := range v {
		if !snakeCaseRe.MatchString(k) {
			pmlog.Errorf("validate snake key fail key=%s", k)
			return false
		}
	}
	return true
}

// writeBadRequest 参数错误。
func writeBadRequest(w http.ResponseWriter, msg string) {
	errcode.WriteErr(w, errcode.CodeInvalidParam, map[string]string{"detail": msg})
}
