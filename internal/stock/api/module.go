package api

import (
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Deps 依赖注入参数。
type Deps struct {
	fx.In
	Logger *zap.Logger
}

// NewClients 创建双数据源客户端列表。
func NewClients(deps Deps) []Client {
	base := NewBaseClient(deps.Logger)
	return []Client{
		NewSinaClient(base, "http://hq.sinajs.cn/list="),
		NewEastMoneyClient(base, "http://push2.eastmoney.com/api/qt/stock/get"),
	}
}

// Module fx 模块。
var Module = fx.Options(
	fx.Provide(NewClients),
)
