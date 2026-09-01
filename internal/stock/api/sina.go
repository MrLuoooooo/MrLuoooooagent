package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// SinaClient 新浪财经 API 客户端。
type SinaClient struct {
	*BaseClient
	url string
}

// NewSinaClient —
func NewSinaClient(base *BaseClient, url string) *SinaClient {
	return &SinaClient{BaseClient: base, url: url}
}

func (c *SinaClient) GetName() string { return "sina" }

func (c *SinaClient) GetStockData(ctx context.Context, code string) (*StockData, error) {
	url := fmt.Sprintf("%s%s", c.url, code)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://finance.sina.com.cn/")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// GBK→UTF-8
	body, err = io.ReadAll(transform.NewReader(bytes.NewReader(body), simplifiedchinese.GBK.NewDecoder()))
	if err != nil {
		return nil, err
	}
	return c.parseRealtime(code, string(body))
}

func (c *SinaClient) parseRealtime(code, raw string) (*StockData, error) {
	re := regexp.MustCompile(`"(.*)"`)
	m := re.FindStringSubmatch(raw)
	if len(m) < 2 {
		return nil, fmt.Errorf("sina: invalid response format")
	}
	parts := strings.Split(m[1], ",")
	if len(parts) < 10 {
		return nil, fmt.Errorf("sina: insufficient data fields (%d)", len(parts))
	}
	name := parts[0]
	open, _ := strconv.ParseFloat(parts[1], 64)
	preClose, _ := strconv.ParseFloat(parts[2], 64)
	price, _ := strconv.ParseFloat(parts[3], 64)
	high, _ := strconv.ParseFloat(parts[4], 64)
	low, _ := strconv.ParseFloat(parts[5], 64)
	volume, _ := strconv.ParseInt(parts[8], 10, 64)
	amount, _ := strconv.ParseFloat(parts[9], 64)

	change := price - preClose
	changeRate := 0.0
	if preClose > 0 {
		changeRate = change / preClose * 100
	}
	return &StockData{
		Code: code, Name: name, Price: price, Open: open, High: high,
		Low: low, PreClose: preClose, Change: change, ChangeRate: changeRate,
		Volume: volume, Amount: amount, Timestamp: time.Now(), Source: "sina",
	}, nil
}

func (c *SinaClient) GetKLineData(ctx context.Context, code, period string, limit int) ([]KLineData, error) {
	scale, ok := sinaPeriod(period)
	if !ok {
		// 新浪 scale 是分钟粒度，只支持 日/周/月。不支持的周期必须显式报错走
		// failover（东财支持季/半年/年），绝不能静默降级成日K数据冒充返回。
		return nil, fmt.Errorf("sina does not support period %q", period)
	}
	url := fmt.Sprintf("http://money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData?symbol=%s&scale=%s&ma=no&datalen=%d", code, scale, limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://finance.sina.com.cn/")

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

func (c *SinaClient) parseKLine(code string, data []byte) ([]KLineData, error) {
	type rawK struct {
		Day    string `json:"day"`
		Open   string `json:"open"`
		High   string `json:"high"`
		Low    string `json:"low"`
		Close  string `json:"close"`
		Volume string `json:"volume"`
		Amount string `json:"amount"`
	}
	var raw []rawK
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	result := make([]KLineData, 0, len(raw))
	for _, r := range raw {
		o, _ := strconv.ParseFloat(r.Open, 64)
		h, _ := strconv.ParseFloat(r.High, 64)
		l, _ := strconv.ParseFloat(r.Low, 64)
		cl, _ := strconv.ParseFloat(r.Close, 64)
		v, _ := strconv.ParseInt(r.Volume, 10, 64)
		a, _ := strconv.ParseFloat(r.Amount, 64)
		t, _ := time.Parse("2006-01-02", r.Day)
		result = append(result, KLineData{
			Code: code, Time: r.Day, Open: o, High: h, Low: l,
			Close: cl, Volume: v, Amount: a, Timestamp: t,
		})
	}
	return result, nil
}

// sinaPeriod 周期 → 新浪 scale（分钟粒度）。季/半年/年不支持，调用方须处理 ok=false。
func sinaPeriod(p string) (string, bool) {
	switch p {
	case "day": return "240", true
	case "week": return "10080", true
	case "month": return "43200", true
	default: return "", false
	}
}
