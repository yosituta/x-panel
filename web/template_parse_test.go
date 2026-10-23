package web

import (
	"io/fs"
	"path/filepath"
	"text/template"
	"testing"
)

// TestTemplatesParse 确保所有 HTML 模板能被 text/template 正确解析。
// 生产环境是在启动时一次性 Parse 全部模板，任何语法/未定义模板名都会导致启动失败，
// 所以这个测试是“改完页面能不能开机”的第一道闸。
func TestTemplatesParse(t *testing.T) {
	funcMap := template.FuncMap{
		"i18n": func(key string, params ...string) string { return key },
	}

	var files []string
	err := filepath.WalkDir("html", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		t.Fatalf("遍历模板目录失败: %v", err)
	}
	if len(files) == 0 {
		t.Fatal("没有找到任何模板文件")
	}

	tmpl := template.New("root").Funcs(funcMap)
	if _, err := tmpl.ParseFiles(files...); err != nil {
		t.Fatalf("模板解析失败: %v", err)
	}
	t.Logf("已成功解析 %d 个模板文件", len(files))
}
