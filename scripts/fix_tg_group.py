import io

def edit(path, pairs):
    s = io.open(path, encoding='utf-8', newline='').read()
    crlf = '\r\n' in s
    s = s.replace('\r\n', '\n')
    for old, new in pairs:
        assert old in s, f'{path} 未找到:\n{old[:80]}'
        s = s.replace(old, new, 1)
    io.open(path, 'w', encoding='utf-8', newline='').write(s.replace('\n', '\r\n') if crlf else s)
    print('OK:', path)

# ---------- 1) /id 同时返回用户 ID 与会话 Chat ID（群组里才有意义） ----------
edit('web/service/tgbot.go', [(
'''	case "id":
		onlyMessage = true
		msg += t.I18nBot("tgbot.commands.getID", "ID=="+strconv.FormatInt(message.From.ID, 10))''',
'''	case "id":
		onlyMessage = true
		msg += t.I18nBot("tgbot.commands.getID", "ID=="+strconv.FormatInt(message.From.ID, 10))
		// 【开源增强】群组里 /id 之前只回个人 ID，用户拿它去填通知会 chat not found。
		// 这里补上「当前会话」的 Chat ID：私聊时等于个人 ID，群组时是 -100 开头的负数。
		if message.Chat != nil {
			chatID := message.Chat.ID
			if chatID != message.From.ID {
				msg += fmt.Sprintf("\\n\\n<b>当前群组 Chat ID</b>：<code>%d</code>", chatID)
				msg += "\\n（要往这个群发通知，就填这一个，前面带负号）"
			} else {
				msg += fmt.Sprintf("\\n\\n<b>当前会话 Chat ID</b>：<code>%d</code>", chatID)
			}
		}''')])

# ---------- 2) 设置页提示：群组 ID 是负数 ----------
edit('web/translation/translate.zh_CN.toml', [(
''' "telegramChatIdDesc" = "Telegram 管理员聊天 ID (多个以逗号分隔)（可通过 @userinfobot 获取，或在机器人中使用 '/id' 命令获取）"''',
''' "telegramChatIdDesc" = "Telegram 管理员聊天 ID (多个以逗号分隔)。个人 ID 用 @userinfobot 查；群组 ID 是负数（-100 开头），把机器人拉进群后在群里发 /id 获取"''')])

edit('web/translation/translate.en_US.toml', [(
''' "telegramChatIdDesc" = "Telegram admin chat ID (multiple separated by commas) (can be obtained from @userinfobot or via '/id' command in the bot)"''',
''' "telegramChatIdDesc" = "Telegram admin chat ID (comma separated). Get a personal ID from @userinfobot; a GROUP id is negative (starts with -100) — add the bot to the group and send /id there"''')])

# ---------- 3) 报错提示补上群组场景 ----------
edit('web/service/cert_notify.go', [(
'''	case strings.Contains(low, "chat not found"):
		return "Chat ID 不存在（chat not found）：确认 ID 正确，且已经给机器人发过 /start"''',
'''	case strings.Contains(low, "chat not found"):
		return "Chat ID 不存在（chat not found）：私聊要先给机器人发 /start；群组必须是 -100 开头的负数，且机器人已被拉进群"''')])
