package job

import (
	"x-ui/logger"
	"x-ui/web/service"
)

// ReportJob 每分钟检查一次是否到达报告发送时间
type ReportJob struct {
	reportService service.ReportService
}

func NewReportJob() *ReportJob {
	return new(ReportJob)
}

func (j *ReportJob) Run() {
	sent, err := j.reportService.CheckAndSend()
	if err != nil {
		logger.Warning("定时报告任务出错:", err)
		return
	}
	if sent {
		logger.Info("定时报告已发送")
	}
}
