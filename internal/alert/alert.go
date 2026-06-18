package alert

import (
	"context"
	"fmt"
	"time"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/stock"
	"go.uber.org/zap"
)

// AlertResult 单只股票的预警结果。
type AlertResult struct {
	Code       string
	Name       string
	Price      float64
	ChangeRate float64
	Threshold  float64
	Direction  string // "up" / "down"
	Message    string
}

// AlertService 自选股异动监控服务。
// 纯调度层，依赖 Collector 获取实时行情。
type AlertService struct {
	collector *stock.Collector
	watchlist []string
	threshold float64 // 涨跌幅阈值（百分比）
	logger    *zap.Logger
}

// NewAlertService —
func NewAlertService(collector *stock.Collector, watchlist []string, threshold float64, logger *zap.Logger) *AlertService {
	return &AlertService{
		collector: collector,
		watchlist: watchlist,
		threshold: threshold,
		logger:    logger,
	}
}

// CheckWatchlist 检查自选股是否有超阈值异动。
// 返回触发预警的股票列表。
func (s *AlertService) CheckWatchlist(ctx context.Context) []AlertResult {
	var alerts []AlertResult
	for _, code := range s.watchlist {
		data, err := s.collector.FetchRealtime(ctx, code)
		if err != nil {
			s.logger.Warn("alert: fetch failed", zap.String("code", code), zap.Error(err))
			continue
		}
		absChange := data.ChangeRate
		if absChange < 0 {
			absChange = -absChange
		}
		if absChange >= s.threshold {
			dir := "up"
			if data.ChangeRate < 0 {
				dir = "down"
			}
			alerts = append(alerts, AlertResult{
				Code:       code,
				Name:       data.Name,
				Price:      data.Price,
				ChangeRate: data.ChangeRate,
				Threshold:  s.threshold,
				Direction:  dir,
				Message: fmt.Sprintf("%s(%s) 现价 ¥%.2f，涨跌幅 %.2f%%，已触发 %.0f%% 阈值",
					data.Name, code, data.Price, data.ChangeRate, s.threshold),
			})
		}
	}
	return alerts
}

// FormatAlerts 格式化预警结果为 Agent 可读文本。
func FormatAlerts(alerts []AlertResult) string {
	if len(alerts) == 0 {
		return ""
	}
	msg := fmt.Sprintf("## 盘中异动预警 (%s)\n\n", time.Now().Format("15:04:05"))
	for _, a := range alerts {
		icon := "🔴"
		if a.Direction == "up" {
			icon = "🟢"
		}
		msg += fmt.Sprintf("%s %s: %+.2f%% → %s\n", icon, a.Name, a.ChangeRate, a.Message)
	}
	return msg
}
