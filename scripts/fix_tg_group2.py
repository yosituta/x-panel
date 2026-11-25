import io

def edit(path, pairs):
    s = io.open(path, encoding='utf-8', newline='').read()
    crlf = '\r\n' in s
    s = s.replace('\r\n', '\n')
    for old, new in pairs:
        assert old in s, f'{path} 未找到:\n{old[:100]}'
        s = s.replace(old, new, 1)
    io.open(path, 'w', encoding='utf-8', newline='').write(s.replace('\n', '\r\n') if crlf else s)
    print('OK:', path)

edit('web/translation/translate.zh_CN.toml', [(
'"telegramChatIdDesc" = "Telegram 管理员聊天 ID (多个以逗号分隔)（可通过 @userinfobot 获取，或在机器人中使用 \'/id\' 命令获取）"',
'"telegramChatIdDesc" = "Telegram 管理员聊天 ID (多个以逗号分隔)。个人 ID 用 @userinfobot 查；群组 ID 是负数（-100 开头），把机器人拉进群后在群里发 /id 获取"')])

edit('web/translation/translate.en_US.toml', [(
'"telegramChatIdDesc" = "Telegram admin chat ID (multiple separated by commas) (can be obtained from @userinfobot or via \'/id\' command in the bot)"',
'"telegramChatIdDesc" = "Telegram admin chat ID (comma separated). Personal ID from @userinfobot; a GROUP id is negative (starts with -100) — add the bot to the group and send /id there"')])

edit('web/service/cert_notify.go', [(
'		return "Chat ID 不存在（chat not found）：确认 ID 正确，且已经给机器人发过 /start"',
'		return "Chat ID 不存在（chat not found）：私聊要先给机器人发 /start；群组必须是 -100 开头的负数，且机器人已被拉进群"')])
