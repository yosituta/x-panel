package service

import (
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// 【开源增强】多面板管理（主从）
//
// 被控端：在设置里生成一个 slaveKey，主控端凭它在内网/公网调用
//         /panel/api/slave/{status,links,restartXray}
// 主控端：把被控端存成列表（settings.remotePanels），可查状态、取链接、重启 xray

type RemotePanel struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Key      string `json:"key"`
	Insecure bool   `json:"insecure"` // 被控端用自签证书 / IP 访问时置 true
}

const remotePanelsKey = "remotePanels"

type RemotePanelService struct{}

func (s *RemotePanelService) List() ([]RemotePanel, error) {
	var ss SettingService
	raw, err := ss.getString(remotePanelsKey)
	if err != nil || strings.TrimSpace(raw) == "" {
		return []RemotePanel{}, nil
	}
	var list []RemotePanel
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil, fmt.Errorf("面板列表解析失败: %w", err)
	}
	return list, nil
}

func (s *RemotePanelService) save(list []RemotePanel) error {
	bs, err := json.Marshal(list)
	if err != nil {
		return err
	}
	var ss SettingService
	return ss.setString(remotePanelsKey, string(bs))
}

func (s *RemotePanelService) Add(p RemotePanel) error {
	p.Name = strings.TrimSpace(p.Name)
	p.URL = strings.TrimRight(strings.TrimSpace(p.URL), "/")
	p.Key = strings.TrimSpace(p.Key)
	if p.Name == "" || p.URL == "" {
		return fmt.Errorf("名称和地址不能为空")
	}
	if _, err := url.Parse(p.URL); err != nil {
		return fmt.Errorf("地址不合法: %w", err)
	}
	list, err := s.List()
	if err != nil {
		return err
	}
	for i := range list {
		if list[i].Name == p.Name {
			list[i] = p // 同名视为更新
			return s.save(list)
		}
	}
	return s.save(append(list, p))
}

func (s *RemotePanelService) Remove(name string) error {
	list, err := s.List()
	if err != nil {
		return err
	}
	out := make([]RemotePanel, 0, len(list))
	for _, p := range list {
		if p.Name != name {
			out = append(out, p)
		}
	}
	return s.save(out)
}

func (s *RemotePanelService) Get(name string) (*RemotePanel, error) {
	list, err := s.List()
	if err != nil {
		return nil, err
	}
	for i := range list {
		if list[i].Name == name {
			return &list[i], nil
		}
	}
	return nil, fmt.Errorf("未找到面板: %s", name)
}

// 调用被控端
func (p *RemotePanel) call(method, path string) ([]byte, error) {
	req, err := http.NewRequest(method, strings.TrimRight(p.URL, "/")+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Slave-Key", p.Key)
	transport := &http.Transport{}
	if p.Insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	client := &http.Client{Timeout: 12 * time.Second, Transport: transport}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("被控端返回 %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

// Status 拉取被控端状态
func (p *RemotePanel) Status() (map[string]any, error) {
	body, err := p.call(http.MethodGet, "/panel/api/slave/status")
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Links 拉取被控端节点链接
func (p *RemotePanel) Links() (string, error) {
	body, err := p.call(http.MethodGet, "/panel/api/slave/links")
	if err != nil {
		return "", err
	}
	var out struct {
		Obj struct {
			Text  string `json:"text"`
			Count int    `json:"count"`
		} `json:"obj"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return out.Obj.Text, nil
}

// RestartXray 让被控端重启 xray
func (p *RemotePanel) RestartXray() error {
	_, err := p.call(http.MethodPost, "/panel/api/slave/restartXray")
	return err
}

// CheckSlaveKey 校验被控端收到的 key
func (s *SettingService) CheckSlaveKey(got string) bool {
	want, err := s.getString("slaveKey")
	if err != nil || strings.TrimSpace(want) == "" {
		return false // 没配置 key 时不开放
	}
	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}

// EnsureSlaveKey 生成（或返回已有的）被控端 key
func (s *SettingService) EnsureSlaveKey() (string, error) {
	cur, err := s.getString("slaveKey")
	if err == nil && strings.TrimSpace(cur) != "" {
		return cur, nil
	}
	key := NewSubID() + NewSubID()
	if err := s.setString("slaveKey", key); err != nil {
		return "", err
	}
	return key, nil
}
