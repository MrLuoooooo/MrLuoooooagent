package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/modelmanager"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/graph"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/store"
	"go.uber.org/zap"
	"unicode/utf8"
)

// ChatHandler 处理 /api/v1/chat，分发到 RAG 流式/非流式或 Agent 三种路径。
type ChatHandler struct {
	svc     *service.ChatService
	convSvc *service.ConversationService
	cpStore *store.CheckpointStore
	logger  *zap.Logger
}

// NewChatHandler —
func NewChatHandler(svc *service.ChatService, convSvc *service.ConversationService, cpStore *store.CheckpointStore, logger *zap.Logger) *ChatHandler {
	return &ChatHandler{svc: svc, convSvc: convSvc, cpStore: cpStore, logger: logger}
}

// Chat 入口：解析请求 → 自动建会话 → load 历史 → 按 stream/agent 分发。
// @Summary      发送聊天消息
// @Description  发送一条消息给 AI，支持普通 RAG 和 Agent 模式，支持流式和非流式返回。流式模式通过 SSE 推送 token/tool_call/tool_result 事件。
// @Tags         聊天
// @Accept       json
// @Produce      json
// @Param        request body model.ChatRequest true "聊天请求参数"
// @Success      200 {object} model.APIEnvelope{data=model.ChatResponseData} "非流式响应"
// @Success      200 "流式响应（SSE），由 Server-Sent Events 推送 token/tool_call/tool_result/done 事件"
// @Failure      400 {object} model.APIEnvelope "请求参数错误"
// @Failure      500 {object} model.APIEnvelope "服务器内部错误"
// @Router       /chat [post]
func (h *ChatHandler) Chat(c *gin.Context) {
	var req model.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Err(400, err.Error()))
		return
	}

	ctx := c.Request.Context()

	convID := req.ConversationID
	var isNew bool
	if convID == "" {
		id, err := h.convSvc.Create(ctx, "新会话")
		if err != nil {
			h.logger.Error("create conversation", zap.Error(err))
			c.JSON(http.StatusInternalServerError, model.Err(500, "创建会话失败"))
			return
		}
		convID = id
		isNew = true
	}

	var history []*schema.Message
	if req.ConversationID != "" {
		var err error
		history, err = h.convSvc.LoadMessages(ctx, convID)
		if err != nil {
			h.logger.Error("load history", zap.String("conv_id", convID), zap.Error(err))
		}
	}

	originalQuestion := req.Question

	// 新会话：用用户第一条消息截取作为标题
	if isNew && originalQuestion != "" {
		title := convTitle(originalQuestion)
		if err := h.convSvc.Rename(ctx, convID, title); err != nil {
			h.logger.Warn("rename conversation", zap.String("conv_id", convID), zap.Error(err))
		}
	}

	if len(history) > 0 {
		// 短期记忆压缩：超出 maxKeep 的旧消息异步做摘要，本次用截断。
		const maxKeep = 20
		if len(history) > maxKeep {
			summary := h.svc.SummarizeHistory(convID, history, maxKeep)
			history = history[len(history)-maxKeep:]
			if summary != "" {
				summaryMsg := &schema.Message{Role: schema.User, Content: summary}
				history = append([]*schema.Message{summaryMsg}, history...)
			}
		}
		req.Question = prependHistory(req.Question, history)
	}

	if req.Agent {
		h.handleAgent(c, req, convID, originalQuestion)
	} else if req.Stream {
		h.handleStream(c, req, convID, originalQuestion)
	} else {
		h.handleInvoke(c, req, convID, originalQuestion)
	}
}

func (h *ChatHandler) handleInvoke(c *gin.Context, req model.ChatRequest, convID string, originalQuestion string) {
	if err := h.svc.SaveUserMessage(convID, originalQuestion); err != nil {
		h.logger.Error("save user message before invoke", zap.String("conv_id", convID), zap.Error(err))
	}

	msg, err := h.svc.Chat(c.Request.Context(), req.Question, convID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, model.OK(model.ChatResponseData{
		Content: msg.Content, Role: string(msg.Role),
	}))
}

