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
