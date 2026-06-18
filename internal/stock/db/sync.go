package db

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// fetchAllStocks 从东方财富 API 全量拉取 A 股列表。
// 单次请求最多 5000 条，A 股总量 ~5000，一次即可。
// 纯函数：输入无，输出 []StockBasic。
func fetchAllStocks() ([]StockBasic, error) {
	// 覆盖沪深两市：m:0+t:6(沪A), m:0+t:80(深A主板), m:1+t:2(中小板), m:1+t:23(创业板)
	url := "http://push2.eastmoney.com/api/qt/clist/get?pn=1&pz=6000&po=1&np=1&fltt=2&invt=2&fid=f3&fs=m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23&fields=f12,f14,f100,f20,f115,f117"
	cli := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return parseStockList(body)
}

func parseStockList(data []byte) ([]StockBasic, error) {
	var resp struct {
		Data struct {
			Total  int `json:"total"`
			Stocks []struct {
				F12  string      `json:"f12"`  // code
				F14  string      `json:"f14"`  // name
				F100 string      `json:"f100"` // industry
				F20  json.Number `json:"f20"`  // market cap (total)
				F115 json.Number `json:"f115"` // PE (TTM)
				F117 json.Number `json:"f117"` // PB
			} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse stock list: %w", err)
	}

	result := make([]StockBasic, 0, resp.Data.Total)
	for _, s := range resp.Data.Stocks {
		// 跳过 B 股（代码以 9 开头的沪B）
		if strings.HasPrefix(s.F12, "9") {
			continue
		}
		st := StockBasic{
			Code:     NormalizeCode(s.F12),
			Name:     s.F14,
			Industry: s.F100,
		}
		if v, err := s.F20.Float64(); err == nil {
			st.MarketCap = v / 1e8 // 元 → 亿
		}
		if v, err := s.F115.Float64(); err == nil {
			st.PE = v
		}
		if v, err := s.F117.Float64(); err == nil {
			st.PB = v
		}
		// 行业为空时用默认值
		if st.Industry == "" {
			st.Industry = "其他"
		}
		result = append(result, st)
	}

	return result, nil
}