func (h *ChatHandler) handleStream(c *gin.Context, req model.ChatRequest, convID string, originalQuestion string) {
	if err := h.svc.SaveUserMessage(convID, originalQuestion); err != nil {
		h.logger.Error("save user message before stream", zap.String("conv_id", convID), zap.Error(err))
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	stream, err := h.svc.ChatStream(ctx, req.Question)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}
	defer stream.Close()

	h.setupSSE(c)

	var full string
	h.streamSSE(c, convID, stream, func(chunk *schema.Message) *model.StreamEvent {
		full += chunk.Content
		return &model.StreamEvent{Type: model.EventToken, Content: chunk.Content}
	})

	if full != "" {
		if err := h.svc.SaveAssistantMessage(convID, full, nil); err != nil {
			h.logger.Error("save stream assistant reply", zap.String("conv_id", convID), zap.Error(err))
		}
	}
}

func (h *ChatHandler) handleAgent(c *gin.Context, req model.ChatRequest, convID string, originalQuestion string) {
	ctx := c.Request.Context()
	// 请求级超时——从进入到回复完成，2 分钟封顶
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	// 股票专精模式：注入 context key，Agent Graph 读取后切换 prompt
	if req.StockMode {
		ctx = context.WithValue(ctx, graph.StockModeKey, true)
		ctx = modelmanager.WithPriority(ctx, modelmanager.PrioStock)
	} else {
		// 通用对话走本地 Ollama 快速通道，不占 API 配额
		ctx = context.WithValue(ctx, modelmanager.UseLocalKey, true)
	}
	// 用 Eino 内置 checkpoint：传入会话 ID 自动管理断点保存和恢复
	cpOpt := compose.WithCheckPointID(convID)

	if req.Stream {
		if err := h.svc.SaveUserMessage(convID, originalQuestion); err != nil {
			h.logger.Error("save user message before agent stream", zap.String("conv_id", convID), zap.Error(err))
		}

		userMsg := &schema.Message{Role: schema.User, Content: req.Question}
		stream, err := h.svc.AgentStream(ctx, userMsg, cpOpt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
			return
		}
		defer stream.Close()

		h.setupSSE(c)

		var full string
		var toolCalls []schema.ToolCall
		seenTools := make(map[string]bool)

		h.streamSSE(c, convID, stream, func(chunk *schema.Message) *model.StreamEvent {
			for _, tc := range chunk.ToolCalls {
				if !seenTools[tc.ID] {
					seenTools[tc.ID] = true
					h.writeSSEEvent(c.Writer, model.StreamEvent{
						Type:     model.EventToolCall,
						Tool:     fmt.Sprintf("%s(%s)", tc.Function.Name, tc.Function.Arguments),
						ToolName: tc.Function.Name,
						ToolArgs: tc.Function.Arguments,
					})
				}
			}

			if chunk.Role == schema.Tool {
				content := toWindowsPath(chunk.Content)
				return &model.StreamEvent{
					Type:    model.EventToolResult,
					Content: content,
				}
			}

			full += chunk.Content
			if len(chunk.ToolCalls) > 0 {
				toolCalls = chunk.ToolCalls
			}
			return &model.StreamEvent{Type: model.EventToken, Content: stripToolCode(chunk.Content)}
		})

		if full != "" {
			if err := h.svc.SaveAssistantMessage(convID, full, toolCalls); err != nil {
				h.logger.Error("save agent assistant reply", zap.String("conv_id", convID), zap.Error(err))
			}
		}

		// Agent 完成：清理 Eino 的 checkpoint
		if h.cpStore != nil {
			_ = h.cpStore.Delete(ctx, convID)
		}
		return
	}

	if err := h.svc.SaveUserMessage(convID, originalQuestion); err != nil {
		h.logger.Error("save user message before agent", zap.String("conv_id", convID), zap.Error(err))
	}
	userMsg := &schema.Message{Role: schema.User, Content: req.Question}
	msg, err := h.svc.Agent(ctx, userMsg, convID, cpOpt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, model.Err(500, err.Error()))
		return
	}

	// Agent 完成：清理 Eino 的 checkpoint
	if h.cpStore != nil {
		_ = h.cpStore.Delete(ctx, convID)
	}

	c.JSON(http.StatusOK, model.OK(model.ChatResponseData{
		Content: msg.Content, Role: string(msg.Role),
	}))
}

