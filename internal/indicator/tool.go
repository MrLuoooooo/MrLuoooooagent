package indicator

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// IndicatorTool 技术指标计算工具。
// 纯调度层：参数解析 → 调用同包纯函数 → 格式化输出。
// 不依赖 DB/context/RPC。
type IndicatorTool struct{}

// NewIndicatorTool —
func NewIndicatorTool() *IndicatorTool { return &IndicatorTool{} }

// indicatorArgs —
type indicatorArgs struct {
	Name   string    `json:"name"`
	Prices []float64 `json:"prices"`
	Period int       `json:"period"`
	Fast   int       `json:"fast"`
	Slow   int       `json:"slow"`
	Signal int       `json:"signal"`
	High   []float64 `json:"high"`
	Low    []float64 `json:"low"`
	Volumes []float64 `json:"volumes"`
	Mult   float64   `json:"mult"`
}

func (a *indicatorArgs) parseAndValidate(argsJSON string) error {
	if err := json.Unmarshal([]byte(argsJSON), a); err != nil {
		return fmt.Errorf("calculate_indicator: %w", err)
	}
	if a.Name == "" {
		return fmt.Errorf("name 不能为空，支持的指标: sma,ema,wma,macd,rsi,boll,kdj,atr,obv")
	}
	if len(a.Prices) == 0 && a.Name != "kdj" && a.Name != "atr" {
		return fmt.Errorf("prices 不能为空")
	}
	return nil
}

func (t *IndicatorTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "calculate_indicator",
		Desc: `计算A股常用技术指标。纯数值计算，基于收盘价/最高价/最低价/成交量序列。

支持的指标:
- sma: 简单移动平均 (period默认20)
- ema: 指数移动平均 (period默认20)
- wma: 加权移动平均 (period默认20)
- macd: MACD (fast=12, slow=26, signal=9)
- rsi: 相对强弱指标 (period默认14)
- boll: 布林带 (period=20, mult=2)
- kdj: 随机指标 (period=9, 需high/low)
- atr: 平均真实波幅 (period=14, 需high/low)
- obv: 能量潮 (需volumes)

输入prices/high/low/volumes为float64数组，至少需要period+1个数据点。
返回格式化的指标数值，含最近5个数据点的值。`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"name": {
				Type: schema.String, Required: true,
				Desc: "指标名称: sma/ema/wma/macd/rsi/boll/kdj/atr/obv",
			},
			"prices": {
				Type: schema.Array, Required: true,
				Desc: "收盘价序列，如 [25.3, 25.5, 25.1, ...]。SMA/EMA/WMA/MACD/RSI/BOLL/OBV 需要。",
			},
			"period": {
				Type: schema.Integer, Required: false,
				Desc: "周期参数。sma/ema/wma 默认20，rsi 默认14，boll 默认20，kdj 默认9，atr 默认14。",
			},
			"fast": {
				Type: schema.Integer, Required: false,
				Desc: "MACD 快线周期，默认12",
			},
			"slow": {
				Type: schema.Integer, Required: false,
				Desc: "MACD 慢线周期，默认26",
			},
			"signal": {
				Type: schema.Integer, Required: false,
				Desc: "MACD 信号线周期，默认9",
			},
			"high": {
				Type: schema.Array, Required: false,
				Desc: "最高价序列，KDJ/ATR 需要",
			},
			"low": {
				Type: schema.Array, Required: false,
				Desc: "最低价序列，KDJ/ATR 需要",
			},
			"volumes": {
				Type: schema.Array, Required: false,
				Desc: "成交量序列，OBV 需要",
			},
			"mult": {
				Type: schema.Number, Required: false,
				Desc: "BOLL 标准差乘数，默认2",
			},
		}),
	}, nil
}

func (t *IndicatorTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args indicatorArgs
	if err := args.parseAndValidate(argsJSON); err != nil {
		return "", err
	}

	if args.Period <= 0 {
		args.Period = defaultPeriod(args.Name)
	}
	if args.Fast <= 0 {
		args.Fast = 12
	}
	if args.Slow <= 0 {
		args.Slow = 26
	}
	if args.Signal <= 0 {
		args.Signal = 9
	}
	if args.Mult <= 0 {
		args.Mult = 2
	}

	switch strings.ToLower(args.Name) {
	case "sma":
		return formatSMA(args.Prices, args.Period), nil
	case "ema":
		return formatEMA(args.Prices, args.Period), nil
	case "wma":
		return formatWMA(args.Prices, args.Period), nil
	case "macd":
		return formatMACD(args.Prices, args.Fast, args.Slow, args.Signal), nil
	case "rsi":
		return formatRSI(args.Prices, args.Period), nil
	case "boll":
		return formatBOLL(args.Prices, args.Period, args.Mult), nil
	case "kdj":
		return formatKDJ(args.High, args.Low, args.Prices, args.Period), nil
	case "atr":
		return formatATR(args.High, args.Low, args.Prices, args.Period), nil
	case "obv":
		return formatOBV(args.Prices, args.Volumes), nil
	default:
		return fmt.Sprintf("不支持的指标: %s。支持: sma,ema,wma,macd,rsi,boll,kdj,atr,obv", args.Name), nil
	}
}

// ── 格式化（调用同包纯函数，不再带 indicator. 前缀） ─

func formatSMA(prices []float64, period int) string {
	result := SMA(prices, period)
	return format1D("SMA", period, result)
}

func formatEMA(prices []float64, period int) string {
	result := EMA(prices, period)
	return format1D("EMA", period, result)
}

func formatWMA(prices []float64, period int) string {
	result := WMA(prices, period)
	return format1D("WMA", period, result)
}

