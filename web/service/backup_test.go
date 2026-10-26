package service

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreRejectsBadName(t *testing.T) {
	var s BackupService
	bad := []string{"", "../x.db.gz", "a/../../b.db.gz", "/etc/passwd", "x-ui.db", "evil.sh"}
	for _, name := range bad {
		if err := s.Restore(name); err == nil {
			t.Errorf("应拒绝非法快照名: %q", name)
		}
	}
}

func TestGzipRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "a.txt")
	content := []byte("SQLite format 3\x00 这是测试内容")
	if err := os.WriteFile(src, content, 0o644); err != nil {
		t.Fatal(err)
	}
	gz := filepath.Join(dir, "a.gz")
	if err := gzipFile(src, gz); err != nil {
		t.Fatalf("压缩失败: %v", err)
	}
	out := filepath.Join(dir, "b.txt")
	if err := gunzipFile(gz, out); err != nil {
		t.Fatalf("解压失败: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(content) {
		t.Errorf("内容不一致: %q", string(got))
	}
}

func TestCheckSQLiteHeader(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.db")
	if err := os.WriteFile(good, []byte("SQLite format 3\x00rest"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkSQLite(good); err != nil {
		t.Errorf("合法 SQLite 头应通过: %v", err)
	}

	bad := filepath.Join(dir, "bad.db")
	if err := os.WriteFile(bad, []byte("not a database"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := checkSQLite(bad); err == nil {
		t.Error("非 SQLite 文件应被拒绝")
	}
}
