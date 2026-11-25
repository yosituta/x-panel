package controller

import (
	"bytes"
	"encoding/json"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// bindBody 同时兼容 application/json 与 application/x-www-form-urlencoded。
//
// 背景：前端 axios-init.js 里有全局请求拦截器，把所有非 FormData 的请求体用
// Qs.stringify 转成了表单（x-www-form-urlencoded）。因此这些接口如果只用
// c.ShouldBindJSON，就会拿到 "enable=true&interval=daily..." 这样的字符串，
// 解析时报：参数错误 (invalid character 'e' looking for beginning of value)。
func bindBody(c *gin.Context, dst any) error {
	raw, err := c.GetRawData()
	if err != nil {
		return err
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		return json.Unmarshal(trimmed, dst)
	}

	vals, err := url.ParseQuery(string(trimmed))
	if err != nil {
		return err
	}
	if len(vals) == 0 {
		return nil
	}

	// 按 json tag 把表单值装成 JSON，再解一次；这样结构体不用额外写 form tag
	obj := map[string]any{}
	t := reflect.TypeOf(dst)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return json.Unmarshal(trimmed, dst)
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := strings.Split(f.Tag.Get("json"), ",")[0]
		if tag == "" || tag == "-" {
			tag = f.Name
		}
		v, ok := vals[tag]
		if !ok || len(v) == 0 {
			continue
		}
		s := v[0]
		switch f.Type.Kind() {
		case reflect.Bool:
			obj[tag] = s == "true" || s == "1"
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			n, _ := strconv.ParseInt(s, 10, 64)
			obj[tag] = n
		case reflect.Float32, reflect.Float64:
			fv, _ := strconv.ParseFloat(s, 64)
			obj[tag] = fv
		case reflect.Slice:
			obj[tag] = v
		default:
			obj[tag] = s
		}
	}
	b, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dst)
}
