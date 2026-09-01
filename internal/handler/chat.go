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

	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/modelmanager"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/graph"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/store"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gin-gonic/gin"
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
		const maxKeep = 30
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
	}
	// 用 Eino 内置 checkpoint：传入会话 ID 自动管理断点保存和恢复
	cpOpt := compose.WithCheckPointID(convID)

	if req.Stream {
		if err := h.svc.SaveUserMessage(convID, originalQuestion); err != nil {
			h.logger.Error("save user message before agent stream", zap.String("conv_id", convID), zap.Error(err))
		}

		userMsg := &schema.Message{Role: schema.User, Content: req.Question}
		prio := 2 // 默认普通优先级
		if req.StockMode {
			prio = 1
		}

		// 产品级排队：永远不拒绝，入队即返回排队状态
		qReq := &service.QueuedRequest{
			ConvID:   convID,
			UserMsg:  userMsg,
			Question: req.Question,
			Priority: prio,
			Ctx:      ctx,
		}
		qr := h.svc.QueueSubmit(qReq)

		h.setupSSE(c)

		// 排队状态 → 推前端等待提示。Position 由队列保证不含自己，单人时为 0。
		if qr.NodeID == "queued" {
			if qr.Position > 0 {
				h.writeSSEEvent(c.Writer, model.StreamEvent{
					Type:    "waiting",
					Content: fmt.Sprintf("前面还有 %d 人，预计等待约 %d 秒", qr.Position, qr.EtaSeconds),
				})
			} else {
				h.writeSSEEvent(c.Writer, model.StreamEvent{Type: "waiting", Content: "正在处理中..."})
			}
		}
		if qr.NodeID == "coalesced" {
			h.writeSSEEvent(c.Writer, model.StreamEvent{Type: "waiting", Content: "类似问题正在处理..."})
		}

		// qReq.ResultCh 由 Submit 保证非 nil
		var full string
		var toolCalls []schema.ToolCall
		seenTools := make(map[string]bool)
		phasePushed := make(map[string]bool)

		// Phase: 开始分析
		h.writeSSEEvent(c.Writer, model.StreamEvent{Type: model.EventPhase, Content: "【准备中】正在分析问题..."})
		phasePushed["prepare"] = true

		// 从队列结果通道读取，可能包含 position 更新、token 和最终结果
		for result := range qReq.ResultCh {
			if result.NodeID == "position" {
				// 队列位置更新：Position 为排在前面的真实人数
				h.writeSSEEvent(c.Writer, model.StreamEvent{
					Type:    "waiting",
					Content: fmt.Sprintf("前面还有 %d 人，预计等待约 %d 秒", result.Position, result.EtaSeconds),
				})
				continue
			}
			if result.NodeID == "error" {
				h.writeSSEEvent(c.Writer, model.StreamEvent{Type: model.EventError, Content: result.Err.Error()})
				return
			}
			if result.Stream == nil {
				continue
			}

			// 读取流式 token
			for {
				chunk, recvErr := result.Stream.Recv()
				if recvErr != nil {
					break
				}
				// Phase: 首次收到 token → 进入推理阶段
				if !phasePushed["reasoning"] && chunk.Content != "" {
					phasePushed["reasoning"] = true
					h.writeSSEEvent(c.Writer, model.StreamEvent{Type: model.EventPhase, Content: "【推理中】正在生成回答..."})
				}
				for _, tc := range chunk.ToolCalls {
					if !seenTools[tc.ID] {
						// Phase: 工具调用 → 执行阶段
						if !phasePushed["executing"] {
							phasePushed["executing"] = true
							h.writeSSEEvent(c.Writer, model.StreamEvent{Type: model.EventPhase, Content: "【执行中】正在调用工具获取数据..."})
						}
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
					h.writeSSEEvent(c.Writer, model.StreamEvent{
						Type:    model.EventToolResult,
						Content: toWindowsPath(chunk.Content),
					})
					continue
				}
				full += chunk.Content
				if len(chunk.ToolCalls) > 0 {
					toolCalls = chunk.ToolCalls
				}
				h.writeSSEEvent(c.Writer, model.StreamEvent{Type: model.EventToken, Content: stripToolCode(chunk.Content)})
			}
			result.Stream.Close()
		}

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

// writeSSEEvent 写一条 SSE 事件并立即 Flush。
// Flush 是逐 token 推送的关键：gin 的 ResponseWriter 满足 http.Flusher，
// Agent 路径不经过 c.Stream，漏掉 Flush 会被 Go http 缓冲攒到 4KB 才发出，
// 前端表现就是"一瞬间全量出现"。
func (h *ChatHandler) writeSSEEvent(w io.Writer, evt model.StreamEvent) {
	data, err := json.Marshal(evt)
	if err != nil {
		h.logger.Error("marshal sse event", zap.String("type", evt.Type), zap.Error(err))
		return
	}
	if _, err := w.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
		h.logger.Warn("write sse event", zap.String("type", evt.Type), zap.Error(err))
		return
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
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
