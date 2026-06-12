package service

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

var fbCounter atomic.Int64

func NewFeedbackID() string {
	return fmt.Sprintf("fb_%d_%d", time.Now().UnixNano(), fbCounter.Add(1))
}

// FeedbackStore 反馈持久化接口（consumer 定义）。
type FeedbackStore interface {
	Save(ctx context.Context, item *model.FeedbackItem) error
	List(ctx context.Context, conversationID string) ([]*model.FeedbackItem, error)
	ListRecent(ctx context.Context, limit int) ([]*model.FeedbackItem, error)
}

// FeedbackService 管理用户反馈和事实校验。
type FeedbackService struct {
	store  FeedbackStore
	logger *zap.Logger
}

// NewFeedbackService —
func NewFeedbackService(store FeedbackStore, logger *zap.Logger) *FeedbackService {
	return &FeedbackService{store: store, logger: logger}
}

// RecordFeedback 记录一条用户反馈。
func (s *FeedbackService) RecordFeedback(ctx context.Context, req *model.FeedbackRequest) (*model.FeedbackItem, error) {
	item := &model.FeedbackItem{
		ID:             NewFeedbackID(),
		ConversationID: req.ConversationID,
		MessageIndex:   req.MessageIndex,
		Type:           req.Type,
		Rating:         req.Rating,
		CorrectAnswer:  req.CorrectAnswer,
		Comment:        req.Comment,
		SourceQuery:    req.SourceQuery,
		SourceAnswer:   req.SourceAnswer,
		CreatedAt:      time.Now().UTC(),
	}
	if err := s.store.Save(ctx, item); err != nil {
		return nil, fmt.Errorf("save feedback: %w", err)
	}
	s.logger.Info("feedback recorded",
		zap.String("type", string(item.Type)),
		zap.String("conv_id", item.ConversationID),
	)
	return item, nil
}

// FactCheck 用工具结果和 LLM 输出做事实性校验。
// 返回：factual（是否一致）、conflict（冲突描述）。
func (s *FeedbackService) FactCheck(
	toolResults []*schema.Message, // tool 返回的消息
	llmOutput string, // LLM 生成的回答
) (factual bool, conflicts []string) {
	if len(toolResults) == 0 || llmOutput == "" {
		return true, nil // 无工具调用，无法校验，默认通过
	}

	for _, tr := range toolResults {
		if tr.Role != schema.Tool || tr.Content == "" {
			continue
		}
		// 简单启发式：工具结果的关键片段是否出现在 LLM 输出中
		snippets := extractKeySnippets(tr.Content, 80)
		for _, snip := range snippets {
			if !strings.Contains(strings.ToLower(llmOutput), strings.ToLower(snip)) {
				conflicts = append(conflicts,
					fmt.Sprintf("工具返回含 '%s'，但回答中未出现", snip))
			}
		}
	}

	return len(conflicts) == 0, conflicts
}

// CompareRAG 比较 RAG 检索结果与 LLM 生成内容的一致性。
func (s *FeedbackService) CompareRAG(
	sources []string, // 检索到的源文档内容
	generated string, // LLM 生成的内容
) (factual bool, conflicts []string) {
	for _, src := range sources {
		if !s.contentOverlaps(src, generated) {
			conflicts = append(conflicts,
				fmt.Sprintf("源文档含 '%s...'，但生成内容未提及", truncate(src, 60)))
		}
	}
	return len(conflicts) == 0, conflicts
}

// contentOverlaps 检查两个文本是否有实质性重叠（非 exact match，取关键短句）。
func (s *FeedbackService) contentOverlaps(src, gen string) bool {
	snippets := extractKeySnippets(src, 40)
	matchCount := 0
	for _, snip := range snippets {
		if strings.Contains(strings.ToLower(gen), strings.ToLower(snip)) {
			matchCount++
		}
	}
	// 至少一半的关键片段要出现在生成内容中。
	return matchCount >= len(snippets)/2
}

// ---- 内部方法 ----

// extractKeySnippets 从文本中提取关键短句（取前几段的核心内容）。
func extractKeySnippets(text string, maxLen int) []string {
	var snippets []string
	// 按句号/换行分段
	parts := strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == '\n' || r == '。' || r == '；' || r == ';'
	})
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) < 20 {
			continue // 跳过太短的
		}
		if len(p) > maxLen {
			p = p[:maxLen]
		}
		snippets = append(snippets, p)
		if len(snippets) >= 5 {
			break
		}
	}
	return snippets
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
