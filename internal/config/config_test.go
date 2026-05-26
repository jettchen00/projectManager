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
		"  ping_timeout_seconds: 3\n"
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
}

// 测试覆盖：环境变量覆盖配置文件值。
func TestLoad_EnvOverridesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "" +
		"http_addr: \":8080\"\n" +
		"mongo:\n" +
		"  uri: \"mongodb://file:27017\"\n" +
		"  db: \"file_db\"\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write tmp config: %v", err)
	}

	t.Setenv("CONFIG_FILE", path)
	t.Setenv("HTTP_ADDR", ":7777")
	t.Setenv("MONGO_URI", "mongodb://env:27017")
	t.Setenv("MONGO_DB", "env_db")

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
