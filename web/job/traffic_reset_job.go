package job

import (
	"time"

	"x-ui/logger"
	"x-ui/web/service"
)

// TrafficResetJob 每分钟检查一次入站的“周期性重置流量”是否到期。
type TrafficResetJob struct {
	settingService service.SettingService
	xrayService    service.XrayService
}

func NewTrafficResetJob() *TrafficResetJob {
	return new(TrafficResetJob)
}

func (j *TrafficResetJob) Run() {
	loc, err := j.settingService.GetTimeLocation()
	if err != nil || loc == nil {
		loc = time.Local
	}
	var resetService service.TrafficResetService
	resetCount, enabledCount, err := resetService.RunDueResets(time.Now().In(loc))
	if err != nil {
		logger.Warning("周期重置流量任务出错:", err)
		return
	}
	if resetCount > 0 {
		logger.Infof("周期重置流量任务: 本次重置 %d 个入站, 恢复 %d 个客户端", resetCount, enabledCount)
	}
	// 有客户端被重新启用时，需要让 Xray 重新加载配置使其真正生效
	if enabledCount > 0 {
		j.xrayService.SetToNeedRestart()
	}
}
