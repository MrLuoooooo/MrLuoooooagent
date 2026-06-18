package modelmanager

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/sony/gobreaker"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// Priority 请求优先级。
type Priority int

const (
	PrioAlert  Priority = 0 // cron 预警
	PrioStock  Priority = 1 // 股票分析 (stock_mode)
	PrioNormal Priority = 2 // 普通对话
)

// prioKey context key 类型。
type prioKey struct{}

var prioContextKey prioKey

// UseLocalKey 设置后走本地 Ollama 快速通道，不排队不限流。
type useLocalKey struct{}
var UseLocalKey useLocalKey

// WithPriority 将优先级注入 context。
func WithPriority(ctx context.Context, p Priority) context.Context {
	return context.WithValue(ctx, prioContextKey, p)
}

// HighConcurrencyManager 包装 ModelManager，提供高并发下的四层保护：
// 1. 信号量限制并发 LLM 请求数 (max 20)
// 2. 令牌桶限制发送速率 (10 QPS, burst 2)
// 3. 优先级队列（预警 > 股票 > 普通）
// 4. Circuit Breaker 熔断（连续5次失败 → 熔断30s → 半开探测）
// 5. 本地模型快速通道：ctx 含 UseLocalKey 时跳速率限制直连 Ollama
type HighConcurrencyManager struct {
	underlying model.ChatModel // *ModelManager (DeepSeek)
	sema       chan struct{}   // 信号量
	limiter    *rate.Limiter
	breaker    *gobreaker.CircuitBreaker
	logger     *zap.Logger
	inflight   atomic.Int32
	rejected   atomic.Int64 // 被拒绝总数
	fastModel  model.ChatModel // 本地 Ollama 快速通道 (nil=不可用)
}

// NewHighConcurrencyManager 创建高并发包装器。
// maxConcurrent: 最大并发 LLM 请求数 (建议 20)
// qps: 每秒最大发送速率 (建议 10)
func NewHighConcurrencyManager(underlying model.ChatModel, maxConcurrent, qps int, logger *zap.Logger) *HighConcurrencyManager {
	m := &HighConcurrencyManager{
		underlying: underlying,
		sema:       make(chan struct{}, maxConcurrent),
		limiter:    rate.NewLimiter(rate.Limit(qps), 2),
		logger:     logger,
	}
	m.breaker = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "llm-api",
		MaxRequests: 3,
		Interval:    60 * time.Second,
		Timeout:     30 * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 5
		},
		OnStateChange: func(name string, from, to gobreaker.State) {
			logger.Warn("circuit breaker state change",
				zap.String("name", name),
				zap.String("from", from.String()),
				zap.String("to", to.String()),
			)
		},
	})
	return m
}

// SetLocalModel 设置本地模型快速通道。
func (m *HighConcurrencyManager) SetLocalModel(local model.ChatModel) {
	m.fastModel = local
}

// Generate 非流式调用，经四层保护。
// 若 ctx 含 UseLocalKey 且本地模型可用，走快速通道不排队。
func (m *HighConcurrencyManager) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if _, bypass := ctx.Value(UseLocalKey).(bool); bypass && m.fastModel != nil {
		return m.fastModel.Generate(ctx, input, opts...)
	}
	if err := m.admit(ctx); err != nil {
		return nil, err
	}
	defer m.release()

	result, err := m.breaker.Execute(func() (interface{}, error) {
		return m.underlying.Generate(ctx, input, opts...)
	})
	if err != nil {
		return nil, err
	}
	return result.(*schema.Message), nil
}

// Stream 流式调用，经四层保护。
func (m *HighConcurrencyManager) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if _, bypass := ctx.Value(UseLocalKey).(bool); bypass && m.fastModel != nil {
		return m.fastModel.Stream(ctx, input, opts...)
	}
	if err := m.admit(ctx); err != nil {
		return nil, err
	}

	result, err := m.breaker.Execute(func() (interface{}, error) {
		return m.underlying.Stream(ctx, input, opts...)
	})
	if err != nil {
		m.release()
		return nil, err
	}
	sr := result.(*schema.StreamReader[*schema.Message])

	// 包装 reader：Close 时释放信号量
	pipeR, pipeW := schema.Pipe[*schema.Message](64)
	go func() {
		defer m.release()
		defer pipeW.Close()
		for {
			msg, err := sr.Recv()
			if err != nil {
				return
			}
			if closed := pipeW.Send(msg, nil); closed {
				return
			}
		}
	}()

	return pipeR, nil
}

func (m *HighConcurrencyManager) BindTools(tools []*schema.ToolInfo) error {
	return m.underlying.BindTools(tools)
}

// ── 准入控制 ────────────────────────────────

func (m *HighConcurrencyManager) admit(ctx context.Context) error {
	// 1. 令牌桶限流
	if err := m.limiter.Wait(ctx); err != nil {
		m.rejected.Add(1)
		return fmt.Errorf("rate limit: %w", err)
	}
	// 2. 信号量限并发
	prio := priorityFromCtx(ctx)
	select {
	case m.sema <- struct{}{}:
		m.inflight.Add(1)
		return nil
	case <-ctx.Done():
		m.rejected.Add(1)
		return ctx.Err()
	default:
	}
	// 信号量满，按优先级排队
	timer := time.NewTimer(30 * time.Second)
	defer timer.Stop()
	for {
		select {
		case m.sema <- struct{}{}:
			m.inflight.Add(1)
			return nil
		case <-timer.C:
			m.rejected.Add(1)
			if prio <= PrioAlert {
				// 高优请求排队超时，返回 503
				return fmt.Errorf("server overloaded (priority=%d)", prio)
			}
			return fmt.Errorf("server overloaded, retry later")
		case <-ctx.Done():
			m.rejected.Add(1)
			return ctx.Err()
		}
	}
}

func (m *HighConcurrencyManager) release() {
	<-m.sema
	m.inflight.Add(-1)
}

// Stats 返回运行时统计。
func (m *HighConcurrencyManager) Stats() (inflight int32, rejected int64) {
	return m.inflight.Load(), m.rejected.Load()
}

func priorityFromCtx(ctx context.Context) Priority {
	if v, ok := ctx.Value(prioContextKey).(Priority); ok {
		return v
	}
	return PrioNormal
}

var _ model.ChatModel = (*HighConcurrencyManager)(nil)
