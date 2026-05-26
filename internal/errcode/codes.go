// Package errcode 集中定义错误码与统一 HTTP 响应。
// 规则 R06：HTTP 响应使用统一 JSON 格式。
// 规则 R19：错误码统一收拢到一个文件。
package errcode

import (
	"encoding/json"
	"net/http"

	pmlog "projectManager/internal/log"
)

// 错误码常量集中定义。
const (
	CodeOK             = 0
	CodeInvalidParam   = 1001
	CodeUnauthorized   = 1002
	CodeForbidden      = 1003
	CodeNotFound       = 1004
	CodeStateConflict  = 1005
	CodeValidationFail = 1006
	CodeInternalError  = 2001
	CodeDBError        = 2002
)

// Message 错误码对应消息。
var Message = map[int]string{
	CodeOK:             "OK",
	CodeInvalidParam:   "INVALID_PARAM",
	CodeUnauthorized:   "UNAUTHORIZED",
	CodeForbidden:      "FORBIDDEN",
	CodeNotFound:       "NOT_FOUND",
	CodeStateConflict:  "STATE_CONFLICT",
	CodeValidationFail: "VALIDATION_FAILED",
	CodeInternalError:  "INTERNAL_ERROR",
	CodeDBError:        "DB_ERROR",
}

// httpStatus 错误码对应的 HTTP 状态。
var httpStatus = map[int]int{
	CodeOK:             http.StatusOK,
	CodeInvalidParam:   http.StatusBadRequest,
	CodeUnauthorized:   http.StatusUnauthorized,
	CodeForbidden:      http.StatusForbidden,
	CodeNotFound:       http.StatusNotFound,
	CodeStateConflict:  http.StatusConflict,
	CodeValidationFail: http.StatusUnprocessableEntity,
	CodeInternalError:  http.StatusInternalServerError,
	CodeDBError:        http.StatusInternalServerError,
}

// Response 统一响应体。
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// WriteJSON 按错误码写入响应。
func WriteJSON(w http.ResponseWriter, code int, data interface{}) {
	status, ok := httpStatus[code]
	if !ok {
		status = http.StatusInternalServerError
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	resp := Response{
		Code:    code,
		Message: Message[code],
		Data:    data,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		// 规则 R18
		pmlog.Errorf("write response err=%v", err)
	}
}

// WriteOK 成功响应快捷方法。
func WriteOK(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, CodeOK, data)
}

// WriteErr 失败响应快捷方法，可携带额外 data（如 missing_fields）。
func WriteErr(w http.ResponseWriter, code int, data interface{}) {
	WriteJSON(w, code, data)
}
