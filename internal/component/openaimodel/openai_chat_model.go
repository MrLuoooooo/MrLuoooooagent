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
)

// OpenAIChatModel implements the Eino ChatModel interface (v0.8.13, model.ChatModel).
type OpenAIChatModel struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewOpenAIChatModel creates a new OpenAI chat model.
// Returns model.ChatModel (which embeds model.BaseChatModel + BindTools).
func NewOpenAIChatModel(apiKey, modelName, baseURL string) model.ChatModel {
	return &OpenAIChatModel{
		apiKey:  apiKey,
		model:   modelName,
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// Generate implements model.BaseChatModel.
func (m *OpenAIChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	reqBody := openAIChatRequest{
		Model:    m.model,
		Messages: convertMessages(input),
		Stream:   false,
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

	resp, err := m.client.Do(req)
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

	choice := respBody.Choices[0]
	return convertToMessage(choice.Message), nil
}

// Stream implements model.BaseChatModel.
func (m *OpenAIChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	reqBody := openAIChatRequest{
		Model:    m.model,
		Messages: convertMessages(input),
		Stream:   true,
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

	resp, err := m.client.Do(req)
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
		defer resp.Body.Close()
		defer streamWriter.Close()
		scanner := bufio.NewScanner(resp.Body)
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
			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta
				if delta.Content == "" {
					continue
				}
				msg := &schema.Message{
					Role:    schema.Assistant,
					Content: delta.Content,
				}
				streamWriter.Send(msg, nil)
			}
		}
	}()

	return streamReader, nil
}

// BindTools implements model.ChatModel (the deprecated but required interface).
func (m *OpenAIChatModel) BindTools(tools []*schema.ToolInfo) error {
	// In the real OpenAI API, tools are attached per-request, not pre-bound.
	// Store for use in Generate/Stream calls if needed in the future.
	return nil
}

// -- helpers -- //

func convertMessages(messages []*schema.Message) []openAIMessage {
	result := make([]openAIMessage, len(messages))
	for i, msg := range messages {
		om := openAIMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
		// Attach tool call info if present.
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

// -- OpenAI API types -- //

type openAIChatRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAIToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openAIChatResponse struct {
	Object  string        `json:"object"`
	Model   string        `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage   `json:"usage"`
}

type openAIChoice struct {
	Index        int          `json:"index"`
	Message      openAIMessage `json:"message"`
	FinishReason string       `json:"finish_reason"`
}

type openAIStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
