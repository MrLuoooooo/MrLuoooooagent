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

	"github.com/MrLuoooooo/MrLuoooooagent/internal/callback"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/modelmanager"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/graph"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	cozeloopobs "github.com/MrLuoooooo/MrLuoooooagent/internal/observability/cozeloop"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
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
	tracer  *cozeloopobs.Tracer // 扣子罗盘 trace 上报，nil = 未启用
	mm      *modelmanager.ModelManager
	logger  *zap.Logger
}

// NewChatHandler —
func NewChatHandler(svc *service.ChatService, convSvc *service.ConversationService, tracer *cozeloopobs.Tracer, mm *modelmanager.ModelManager, logger *zap.Logger) *ChatHandler {
	return &ChatHandler{svc: svc, convSvc: convSvc, tracer: tracer, mm: mm, logger: logger}
}

// modelName 审计用：本次 agent 流程实际使用的模型名。
func (h *ChatHandler) modelName() string {
	if h.mm == nil {
		return ""
	}
	return h.mm.CurrentName()
}

// streamOrInvoke 审计用：区分流式/非流式。
func streamOrInvoke(req model.ChatRequest) string {
	if req.Stream {
		return "stream"
	}
	return "invoke"
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
	} else {
		// 客户端自带会话 ID（如股票页的 stock_<code> 固定会话）：
		// 先校验格式防注入，再保证会话元数据存在——否则消息落库但会话
		// 列表看不到、刷新后无法回看（此前股票对话丢记录的根因）。
		if !convIDPattern.MatchString(convID) {
			c.JSON(http.StatusBadRequest, model.Err(400, "非法会话 ID"))
			return
		}
		title := "历史会话"
		if code, ok := strings.CutPrefix(convID, "stock_"); ok {
			title = "股票对话 " + code
		}
		if err := h.convSvc.Ensure(ctx, convID, title); err != nil {
			h.logger.Warn("ensure conversation", zap.String("conv_id", convID), zap.Error(err))
		}
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
		// 短期记忆压缩：service 层按模型 token 预算裁剪 + 同步结构化摘要，
		// 绝不返回占位符。handler 不保留任何裁剪算法（文档 §3.1 分层要求）。
		history = h.svc.TrimHistory(convID, history)
		if len(history) > 0 {
			req.Question = prependHistory(req.Question, history)
		}
	}

	if req.Agent {
		rid := c.GetString("request_id")
		h.handleAgent(c, req, convID, originalQuestion, rid)
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

func (h *ChatHandler) handleAgent(c *gin.Context, req model.ChatRequest, convID string, originalQuestion string, requestID string) {
	ctx := c.Request.Context()
	// 审计锚点：本次 agent 流程的模型/prompt 版本随日志落盘，
	// 与 CozeLoop trace 的 baggage request_id 双向可查。
	h.logger.Info("agent run start",
		zap.String("request_id", requestID),
		zap.String("conv_id", convID),
		zap.String("model", h.modelName()),
		zap.String("prompt_version", graph.PromptVersion),
		zap.String("mode", streamOrInvoke(req)),
	)
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

		// 工具调用收集器：eino branch 语义下工具消息不出流（tool_call/result 事件同理），
		// 来源提取改走 per-request callback——bag 在图执行期间被填充，流结束后读取。
		toolBag := &callback.ToolResultsBag{}
		cb := callback.NewToolCollector(toolBag)

		opts := []compose.Option{compose.WithCallbacks(cb)}
		// 扣子罗盘 trace：注入后 cozeloop handler 会为本次 agent 图执行
		// 建根 span，chat_model / tools / lambda 各节点一个子 span，
		// loop.coze.cn 的 Trace 页可回放完整流程。
		if h.tracer != nil {
			opts = append(opts, compose.WithCallbacks(h.tracer.Handler()))
		}

		// 扣子罗盘 root span：baggage 携带 request_id，图内所有节点 span 挂到
		// 它下面——一次请求一棵 trace 树，罗盘与 zap 日志经 request_id 互查。
		traceCtx, finishTrace := h.tracer.StartRequest(ctx, requestID)

		// 产品级排队：永远不拒绝，入队即返回排队状态
		qReq := &service.QueuedRequest{
			ConvID:   convID,
			UserMsg:  userMsg,
			Question: req.Question,
			Priority: prio,
			Ctx:      traceCtx,
			Opts:     opts,
		}
		qr := h.svc.QueueSubmit(qReq)
		defer finishTrace()

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
		phasePushed := make(map[string]bool)
		toolRounds := 0 // 工具轮水位：跨整个请求生命周期，不随单个流对象重置

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
					// 流中断不再静默：非 EOF 的错误（超时/上游断流）推给前端，
					// 否则客户端把中断当正常结束，气泡停在半截话上。
					if recvErr != io.EOF {
						h.writeSSEEvent(c.Writer, model.StreamEvent{Type: model.EventError, Content: recvErr.Error()})
					}
					break
				}
				// Phase: 首次收到 token → 进入推理阶段
				if !phasePushed["reasoning"] && chunk.Content != "" {
					phasePushed["reasoning"] = true
					h.writeSSEEvent(c.Writer, model.StreamEvent{Type: model.EventPhase, Content: "【推理中】正在生成回答..."})
				}
				// 工具轮边界检测：parse_tool_calls 会剥掉流式 chunk 的 ToolCalls
				// （聚合消息走 branch 进 tools 节点，不出流），所以 chunk.ToolCalls
				// 恒空、tool_call SSE 事件在此图拓扑下不可达。bag 由工具 callback
				// 并发填充，其增长即"上一轮工具已执行、即将进入新一轮生成"——
				// 此前累积的文本是中间播报：通知前端清空气泡，落库只存最终轮
				// （此前两轮全文拼接落库，气泡重复的根因）。
				if n := len(toolBag.Records); n > toolRounds {
					toolRounds = n
					if !phasePushed["executing"] {
						phasePushed["executing"] = true
						h.writeSSEEvent(c.Writer, model.StreamEvent{Type: model.EventPhase, Content: "【执行中】正在调用工具获取数据..."})
					}
					if full != "" {
						h.writeSSEEvent(c.Writer, model.StreamEvent{Type: model.EventClear})
						full = ""
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
				h.writeSSEEvent(c.Writer, model.StreamEvent{Type: model.EventToken, Content: stripToolCode(chunk.Content)})
			}
			result.Stream.Close()
		}

		if full != "" {
			// 工具调用回填落库（audit 复盘需要"解析后的工具调用"）：
			// bag 只在 OnStart 记录 ID/Name/Args（Result 另行回填），
			// 转 schema.ToolCall 后走既有 tool_calls 列（MySQL JSON/ES object）。
			var savedCalls []schema.ToolCall
			for _, rec := range toolBag.Records {
				if rec.ToolName == "" {
					continue
				}
				savedCalls = append(savedCalls, schema.ToolCall{
					ID:       rec.ToolCallID,
					Function: schema.FunctionCall{Name: rec.ToolName, Arguments: rec.Args},
				})
			}
			if err := h.svc.SaveAssistantMessage(convID, full, savedCalls); err != nil {
				h.logger.Error("save agent assistant reply", zap.String("conv_id", convID), zap.Error(err))
			}
		}

		// 来源引用：流结束后从收集器读全部工具记录，提取/去重/封顶
		// 收口在 service 纯函数 CollectSources，done 前发一次供前端渲染。
		sources := service.CollectSources(toolBag.Records)
		for _, rec := range toolBag.Records {
			args := truncateStr(service.MaskSensitive(rec.Args), 200)
			// 审计要求工具返回摘要落日志：截断 300 字符 + 脱敏
			result := truncateStr(service.MaskSensitive(rec.Result), 300)
			h.logger.Info("agent tool record",
				zap.String("conv_id", convID),
				zap.String("request_id", requestID),
				zap.String("tool", rec.ToolName),
				zap.String("args", args),
				zap.Int("result_len", len(rec.Result)),
				zap.String("result", result),
			)
		}
		if len(sources) > 0 {
			sseSources := make([]model.SourceRef, len(sources))
			for i, s := range sources {
				sseSources[i] = model.SourceRef{Title: s.Title, URL: s.URL, Kind: s.Kind}
			}
			h.writeSSEEvent(c.Writer, model.StreamEvent{Type: model.EventSources, Sources: sseSources})
		}

		// Agent 完成：清理 Eino 的 checkpoint（断点清理编排归 service）
		h.svc.CleanupCheckpoint(ctx, convID)

		// 与 streamSSE 对齐：收尾显式发 conversation_id + done，
		// 前端不必依赖"连接关闭"这种隐式结束信号（error 路径除外）。
		h.writeSSEEvent(c.Writer, model.StreamEvent{Type: model.EventConversationID, Content: convID})
		h.writeSSEEvent(c.Writer, model.StreamEvent{Type: model.EventDone})
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

	// Agent 完成：清理 Eino 的 checkpoint（断点清理编排归 service）
	h.svc.CleanupCheckpoint(ctx, convID)

	c.JSON(http.StatusOK, model.OK(model.ChatResponseData{
		Content: msg.Content, Role: string(msg.Role),
	}))
}

// convIDPattern 客户端自带会话 ID 的白名单：字母/数字/下划线/连字符，1-64 位。
// 防止任意字符串作为 ES 文档 ID 注入（如 stock_sh600519 合法，路径拼接类非法）。
var convIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

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
	// 总量裁剪由 service.TrimHistory 按 token 预算统一负责，这里不再按条数截断；
	// 单条 2000 字符截断保留——防单条工具结果把 prompt 撑爆的渲染兜底。
	const maxContentLen = 2000

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

// 同时剥 <tool_code> 与 <tool_call> 两种标签块：
// 前者是模型被提示用的输出格式，后者是 XML 兜底解析器（graph.parsePromptToolCalls）的输入格式。
// parse_tool_calls 改为流式透传后，graph 层不再剥块，显示过滤统一收敛到 handler。
var toolCodeRe = regexp.MustCompile(`(?s)<tool_call>.*?</tool_call>|<tool_code>.*?</tool_code>`)

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
