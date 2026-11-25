package service

import "testing"

func TestTelegramErrMapping(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"401", 401, `{"ok":false,"description":"Unauthorized"}`, "Bot Token 不对（401 Unauthorized）"},
		{"chat not found", 400, `{"ok":false,"description":"Bad Request: chat not found"}`, "Chat ID 不存在（chat not found）：确认 ID 正确，且已经给机器人发过 /start"},
		{"blocked", 403, `{"ok":false,"description":"Forbidden: bot was blocked by the user"}`, "机器人被你拉黑了（bot was blocked）：先在 Telegram 里解除屏蔽"},
		{"empty chat", 400, `{"ok":false,"description":"Bad Request: chat_id is empty"}`, "Chat ID 为空"},
		{"429", 429, `{"ok":false,"description":"Too Many Requests: retry after 5"}`, "Telegram 限流（429），稍后再试"},
		{"other", 500, `{"ok":false,"description":"Internal Server Error"}`, "Telegram 返回 500：Internal Server Error"},
		{"bare", 502, `not json`, "Telegram 返回 502"},
	}
	for _, c := range cases {
		got := telegramErr(c.status, []byte(c.body))
		if got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
