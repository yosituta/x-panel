package service

import (
	"time"

	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"
	"x-ui/xray"

	"gorm.io/gorm"
)

// 中文注释: 入站级“周期性重置流量”能力（开源增强功能，非上游私有代码）。
//
// 设计要点：
//  1. 只清零数据库中的 up/down（含该入站下所有 clientStats）；
//     Xray 统计链路是“读即清零、只累加增量”（GetXrayTraffic -> GetTraffic(true)），
//     因此清零库值不会被下一次统计补回来。
//  2. 重置时刻按“日历边界”计算，而不是简单加 24h/7d：
//     每日 = 当天 00:00；每周 = 本周一 00:00；每月 = 当月 resetDay 号 00:00
//     （当月天数不足时按当月最后一天）。
//  3. 幂等：判断依据是 LastResetTime 是否早于本周期应重置的时刻，
//     重置后把 LastResetTime 写成“本周期应重置的时刻”，同一周期内不会重复触发。
type TrafficResetService struct{}

const (
	ResetPeriodNever   = "never"
	ResetPeriodDaily   = "daily"
	ResetPeriodWeekly  = "weekly"
	ResetPeriodMonthly = "monthly"
)

// scheduledAt 计算“本周期应重置的时刻”，返回零值表示该入站不参与重置
func (s *TrafficResetService) scheduledAt(period string, resetDay int, now time.Time) (time.Time, bool) {
	loc := now.Location()
	switch period {
	case ResetPeriodDaily:
		t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		return t, true
	case ResetPeriodWeekly:
		// Go: Sunday=0 ... Saturday=6，转成“距本周一几天”
		offset := (int(now.Weekday()) + 6) % 7
		t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, -offset)
		return t, true
	case ResetPeriodMonthly:
		day := resetDay
		if day < 1 || day > 31 {
			day = 1
		}
		lastDay := time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, loc).Day()
		if day > lastDay {
			day = lastDay
		}
		t := time.Date(now.Year(), now.Month(), day, 0, 0, 0, 0, loc)
		return t, true
	default:
		return time.Time{}, false
	}
}

// IsDue 判断某入站在 now 时刻是否需要重置流量
func (s *TrafficResetService) IsDue(inbound *model.Inbound, now time.Time) (time.Time, bool) {
	if inbound == nil || !inbound.Enable {
		return time.Time{}, false
	}
	scheduled, ok := s.scheduledAt(inbound.ResetPeriod, inbound.ResetDay, now)
	if !ok || now.Before(scheduled) {
		return time.Time{}, false
	}
	if inbound.LastResetTime > 0 {
		last := time.UnixMilli(inbound.LastResetTime).In(now.Location())
		if !last.Before(scheduled) {
			return time.Time{}, false
		}
	}
	return scheduled, true
}

// ResetInboundTraffic 周期重置：清零入站与其下所有客户端的已用流量，
// 并重新启用“因流量耗尽而被动禁用”的客户端。返回被重新启用的客户端数量。
func (s *TrafficResetService) ResetInboundTraffic(inboundId int, scheduled time.Time) (int, error) {
	enabled := 0
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		// 1. 找出因超量而停用的客户端（重置后应恢复可用）
		var depleted []xray.ClientTraffic
		if err := tx.Model(xray.ClientTraffic{}).
			Where("inbound_id = ? AND total > 0 AND (up + down) >= total", inboundId).
			Find(&depleted).Error; err != nil {
			return err
		}

		// 2. 清零入站自身流量
		if err := tx.Model(model.Inbound{}).Where("id = ?", inboundId).Updates(map[string]any{
			"up":              0,
			"down":            0,
			"last_reset_time": scheduled.UnixMilli(),
		}).Error; err != nil {
			return err
		}

		// 3. 清零该入站下所有客户端流量
		if err := tx.Model(xray.ClientTraffic{}).Where("inbound_id = ?", inboundId).Updates(map[string]any{
			"up":   0,
			"down": 0,
		}).Error; err != nil {
			return err
		}

		// 4. 恢复被流量耗尽的客户端
		for _, c := range depleted {
			if !c.Enable {
				if err := tx.Model(xray.ClientTraffic{}).Where("id = ?", c.Id).Update("enable", true).Error; err != nil {
					return err
				}
				enabled++
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	if enabled > 0 {
		logger.Infof("周期重置流量: 入站 #%d, 恢复客户端 %d 个", inboundId, enabled)
	} else {
		logger.Infof("周期重置流量: 入站 #%d, 重置时刻 %s", inboundId, scheduled.Format("2006-01-02 15:04:05"))
	}
	return enabled, nil
}

// RunDueResets 扫描所有入站，执行到期的重置。返回 (重置入站数, 恢复客户端数, 错误)
func (s *TrafficResetService) RunDueResets(now time.Time) (int, int, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound
	err := db.Model(model.Inbound{}).
		Where("reset_period IS NOT NULL AND reset_period != '' AND reset_period != ?", ResetPeriodNever).
		Find(&inbounds).Error
	if err != nil {
		return 0, 0, err
	}

	resetCount, enabledCount := 0, 0
	for _, inbound := range inbounds {
		// 每次循环重新取一次，避免使用被前面逻辑改过的旧值
		scheduled, ok := s.IsDue(inbound, now)
		if !ok {
			continue
		}
		enabled, err := s.ResetInboundTraffic(inbound.Id, scheduled)
		if err != nil {
			logger.Warningf("周期重置流量失败: 入站 #%d: %v", inbound.Id, err)
			continue
		}
		resetCount++
		enabledCount += enabled
	}
	return resetCount, enabledCount, nil
}
