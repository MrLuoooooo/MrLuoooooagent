package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ── stock data models (matching stock-middleware output) ──

type stockRealtime struct {
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

type stockKLine struct {
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

// ── Tool: get_stock_realtime ─────────────────────────────

// StockRealtimeTool reads realtime stock data from the stock-middleware output.
type StockRealtimeTool struct {
	dataDir string
}

// NewStockRealtimeTool creates a StockRealtimeTool.
func NewStockRealtimeTool(dataDir string) *StockRealtimeTool {
	if dataDir == "" {
		dataDir = `D:\stock\data\stocks`
	}
	return &StockRealtimeTool{dataDir: dataDir}
}

func (t *StockRealtimeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "get_stock_realtime",
		Desc: "获取A股实时行情数据。数据来自 stock-middleware 中间件的定时采集。支持批量查询多个股票（逗号分隔）。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"codes": {
				Type:     schema.String,
				Desc:     "股票代码，逗号分隔，如 sh600519,sz000001",
				Required: true,
			},
		}),
	}, nil
}

func (t *StockRealtimeTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Codes string `json:"codes"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("get_stock_realtime: invalid args: %w", err)
	}
	if args.Codes == "" {
		return "", fmt.Errorf("get_stock_realtime: codes 不能为空")
	}

	codes := strings.Split(args.Codes, ",")
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## 实时行情 (%d只股票)\n\n", len(codes)))

	found := 0
	for _, code := range codes {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		data, err := t.readRealtime(code)
		if err != nil {
			b.WriteString(fmt.Sprintf("- **%s**: 数据暂不可用 (%v)\n", code, err))
			continue
		}
		found++
		dir := "↑"
		if data.Change < 0 {
			dir = "↓"
		}
		b.WriteString(fmt.Sprintf("### %s %s\n", data.Name, data.Code))
		b.WriteString(fmt.Sprintf("- **现价**: ¥%.2f  |  **涨跌**: %s%.2f (%.2f%%)  |  **昨收**: ¥%.2f\n",
			data.Price, dir, data.Change, data.ChangeRate, data.PreClose))
		b.WriteString(fmt.Sprintf("- 开盘 ¥%.2f  /  最高 ¥%.2f  /  最低 ¥%.2f\n",
			data.Open, data.High, data.Low))
		b.WriteString(fmt.Sprintf("- 成交量: %d股  /  成交额: ¥%.2f亿  /  来源: %s\n",
			data.Volume, data.Amount/1e8, data.Source))
		b.WriteString(fmt.Sprintf("- 更新时间: %s\n\n", data.Timestamp.Format("2006-01-02 15:04:05")))
	}

	if found == 0 {
		return "无可用实时行情数据。请确认 stock-middleware 已运行且股票代码正确。", nil
	}
	return b.String(), nil
}

func (t *StockRealtimeTool) readRealtime(code string) (*stockRealtime, error) {
	path := filepath.Join(t.dataDir, "realtime", code+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("文件读取失败: %w", err)
	}
	var s stockRealtime
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}
	return &s, nil
}

// ── Tool: get_stock_kline ────────────────────────────────

// StockKLineTool reads K-line history from the stock-middleware output.
type StockKLineTool struct {
	dataDir string
}

// NewStockKLineTool creates a StockKLineTool.
func NewStockKLineTool(dataDir string) *StockKLineTool {
	if dataDir == "" {
		dataDir = `D:\stock\data\stocks`
	}
	return &StockKLineTool{dataDir: dataDir}
}

func (t *StockKLineTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "get_stock_kline",
		Desc: "获取A股历史K线数据。数据来自 stock-middleware 中间件。返回最近N条K线（默认20条）。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"code": {
				Type:     schema.String,
				Desc:     "股票代码，如 sh600519",
				Required: true,
			},
			"period": {
				Type:     schema.String,
				Desc:     "K线周期：day(日线)/week(周线)/month(月线)，默认 day",
				Required: false,
			},
			"limit": {
				Type:     schema.Integer,
				Desc:     "返回最近的K线条数（默认20，最大100）",
				Required: false,
			},
		}),
	}, nil
}

func (t *StockKLineTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Code   string `json:"code"`
		Period string `json:"period"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("get_stock_kline: invalid args: %w", err)
	}
	if args.Code == "" {
		return "", fmt.Errorf("get_stock_kline: code 不能为空")
	}
	if args.Period == "" {
		args.Period = "day"
	}
	if args.Limit <= 0 {
		args.Limit = 20
	}
	if args.Limit > 100 {
		args.Limit = 100
	}

	klines, err := t.readKLine(args.Code, args.Period)
	if err != nil {
		// Graceful degradation: return hint instead of error.
		return fmt.Sprintf("K线数据暂不可用 (%s, %s): %v。请等待 stock-middleware 采集数据后重试。", args.Code, args.Period, err), nil
	}

	// Take last N.
	if len(klines) > args.Limit {
		klines = klines[len(klines)-args.Limit:]
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %s K线数据 (最近%d条)\n\n", args.Code, len(klines)))
	b.WriteString("| 日期 | 开盘 | 收盘 | 最高 | 最低 | 成交量 |\n")
	b.WriteString("|------|------|------|------|------|--------|\n")
	for _, k := range klines {
		b.WriteString(fmt.Sprintf("| %s | %.2f | %.2f | %.2f | %.2f | %d |\n",
			k.Time, k.Open, k.Close, k.High, k.Low, k.Volume))
	}

	if len(klines) >= 5 {
		first := klines[0]
		last := klines[len(klines)-1]
		chg := (last.Close - first.Close) / first.Close * 100
		b.WriteString(fmt.Sprintf("\n**区间涨跌**: %.2f → %.2f (%.2f%%) | 数据更新时间: %s\n",
			first.Close, last.Close, chg, klines[len(klines)-1].Timestamp.Format("2006-01-02 15:04:05")))
	}

	return b.String(), nil
}

func (t *StockKLineTool) readKLine(code, period string) ([]stockKLine, error) {
	path := filepath.Join(t.dataDir, "historical", code+"_"+period+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("文件读取失败: %w", err)
	}
	var klines []stockKLine
	if err := json.Unmarshal(data, &klines); err != nil {
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}
	return klines, nil
}

// ── compile-time checks ──

var (
	_ tool.InvokableTool = (*StockRealtimeTool)(nil)
	_ tool.InvokableTool = (*StockKLineTool)(nil)
)
