package openaimodel

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// OpenAIChatModel 封装了 OpenAI 兼容的 /chat/completions 接口，支持流式和非流式调用。
type OpenAIChatModel struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
	tools   []openAITool
	logger  *zap.Logger
}

// NewOpenAIChatModel 初始化一个 OpenAI 兼容 ChatModel。
func NewOpenAIChatModel(apiKey, modelName, baseURL string, logger *zap.Logger) model.ChatModel {
	return &OpenAIChatModel{
		apiKey:  apiKey,
		model:   modelName,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
		logger: logger,
	}
}

// chatOptions holds request-level options parsed from model.Option.
type chatOptions struct {
	temperature *float64
	maxTokens   *int
	topP        *float64
}

// parseOptions extracts common options without mutating shared state.
func parseOptions(opts []model.Option) chatOptions {
	o := chatOptions{}
	common := model.GetCommonOptions(nil, opts...)
	if common.Temperature != nil {
		t := float64(*common.Temperature)
		o.temperature = &t
	}
	if common.MaxTokens != nil {
		o.maxTokens = common.MaxTokens
	}
	if common.TopP != nil {
		p := float64(*common.TopP)
		o.topP = &p
	}
	return o
}

// buildRequest constructs the OpenAI API request body from the shared model config
// and request-specific inputs/options.
func (m *OpenAIChatModel) buildRequest(input []*schema.Message, stream bool, opts chatOptions) openAIChatRequest {
	req := openAIChatRequest{
		Model:       m.model,
		Messages:    convertMessages(input),
		Stream:      stream,
		Tools:       m.tools,
		Temperature: opts.temperature,
		MaxTokens:   opts.maxTokens,
		TopP:        opts.topP,
	}
	return req
}

// doChat sends a POST /chat/completions request and returns the raw response body.
func (m *OpenAIChatModel) doChat(ctx context.Context, reqBody openAIChatRequest) (*http.Response, error) {
	if m.logger != nil {
		m.logger.Debug("OpenAI request",
			zap.String("model", reqBody.Model),
			zap.String("url", m.baseURL+"/chat/completions"),
			zap.Int("messages", len(reqBody.Messages)),
			zap.Int("tools", len(reqBody.Tools)),
			zap.Bool("stream", reqBody.Stream),
			zap.Any("tool_names", toolNames(reqBody.Tools)),
		)
	}
	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	url := m.baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)

	return m.client.Do(req)
}

func toolNames(tools []openAITool) []string {
	names := make([]string, len(tools))
	for i, t := range tools {
		names[i] = t.Function.Name
	}
	return names
}

// Generate 非流式调用，发一次请求返回完整回复。
func (m *OpenAIChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	reqBody := m.buildRequest(input, false, parseOptions(opts))

	resp, err := m.doChat(ctx, reqBody)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI error (%d): %s", resp.StatusCode, string(body))
	}

	var respBody openAIChatResponse
	if err := json.Unmarshal(body, &respBody); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if len(respBody.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return convertToMessage(respBody.Choices[0].Message), nil
}

