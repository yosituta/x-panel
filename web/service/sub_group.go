package service

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"x-ui/database"
	"x-ui/database/model"
)

// 中文注释: 【开源增强】订阅系统
//
// 与「每个客户端一个 subId」不同：这里允许把多台服务器的分享链接聚合成一个订阅，
// 客户端扫一个链接拿到全部节点。数据存在 sub_groups 表，每行一个分享链接。

type SubGroupService struct{}

func (s *SubGroupService) List() ([]model.SubGroup, error) {
	var groups []model.SubGroup
	err := database.GetDB().Order("id desc").Find(&groups).Error
	return groups, err
}

func (s *SubGroupService) Get(id int) (*model.SubGroup, error) {
	var g model.SubGroup
	if err := database.GetDB().First(&g, id).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

func (s *SubGroupService) GetByToken(token string) (*model.SubGroup, error) {
	var g model.SubGroup
	if err := database.GetDB().Where("token = ?", token).First(&g).Error; err != nil {
		return nil, err
	}
	return &g, nil
}

// normalizeLinks 去掉空行、注释行，并按顺序去重
func normalizeLinks(raw string) string {
	seen := map[string]bool{}
	var out []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if seen[line] {
			continue
		}
		seen[line] = true
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func (s *SubGroupService) Create(name, nodes, remark string) (*model.SubGroup, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("订阅名称不能为空")
	}
	nodes = normalizeLinks(nodes)
	if nodes == "" {
		return nil, fmt.Errorf("至少需要一个节点链接")
	}
	now := time.Now().Unix()
	g := &model.SubGroup{
		Name: name, Token: NewSubID() + NewSubID(), Nodes: nodes,
		Remark: remark, CreatedAt: now, UpdatedAt: now,
	}
	if err := database.GetDB().Create(g).Error; err != nil {
		return nil, err
	}
	return g, nil
}

func (s *SubGroupService) Update(id int, name, nodes, remark string) error {
	g, err := s.Get(id)
	if err != nil {
		return err
	}
	if v := strings.TrimSpace(name); v != "" {
		g.Name = v
	}
	g.Nodes = normalizeLinks(nodes)
	g.Remark = remark
	g.UpdatedAt = time.Now().Unix()
	return database.GetDB().Save(g).Error
}

func (s *SubGroupService) Delete(id int) error {
	return database.GetDB().Delete(&model.SubGroup{}, id).Error
}

// CountNodes 统计有效节点数
func (s *SubGroupService) CountNodes(nodes string) int {
	n := 0
	for _, line := range strings.Split(nodes, "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n
}

// ---------- 解析分享链接 ----------

func b64decode(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.StdEncoding.DecodeString(s)
}

// ParseShareLink 把一条分享链接解析成 SubNode。支持 vless / trojan / ss / vmess。
func ParseShareLink(link string) (SubNode, error) {
	link = strings.TrimSpace(link)
	var n SubNode
	switch {
	case strings.HasPrefix(link, "vless://"):
		return parseVless(link)
	case strings.HasPrefix(link, "trojan://"):
		return parseTrojan(link)
	case strings.HasPrefix(link, "ss://"):
		return parseSS(link)
	case strings.HasPrefix(link, "vmess://"):
		return parseVmess(link)
	}
	return n, fmt.Errorf("不支持的链接类型: %s", firstN(link, 20))
}

func firstN(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// stripFragment 拆出 #备注 与主体
func splitFragment(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	name := ""
	if i := strings.LastIndex(raw, "#"); i >= 0 {
		name, _ = url.PathUnescape(raw[i+1:])
		name, _ = url.QueryUnescape(name)
		raw = raw[:i]
	}
	return raw, strings.TrimSpace(name)
}

// applyQuery 把分享链接里的常见参数映射到 SubNode
func applyQuery(n *SubNode, q url.Values) {
	n.Network = orDefault(q.Get("type"), "tcp")
	if v := q.Get("security"); v != "" {
		n.Security = v
	}
	n.SNI = q.Get("sni")
	n.HostHeader = q.Get("host")
	n.Path = q.Get("path")
	n.Flow = q.Get("flow")
	n.PublicKey = q.Get("pbk")
	n.ShortID = q.Get("sid")
	n.Fingerprint = q.Get("fp")
	n.SpiderX = q.Get("spx")
	n.ServiceName = orDefault(q.Get("serviceName"), q.Get("path"))
	if v := q.Get("allowInsecure"); v == "1" || strings.EqualFold(v, "true") {
		n.AllowInsecure = true
	}
}

func parseVless(link string) (SubNode, error) {
	var n SubNode
	body, name := splitFragment(link)
	u, err := url.Parse(body)
	if err != nil {
		return n, err
	}
	n.Protocol = "vless"
	n.UUID = u.User.Username()
	n.Server = u.Hostname()
	n.Port, _ = strconv.Atoi(u.Port())
	n.Remark = name
	if n.Server == "" || n.Port == 0 || n.UUID == "" {
		return n, fmt.Errorf("vless 链接缺少必要字段")
	}
	applyQuery(&n, u.Query())
	if n.Security == "" {
		n.Security = "none"
	}
	return n, nil
}

func parseTrojan(link string) (SubNode, error) {
	var n SubNode
	body, name := splitFragment(link)
	u, err := url.Parse(body)
	if err != nil {
		return n, err
	}
	n.Protocol = "trojan"
	n.Password = u.User.Username()
	n.Server = u.Hostname()
	n.Port, _ = strconv.Atoi(u.Port())
	n.Remark = name
	if n.Server == "" || n.Port == 0 || n.Password == "" {
		return n, fmt.Errorf("trojan 链接缺少必要字段")
	}
	applyQuery(&n, u.Query())
	if n.Security == "" {
		n.Security = "tls"
	}
	return n, nil
}

func parseSS(link string) (SubNode, error) {
	var n SubNode
	body, name := splitFragment(link)
	rest := strings.TrimPrefix(body, "ss://")

	// 形态一：ss://base64(method:password)@host:port?params
	// 形态二：ss://base64(method:password@host:port)
	at := strings.LastIndex(rest, "@")
	if at < 0 {
		dec, err := b64decode(rest)
		if err != nil {
			return n, err
		}
		rest = string(dec)
		at = strings.LastIndex(rest, "@")
	}
	if at < 0 {
		return n, fmt.Errorf("ss 链接格式无法识别")
	}
	userinfo, hostport := rest[:at], rest[at+1:]
	if dec, err := b64decode(userinfo); err == nil {
		userinfo = string(dec)
	}
	parts := strings.SplitN(userinfo, ":", 2)
	if len(parts) != 2 {
		return n, fmt.Errorf("ss 链接缺少加密方式")
	}
	hostport = strings.TrimSuffix(hostport, "/")
	host := hostport
	portStr := ""
	if i := strings.LastIndex(hostport, ":"); i >= 0 {
		host, portStr = hostport[:i], hostport[i+1:]
	}
	if q := strings.Index(portStr, "?"); q >= 0 {
		portStr = portStr[:q]
	}
	n.Protocol = "shadowsocks"
	n.Method = parts[0]
	n.Password = parts[1]
	n.Server = host
	n.Port, _ = strconv.Atoi(portStr)
	n.Remark = name
	n.Network = "tcp"
	n.Security = "none"
	if n.Server == "" || n.Port == 0 {
		return n, fmt.Errorf("ss 链接缺少服务器或端口")
	}
	return n, nil
}

func parseVmess(link string) (SubNode, error) {
	var n SubNode
	raw := strings.TrimPrefix(strings.TrimSpace(link), "vmess://")
	dec, err := b64decode(raw)
	if err != nil {
		return n, err
	}
	var v struct {
		PS   string `json:"ps"`
		Add  string `json:"add"`
		Port any    `json:"port"`
		ID   string `json:"id"`
		Aid  any    `json:"aid"`
		Net  string `json:"net"`
		Host string `json:"host"`
		Path string `json:"path"`
		TLS  string `json:"tls"`
		SNI  string `json:"sni"`
	}
	if err := json.Unmarshal(dec, &v); err != nil {
		return n, err
	}
	n.Protocol = "vmess"
	n.Remark = v.PS
	n.Server = v.Add
	n.UUID = v.ID
	n.Network = orDefault(v.Net, "tcp")
	n.HostHeader = v.Host
	n.Path = v.Path
	n.SNI = orDefault(v.SNI, v.Host)
	if v.TLS == "tls" {
		n.Security = "tls"
	} else {
		n.Security = "none"
	}
	switch p := v.Port.(type) {
	case float64:
		n.Port = int(p)
	case string:
		n.Port, _ = strconv.Atoi(p)
	}
	switch a := v.Aid.(type) {
	case float64:
		n.AlterID = int(a)
	case string:
		n.AlterID, _ = strconv.Atoi(a)
	}
	if n.Server == "" || n.Port == 0 {
		return n, fmt.Errorf("vmess 链接缺少服务器或端口")
	}
	return n, nil
}

// ---------- 渲染 ----------

// Render 把订阅组渲染成目标格式。
// v2ray（默认）走原始链接直出，能保留 vmess 等本面板不解析的协议；
// clash / singbox / surge 走结构化转换，不支持的协议会跳过并在标题里注明。
func (s *SubGroupService) Render(g *model.SubGroup, target string) (string, string) {
	lines := []string{}
	for _, line := range strings.Split(g.Nodes, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, strings.TrimSpace(line))
		}
	}
	title := fmt.Sprintf("%s（%d 个节点）", g.Name, len(lines))

	if target == "" || target == "v2ray" || target == "base64" {
		return base64.StdEncoding.EncodeToString([]byte(strings.Join(lines, "\n"))),
			"text/plain; charset=utf-8"
	}

	var nodes []SubNode
	skipped := 0
	for _, line := range lines {
		n, err := ParseShareLink(line)
		if err != nil {
			skipped++
			continue
		}
		nodes = append(nodes, n)
	}
	if skipped > 0 {
		title += fmt.Sprintf("，%d 个节点该格式不支持已跳过", skipped)
	}

	switch target {
	case "clash", "mihomo":
		return ClashConfig(nodes, title), "text/yaml; charset=utf-8"
	case "singbox", "sing-box":
		return SingboxConfig(nodes, title), "application/json; charset=utf-8"
	case "surge":
		return SurgeConfig(nodes, title), "text/plain; charset=utf-8"
	}
	return "", "text/plain; charset=utf-8"
}
