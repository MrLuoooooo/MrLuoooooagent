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
	Type        string   `json:"type"`
	Content     string   `json:"content"`
	Keywords    []string `json:"keywords"`
	Supersedes  bool     `json:"supersedes"`
	MemoryLayer string   `json:"memory_layer,omitempty"`
	Confidence  float64  `json:"confidence,omitempty"`
	ValidUntil  string   `json:"valid_until,omitempty"` // LLM 返回的日期字符串
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

		// 硬过滤：不靠 LLM，固定规则拦截
		extracted = validateExtracted(extracted)

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
				UserID:      userID,
				Type:        em.Type,
				Content:     em.Content,
				Keywords:    em.Keywords,
				Source:      conversationID,
				Status:      "active",
				Version:     1,
				MemoryLayer: resolveLayer(em),
				Confidence:  resolveConfidence(em),
				ValidUntil:  parseValidUntil(em.ValidUntil),
			}
			if err := s.store.Supersede(bgCtx, userID, old.ID, old.Version, newMeta); err != nil {
					s.logger.Error("supersede memory failed", zap.Error(err))
				}
				continue
			}

			meta := store.MemoryMeta{
				ID:          NewMemoryID(),
				UserID:      userID,
				Type:        em.Type,
				Content:     em.Content,
				Keywords:    em.Keywords,
				Source:      conversationID,
				Status:      "active",
				Version:     1,
				MemoryLayer: resolveLayer(em),
				Confidence:  resolveConfidence(em),
				ValidUntil:  parseValidUntil(em.ValidUntil),
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

// InjectIntoPrompt 分层检索记忆并拼成 prompt 文本块。
// L1(用户画像)全量注入常驻优先；L2(事实)按最新时效过滤；L3(分析)低优先级标注主观。
func (s *MemoryService) InjectIntoPrompt(ctx context.Context, userID, currentQuery string) string {
	mems := s.RetrieveRelevant(ctx, userID, currentQuery)
	if len(mems) == 0 {
		return ""
	}

	var l1, l2, l3 []store.MemoryMeta
	now := time.Now()
	for _, m := range mems {
		switch m.MemoryLayer {
		case "L1":
			if m.Confidence >= 0.9 {
				l1 = append(l1, m)
			}
		case "L2":
			// 时效三级过滤：fresh 直接用 / stale 标注 / invalid 跳过
			if isInvalid(m.ValidUntil) {
				continue // invalid 不注入
			}
			if !m.ValidUntil.IsZero() && now.After(m.ValidUntil) {
				m.Status = "stale"
			}
			l2 = append(l2, m)
		case "L3":
			if m.Status == "active" {
				l3 = append(l3, m)
			}
		default:
			// 旧数据没有 memory_layer 字段，按 type 推断
			if m.Type == "preference" && m.Confidence >= 0.9 {
				l1 = append(l1, m)
			} else {
				l2 = append(l2, m)
			}
		}
	}

	var sb strings.Builder
	sb.WriteString("\n\n## 用户记忆\n")

	// L1 画像：常驻最高优
	if len(l1) > 0 {
		sb.WriteString("\n### 你的偏好与画像（长期稳定，请务必遵守）\n")
		for _, m := range l1 {
			sb.WriteString(fmt.Sprintf("- %s\n", m.Content))
		}
	}

	// L2 事实：带时效标注
	if len(l2) > 0 {
		sb.WriteString("\n### 已知事实\n")
		for _, m := range l2 {
			staleMark := ""
			if m.Status == "stale" {
				staleMark = " ⚠️数据可能已过期"
			}
			sb.WriteString(fmt.Sprintf("- %s%s\n", m.Content, staleMark))
		}
	}

	// L3 分析观点：低优先级标注主观
	if len(l3) > 0 {
		sb.WriteString("\n### 你之前表达的分析观点（主观推断，仅供参考）\n")
		for _, m := range l3 {
			sb.WriteString(fmt.Sprintf("- %s\n", m.Content))
		}
	}

	return sb.String()
}

// ——— 辅助函数 ———

func resolveLayer(em extractedMemory) string {
	if em.MemoryLayer != "" {
		return em.MemoryLayer
	}
	// 向后兼容：根据 type 推断
	if em.Type == "preference" {
		return "L1"
	}
	return "L2"
}

func resolveConfidence(em extractedMemory) float64 {
	if em.Confidence > 0 {
		return em.Confidence
	}
	// 默认值
	if em.Type == "preference" {
		return 0.95
	}
	return 0.8
}

func parseValidUntil(s string) time.Time {
	if s == "" || s == "null" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, err = time.Parse("2006-01-02", s)
		if err != nil {
			return time.Time{}
		}
	}
	return t
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
- memory_layer: L1(用户画像/偏好) | L2(可验证事实/数据) | L3(分析推断/观点)
- confidence: 0到1的置信度，L1必须>0.9，L3可为0.6-0.8
- valid_until: 事实类记忆的有效期，如无明确数据则留null

只返回JSON数组，不要其他文字。示例：
[{"type":"preference","content":"用户偏好Go语言","keywords":["Go","语言偏好"],"supersedes":false,"memory_layer":"L1","confidence":0.95}]`,
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

// ——— 时效 + 验证 ———

// isInvalid 判断 valid_until 是否超过 3 倍窗口，超出的记忆应软删除不注入。
func isInvalid(validUntil time.Time) bool {
	if validUntil.IsZero() {
		return false // 未设过期 = 永不过期
	}
	maxWindow := time.Since(validUntil)
	// 默认 valid_until 窗口为 90 天，超过 3 倍 = 270 天即视为 invalid
	const defaultValidWindow = 90 * 24 * time.Hour
	return maxWindow > 3*defaultValidWindow
}

// validateExtracted 对 Extract 结果做硬过滤，不靠 LLM 保证质量。
// 返回通过验证的记忆。
func validateExtracted(extracted []extractedMemory) []extractedMemory {
	var result []extractedMemory
	for _, em := range extracted {
		// 规则 1: 含绝对化保证类词汇 → 丢弃
		if containsGuarantee(em.Content) {
			continue
		}
		// 规则 2: confidence < 0.5 → 丢弃
		if em.Confidence > 0 && em.Confidence < 0.5 {
			continue
		}
		// 规则 3: L1 推断（inferred）→ 降级为 L3，不直接落画像
		layer := resolveLayer(em)
		if layer == "L1" && em.Type != "preference" {
			em.MemoryLayer = "L3"
		}
		// 规则 4: L2 需要置信度 ≥ 0.7
		if layer == "L2" && resolveConfidence(em) < 0.7 {
			continue
		}
		result = append(result, em)
	}
	return result
}

var guaranteeWords = []string{
	"保证", "必然", "肯定", "稳赚", "必涨", "百分百",
	"稳赢", "一定赚", "绝对", "包赚", "无风险",
}

func containsGuarantee(content string) bool {
	for _, w := range guaranteeWords {
		if strings.Contains(content, w) {
			return true
		}
	}
	return false
}