// Stream 流式调用，返回 SSE 事件的 StreamReader。
func (m *OpenAIChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	reqBody := m.buildRequest(input, true, parseOptions(opts))

	resp, err := m.doChat(ctx, reqBody)
	if err != nil {
		return nil, fmt.Errorf("execute: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("OpenAI error (%d): %s", resp.StatusCode, string(body))
	}

	streamReader, streamWriter := schema.Pipe[*schema.Message](64)

	go func() {
		// Stop scanning on context cancellation
		scanDone := make(chan struct{})
		go func() {
			select {
			case <-ctx.Done():
				resp.Body.Close()
			case <-scanDone:
			}
		}()
		defer close(scanDone)
		defer resp.Body.Close()
		defer streamWriter.Close()
		scanner := bufio.NewScanner(resp.Body)
		tcAcc := make(map[int]*accumToolCall)
		sentMsg := false

		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}
			var chunk openAIStreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue
			}
			if len(chunk.Choices) == 0 {
				continue
			}

			delta := chunk.Choices[0].Delta

			for _, tc := range delta.ToolCalls {
				idx := tc.Index
				if tcAcc[idx] == nil {
					tcAcc[idx] = &accumToolCall{}
				}
				acc := tcAcc[idx]
				if tc.ID != "" {
					acc.id = tc.ID
				}
				if tc.Function.Name != "" {
					acc.fnName = tc.Function.Name
				}
				acc.fnArgs += tc.Function.Arguments
			}

			hasToolCallsDone := chunk.Choices[0].FinishReason != nil &&
				*chunk.Choices[0].FinishReason == "tool_calls" &&
				len(tcAcc) > 0

			if delta.Content == "" && !hasToolCallsDone {
				continue
			}

			msg := &schema.Message{
				Role:    schema.Assistant,
				Content: delta.Content,
			}

			if hasToolCallsDone {
				tcs := make([]schema.ToolCall, 0, len(tcAcc))
				for i := 0; i < len(tcAcc); i++ {
					acc := tcAcc[i]
					if acc == nil {
						continue
					}
					tcs = append(tcs, schema.ToolCall{
						ID:   acc.id,
						Type: "function",
						Function: schema.FunctionCall{
							Name:      acc.fnName,
							Arguments: acc.fnArgs,
						},
					})
				}
				if len(tcs) > 0 {
					msg.ToolCalls = tcs
				}
				tcAcc = make(map[int]*accumToolCall)
			}

			streamWriter.Send(msg, nil)
			sentMsg = true
		}

		// Fallback: 当所有 chunk 被过滤，发送空消息防 Eino concat 报错
		if !sentMsg {
			streamWriter.Send(&schema.Message{Role: schema.Assistant}, nil)
		}
	}()

	return streamReader, nil
}

// BindTools 把工具定义转成 OpenAI function calling 格式，后续请求自动带上。
func (m *OpenAIChatModel) BindTools(tools []*schema.ToolInfo) error {
	m.tools = make([]openAITool, len(tools))
	for i, t := range tools {
		var params any
		if t.ParamsOneOf != nil {
			s, err := t.ParamsOneOf.ToJSONSchema()
			if err == nil && s != nil {
				params = s
			}
		}
		m.tools[i] = openAITool{
			Type: "function",
			Function: openAIToolFunction{
				Name:        t.Name,
				Description: t.Desc,
				Parameters:  params,
			},
		}
	}
	return nil
}

type accumToolCall struct {
	id     string
	fnName string
	fnArgs string
}

func convertMessages(messages []*schema.Message) []openAIMessage {
	result := make([]openAIMessage, len(messages))
	for i, msg := range messages {
		om := openAIMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
		if len(msg.ToolCalls) > 0 {
			tcs := make([]openAIToolCall, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				tcs[j] = openAIToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: openAIFunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
			om.ToolCalls = tcs
		}
		if msg.ToolCallID != "" {
			om.ToolCallID = msg.ToolCallID
		}
		result[i] = om
	}
	return result
}

func convertToMessage(om openAIMessage) *schema.Message {
	msg := &schema.Message{
		Role:    schema.RoleType(om.Role),
		Content: om.Content,
	}
	if len(om.ToolCalls) > 0 {
		tcs := make([]schema.ToolCall, len(om.ToolCalls))
		for i, tc := range om.ToolCalls {
			tcs[i] = schema.ToolCall{
				ID:   tc.ID,
				Type: tc.Type,
				Function: schema.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
		msg.ToolCalls = tcs
	}
	if om.ToolCallID != "" {
		msg.ToolCallID = om.ToolCallID
	}
	return msg
}

type openAIChatRequest struct {
	Model       string          `json:"model"`
	Messages    []openAIMessage `json:"messages"`
	Stream      bool            `json:"stream"`
	Tools       []openAITool    `json:"tools,omitempty"`
	Temperature *float64        `json:"temperature,omitempty"`
	MaxTokens   *int            `json:"max_tokens,omitempty"`
	TopP        *float64        `json:"top_p,omitempty"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAITool struct {
	Type     string             `json:"type"`
	Function openAIToolFunction `json:"function"`
}

type openAIToolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters,omitempty"`
}

type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIChatResponse struct {
	Object  string         `json:"object"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

type openAIChoice struct {
	Index        int           `json:"index"`
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

var _ model.ChatModel = (*OpenAIChatModel)(nil)
