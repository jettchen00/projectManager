// Package log 提供轻量结构化日志封装。
// 规则 R10：关键流程 INFO，异常流程 ERROR。
// 规则 R18：所有异常分支必须打印 err。
package log

import (
	"fmt"
	stdlog "log"
	"os"
	"strings"
)

var logger = stdlog.New(os.Stdout, "", stdlog.LstdFlags|stdlog.Lmicroseconds)

// Infof 关键流程日志。
func Infof(format string, args ...interface{}) {
	logger.Output(2, "[INFO] "+fmt.Sprintf(format, args...))
}

// Errorf 异常流程日志。
func Errorf(format string, args ...interface{}) {
	logger.Output(2, "[ERROR] "+fmt.Sprintf(format, args...))
}

// KV 用于结构化字段拼接，例：log.Infof("save form %s", log.KV("project_id", id, "revision", rev))
func KV(kv ...interface{}) string {
	if len(kv)%2 != 0 {
		return fmt.Sprint(kv...)
	}
	parts := make([]string, 0, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		parts = append(parts, fmt.Sprintf("%v=%v", kv[i], kv[i+1]))
	}
	return strings.Join(parts, " ")
}
