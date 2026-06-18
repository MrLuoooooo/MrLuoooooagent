package api

import "time"

// StockData 股票实时行情。
type StockData struct {
	Code       string    `json:"code"`
	Name       string    `json:"name"`
	Price      float64   `json:"price"`
	Open       float64   `json:"open"`
	High       float64   `json:"high"`
	Low        float64   `json:"low"`
	PreClose   float64   `json:"pre_close"`
	Change     float64   `json:"change"`
	ChangeRate float64   `json:"change_rate"`
	Volume     int64     `json:"volume"`
	Amount     float64   `json:"amount"`
	Timestamp  time.Time `json:"timestamp"`
	Source     string    `json:"source"`
}

// KLineData K线数据。
type KLineData struct {
	Code      string    `json:"code"`
	Time      string    `json:"time"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    int64     `json:"volume"`
	Amount    float64   `json:"amount"`
	Timestamp time.Time `json:"timestamp"`
}

// FinancialData 核心财务指标摘要。
// 从东方财富 push2 API 获取，非三大报表全文，而是关键比率。
type FinancialData struct {
	Code        string  `json:"code"`
	Name        string  `json:"name"`
	PE          float64 `json:"pe"`           // 动态市盈率
	PB          float64 `json:"pb"`           // 市净率
	ROE         float64 `json:"roe"`          // 净资产收益率(%)
	EPS         float64 `json:"eps"`          // 基本每股收益
	BPS         float64 `json:"bps"`          // 每股净资产
	CFPS        float64 `json:"cfps"`         // 每股经营现金流
	Revenue     float64 `json:"revenue"`      // 营业总收入(亿)
	NetProfit   float64 `json:"net_profit"`   // 归属净利润(亿)
	RevenueYoY  float64 `json:"revenue_yoy"`  // 营收同比(%)
	ProfitYoY   float64 `json:"profit_yoy"`   // 净利同比(%)
	GrossMargin float64 `json:"gross_margin"` // 毛利率(%)
	NetMargin   float64 `json:"net_margin"`   // 净利率(%)
	Source      string  `json:"source"`
}
