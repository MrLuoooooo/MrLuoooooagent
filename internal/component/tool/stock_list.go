package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/stock/db"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// StockListTool 条件选股工具。
// 依赖 StockDB 接口做筛选查询，不依赖具体存储实现。
type StockListTool struct {
	db db.StockDB
}

// NewStockListTool —
func NewStockListTool(d db.StockDB) *StockListTool {
	return &StockListTool{db: d}
}

type stockListArgs struct {
	Industry    string  `json:"industry"`
	MinMarketCap float64 `json:"min_market_cap"`
	MaxMarketCap float64 `json:"max_market_cap"`
	MinPE       float64 `json:"min_pe"`
	MaxPE       float64 `json:"max_pe"`
	MinPB       float64 `json:"min_pb"`
	MaxPB       float64 `json:"max_pb"`
	Limit       int     `json:"limit"`
}

func (a stockListArgs) Validate() error {
	return nil
}

func (t *StockListTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "stock_list",
		Desc: `条件筛选A股股票。支持按行业、市值、PE、PB组合筛选。至少填一个条件。
示例: 筛选医药行业PE<30的股票 → industry=医药, max_pe=30
示例: 筛选市值100-500亿的科技股 → industry=科技, min_market_cap=100, max_market_cap=500`,
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"industry": {
				Type:     schema.String,
				Desc:     "行业名称（模糊匹配），如 医药、新能源、白酒",
				Required: false,
			},
			"min_market_cap": {
				Type:     schema.Number,
				Desc:     "最低市值（亿元）",
				Required: false,
			},
			"max_market_cap": {
				Type:     schema.Number,
				Desc:     "最高市值（亿元）",
				Required: false,
			},
			"min_pe": {
				Type:     schema.Number,
				Desc:     "最低市盈率",
				Required: false,
			},
			"max_pe": {
				Type:     schema.Number,
				Desc:     "最高市盈率",
				Required: false,
			},
			"min_pb": {
				Type:     schema.Number,
				Desc:     "最低市净率",
				Required: false,
			},
			"max_pb": {
				Type:     schema.Number,
				Desc:     "最高市净率",
				Required: false,
			},
			"limit": {
				Type:     schema.Integer,
				Desc:     "返回数量上限，默认20，最大50",
				Required: false,
			},
		}),
	}, nil
}

func (t *StockListTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args stockListArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("stock_list: %w", err)
	}
	if args.Limit <= 0 {
		args.Limit = 20
	}
	if args.Limit > 50 {
		args.Limit = 50
	}

	filter := db.StockFilter{
		Industry:     args.Industry,
		MinMarketCap: args.MinMarketCap,
		MaxMarketCap: args.MaxMarketCap,
		MinPE:        args.MinPE,
		MaxPE:        args.MaxPE,
		MinPB:        args.MinPB,
		MaxPB:        args.MaxPB,
		Limit:        args.Limit,
	}

	results, err := t.db.List(filter)
	if err != nil {
		return fmt.Sprintf("查询失败: %v", err), nil
	}
	if len(results) == 0 {
		return fmt.Sprintf("未找到符合条件的股票。尝试放宽筛选条件。"), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## 条件筛选 (%d条)\n\n", len(results)))
	b.WriteString("| 代码 | 名称 | 行业 | 市值(亿) | PE | PB |\n")
	b.WriteString("|------|------|------|---------|----|----|\n")
	for _, r := range results {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %.0f | %.1f | %.1f |\n",
			r.Code, r.Name, r.Industry, r.MarketCap, r.PE, r.PB))
	}
	return b.String(), nil
}

// ── compile-time check ──

var _ tool.InvokableTool = (*StockListTool)(nil)
