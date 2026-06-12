package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/store"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

var memCounter atomic.Int64

// NewMemoryID 生成唯一记忆 ID。
func NewMemoryID() string {
	return fmt.Sprintf("mem_%d_%d", time.Now().UnixNano(), memCounter.Add(1))
}

// MemoryStore service 层定义的持久化接口（consumer 定义，provider 实现）。
type MemoryStore interface {
	Index(ctx context.Context, meta store.MemoryMeta) error
	Search(ctx context.Context, userID, query string, topK int) ([]store.MemoryMeta, error)
	Supersede(ctx context.Context, userID, oldID string, oldVersion int, newMeta store.MemoryMeta) error
	FindByKeyword(ctx context.Context, userID, keyword string) ([]store.MemoryMeta, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, userID string) ([]store.MemoryMeta, error)
	DeleteAll(ctx context.Context, userID string) error
}

// ModelCaller LLM 最小接口——只暴露 Generate，不绑定工具。
type ModelCaller interface {
	Generate(ctx context.Context, input []*schema.Message, opts ...any) (*schema.Message, error)
}

// extractedMemory LLM 返回的 JSON 数组元素。
type extractedMemory struct {
	Type       string   `json:"type"`
	Content    string   `json:"content"`
	Keywords   []string `json:"keywords"`
	Supersedes bool     `json:"supersedes"`
}

// MemoryService 记忆的提取、检索和 prompt 注入。
type MemoryService struct {
	store   MemoryStore
	logger  *zap.Logger
	model   ModelCaller
	topK    int
	enabled bool
}

// NewMemoryService —
func NewMemoryService(store MemoryStore, logger *zap.Logger, model ModelCaller, topK int, enabled bool) *MemoryService {
	return &MemoryService{store: store, logger: logger, model: model, topK: topK, enabled: enabled}
}

// ExtractAndStore 从对话消息中异步提取记忆写入存储。
func (s *MemoryService) ExtractAndStore(ctx context.Context, userID, conversationID string, messages []*schema.Message) {
	if !s.enabled {
		return
	}
	// 拷贝切片，防止 goroutine 内原始 slice 被修改。
	msgs := make([]*schema.Message, len(messages))
	copy(msgs, messages)

	go func() {
		bgCtx := context.Background()
		extracted, err := s.extractMeanings(bgCtx, msgs)
		if err != nil {
			s.logger.Error("extract memories failed", zap.Error(err))
			return
		}

		for _, em := range extracted {
			if strings.TrimSpace(em.Content) == "" {
				continue
			}
			if !s.validType(em.Type) {
				em.Type = "knowledge"
			}
			if len(em.Keywords) == 0 {
				em.Keywords = []string{em.Type}
			}

			// 去重：跨所有关键词搜索已有记忆，不只看第一个。
			var bestConflict *store.MemoryMeta
			for _, kw := range em.Keywords {
				if c, _ := s.store.FindByKeyword(bgCtx, userID, kw); len(c) > 0 {
					bestConflict = &c[0]
					break
				}
			}
			if bestConflict != nil && em.Supersedes {
				old := *bestConflict
				newMeta := store.MemoryMeta{
					ID:       NewMemoryID(),
					UserID:   userID,
					Type:     em.Type,
					Content:  em.Content,
					Keywords: em.Keywords,
					Source:   conversationID,
					Status:   "active",
					Version:  1,
				}
				if err := s.store.Supersede(bgCtx, userID, old.ID, old.Version, newMeta); err != nil {
					s.logger.Error("supersede memory failed", zap.Error(err))
				}
				continue
			}

			meta := store.MemoryMeta{
				ID:       NewMemoryID(),
				UserID:   userID,
				Type:     em.Type,
				Content:  em.Content,
				Keywords: em.Keywords,
				Source:   conversationID,
				Status:   "active",
				Version:  1,
			}
			if err := s.store.Index(bgCtx, meta); err != nil {
				s.logger.Error("index memory failed", zap.Error(err))
			}
		}
		s.logger.Info("memory extraction done", zap.String("conv_id", conversationID), zap.Int("extracted", len(extracted)))
	}()
}

// RetrieveRelevant 根据当前查询检索最相关的记忆。
func (s *MemoryService) RetrieveRelevant(ctx context.Context, userID, query string) []store.MemoryMeta {
	if !s.enabled {
		return nil
	}
	mems, err := s.store.Search(ctx, userID, query, s.topK)
	if err != nil {
		s.logger.Error("retrieve memories failed", zap.Error(err))
		return nil
	}
	return mems
}

// InjectIntoPrompt 把检索到的记忆拼成 prompt 文本块。
func (s *MemoryService) InjectIntoPrompt(ctx context.Context, userID, currentQuery string) string {
	mems := s.RetrieveRelevant(ctx, userID, currentQuery)
	if len(mems) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n## 用户记忆\n")
	sb.WriteString("以下是关于该用户的已知信息，请在回答时参考：\n\n")
	for _, m := range mems {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", s.typeLabel(m.Type), m.Content))
	}
	return sb.String()
}

// ---- 内部方法 ----

func (s *MemoryService) extractMeanings(ctx context.Context, messages []*schema.Message) ([]extractedMemory, error) {
	convText := s.messagesToText(messages)

	sysMsg := &schema.Message{
		Role: schema.System,
		Content: `你是记忆提取器。分析对话，提取关于用户的重要信息。

规则：
- type: "fact" | "preference" | "decision" | "knowledge"
- content: 一句完整描述
- keywords: 3-5个关键词
- supersedes: 如果会覆盖之前表述则为true

只返回JSON数组，不要其他文字。示例：
[{"type":"preference","content":"用户偏好Go语言","keywords":["Go","语言偏好"],"supersedes":false}]`,
	}

	userMsg := &schema.Message{Role: schema.User, Content: "对话：\n" + convText}
	resp, err := s.model.Generate(ctx, []*schema.Message{sysMsg, userMsg})
	if err != nil {
		return nil, fmt.Errorf("model generate: %w", err)
	}

	jsonStr := strings.TrimSpace(resp.Content)
	jsonStr = strings.TrimPrefix(jsonStr, "```json")
	jsonStr = strings.TrimPrefix(jsonStr, "```")
	jsonStr = strings.TrimSuffix(jsonStr, "```")
	jsonStr = strings.TrimSpace(jsonStr)

	var extracted []extractedMemory
	if err := json.Unmarshal([]byte(jsonStr), &extracted); err != nil {
		s.logger.Warn("extract memory: json parse failed, fallback", zap.Error(err))
		if jsonStr != "" {
			extracted = []extractedMemory{{Type: "knowledge", Content: jsonStr}}
		}
	}
	return extracted, nil
}

func (s *MemoryService) messagesToText(messages []*schema.Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		role := string(msg.Role)
		content := msg.Content
		if msg.Role == schema.Tool && len(content) > 200 {
			content = content[:200] + "..."
		}
		if content == "" && len(msg.ToolCalls) > 0 {
			names := make([]string, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				names[i] = tc.Function.Name
			}
			content = "调用工具: " + strings.Join(names, ", ")
		}
		sb.WriteString(fmt.Sprintf("[%s] %s\n", role, content))
	}
	return sb.String()
}

func (s *MemoryService) validType(t string) bool {
	switch t {
	case "fact", "preference", "decision", "knowledge":
		return true
	}
	return false
}

func (s *MemoryService) typeLabel(t string) string {
	switch t {
	case "fact":
		return "事实"
	case "preference":
		return "偏好"
	case "decision":
		return "决策"
	case "knowledge":
		return "知识"
	default:
		return "其他"
	}
}
