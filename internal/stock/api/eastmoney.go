package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// EastMoneyClient 东方财富 API 客户端。
type EastMoneyClient struct {
	*BaseClient
	url string
}

// NewEastMoneyClient —
func NewEastMoneyClient(base *BaseClient, url string) *EastMoneyClient {
	return &EastMoneyClient{BaseClient: base, url: url}
}

func (c *EastMoneyClient) GetName() string { return "eastmoney" }

func (c *EastMoneyClient) GetStockData(ctx context.Context, code string) (*StockData, error) {
	secid := toSecID(code)
	url := fmt.Sprintf("%s?secid=%s&fields=f43,f44,f45,f46,f47,f48,f49,f50,f51,f52,f53&ut=fa5fd1943c7b386f172d6893dbfba10b&fltt=2&invt=2&fid=f3", c.url, secid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return c.parseRealtime(code, body)
}

func (c *EastMoneyClient) parseRealtime(code string, data []byte) (*StockData, error) {
	var resp struct {
		Data struct {
			Name  string  `json:"name"`
			F43   float64 `json:"f43"`
			F44   float64 `json:"f44"`
			F45   float64 `json:"f45"`
			F46   float64 `json:"f46"`
			F47   float64 `json:"f47"`
			F48   float64 `json:"f48"`
			F49   float64 `json:"f49"`
			F51   float64 `json:"f51"`
			F52   float64 `json:"f52"`
			F53   float64 `json:"f53"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	d := resp.Data
	ts := time.Now()
	if d.F53 > 0 {
		ts = time.Unix(int64(d.F53/1000), 0)
	}
	return &StockData{
		Code: code, Name: d.Name, Price: d.F43, Open: d.F46,
		High: d.F44, Low: d.F45, PreClose: d.F47,
		Change: d.F48, ChangeRate: d.F49,
		Volume: int64(d.F51), Amount: d.F52,
		Timestamp: ts, Source: "eastmoney",
	}, nil
}

func (c *EastMoneyClient) GetKLineData(ctx context.Context, code, period string, limit int) ([]KLineData, error) {
	secid := toSecID(code)
	klt := eastPeriod(period)
	url := fmt.Sprintf("http://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&klt=%s&fqt=1&end=20500101&lmt=%d", secid, klt, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return c.parseKLine(code, body)
}

// GetFinancialData 获取核心财务指标摘要。
func (c *EastMoneyClient) GetFinancialData(ctx context.Context, code string) (*FinancialData, error) {
	secid := toSecID(code)
	url := fmt.Sprintf("%s?secid=%s&fields=f37,f38,f39,f40,f41,f42,f46,f47,f48,f49,f100,f115,f117&ut=fa5fd1943c7b386f172d6893dbfba10b&fltt=2&invt=2", c.url, secid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return c.parseFinancial(code, body)
}

func (c *EastMoneyClient) parseFinancial(code string, data []byte) (*FinancialData, error) {
	var resp struct {
		Data struct {
			Name string  `json:"f58"`
			F37  float64 `json:"f37"`  // ROE
			F38  float64 `json:"f38"`  // 每股净资产
			F39  float64 `json:"f39"`  // 每股经营现金流
			F40  float64 `json:"f40"`  // 营业总收入
			F41  float64 `json:"f41"`  // 归属净利润
			F42  float64 `json:"f42"`  // 基本每股收益
			F46  float64 `json:"f46"`  // 营收同比
			F47  float64 `json:"f47"`  // 净利同比
			F48  float64 `json:"f48"`  // 毛利率
			F49  float64 `json:"f49"`  // 净利率
			F115 float64 `json:"f115"` // PE
			F117 float64 `json:"f117"` // PB
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parse financial: %w", err)
	}
	d := resp.Data
	return &FinancialData{
		Code:        code,
		Name:        d.Name,
		PE:          d.F115,
		PB:          d.F117,
		ROE:         d.F37,
		EPS:         d.F42,
		BPS:         d.F38,
		CFPS:        d.F39,
		Revenue:     d.F40 / 1e8, // 元→亿
		NetProfit:   d.F41 / 1e8,
		RevenueYoY:  d.F46,
		ProfitYoY:   d.F47,
		GrossMargin: d.F48,
		NetMargin:   d.F49,
		Source:      "eastmoney",
	}, nil
}

func (c *EastMoneyClient) parseKLine(code string, data []byte) ([]KLineData, error) {
	var resp struct {
		Data struct {
			Klines []string `json:"klines"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	result := make([]KLineData, 0, len(resp.Data.Klines))
	for _, line := range resp.Data.Klines {
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			continue
		}
		o, _ := strconv.ParseFloat(parts[1], 64)
		cl, _ := strconv.ParseFloat(parts[2], 64)
		h, _ := strconv.ParseFloat(parts[3], 64)
		l, _ := strconv.ParseFloat(parts[4], 64)
		v, _ := strconv.ParseInt(parts[5], 10, 64)
		var a float64
		if len(parts) > 6 {
			a, _ = strconv.ParseFloat(parts[6], 64)
		}
		t, _ := time.Parse("2006-01-02", parts[0])
		result = append(result, KLineData{
			Code: code, Time: parts[0], Open: o, High: h,
			Low: l, Close: cl, Volume: v, Amount: a, Timestamp: t,
		})
	}
	return result, nil
}

func toSecID(code string) string {
	if strings.HasPrefix(code, "sh") {
		return "1." + strings.TrimPrefix(code, "sh")
	}
	if strings.HasPrefix(code, "sz") {
		return "0." + strings.TrimPrefix(code, "sz")
	}
	return code
}

func eastPeriod(p string) string {
	switch p {
	case "day": return "101"
	case "week": return "102"
	case "month": return "103"
	default: return "101"
	}
}
