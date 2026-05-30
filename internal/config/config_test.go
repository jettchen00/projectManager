package config

import (
	"os"
	"path/filepath"
	"testing"
)

// 测试覆盖：YAML 解析能正确加载顶层与一层嵌套字段；缺失项使用默认值。
func TestLoadYAMLFile_ParsesTopLevelAndSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "" +
		"# 注释\n" +
		"http_addr: \":9090\"\n" +
		"web_dir: \"public\"\n" +
		"shutdown_timeout_seconds: 7\n" +
		"mongo:\n" +
		"  uri: \"mongodb://example:27017\"\n" +
		"  db: \"pm_test\"\n" +
		"  connect_timeout_seconds: 15\n" +
		"  ping_timeout_seconds: 3\n" +
		"log:\n" +
		"  dir: \"var/log/pm\"\n" +
		"  max_size_mb: 50\n" +
		"  max_backups: 3\n" +
		"  format: \"console\"\n" +
		"  level: \"warn\"\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write tmp config: %v", err)
	}

	cfg := defaults()
	if err := loadYAMLFile(path, cfg); err != nil {
		t.Fatalf("loadYAMLFile err=%v", err)
	}

	if cfg.HTTPAddr != ":9090" {
		t.Errorf("HTTPAddr=%q", cfg.HTTPAddr)
	}
	if cfg.WebDir != "public" {
		t.Errorf("WebDir=%q", cfg.WebDir)
	}
	if cfg.ShutdownTimeoutSeconds != 7 {
		t.Errorf("ShutdownTimeoutSeconds=%d", cfg.ShutdownTimeoutSeconds)
	}
	if cfg.Mongo.URI != "mongodb://example:27017" {
		t.Errorf("Mongo.URI=%q", cfg.Mongo.URI)
	}
	if cfg.Mongo.DB != "pm_test" {
		t.Errorf("Mongo.DB=%q", cfg.Mongo.DB)
	}
	if cfg.Mongo.ConnectTimeoutSeconds != 15 {
		t.Errorf("Mongo.ConnectTimeoutSeconds=%d", cfg.Mongo.ConnectTimeoutSeconds)
	}
	if cfg.Mongo.PingTimeoutSeconds != 3 {
		t.Errorf("Mongo.PingTimeoutSeconds=%d", cfg.Mongo.PingTimeoutSeconds)
	}
	if cfg.Log.Dir != "var/log/pm" {
		t.Errorf("Log.Dir=%q", cfg.Log.Dir)
	}
	if cfg.Log.MaxSizeMB != 50 {
		t.Errorf("Log.MaxSizeMB=%d", cfg.Log.MaxSizeMB)
	}
	if cfg.Log.MaxBackups != 3 {
		t.Errorf("Log.MaxBackups=%d", cfg.Log.MaxBackups)
	}
	if cfg.Log.Format != "console" {
		t.Errorf("Log.Format=%q", cfg.Log.Format)
	}
	if cfg.Log.Level != "warn" {
		t.Errorf("Log.Level=%q", cfg.Log.Level)
	}
}

// 测试覆盖：环境变量覆盖配置文件值。
func TestLoad_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "" +
		"http_addr: \":8080\"\n" +
		"mongo:\n" +
		"  uri: \"mongodb://file:27017\"\n" +
		"  db: \"file_db\"\n" +
		"log:\n" +
		"  dir: \"file_logs\"\n" +
		"  max_size_mb: 10\n" +
		"  max_backups: 2\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write tmp config: %v", err)
	}

	t.Setenv("CONFIG_FILE", path)
	t.Setenv("HTTP_ADDR", ":7777")
	t.Setenv("MONGO_URI", "mongodb://env:27017")
	t.Setenv("MONGO_DB", "env_db")
	t.Setenv("LOG_DIR", "env_logs")
	t.Setenv("LOG_MAX_SIZE_MB", "200")
	t.Setenv("LOG_MAX_BACKUPS", "9")
	t.Setenv("LOG_FORMAT", "console")
	t.Setenv("LOG_LEVEL", "error")

	cfg := Load()
	if cfg.HTTPAddr != ":7777" {
		t.Errorf("HTTPAddr=%q", cfg.HTTPAddr)
	}
	if cfg.Mongo.URI != "mongodb://env:27017" {
		t.Errorf("Mongo.URI=%q", cfg.Mongo.URI)
	}
	if cfg.Mongo.DB != "env_db" {
		t.Errorf("Mongo.DB=%q", cfg.Mongo.DB)
	}
	if cfg.Log.Dir != "env_logs" {
		t.Errorf("Log.Dir=%q", cfg.Log.Dir)
	}
	if cfg.Log.MaxSizeMB != 200 {
		t.Errorf("Log.MaxSizeMB=%d", cfg.Log.MaxSizeMB)
	}
	if cfg.Log.MaxBackups != 9 {
		t.Errorf("Log.MaxBackups=%d", cfg.Log.MaxBackups)
	}
	if cfg.Log.Format != "console" {
		t.Errorf("Log.Format=%q", cfg.Log.Format)
	}
	if cfg.Log.Level != "error" {
		t.Errorf("Log.Level=%q", cfg.Log.Level)
	}
}

// 测试覆盖：配置文件不存在时使用默认值。
func TestLoad_MissingFileFallback(t *testing.T) {
	t.Setenv("CONFIG_FILE", filepath.Join(t.TempDir(), "not_exist.yaml"))
	// 清空可能影响默认值的环境变量。
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("MONGO_URI", "")
	t.Setenv("MONGO_DB", "")
	t.Setenv("WEB_DIR", "")

	cfg := Load()
	def := defaults()
	if cfg.HTTPAddr != def.HTTPAddr || cfg.Mongo.URI != def.Mongo.URI || cfg.Mongo.DB != def.Mongo.DB {
		t.Errorf("fallback not used: %+v", cfg)
	}
	if cfg.ConnectTimeout() <= 0 || cfg.PingTimeout() <= 0 || cfg.ShutdownTimeout() <= 0 {
		t.Errorf("durations should be positive: %v %v %v",
			cfg.ConnectTimeout(), cfg.PingTimeout(), cfg.ShutdownTimeout())
	}
}
