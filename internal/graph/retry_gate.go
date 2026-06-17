package graph

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/schema"
)

const defaultMaxParamRetries = 3

// RetryGate 参数重试门控。
// 嵌入 ReAct 循环的 tools → tool_as_user 之间，拦截工具参数错误消息。
// 当连续参数错误超过上限时，将错误消息替换为终止指令，防止 LLM 无限重试。
//
// SOLID:
//   - SRP: 只负责参数错误计数与拦截
//   - OCP: isParamError 可独立扩展，不修改 Intercept 主流程
//   - LSP: 可作为 compose.InvokableLambda 嵌入任意 graph
//   - ISP: 仅暴露 Intercept 一个入口
//   - DIP: 通过工厂函数注入 maxRetries，不硬编码
type RetryGate struct {
	mu         sync.Mutex
	count      int
	maxRetries int
}

// NewRetryGate 创建重试门控，maxRetries≤0 时使用默认值 3。
func NewRetryGate(maxRetries int) *RetryGate {
	if maxRetries <= 0 {
		maxRetries = defaultMaxParamRetries
	}
	return &RetryGate{maxRetries: maxRetries}
}

// Intercept 检查工具结果消息，拦截连续参数错误。
// 签名匹配 compose.InvokableLambda，可直���作为 LambdaNode 注册。
func (g *RetryGate) Intercept(ctx context.Context, msgs []*schema.Message) ([]*schema.Message, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for _, m := range msgs {
		if m == nil {
			continue
		}
		if isParamError(m.Content) {
			g.count++
			if g.count > g.maxRetries {
				m.Content = fmt.Sprintf(
					"工具调用参数连续错误已达 %d 次（上限 %d），请基于已有信息直接回复用户，不要再尝试调用工具。",
					g.count, g.maxRetries,
				)
			}
		} else {
			g.count = 0 // 成功的工具调用重置计数器
		}
	}
	return msgs, nil
}

// Reset 显式重置计数器（每次新对话开始时调用）。
func (g *RetryGate) Reset() {
	g.mu.Lock()
	g.count = 0
	g.mu.Unlock()
}

// isParamError 判断工具返回内容是否为参数级错误（非运行时错误）。
// 参数错误应计入重试计数；运行时/数据错误不计数，允许 LLM 继续尝试。
func isParamError(content string) bool {
	lowered := strings.ToLower(content)

	paramMarkers := []string{
		"参数解析失败",
		"不能为空",
		"invalid args",
		"参数格式错误",
		"缺少参数",
		"missing required",
	}

	for _, m := range paramMarkers {
		if strings.Contains(lowered, strings.ToLower(m)) {
			return true
		}
	}
	return false
}