func (h *ChatHandler) setupSSE(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
	c.Writer.Flush()
}

func (h *ChatHandler) writeSSEEvent(w io.Writer, evt model.StreamEvent) {
	data, _ := json.Marshal(evt)
	w.Write([]byte("data: " + string(data) + "\n\n"))
}

type emitFn func(chunk *schema.Message) *model.StreamEvent

func (h *ChatHandler) streamSSE(c *gin.Context, convID string, stream *schema.StreamReader[*schema.Message], emit emitFn) {
	c.Stream(func(w io.Writer) bool {
		chunk, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				h.sendConvIDEvent(w, convID)
				h.writeSSEEvent(w, model.StreamEvent{Type: model.EventDone})
				return false
			}
			h.logger.Warn("stream recv error, sending error to client", zap.Error(err))
			h.writeSSEEvent(w, model.StreamEvent{Type: model.EventError, Content: err.Error()})
			return false
		}

		if evt := emit(chunk); evt != nil {
			h.writeSSEEvent(w, *evt)
		}
		return true
	})
}

func (h *ChatHandler) sendConvIDEvent(w io.Writer, convID string) {
	h.writeSSEEvent(w, model.StreamEvent{
		Type:    model.EventConversationID,
		Content: convID,
	})
}

func prependHistory(q string, history []*schema.Message) string {
	const maxHistory = 20
	const maxContentLen = 2000

	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}

	truncate := func(s string) string {
		if len(s) > maxContentLen {
			return s[:maxContentLen] + "...[截断]"
		}
		return s
	}

	var b strings.Builder
	b.WriteString("以下是之前的对话历史:\n")
	for _, m := range history {
		switch m.Role {
		case schema.Tool:
			b.WriteString("tool_result: ")
			b.WriteString(truncate(m.Content))
			b.WriteString("\n")
		case schema.Assistant:
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					b.WriteString("assistant: [调用工具 ")
					b.WriteString(tc.Function.Name)
					b.WriteString("，参数: ")
					b.WriteString(truncate(tc.Function.Arguments))
					b.WriteString("]\n")
				}
			}
			if m.Content != "" {
				b.WriteString("assistant: ")
				b.WriteString(truncate(m.Content))
				b.WriteString("\n")
			}
		default:
			b.WriteString(string(m.Role))
			b.WriteString(": ")
			b.WriteString(truncate(m.Content))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n用户当前问题: ")
	b.WriteString(q)
	return b.String()
}

func toWindowsPath(s string) string {
	var b strings.Builder
	i := 0
	for i < len(s) {
		if i+2 < len(s) && s[i] == '/' && isDriveLetter(s[i+1]) && s[i+2] == '/' {
			b.WriteByte(s[i+1])
			b.WriteString(":\\")
			i += 3
			for i < len(s) {
				if s[i] == '/' {
					b.WriteByte('\\')
				} else if s[i] == ' ' || s[i] == '\n' || s[i] == ',' || s[i] == ')' || s[i] == ']' {
					break
				} else {
					b.WriteByte(s[i])
				}
				i++
			}
			continue
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func isDriveLetter(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

var toolCodeRe = regexp.MustCompile(`(?s)<tool_code>.*?</tool_code>`)

func stripToolCode(s string) string {
	return toolCodeRe.ReplaceAllString(s, "")
}

func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// convTitle 从用户第一句对话截取标题，最多 30 个字符（按 UTF-8 rune 计数）。
// 截断后加 "..." 确保不超过 33 个字符。
func convTitle(q string) string {
	const maxRunes = 30
	if !utf8.ValidString(q) {
		return q
	}
	runes := []rune(q)
	if len(runes) <= maxRunes {
		return q
	}
	return string(runes[:maxRunes]) + "..."
}
