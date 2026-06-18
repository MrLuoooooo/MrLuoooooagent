package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/stock/db"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// ScreenStocksTool 灵活条件选股。
// 支持任意 Field/Operator/Value 组合的 AND 条件。
type ScreenStocksTool struct {
	db db.StockDB
}

// NewScreenStocksTool —
func NewScreenStocksTool(d db.StockDB) *ScreenStocksTool {
	return &ScreenStocksTool{db: d}
}

// ScreenCondition 单个筛选条件。
// 纯数据结构，无行为。
type ScreenCondition struct {
	Field    string `json:"field"`    // pe, pb, market_cap, industry, roe
	Operator string `json:"operator"` // lt, gt, eq, between, contains
	Value    string `json:"value"`    // 单值：如 "20"；between：如 "10,50"
}

func (t *ScreenStocksTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "screen_stocks",
		Desc: `灵活条件选股。支持任意 Field/Operator/Value 组合，所有条件 AND 连接。

支持的字段和操作符：
- pe: lt(小于)/gt(大于)/eq(等于)/between(区间)
- pb: lt/gt/eq/between
- market_cap: lt/gt/eq/between（市值，单位亿）
- industry: contains(模糊匹配)
- roe: lt/gt/eq/between

示例：
单条件：{"field":"pe","operator":"lt","value":"15"}
区间：{"field":"market_cap","operator":"between","value":"100,500"}
行业：{"field":"industry","operator":"contains","value":"医药"}`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"conditions": {
				Type:     schema.Array,
				Desc:     "筛选条件数组，至少一条。[{\"field\":\"pe\",\"operator\":\"lt\",\"value\":\"20\"},...]",
				Required: true,
			},
			"limit": {
				Type:     schema.Integer,
				Desc:     "返回数量上限，默认20，最大50",
				Required: false,
			},
		}),
	}, nil
}

func (t *ScreenStocksTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Conditions []ScreenCondition `json:"conditions"`
		Limit      int               `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("screen_stocks: %w", err)
	}
	if len(args.Conditions) == 0 {
		return "", fmt.Errorf("screen_stocks: conditions 不能为空")
	}
	if args.Limit <= 0 {
		args.Limit = 20
	}
	if args.Limit > 50 {
		args.Limit = 50
	}

	filter, convErr := conditionsToFilter(args.Conditions)
	if convErr != nil {
		return fmt.Sprintf("条件错误: %v", convErr), nil
	}

	results, err := t.db.List(filter)
	if err != nil {
		return fmt.Sprintf("查询失败: %v", err), nil
	}
	if len(results) == 0 {
		return "未找到符合条件的股票。尝试放宽筛选条件。", nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## 条件选股 (%d条)\n\n", len(results)))
	b.WriteString("| 代码 | 名称 | 行业 | 市值(亿) | PE | PB |\n")
	b.WriteString("|------|------|------|---------|----|----|\n")
	for _, r := range results {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %.0f | %.1f | %.1f |\n",
			r.Code, r.Name, r.Industry, r.MarketCap, r.PE, r.PB))
	}
	return b.String(), nil
}

// conditionsToFilter 将 ScreenCondition 数组转为 StockFilter。
// 纯函数，输入条件列表，输出筛选参数。
func conditionsToFilter(conditions []ScreenCondition) (db.StockFilter, error) {
	var f db.StockFilter
	for _, c := range conditions {
		switch c.Field {
		case "industry":
			if c.Operator != "contains" {
				return f, fmt.Errorf("industry 仅支持 contains 操作符")
			}
			f.Industry = c.Value
		case "pe":
			v, v2, err := parseBound(c.Value, c.Operator)
			if err != nil {
				return f, fmt.Errorf("pe: %w", err)
			}
			switch c.Operator {
			case "lt":
				f.MaxPE = v
			case "gt":
				f.MinPE = v
			case "eq":
				f.MinPE, f.MaxPE = v, v
			case "between":
				f.MinPE, f.MaxPE = v, v2
			}
		case "pb":
			v, v2, err := parseBound(c.Value, c.Operator)
			if err != nil {
				return f, fmt.Errorf("pb: %w", err)
			}
			switch c.Operator {
			case "lt":
				f.MaxPB = v
			case "gt":
				f.MinPB = v
			case "eq":
				f.MinPB, f.MaxPB = v, v
			case "between":
				f.MinPB, f.MaxPB = v, v2
			}
		case "market_cap":
			v, v2, err := parseBound(c.Value, c.Operator)
			if err != nil {
				return f, fmt.Errorf("market_cap: %w", err)
			}
			switch c.Operator {
			case "lt":
				f.MaxMarketCap = v
			case "gt":
				f.MinMarketCap = v
			case "eq":
				f.MinMarketCap, f.MaxMarketCap = v, v
			case "between":
				f.MinMarketCap, f.MaxMarketCap = v, v2
			}
		default:
			return f, fmt.Errorf("不支持的字段: %s。支持: pe, pb, market_cap, industry", c.Field)
		}
	}
	f.Limit = 50
	return f, nil
}

// parseBound 解析单值或区间值。
func parseBound(raw, op string) (float64, float64, error) {
	if op == "between" {
		parts := strings.Split(raw, ",")
		if len(parts) != 2 {
			return 0, 0, fmt.Errorf("between 需要两个值，逗号分隔，如 10,50")
		}
		v1, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		v2, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if err1 != nil || err2 != nil {
			return 0, 0, fmt.Errorf("无法解析数值: %s", raw)
		}
		return v1, v2, nil
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	return v, 0, err
}

// ── compile-time check ──

var _ tool.InvokableTool = (*ScreenStocksTool)(nil)
