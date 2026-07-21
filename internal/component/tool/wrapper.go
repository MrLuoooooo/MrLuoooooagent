package tool

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/sony/gobreaker"
)

// WrapperConfig 工具包装器的运行时配置。
type WrapperConfig struct {
	PerToolTimeout time.Duration // 单次工具调用超时（默认 15s）
	MaxFailures    int           // 熔断前允许的连续失败次数（默认 5）
	BreakerTimeout time.Duration // 熔断后多久尝试半开（默认 60s）
}

// DefaultWrapperConfig 生产推荐值。
var DefaultWrapperConfig = WrapperConfig{
	PerToolTimeout: 15 * time.Second,
	MaxFailures:    5,
	BreakerTimeout: 60 * time.Second,
}

// timeoutBreakerWrapper 为 Tool 加 per-tool 超时 + 熔断。
// 不改 Eino 框架源码，不改 Agent Graph，纯包装器。
type timeoutBreakerWrapper struct {
	inner   Tool
	name    string
	cfg     WrapperConfig
	breaker *gobreaker.CircuitBreaker
}

// WrapWithTimeoutBreaker 包装一个 Tool，返回带超时+熔断的新 Tool。
// inner 不能为 nil。
func WrapWithTimeoutBreaker(inner Tool, cfg WrapperConfig) Tool {
	if cfg.PerToolTimeout <= 0 {
		cfg.PerToolTimeout = DefaultWrapperConfig.PerToolTimeout
	}
	if cfg.MaxFailures <= 0 {
		cfg.MaxFailures = DefaultWrapperConfig.MaxFailures
	}
	if cfg.BreakerTimeout <= 0 {
		cfg.BreakerTimeout = DefaultWrapperConfig.BreakerTimeout
	}

	info, _ := inner.Info(context.Background())
	name := "unknown"
	if info != nil {
		name = info.Name
	}

	return &timeoutBreakerWrapper{
		inner: inner,
		name:  name,
		cfg:   cfg,
		breaker: gobreaker.NewCircuitBreaker(gobreaker.Settings{
			Name:        fmt.Sprintf("tool:%s", name),
			MaxRequests: 3,
			Interval:    cfg.BreakerTimeout,
			Timeout:     30 * time.Second,
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.ConsecutiveFailures >= uint32(cfg.MaxFailures)
			},
			OnStateChange: func(name string, from, to gobreaker.State) {
				// 生产环境替换为 zap 日志
				_ = name
				_ = from
				_ = to
			},
		}),
	}
}

func (w *timeoutBreakerWrapper) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return w.inner.Info(ctx)
}

func (w *timeoutBreakerWrapper) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	result, err := w.breaker.Execute(func() (any, error) {
		return w.invokeWithTimeout(ctx, argumentsInJSON, opts...)
	})
	if err != nil {
		if err == gobreaker.ErrOpenState {
			return "", fmt.Errorf("工具 %s 暂时不可用（熔断中），请跳过此工具继续", w.name)
		}
		if err == gobreaker.ErrTooManyRequests {
			return "", fmt.Errorf("工具 %s 请求过多，请稍后重试", w.name)
		}
		return "", err
	}
	return result.(string), nil
}

func (w *timeoutBreakerWrapper) invokeWithTimeout(ctx context.Context, args string, opts ...tool.Option) (string, error) {
	childCtx, cancel := context.WithTimeout(ctx, w.cfg.PerToolTimeout)
	defer cancel()

	type result struct {
		out string
		err error
	}
	ch := make(chan result, 1)

	go func() {
		out, err := w.inner.InvokableRun(childCtx, args, opts...)
		ch <- result{out, err}
	}()

	select {
	case r := <-ch:
		return r.out, r.err
	case <-childCtx.Done():
		return "", fmt.Errorf("工具 %s 响应超时(%v)，请基于已有数据继续", w.name, w.cfg.PerToolTimeout)
	}
}

// 编译期验证 wrapper 实现 Tool 接口。
var _ Tool = (*timeoutBreakerWrapper)(nil)
