package controller

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type bindSample struct {
	Enable   bool     `json:"enable"`
	Interval string   `json:"interval"`
	Time     string   `json:"time"`
	Weekday  int      `json:"weekday"`
	Day      int      `json:"day"`
	Content  []string `json:"content"`
}

func newCtx(body, ctype string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	req := httptest.NewRequest("POST", "/panel/api/server/reportConfig", strings.NewReader(body))
	req.Header.Set("Content-Type", ctype)
	c.Request = req
	return c
}

// 前端 axios 全局拦截器会把请求体转成表单，这是用户实际遇到的那条路径
func TestBindBodyForm(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := newCtx("enable=true&interval=weekly&time=09%3A00&weekday=3&day=1&content=server&content=traffic",
		"application/x-www-form-urlencoded")
	var s bindSample
	if err := bindBody(c, &s); err != nil {
		t.Fatalf("表单解析失败: %v", err)
	}
	if !s.Enable || s.Interval != "weekly" || s.Time != "09:00" || s.Weekday != 3 || s.Day != 1 {
		t.Fatalf("字段不对: %+v", s)
	}
	if len(s.Content) != 2 || s.Content[1] != "traffic" {
		t.Fatalf("数组没解出来: %+v", s.Content)
	}
}

func TestBindBodyJSON(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := newCtx(`{"enable":true,"interval":"daily","time":"09:00","weekday":1,"day":1,"content":["server"]}`,
		"application/json")
	var s bindSample
	if err := bindBody(c, &s); err != nil {
		t.Fatalf("JSON 解析失败: %v", err)
	}
	if !s.Enable || s.Interval != "daily" || len(s.Content) != 1 {
		t.Fatalf("字段不对: %+v", s)
	}
}

func TestBindBodyEmpty(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c := newCtx("", "application/json")
	var s bindSample
	if err := bindBody(c, &s); err != nil {
		t.Fatalf("空请求体不应报错: %v", err)
	}
}
