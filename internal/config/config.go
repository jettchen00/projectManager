// Package config 加载运行时配置。
// 规则 R08：敏感信息禁止硬编码；本文件仅承载默认值与配置文件解析，
// 生产环境敏感字段（如 mongo URI）应通过环境变量覆盖。
package config

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"

	pmlog "projectManager/internal/log"
)

// MongoConfig MongoDB 相关配置。
type MongoConfig struct {
	URI                   string
	DB                    string
	ConnectTimeoutSeconds int
	PingTimeoutSeconds    int
}

// LogConfig 日志相关配置。
// Dir 为空表示输出到 stdout（默认行为，便于本地与测试环境）。
// MaxSizeMB <=0 时由日志库使用其默认值（100MB）。
// MaxBackups <=0 表示不限制历史备份数量。
// Format："json"（默认）或 "console"。
// Level："debug" / "info"（默认） / "warn" / "error"。
type LogConfig struct {
	Dir        string
	MaxSizeMB  int
	MaxBackups int
	Format     string
	Level      string
}

// Config 全局配置。
type Config struct {
	HTTPAddr               string
	WebDir                 string
	ShutdownTimeoutSeconds int
	Mongo                  MongoConfig
	Log                    LogConfig
}

// 默认配置文件路径（相对工作目录）。
const defaultConfigFile = "etc/config.yaml"

// 默认值（兜底，不含敏感信息）。
func defaults() *Config {
	return &Config{
		HTTPAddr:               ":8080",
		WebDir:                 "web",
		ShutdownTimeoutSeconds: 5,
		Mongo: MongoConfig{
			URI:                   "mongodb://127.0.0.1:27017",
			DB:                    "project_manager",
			ConnectTimeoutSeconds: 10,
			PingTimeoutSeconds:    5,
		},
		Log: LogConfig{
			Dir:        "", // 默认输出到 stdout
			MaxSizeMB:  100,
			MaxBackups: 7,
			Format:     "json",
			Level:      "info",
		},
	}
}

// Load 优先从配置文件加载（默认 etc/config.yaml，可由环境变量 CONFIG_FILE 指定），
// 然后再用环境变量覆盖敏感/部署相关项（兼容历史用法）。
func Load() *Config {
	cfg := defaults()

	path := strings.TrimSpace(os.Getenv("CONFIG_FILE"))
	if path == "" {
		path = defaultConfigFile
	}
	if err := loadYAMLFile(path, cfg); err != nil {
		// 配置文件缺失不阻断启动，仅以默认值 + 环境变量启动；记录 ERROR（规则 R18）。
		pmlog.Errorf("config load file err=%v path=%s (fallback to defaults+env)", err, path)
	} else {
		pmlog.Infof("config loaded path=%s", path)
	}

	// 环境变量覆盖（优先级最高）。
	cfg.HTTPAddr = envOr("HTTP_ADDR", cfg.HTTPAddr)
	cfg.WebDir = envOr("WEB_DIR", cfg.WebDir)
	cfg.Mongo.URI = envOr("MONGO_URI", cfg.Mongo.URI)
	cfg.Mongo.DB = envOr("MONGO_DB", cfg.Mongo.DB)
	cfg.Log.Dir = envOr("LOG_DIR", cfg.Log.Dir)
	if v := strings.TrimSpace(os.Getenv("LOG_MAX_SIZE_MB")); v != "" {
		if n, ok := parseInt(v); ok {
			cfg.Log.MaxSizeMB = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("LOG_MAX_BACKUPS")); v != "" {
		if n, ok := parseInt(v); ok {
			cfg.Log.MaxBackups = n
		}
	}
	cfg.Log.Format = envOr("LOG_FORMAT", cfg.Log.Format)
	cfg.Log.Level = envOr("LOG_LEVEL", cfg.Log.Level)
	return cfg
}

// ConnectTimeout 返回 mongo 连接超时。
func (c *Config) ConnectTimeout() time.Duration {
	return secondsToDuration(c.Mongo.ConnectTimeoutSeconds, 10)
}

// PingTimeout 返回 mongo ping 超时。
func (c *Config) PingTimeout() time.Duration {
	return secondsToDuration(c.Mongo.PingTimeoutSeconds, 5)
}

// ShutdownTimeout 返回优雅退出超时。
func (c *Config) ShutdownTimeout() time.Duration {
	return secondsToDuration(c.ShutdownTimeoutSeconds, 5)
}

func secondsToDuration(s, def int) time.Duration {
	if s <= 0 {
		s = def
	}
	return time.Duration(s) * time.Second
}

func envOr(k, def string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	return v
}

// ----------------------------------------------------------------------------
// 极简 YAML 解析：仅支持当前 etc/config.yaml 所需的子集——
//   1) 行注释以 # 开头；
//   2) 顶层 key: value；
//   3) 单层嵌套 map（如 mongo:），子项以 2 个空格缩进；
//   4) value 为字符串/整数；字符串可选用双引号包裹。
// 不引入第三方依赖，遵循规则 R12（最简化）。
// ----------------------------------------------------------------------------

func loadYAMLFile(path string, cfg *Config) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	var currentSection string
	for scanner.Scan() {
		raw := scanner.Text()
		line := stripComment(raw)
		if strings.TrimSpace(line) == "" {
			continue
		}

		indent := leadingSpaces(line)
		trimmed := strings.TrimSpace(line)
		idx := strings.Index(trimmed, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		val := strings.TrimSpace(trimmed[idx+1:])

		if indent == 0 {
			if val == "" {
				// 进入子段
				currentSection = key
				continue
			}
			currentSection = ""
			applyTopLevel(cfg, key, unquote(val))
		} else {
			// 子段字段
			if currentSection == "" {
				continue
			}
			applySection(cfg, currentSection, key, unquote(val))
		}
	}
	return scanner.Err()
}

func applyTopLevel(cfg *Config, key, val string) {
	switch key {
	case "http_addr":
		cfg.HTTPAddr = val
	case "web_dir":
		cfg.WebDir = val
	case "shutdown_timeout_seconds":
		if n, ok := parseInt(val); ok {
			cfg.ShutdownTimeoutSeconds = n
		}
	}
}

func applySection(cfg *Config, section, key, val string) {
	switch section {
	case "mongo":
		switch key {
		case "uri":
			cfg.Mongo.URI = val
		case "db":
			cfg.Mongo.DB = val
		case "connect_timeout_seconds":
			if n, ok := parseInt(val); ok {
				cfg.Mongo.ConnectTimeoutSeconds = n
			}
		case "ping_timeout_seconds":
			if n, ok := parseInt(val); ok {
				cfg.Mongo.PingTimeoutSeconds = n
			}
		}
	case "log":
		switch key {
		case "dir":
			cfg.Log.Dir = val
		case "max_size_mb":
			if n, ok := parseInt(val); ok {
				cfg.Log.MaxSizeMB = n
			}
		case "max_backups":
			if n, ok := parseInt(val); ok {
				cfg.Log.MaxBackups = n
			}
		case "format":
			cfg.Log.Format = val
		case "level":
			cfg.Log.Level = val
		}
	}
}

func stripComment(s string) string {
	// 简化处理：# 之前若位于双引号内则保留；本配置不含此情形，按首个 # 截断。
	if i := strings.Index(s, "#"); i >= 0 {
		return s[:i]
	}
	return s
}

func leadingSpaces(s string) int {
	n := 0
	for _, r := range s {
		if r == ' ' {
			n++
			continue
		}
		break
	}
	return n
}

func unquote(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

func parseInt(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0, false
	}
	return n, true
}
