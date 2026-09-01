package db

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// FlexFloat 兼容东财对同一字段混用 JSON 类型的返回：
// 正常值是 number（fltt=2 格式化），无数据时是字符串 "-"。
// 接收失败一律按 0 处理，不让脏数据打挂整批同步。
type FlexFloat float64

func (f *FlexFloat) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "-" || s == "null" {
		*f = 0
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		*f = 0
		return nil
	}
	*f = FlexFloat(v)
	return nil
}

// clistHosts 东财行情列表的候选域名，按序 failover。
// push2 的 IPv4 边缘对部分网络（含 Docker Desktop 内网）拒响应，
// 其 DNS 又可能解析成纯 IPv6（push2ipv6 CNAME）导致容器内不可达；
// push2delay（延时行情镜像）走独立 IPv4 边缘，作为兜底。
var clistHosts = []string{"push2.eastmoney.com", "push2delay.eastmoney.com"}

// clistPageSize 分页大小：东财镜像域名（push2delay）单页封顶 100 条。
const clistPageSize = 100

// clistMaxPages 分页安全上限：A 股 ~5600 条 / 100 ≈ 56 页，留裕量防 total 异常导致死循环。
const clistMaxPages = 120

// fetchAllStocks 从东方财富 API 全量拉取 A 股列表，域名按序 failover。
// 纯函数：输入无，输出 []StockBasic。
func fetchAllStocks() ([]StockBasic, error) {
	var lastErr error
	for _, host := range clistHosts {
		stocks, err := fetchAllStocksFrom(host)
		if err == nil {
			return stocks, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// fetchAllStocksFrom 从指定域名分页拉全量列表（pn 循环直到取满 total 或空页）。
func fetchAllStocksFrom(host string) ([]StockBasic, error) {
	cli := &http.Client{Timeout: 15 * time.Second}
	all := make([]StockBasic, 0, 6000)
	for pn := 1; pn <= clistMaxPages; pn++ {
		stocks, total, err := fetchStockPage(cli, host, pn)
		if err != nil {
			return nil, err
		}
		if len(stocks) == 0 {
			break // 空页：越界或数据耗尽
		}
		all = append(all, stocks...)
		if len(all) >= total {
			break
		}
	}
	if len(all) == 0 {
		return nil, fmt.Errorf("%s: empty stock list", host)
	}
	return all, nil
}

// fetchStockPage 拉一页原始 JSON 并解析，返回该页股票与全量 total。
func fetchStockPage(cli *http.Client, host string, pn int) ([]StockBasic, int, error) {
	// 覆盖沪深两市：m:0+t:6(沪A), m:0+t:80(深A主板), m:1+t:2(中小板), m:1+t:23(创业板)
	url := fmt.Sprintf("http://%s/api/qt/clist/get?pn=%d&pz=%d&po=1&np=1&fltt=2&invt=2&fid=f3&fs=m:0+t:6,m:0+t:80,m:1+t:2,m:1+t:23&fields=f12,f14,f100,f20,f115,f117",
		host, pn, clistPageSize)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := cli.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, 0, fmt.Errorf("%s: http %d", host, resp.StatusCode)
	}
	return parseStockPage(body)
}

// parseStockPage 解析单页列表 JSON，返回本页股票与全量 total。
func parseStockPage(data []byte) ([]StockBasic, int, error) {
	var resp struct {
		Data struct {
			Total  int `json:"total"`
			Stocks []struct {
				F12  string    `json:"f12"`  // code
				F14  string    `json:"f14"`  // name
				F100 string    `json:"f100"` // industry
				F20  FlexFloat `json:"f20"`  // market cap (total)
				F115 FlexFloat `json:"f115"` // PE (TTM)
				F117 FlexFloat `json:"f117"` // PB
			} `json:"diff"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, 0, fmt.Errorf("parse stock page: %w", err)
	}
	if resp.Data.Stocks == nil {
		return nil, resp.Data.Total, nil // data 为 null（越界页）
	}

	result := make([]StockBasic, 0, len(resp.Data.Stocks))
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
		st.MarketCap = float64(s.F20) / 1e8 // 元 → 亿
		st.PE = float64(s.F115)
		st.PB = float64(s.F117)
		// 行业为空时用默认值
		if st.Industry == "" {
			st.Industry = "其他"
		}
		result = append(result, st)
	}

	return result, resp.Data.Total, nil
}
