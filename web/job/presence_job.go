package job

import (
	"x-ui/logger"
	"x-ui/web/service"
)

// PresenceJob 每 5 分钟检查一次节点上下线状态，变化时推送 TG
type PresenceJob struct {
	presenceService service.PresenceService
}

func NewPresenceJob() *PresenceJob {
	return new(PresenceJob)
}

func (j *PresenceJob) Run() {
	count, err := j.presenceService.CheckAndNotify()
	if err != nil {
		logger.Warning("上下线通知任务出错:", err)
		return
	}
	if count > 0 {
		logger.Infof("上下线通知任务: 本次发送 %d 条", count)
	}
}
