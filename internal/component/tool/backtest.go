package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/backtest"
	stockapi "github.com/MrLuoooooo/MrLuoooooagent/internal/stock/api"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// BacktestTool 回测工具。
// 将 K 线数据转换为回测引擎输入，内置一个默认均线策略。
type BacktestTool struct {
	// 不依赖外部服务，引擎是纯函数。K 线数据由用户通过参数传入或从 Collector 获取。
	collector interface {
		FetchKLine(ctx context.Context, code string, period string, limit int) ([]stockapi.KLineData, error)
	}
}

// NewBacktestTool collector 传入 stock.Collector 用于获取 K 线数据。
func NewBacktestTool(collector interface{ FetchKLine(ctx context.Context, code string, period string, limit int) ([]stockapi.KLineData, error) }) *BacktestTool {
	return &BacktestTool{collector: collector}
}

func (t *BacktestTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "run_backtest",
		Desc: `对指定股票执行回测。使用默认双均线策略（5日/20日金叉买入、死叉卖出）。

参数：
- code: 股票代码
- 引擎自动获取历史 K 线作为回测输入
- initial_capital: 初始资金（默认 100000）

返回：总收益率、年化收益、最大回撤、夏普比率、胜率、交易明细。

注意：回测结果基于历史数据，不代表未来表现。`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"code": {
				Type:     schema.String,
				Desc:     "股票代码，如 sh600519",
				Required: true,
			},
			"initial_capital": {
				Type:     schema.Number,
				Desc:     "初始资金（元），默认 100000",
				Required: false,
			},
			"short_period": {
				Type:     schema.Integer,
				Desc:     "短期均线周期，默认5",
				Required: false,
			},
			"long_period": {
				Type:     schema.Integer,
				Desc:     "长期均线周期，默认20",
				Required: false,
			},
		}),
	}, nil
}

func (t *BacktestTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	args, err := ParseArgs[backtestArgs](argsJSON)
	if err != nil {
		return "", fmt.Errorf("run_backtest: %w", err)
	}
	if args.InitialCapital <= 0 {
		args.InitialCapital = 100000
	}
	if args.ShortPeriod <= 0 {
		args.ShortPeriod = 5
	}
	if args.LongPeriod <= 0 {
		args.LongPeriod = 20
	}

	// 获取 K 线数据
	klines, err := t.collector.FetchKLine(ctx, args.Code, "day", 500)
	if err != nil {
		return fmt.Sprintf("回测失败：无法获取K线数据 (%v)", err), nil
	}
	if len(klines) < args.LongPeriod+10 {
		return fmt.Sprintf("回测失败：K线数据不足（需要至少%d条，实际%d条）", args.LongPeriod+10, len(klines)), nil
	}

	// 转换为 backtest.Bar
	bars := klinesToBars(klines)

	strategy := &defaultStrategy{shortPeriod: args.ShortPeriod, longPeriod: args.LongPeriod}
	report := backtest.RunBacktest(strategy, bars, float64(args.InitialCapital), 0.0003)

	return formatBacktestReport(report, klines[0].Time, klines[len(klines)-1].Time), nil
}

type backtestArgs struct {
	Code           string  `json:"code"`
	InitialCapital float64 `json:"initial_capital"`
	ShortPeriod    int     `json:"short_period"`
	LongPeriod     int     `json:"long_period"`
}

func (a backtestArgs) Validate() error {
	if a.Code == "" {
		return fmt.Errorf("code 不能为空")
	}
	return nil
}

// defaultStrategy 双均线交叉策略。
type defaultStrategy struct {
	shortPeriod int
	longPeriod  int
	bought      bool
}

func (s *defaultStrategy) Evaluate(ctx backtest.BacktestContext) backtest.Signal {
	if len(ctx.History) < s.longPeriod+1 {
		return backtest.Signal{Action: backtest.Hold}
	}
	shortMA := backtest.BarSMA(ctx.History, s.shortPeriod)
	longMA := backtest.BarSMA(ctx.History, s.longPeriod)
	prevShort := backtest.BarSMA(ctx.History[:len(ctx.History)-1], s.shortPeriod)
	prevLong := backtest.BarSMA(ctx.History[:len(ctx.History)-1], s.longPeriod)

	if !s.bought && prevShort <= prevLong && shortMA > longMA {
		s.bought = true
		return backtest.Signal{Action: backtest.Buy, Quantity: 0.8, Reason: "金叉买入"}
	}
	if s.bought && prevShort >= prevLong && shortMA < longMA {
		s.bought = false
		return backtest.Signal{Action: backtest.Sell, Quantity: 1.0, Reason: "死叉卖出"}
	}
	return backtest.Signal{Action: backtest.Hold}
}

func klinesToBars(klines []stockapi.KLineData) []backtest.Bar {
	bars := make([]backtest.Bar, len(klines))
	for i, k := range klines {
		bars[i] = backtest.Bar{
			Time:   k.Time,
			Open:   k.Open,
			High:   k.High,
			Low:    k.Low,
			Close:  k.Close,
			Volume: k.Volume,
		}
	}
	return bars
}

func formatBacktestReport(r *backtest.BacktestReport, startDate, endDate string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## 回测报告 (%s ~ %s)\n\n", startDate, endDate))
	b.WriteString(fmt.Sprintf("初始资金: ¥%.0f\n", r.InitialCapital))
	b.WriteString(fmt.Sprintf("最终资产: ¥%.2f\n\n", r.FinalEquity))

	b.WriteString("### 收益指标\n")
	b.WriteString(fmt.Sprintf("- 总收益率: %.2f%%\n", r.TotalReturn))
	b.WriteString(fmt.Sprintf("- 年化收益: %.2f%%\n", r.AnnualReturn))
	b.WriteString(fmt.Sprintf("- 最大回撤: %.2f%%\n", r.MaxDrawdown))
	b.WriteString(fmt.Sprintf("- 年化波动: %.2f%%\n\n", r.AnnualVolatility))

	b.WriteString("### 风险调整\n")
	b.WriteString(fmt.Sprintf("- 夏普比率: %.2f\n", r.SharpeRatio))
	b.WriteString(fmt.Sprintf("- 胜率: %.2f%% (%d/%d)\n", r.WinRate, r.WinningTrades, r.TotalTrades))
	b.WriteString(fmt.Sprintf("- 总手续费: ¥%.2f\n\n", r.Commission))

	if len(r.Trades) > 0 {
		b.WriteString("### 交易明细（最近20笔）\n")
		b.WriteString("| 日期 | 操作 | 价格 | 数量 | 金额 | 理由 |\n")
		b.WriteString("|------|------|------|------|------|------|\n")
		start := 0
		if len(r.Trades) > 20 {
			start = len(r.Trades) - 20
		}
		for _, tr := range r.Trades[start:] {
			b.WriteString(fmt.Sprintf("| %s | %s | %.2f | %.0f | %.0f | %s |\n",
				tr.Time, tr.Action, tr.Price, tr.Quantity, tr.Cost, tr.Reason))
		}
	}

	b.WriteString("\n> ⚠️ 回测结果基于历史数据，不构成投资建议。实盘需考虑滑点、冲击成本和市场环境变化。\n")
	return b.String()
}

var _ tool.InvokableTool = (*BacktestTool)(nil)
