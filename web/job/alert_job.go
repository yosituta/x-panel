package job

import (
	"x-ui/logger"
	"x-ui/web/service"
)

// AlertJob 每 10 分钟扫描一次客户端用量与到期时间，
// 达到阈值时推送到 Telegram 管理员（同一阈值只提醒一次）
type AlertJob struct {
	alertService service.AlertService
}

func NewAlertJob() *AlertJob {
	return new(AlertJob)
}

func (j *AlertJob) Run() {
	count, err := j.alertService.CheckAndNotify()
	if err != nil {
		logger.Warning("流量/到期告警任务出错:", err)
		return
	}
	if count > 0 {
		logger.Infof("流量/到期告警任务: 本次发送 %d 条提醒", count)
	}
}
