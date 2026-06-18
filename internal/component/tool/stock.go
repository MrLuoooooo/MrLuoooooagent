package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/stock"
	stockapi "github.com/MrLuoooooo/MrLuoooooagent/internal/stock/api"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ── Tool: get_stock_realtime ─────────────────────────────

// StockRealtimeTool 获取 A 股实时行情，Collector 多源降级。
type StockRealtimeTool struct {
	collector *stock.Collector
}

// NewStockRealtimeTool —
func NewStockRealtimeTool(collector *stock.Collector) *StockRealtimeTool {
	return &StockRealtimeTool{collector: collector}
}

// stockRealtimeArgs 实现 ArgsValidator，参数错误可由 RetryGate 统一拦截。
type stockRealtimeArgs struct {
	Codes string `json:"codes"`
}

func (a stockRealtimeArgs) Validate() error {
	if a.Codes == "" {
		return fmt.Errorf("codes 不能为空")
	}
	return nil
}

func (t *StockRealtimeTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "get_stock_realtime",
		Desc: "获取A股实时行情。双源降级（新浪+东方财富），失败回退本地缓存。支持批量查询（逗号分隔）。",
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
	args, err := ParseArgs[stockRealtimeArgs](argsJSON)
	if err != nil {
		return "", fmt.Errorf("get_stock_realtime: %w", err)
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
		data, err := t.collector.FetchRealtime(ctx, code)
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
		return "无可用实时行情数据。请确认网络正常且股票代码正确。", nil
	}
	return b.String(), nil
}

// ── Tool: get_stock_kline ────────────────────────────────

// StockKLineTool 获取 A 股 K 线历史，Collector 多源降级。
type StockKLineTool struct {
	collector *stock.Collector
}

// NewStockKLineTool —
func NewStockKLineTool(collector *stock.Collector) *StockKLineTool {
	return &StockKLineTool{collector: collector}
}

func (t *StockKLineTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "get_stock_kline",
		Desc: "获取A股历史K线数据。双源降级（新浪+东方财富），失败回退本地缓存。返回最近N条K线。",
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

// stockKLineArgs 实现 ArgsValidator，默认值由 InvokableRun 在 ParseArgs 后设置。
type stockKLineArgs struct {
	Code   string `json:"code"`
	Period string `json:"period"`
	Limit  int    `json:"limit"`
}

func (a stockKLineArgs) Validate() error {
	if a.Code == "" {
		return fmt.Errorf("code 不能为空")
	}
	return nil
}

func (t *StockKLineTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	args, err := ParseArgs[stockKLineArgs](argsJSON)
	if err != nil {
		return "", fmt.Errorf("get_stock_kline: %w", err)
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

	klines, err := t.collector.FetchKLine(ctx, args.Code, args.Period, args.Limit)
	if err != nil {
		return fmt.Sprintf("K线数据暂不可用 (%s, %s): %v", args.Code, args.Period, err), nil
	}
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
		b.WriteString(fmt.Sprintf("\n**区间涨跌**: %.2f → %.2f (%.2f%%) | 更新: %s\n",
			first.Close, last.Close, chg, klines[len(klines)-1].Timestamp.Format("2006-01-02 15:04:05")))
	}
	return b.String(), nil
}

// ── compile-time checks ──

var (
	_ tool.InvokableTool = (*StockRealtimeTool)(nil)
	_ tool.InvokableTool = (*StockKLineTool)(nil)
	_ tool.InvokableTool = (*FinancialReportTool)(nil)
)

// ═══════════════════════════════════════════════════════════
// Tool: get_financial_report — 核心财务指标摘要
// ═══════════════════════════════════════════════════════════

// FinancialReportTool 获取个股核心财务指标。
// 直接依赖 EastMoneyClient（不经过 Collector，财报不需要多源降级）。
type FinancialReportTool struct {
	client *stockapi.EastMoneyClient
}

// NewFinancialReportTool —
func NewFinancialReportTool(client *stockapi.EastMoneyClient) *FinancialReportTool {
	return &FinancialReportTool{client: client}
}

func (t *FinancialReportTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "get_financial_report",
		Desc: `获取A股核心财务指标摘要。数据来源于东方财富。

返回指标：
- PE(动态市盈率) / PB(市净率) — 估值指标
- ROE(净资产收益率) / EPS(每股收益) — 盈利指标
- 毛利率 / 净利率 — 盈利质量
- 每股净资产 / 每股经营现金流 — 资产质量
- 营收/净利及同比增速 — 成长性`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"code": {
				Type:     schema.String,
				Desc:     "股票代码，如 sh600519",
				Required: true,
			},
		}),
	}, nil
}

func (t *FinancialReportTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("get_financial_report: %w", err)
	}
	if args.Code == "" {
		return "", fmt.Errorf("get_financial_report: code 不能为空")
	}

	data, err := t.client.GetFinancialData(ctx, args.Code)
	if err != nil {
		return fmt.Sprintf("财务数据暂不可用 (%s): %v", args.Code, err), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## 财务指标 %s %s\n\n", data.Name, data.Code))
	b.WriteString("### 估值指标\n")
	b.WriteString(fmt.Sprintf("- 动态市盈率(PE): %.2f\n", data.PE))
	b.WriteString(fmt.Sprintf("- 市净率(PB): %.2f\n\n", data.PB))

	b.WriteString("### 盈利能力\n")
	b.WriteString(fmt.Sprintf("- 净资产收益率(ROE): %.2f%%\n", data.ROE))
	b.WriteString(fmt.Sprintf("- 基本每股收益(EPS): %.4f\n", data.EPS))
	b.WriteString(fmt.Sprintf("- 毛利率: %.2f%%\n", data.GrossMargin))
	b.WriteString(fmt.Sprintf("- 净利率: %.2f%%\n\n", data.NetMargin))

	b.WriteString("### 成长性\n")
	b.WriteString(fmt.Sprintf("- 营业总收入: %.2f亿  |  同比: %.2f%%\n", data.Revenue, data.RevenueYoY))
	b.WriteString(fmt.Sprintf("- 归属净利润: %.2f亿  |  同比: %.2f%%\n\n", data.NetProfit, data.ProfitYoY))

	b.WriteString("### 资产质量\n")
	b.WriteString(fmt.Sprintf("- 每股净资产: %.2f\n", data.BPS))
	b.WriteString(fmt.Sprintf("- 每股经营现金流: %.4f\n\n", data.CFPS))

	b.WriteString(fmt.Sprintf("> 数据来源: %s  |  以上数据为最新财报期\n", data.Source))
	return b.String(), nil
}
