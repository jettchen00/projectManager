package log

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 测试覆盖：Init 在 dir 为空时回退 stdout，不会 panic。
func TestInit_StdoutWhenDirEmpty(t *testing.T) {
	Init(Options{Dir: "", Format: "console", Level: "info"})
	Infof("hello %s", "world") // 不应 panic
	Errorf("oops %s", "x")
}

// 测试覆盖：Init 文件模式下后续日志会写入到目标文件。
func TestInit_FileMode(t *testing.T) {
	dir := t.TempDir()
	Init(Options{Dir: dir, MaxSizeMB: 10, MaxBackups: 3, Format: "json", Level: "info"})
	defer Init(Options{Dir: ""}) // 还原 stdout，避免影响其它测试

	Infof("hello-from-test")
	data, err := os.ReadFile(filepath.Join(dir, "app.log"))
	if err != nil {
		t.Fatalf("read app.log err=%v", err)
	}
	if !strings.Contains(string(data), "hello-from-test") {
		t.Fatalf("log content not written, got=%q", string(data))
	}
}

// 测试覆盖：KV 拼接保持 key=value 形式，长度奇数时降级。
func TestKV_Format(t *testing.T) {
	got := KV("project_id", "p-001", "revision", 3)
	if got != "project_id=p-001 revision=3" {
		t.Errorf("KV got=%q", got)
	}
	if KV("only_key") == "" {
		t.Errorf("KV with odd args should not be empty")
	}
}

// 测试覆盖：parseLevel / normalizeFormat 的兜底分支。
func TestParseLevelAndFormatDefaults(t *testing.T) {
	if parseLevel("").String() != "info" {
		t.Errorf("default level expect info")
	}
	if parseLevel("DEBUG").String() != "debug" {
		t.Errorf("debug level lowercase failed")
	}
	if normalizeFormat("") != "json" {
		t.Errorf("default format expect json")
	}
	if normalizeFormat("Console") != "console" {
		t.Errorf("console format lowercase failed")
	}
}
