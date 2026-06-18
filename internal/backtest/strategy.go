package backtest

// Strategy 交易策略接口。
// 每次新 K 线到达时调用 Evaluate，返回交易信号。
// 纯接口，不依赖外部系统。
type Strategy interface {
	Evaluate(ctx BacktestContext) Signal
}

// Bar 单根 K 线，回测引擎的输入单位。
// 不依赖 api.KLineData，引擎完全自包含。
type Bar struct {
	Time   string  // 日期 "2025-06-18"
	Open   float64 // 开盘价
	High   float64 // 最高价
	Low    float64 // 最低价
	Close  float64 // 收盘价
	Volume int64   // 成交量
}

// ActionType 交易动作。
type ActionType string

const (
	Buy  ActionType = "buy"
	Sell ActionType = "sell"
	Hold ActionType = "hold"
)

// Signal 策略信号。
type Signal struct {
	Action   ActionType // 买入/卖出/持仓
	Quantity float64    // 仓位比例 0~1（买入时表示投入资金比例，卖出时表示卖出持仓比例）
	Reason   string     // 决策理由
}

// Portfolio 当前持仓状态。
type Portfolio struct {
	Cash     float64 // 可用现金
	Position float64 // 持仓数量（股）
	AvgCost  float64 // 平均成本价
}

// BacktestContext 策略决策上下文。
type BacktestContext struct {
	Index     int       // 当前 K 线索引
	Bar       Bar       // 当前 K 线
	History   []Bar     // 历史 K 线（含当前）
	Portfolio Portfolio // 当前持仓
}

// Trade 单笔交易记录。
type Trade struct {
	Time     string    // 日期
	Action   ActionType // 买/卖
	Price    float64   // 成交价
	Quantity float64   // 数量（股）
	Cost     float64   // 金额（含手续费）
	Reason   string    // 决策理由
}

// Compound 按时间复利：时间(日期)、当前总资产。
type Compound struct {
	Time  string
	Equity float64
}

// BacktestReport 回测结果报告。
type BacktestReport struct {
	TotalReturn    float64     // 总收益率(%)
	AnnualReturn   float64     // 年化收益率(%)
	MaxDrawdown    float64     // 最大回撤(%)
	SharpeRatio    float64     // 夏普比率
	WinRate        float64     // 胜率(%)
	TotalTrades    int         // 总交易次数
	WinningTrades  int         // 盈利交易次数
	InitialCapital float64     // 初始资金
	FinalEquity    float64     // 最终资产
	Trades         []Trade     // 交易明细（最多100条）
	EquityCurve    []Compound  // 资产曲线（按日期）
	Commission     float64     // 总手续费
	AnnualVolatility float64   // 年化波动率(%)
}