func formatMACD(prices []float64, fast, slow, signal int) string {
	r := MACD(prices, fast, slow, signal)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## MACD(%d,%d,%d)\n\n", fast, slow, signal))
	b.WriteString("| 位置 | DIFF | DEA | MACD | 信号 |\n")
	b.WriteString("|------|------|-----|------|------|\n")
	tail := tail5(len(prices), r.DIFF, r.DEA, r.MACD)
	for _, item := range tail {
		sig := ""
		if item.v3 > 0 {
			sig = "多头"
		} else if item.v3 < 0 {
			sig = "空头"
		}
		b.WriteString(fmt.Sprintf("| %d | %.4f | %.4f | %.4f | %s |\n", item.idx+1, item.v1, item.v2, item.v3, sig))
	}
	if len(tail) >= 2 {
		last, prev := tail[len(tail)-1], tail[len(tail)-2]
		b.WriteString(fmt.Sprintf("\n**最新**: DIFF=%.4f, DEA=%.4f, MACD柱=%.4f\n", last.v1, last.v2, last.v3))
		if prev.v3*last.v3 <= 0 && last.v3 != 0 {
			if last.v3 > 0 {
				b.WriteString("**金叉信号**: DIFF 上穿 DEA\n")
			} else {
				b.WriteString("**死叉信号**: DIFF 下穿 DEA\n")
			}
		}
	}
	return b.String()
}

func formatRSI(prices []float64, period int) string {
	result := RSI(prices, period)
	return format1D("RSI", period, result)
}

func formatBOLL(prices []float64, period int, mult float64) string {
	r := BOLL(prices, period, mult)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## BOLL(%d, %.1f)\n\n", period, mult))
	b.WriteString("| 位置 | 上轨 | 中轨 | 下轨 | 带宽%% |\n")
	b.WriteString("|------|------|------|------|-------|\n")
	tail := tail5(len(prices), r.Upper, r.Middle, r.Lower)
	for _, item := range tail {
		bandWidth := (item.v1 - item.v3) / (item.v2 + 0.0001) * 100
		b.WriteString(fmt.Sprintf("| %d | %.2f | %.2f | %.2f | %.1f%% |\n", item.idx+1, item.v1, item.v2, item.v3, bandWidth))
	}
	if len(tail) > 0 {
		last := tail[len(tail)-1]
		posInBand := (last.v1 - last.v2) / (last.v1 - last.v3 + 0.0001) * 100
		b.WriteString(fmt.Sprintf("\n**最新**: 上轨=%.2f, 中轨=%.2f, 下轨=%.2f\n", last.v1, last.v2, last.v3))
		b.WriteString(fmt.Sprintf("价格在带宽内的位置: %.0f%% (0%%=下轨, 100%%=上轨)\n", posInBand))
	}
	return b.String()
}

func formatKDJ(high, low, close []float64, period int) string {
	r := KDJ(high, low, close, period)
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## KDJ(%d)\n\n", period))
	b.WriteString("| 位置 | K | D | J | 区域 |\n")
	b.WriteString("|------|---|---|---|------|\n")
	tail := tail5(len(close), r.K, r.D, r.J)
	for _, item := range tail {
		zone := "中性"
		if item.v3 > 80 {
			zone = "超买"
		} else if item.v3 < 20 {
			zone = "超卖"
		}
		b.WriteString(fmt.Sprintf("| %d | %.2f | %.2f | %.2f | %s |\n", item.idx+1, item.v1, item.v2, item.v3, zone))
	}
	if len(tail) >= 2 {
		last := tail[len(tail)-1]
		b.WriteString(fmt.Sprintf("\n**最新**: K=%.2f, D=%.2f, J=%.2f\n", last.v1, last.v2, last.v3))
	}
	return b.String()
}

func formatATR(high, low, close []float64, period int) string {
	result := ATR(high, low, close, period)
	return format1D("ATR", period, result)
}

func formatOBV(prices, volumes []float64) string {
	result := OBV(prices, volumes)
	if result == nil {
		return "OBV 计算失败：prices 和 volumes 长度不一致或数据不足"
	}
	var b strings.Builder
	b.WriteString("## OBV\n\n")
	n := len(result)
	b.WriteString("| 位置 | OBV |\n|------|------|\n")
	start := n - 5
	if start < 0 {
		start = 0
	}
	for i := start; i < n; i++ {
		b.WriteString(fmt.Sprintf("| %d | %.0f |\n", i+1, result[i]))
	}
	return b.String()
}

// ── 内部工具 ──────────────────────────────────────

type triple struct{ idx int; v1, v2, v3 float64 }

func tail5(n int, slices ...[]float64) []triple {
	start := n - 5
	if start < 0 {
		start = 0
	}
	result := make([]triple, 0, 5)
	for i := start; i < n; i++ {
		if len(slices) >= 3 {
			result = append(result, triple{i, slices[0][i], slices[1][i], slices[2][i]})
		}
	}
	return result
}

func format1D(name string, period int, result []float64) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("## %s(%d)\n\n", name, period))
	n := len(result)
	start := n - 5
	if start < 0 {
		start = 0
	}
	b.WriteString("| 位置 | 数值 |\n|------|------|\n")
	for i := start; i < n; i++ {
		b.WriteString(fmt.Sprintf("| %d | %.4f |\n", i+1, result[i]))
	}
	if n > 0 && start < n-1 {
		b.WriteString(fmt.Sprintf("\n**最新值**: %.4f (第%d个)\n", result[n-1], n))
	}
	return b.String()
}

func defaultPeriod(name string) int {
	switch strings.ToLower(name) {
	case "rsi", "atr":
		return 14
	case "kdj":
		return 9
	default:
		return 20
	}
}

// ── compile-time check ──

var _ tool.InvokableTool = (*IndicatorTool)(nil)
