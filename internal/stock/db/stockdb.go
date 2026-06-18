package db

// StockDB A股股票基本信息数据库接口。
// 实现需支持关键字搜索、条件筛选、精确代码查询。
// 不涉及 DB/context/RPC 依赖（数据从 SQLite 本地读取）。
type StockDB interface {
	// Search 按关键字模糊搜索（名称/代码/行业），limit 控制返回条数。
	Search(keyword string, limit int) ([]StockBasic, error)

	// GetByCode 按股票代码精确查询。
	GetByCode(code string) (*StockBasic, error)

	// List 条件筛选。
	List(filter StockFilter) ([]StockBasic, error)

	// Refresh 从东方财富 API 全量同步股票列表。
	Refresh() error

	// Count 返回数据库中股票总数。
	Count() int
}

// StockBasic 股票基本信息。
type StockBasic struct {
	Code      string  `json:"code"`       // sh600519
	Name      string  `json:"name"`       // 贵州茅台
	Industry  string  `json:"industry"`   // 白酒
	MarketCap float64 `json:"market_cap"` // 总市值（亿元）
	PE        float64 `json:"pe"`         // 市盈率
	PB        float64 `json:"pb"`         // 市净率
}

// StockFilter 条件筛选参数。
// 零值字段不参与筛选。
type StockFilter struct {
	Industry    string  `json:"industry"`     // 行业名（模糊匹配）
	MinMarketCap float64 `json:"min_market_cap"` // 最低市值（亿）
	MaxMarketCap float64 `json:"max_market_cap"` // 最高市值（亿）
	MaxPE       float64 `json:"max_pe"`       // 最高市盈率
	MinPE       float64 `json:"min_pe"`       // 最低市盈率
	MaxPB       float64 `json:"max_pb"`       // 最高市净率
	MinPB       float64 `json:"min_pb"`       // 最低市净率
	Limit       int     `json:"limit"`        // 返回数量上限，默认100
}
