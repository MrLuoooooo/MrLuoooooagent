// Package cozeloop 提供扣子罗盘（loop.coze.cn）trace 上报能力。
// 通过 eino callbacks 机制捕获 agent 图执行链路（chat_model / tools /
// lambda 各节点一个 span），在扣子罗盘 Trace 页可视化每一次 agent 流程。
package cozeloop

import (
	"context"
	"fmt"

	eino_callbacks "github.com/cloudwego/eino/callbacks"
	cozeloop "github.com/coze-dev/cozeloop-go"

	eino_loop "github.com/cloudwego/eino-ext/callbacks/cozeloop"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/config"
	"go.uber.org/zap"
)

// Tracer 持有 cozeloop 客户端与 eino callback handler。
// nil *Tracer 表示未启用，所有方法安全空操作（配合 fx 优雅禁用）。
type Tracer struct {
	client  cozeloop.Client
	handler eino_callbacks.Handler
	logger  *zap.Logger
}

// New 按 cfg 构造 Tracer；未启用时返回 (nil, nil)，调用方无需判错。
// token 未配置时视为未启用——trace 是旁路能力，绝不能因它阻断主流程。
func New(cfg *config.CozeLoopConfig, logger *zap.Logger) (*Tracer, error) {
	if cfg == nil || !cfg.Enabled {
		return nil, nil
	}
	if cfg.APIToken == "" || cfg.WorkspaceID == "" {
		logger.Warn("cozeloop: enabled 但 token/workspace_id 为空，trace 上报禁用（在 .env 配 GOAGENT_COZELOOP_API_TOKEN / GOAGENT_COZELOOP_WORKSPACE_ID）")
		return nil, nil
	}

	opts := []cozeloop.Option{
		cozeloop.WithAPIToken(cfg.APIToken),
		cozeloop.WithWorkspaceID(cfg.WorkspaceID),
	}
	if cfg.BaseURL != "" {
		// 自建 coze-loop 服务端时指定；留空走 SaaS
		opts = append(opts, cozeloop.WithAPIBaseURL(cfg.BaseURL))
	}

	client, err := cozeloop.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("cozeloop: new client: %w", err)
	}

	logger.Info("cozeloop: trace 上报已启用", zap.String("workspace_id", cfg.WorkspaceID))
	return &Tracer{
		client:  client,
		handler: eino_loop.NewLoopHandler(client),
		logger:  logger,
	}, nil
}

// Handler 返回 eino callback handler，经 compose.WithCallbacks 注入
// agent 图执行。Tracer 为 nil（未启用）时返回 nil，调用方据此跳过注入。
func (t *Tracer) Handler() eino_callbacks.Handler {
	if t == nil {
		return nil
	}
	return t.handler
}

// Close 停止上报并 flush 异步队列。SDK 的 trace 数据走内存队列异步批量
// 上报，进程退出前不 Close 会丢尾部数据——必须挂在 fx lifecycle OnStop。
func (t *Tracer) Close() {
	if t == nil || t.client == nil {
		return
	}
	// 不传 nil ctx：CloseTrace 会把 ctx 透传给 otel spanProcessor.Shutdown，
	// nil ctx 在部分实现里触发 nil interface 调用 panic。
	t.client.Close(context.Background())
	t.logger.Info("cozeloop: client closed, trace 队列已 flush")
}
