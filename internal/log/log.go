// Package log 提供轻量日志门面，底层基于 go.uber.org/zap + lumberjack。
// 规则 R10：关键流程 INFO，异常流程 ERROR。
// 规则 R18：所有异常分支必须打印 err。
//
// 设计目标：保留旧门面 API（Infof / Errorf / KV），业务侧 0 改动，
// 仅在进程启动时通过 Init() 切换底层实现。
package log

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Options 初始化参数。
//
// Dir 为空：输出到 stdout（兼容旧行为，便于本地与测试）。
// Dir 非空：输出到 Dir/app.log，并由 lumberjack 在文件达到 MaxSizeMB 时
// 自动滚动；同目录下保留最近 MaxBackups 个历史文件（<=0 表示不限）。
//
// Format 取值 "json"（默认）或 "console"。
// Level 取值 "debug" / "info"（默认） / "warn" / "error"。
type Options struct {
	Dir        string
	MaxSizeMB  int
	MaxBackups int
	Format     string
	Level      string
}

var (
	mu      sync.Mutex
	sugared = mustDefaultSugared() // 兜底：进程未 Init 前也可用
)

// mustDefaultSugared 在未调用 Init 前返回一个 stdout console logger。
func mustDefaultSugared() *zap.SugaredLogger {
	enc := zapcore.NewConsoleEncoder(consoleEncoderConfig())
	core := zapcore.NewCore(enc, zapcore.Lock(os.Stdout), zap.InfoLevel)
	return zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)).Sugar()
}

// Init 在进程启动时调用一次，按配置切换日志输出。
// 失败（如目录不可创建）会回退到 stdout 并以 ERROR 记录（规则 R18）。
func Init(opts Options) {
	mu.Lock()
	defer mu.Unlock()

	level := parseLevel(opts.Level)
	enc := buildEncoder(opts.Format)

	var ws zapcore.WriteSyncer
	if strings.TrimSpace(opts.Dir) == "" {
		ws = zapcore.Lock(os.Stdout)
	} else {
		// MkdirAll 失败时 lumberjack 写入也会失败；提前显式创建以便给出明确错误。
		if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
			// 回退 stdout，并保留错误日志（规则 R18）。
			ws = zapcore.Lock(os.Stdout)
			sugared = newSugared(enc, ws, level)
			sugared.Errorf("log init err=%v dir=%s (fallback to stdout)", err, opts.Dir)
			return
		}
		ws = zapcore.AddSync(&lumberjack.Logger{
			Filename:   opts.Dir + string(os.PathSeparator) + "app.log",
			MaxSize:    maxSize(opts.MaxSizeMB), // MB
			MaxBackups: maxBackups(opts.MaxBackups),
			LocalTime:  true,
			Compress:   false,
		})
	}

	sugared = newSugared(enc, ws, level)
	mode := "stdout"
	if opts.Dir != "" {
		mode = "file"
	}
	sugared.Infof("log init mode=%s dir=%s max_size_mb=%d max_backups=%d format=%s level=%s",
		mode, opts.Dir, opts.MaxSizeMB, opts.MaxBackups, normalizeFormat(opts.Format), level.String())
}

// Infof 关键流程日志。
func Infof(format string, args ...interface{}) {
	sugared.Infof(format, args...)
}

// Errorf 异常流程日志。
func Errorf(format string, args ...interface{}) {
	sugared.Errorf(format, args...)
}

// KV 用于结构化字段拼接（保留旧接口，避免业务侧改动）。
// 例：log.Infof("save form %s", log.KV("project_id", id, "revision", rev))
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

// ---------------------------------------------------------------------------
// 内部辅助
// ---------------------------------------------------------------------------

func newSugared(enc zapcore.Encoder, ws zapcore.WriteSyncer, level zapcore.Level) *zap.SugaredLogger {
	core := zapcore.NewCore(enc, ws, level)
	return zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)).Sugar()
}

func buildEncoder(format string) zapcore.Encoder {
	if normalizeFormat(format) == "console" {
		return zapcore.NewConsoleEncoder(consoleEncoderConfig())
	}
	return zapcore.NewJSONEncoder(jsonEncoderConfig())
}

func normalizeFormat(f string) string {
	switch strings.ToLower(strings.TrimSpace(f)) {
	case "console":
		return "console"
	default:
		return "json"
	}
}

func jsonEncoderConfig() zapcore.EncoderConfig {
	c := zap.NewProductionEncoderConfig()
	c.TimeKey = "ts"
	c.EncodeTime = zapcore.ISO8601TimeEncoder
	c.EncodeLevel = zapcore.LowercaseLevelEncoder
	return c
}

func consoleEncoderConfig() zapcore.EncoderConfig {
	c := zap.NewDevelopmentEncoderConfig()
	c.EncodeTime = zapcore.ISO8601TimeEncoder
	c.EncodeLevel = zapcore.CapitalLevelEncoder
	return c
}

func parseLevel(s string) zapcore.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return zap.DebugLevel
	case "warn", "warning":
		return zap.WarnLevel
	case "error":
		return zap.ErrorLevel
	default:
		return zap.InfoLevel
	}
}

func maxSize(mb int) int {
	if mb <= 0 {
		// lumberjack 默认 100MB；这里同步默认值，避免传 0 被库当默认。
		return 100
	}
	return mb
}

func maxBackups(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
