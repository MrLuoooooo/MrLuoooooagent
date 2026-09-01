package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

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
	if !s.enabled || s.store == nil {
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
	if !s.enabled || s.store == nil {
		return nil
	}
	mems, err := s.store.Search(ctx, userID, query, s.topK)
	if err != nil {
		s.logger.Error("retrieve memories failed", zap.Error(err))
		return nil
	}
	return mems
}

// maxMemoryInjectChars 注入 prompt 的记忆文本字符上限（P1 §4.3），超限丢弃低优先级并 WARN。
const maxMemoryInjectChars = 2048

// InjectIntoPrompt 分层检索记忆并拼成 prompt 文本块。
// 编排层：检索 + 调纯函数 + 丢弃告警；分层/老化/裁剪逻辑见 buildMemoryPrompt。
func (s *MemoryService) InjectIntoPrompt(ctx context.Context, userID, currentQuery string) string {
	mems := s.RetrieveRelevant(ctx, userID, currentQuery)
	if len(mems) == 0 {
		return ""
	}
	prompt, dropped := buildMemoryPrompt(mems, time.Now(), maxMemoryInjectChars)
	if dropped > 0 {
		s.logger.Warn("memory injection truncated by budget",
			zap.Int("retrieved", len(mems)),
			zap.Int("dropped", dropped),
			zap.Int("max_chars", maxMemoryInjectChars),
		)
	}
	return prompt
}

// memEntry 记忆渲染单元：line 为最终注入行，layer 用于分组与优先级排序。
type memEntry struct {
	line       string
	layer      string // "L1"/"L2"/"L3"
	confidence float64
	updatedAt  time.Time
}

var layerRank = map[string]int{"L1": 0, "L2": 1, "L3": 2}

// buildMemoryPrompt 纯函数：分层过滤 + 时间老化警告 + 体积上限裁剪。
// 超限时按"L1 > 置信度 > 新鲜度"优先级保留，返回 prompt 与被丢弃条数。
func buildMemoryPrompt(mems []store.MemoryMeta, now time.Time, maxChars int) (string, int) {
	if len(mems) == 0 {
		return "", 0
	}
	var entries []memEntry
	for _, m := range mems {
		line, layer, ok := memoryEntryLine(m, now)
		if !ok {
			continue
		}
		entries = append(entries, memEntry{line: line, layer: layer, confidence: m.Confidence, updatedAt: m.UpdatedAt})
	}
	if len(entries) == 0 {
		return "", 0
	}
	kept, dropped := selectWithinBudget(entries, maxChars)
	if len(kept) == 0 {
		return "", dropped
	}
	return renderMemoryPrompt(kept), dropped
}

// memoryEntryLine 单条记忆 → 注入行。返回 ok=false 表示该记忆被分层规则过滤。
func memoryEntryLine(m store.MemoryMeta, now time.Time) (line, layer string, ok bool) {
	switch m.MemoryLayer {
	case "L1":
		if m.Confidence < 0.9 {
			return "", "", false
		}
		layer = "L1"
	case "L2":
		// 时效过滤：invalid 直接不注入
		if isInvalid(m.ValidUntil) {
			return "", "", false
		}
		layer = "L2"
	case "L3":
		if m.Status != "active" {
			return "", "", false
		}
		layer = "L3"
	default:
		// 旧数据没有 memory_layer 字段，按 type 推断
		if m.Type == "preference" && m.Confidence >= 0.9 {
			layer = "L1"
		} else {
			layer = "L2"
			if isInvalid(m.ValidUntil) {
				return "", "", false
			}
		}
	}

	line = "- " + m.Content
	// L2 有效期标注（保留原语义）
	if layer == "L2" && !m.ValidUntil.IsZero() && now.After(m.ValidUntil) {
		line += " ⚠️数据可能已过期"
	}
	// 时间老化（对齐 CC memoryAge）：>1 天的记忆强制附加过期警告——
	// 带 file:line 引用的陈旧记忆会被模型当"权威事实"，引用反而让错误更可信。
	if age := now.Sub(m.UpdatedAt); age > 24*time.Hour {
		line += fmt.Sprintf(" ⚠️此记忆是%s写入的，记忆是时间点的观察而非实时状态，涉及代码行为或 file:line 引用的描述可能已过期，断言前请对照当前实际核实。",
			memoryAgeLabel(m.UpdatedAt, now))
	}
	return line, layer, true
}

// memoryAgeLabel 纯函数：记忆年龄 → "今天/昨天/N 天前"。
func memoryAgeLabel(t, now time.Time) string {
	d := now.Sub(t)
	if d < 0 {
		d = 0
	}
	days := int(d.Hours() / 24)
	switch {
	case days <= 0:
		return "今天"
	case days == 1:
		return "昨天"
	default:
		return fmt.Sprintf("%d 天前", days)
	}
}

// selectWithinBudget 按优先级（L1 > 置信度 > 新鲜度）贪心选取，字符总量不超 maxChars。
// 头部标题与分组标题约占 100 字符，预算预留后按行累计。
func selectWithinBudget(entries []memEntry, maxChars int) (kept []memEntry, dropped int) {
	budget := maxChars - 100
	if budget < 200 {
		budget = 200 // 保底：配置误配过小值时至少能注入少量记忆
	}
	sorted := make([]memEntry, len(entries))
	copy(sorted, entries)
	sort.Slice(sorted, func(i, j int) bool {
		ri, rj := layerRank[sorted[i].layer], layerRank[sorted[j].layer]
		if ri != rj {
			return ri < rj
		}
		if sorted[i].confidence != sorted[j].confidence {
			return sorted[i].confidence > sorted[j].confidence
		}
		return sorted[i].updatedAt.After(sorted[j].updatedAt)
	})
	acc := 0
	for _, e := range sorted {
		n := utf8.RuneCountInString(e.line) + 1 // 换行
		if acc+n > budget {
			dropped++
			continue // 该条放不下，但更小的低优先级条目仍有机会
		}
		acc += n
		kept = append(kept, e)
	}
	return kept, dropped
}

// renderMemoryPrompt 按分层分组渲染（L1 → L2 → L3），空层跳过。
func renderMemoryPrompt(kept []memEntry) string {
	var sb strings.Builder
	sb.WriteString("\n\n## 用户记忆\n")
	sections := []struct {
		layer string
		title string
	}{
		{"L1", "\n### 你的偏好与画像（长期稳定，请务必遵守）\n"},
		{"L2", "\n### 已知事实\n"},
		{"L3", "\n### 你之前表达的分析观点（主观推断，仅供参考）\n"},
	}
	for _, sec := range sections {
		var lines []string
		for _, e := range kept {
			if e.layer == sec.layer {
				lines = append(lines, e.line)
			}
		}
		if len(lines) == 0 {
			continue
		}
		sb.WriteString(sec.title)
		for _, l := range lines {
			sb.WriteString(l)
			sb.WriteString("\n")
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
