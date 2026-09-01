package tool

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// AgentDelegateTool 把任务委派给专家子 agent（agent-as-tool，supervisor 模式）。
// 主 agent 的 ToolsNode 注册此工具后，LLM 通过 tool_call 决定委派给谁。
//
// 注意：此工具不走 WrapWithTimeoutBreaker 默认 15s 超时（子 agent 多轮
// 工具调用必然超时），超时由本工具独立管理（timeout 字段）。
type AgentDelegateTool struct {
	agents  map[string]compose.Runnable[*schema.Message, *schema.Message]
	timeout time.Duration
	logger  *zap.Logger
}

// NewAgentDelegateTool 构造委派工具。
func NewAgentDelegateTool(
	agents map[string]compose.Runnable[*schema.Message, *schema.Message],
	timeout time.Duration,
	logger *zap.Logger,
) *AgentDelegateTool {
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return &AgentDelegateTool{agents: agents, timeout: timeout, logger: logger}
}

// delegateArgs 委派参数，实现 ArgsValidator 供 RetryGate 拦截。
type delegateArgs struct {
	Agent string `json:"agent"`
	Query string `json:"query"`
}

func (a delegateArgs) Validate() error {
	if a.Agent == "" {
		return fmt.Errorf("agent 不能为空（可选值见工具描述）")
	}
	if a.Query == "" {
		return fmt.Errorf("query 不能为空")
	}
	return nil
}

func (t *AgentDelegateTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	names := make([]string, 0, len(t.agents))
	for n := range t.agents {
		names = append(names, n)
	}
	return &schema.ToolInfo{
		Name: "delegate_to_agent",
		Desc: "把专业领域任务委派给专家子代理，返回专家结论。当任务明显属于某个子代理的专长（如股票分析、代码编程）时调用，不要自己硬答。可用子代理: " + fmt.Sprint(names),
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"agent": {
				Type:     schema.String,
				Desc:     "子代理名，必须是可用列表之一",
				Required: true,
			},
			"query": {
				Type:     schema.String,
				Desc:     "要委派给子代理的完整问题",
				Required: true,
			},
		}),
	}, nil
}

func (t *AgentDelegateTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	args, err := ParseArgs[delegateArgs](argsJSON)
	if err != nil {
		return "", fmt.Errorf("delegate_to_agent: %w", err)
	}

	run, ok := t.agents[args.Agent]
	if !ok {
		return "", fmt.Errorf("delegate_to_agent: unknown sub agent %q (available: %v)", args.Agent, t.agentNames())
	}

	ctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	out, err := run.Invoke(ctx, &schema.Message{Role: schema.User, Content: args.Query})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("delegate_to_agent: sub agent %q timeout after %s", args.Agent, t.timeout)
		}
		return "", fmt.Errorf("delegate_to_agent: sub agent %q failed: %w", args.Agent, err)
	}
	t.logger.Info("delegate done",
		zap.String("agent", args.Agent),
		zap.Int("answer_len", len(out.Content)),
	)
	return out.Content, nil
}

func (t *AgentDelegateTool) agentNames() []string {
	names := make([]string, 0, len(t.agents))
	for n := range t.agents {
		names = append(names, n)
	}
	return names
}
