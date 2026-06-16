package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

// ConfidenceScore 0.0~1.0，越高越可信。
type ConfidenceScore float64

const (
	ConfidenceHigh   ConfidenceScore = 0.8
	ConfidenceMedium ConfidenceScore = 0.5
	ConfidenceLow    ConfidenceScore = 0.3
	ConfidenceNone   ConfidenceScore = 0.0
)

// ConfidenceLevel 人类可读级别。
type ConfidenceLevel string

const (
	LevelHigh   ConfidenceLevel = "high"
	LevelMedium ConfidenceLevel = "medium"
	LevelLow    ConfidenceLevel = "low"
)

// ConfidenceResult 单次评估结果。
type ConfidenceResult struct {
	Score      float64         `json:"score"`
	Level      ConfidenceLevel `json:"level"`
	Factors    []string        `json:"factors"`
	Warnings   []string        `json:"warnings"`
	NeedVerify bool            `json:"need_verify"`
}

// ConfidenceService 基于启发式的置信度评估。
// 没有模型内部状态（logprobs/hidden states），用可观测信号估算。
type ConfidenceService struct {
	logger *zap.Logger
}

// NewConfidenceService —
func NewConfidenceService(logger *zap.Logger) *ConfidenceService {
	return &ConfidenceService{logger: logger}
}

// Evaluate 评估 LLM 输出的置信度。
func (s *ConfidenceService) Evaluate(
	ctx context.Context,
	llmOutput string,
	toolResults []*schema.Message,
	sourceDocs []string,
) *ConfidenceResult {
	result := &ConfidenceResult{
		Score:   1.0, // 起始满分，逐步扣减
		Factors: make([]string, 0),
	}

	// 因子 1：不确定性语言标记。
	uncertaintyPenalty, markers := s.detectUncertainty(llmOutput)
	if uncertaintyPenalty > 0 {
		result.Score -= uncertaintyPenalty
		result.Warnings = append(result.Warnings, markers...)
		result.Factors = append(result.Factors,
			fmt.Sprintf("检测到不确定性语言，扣 %.2f", uncertaintyPenalty))
	}

	// 因子 2：工具结果对齐。
	if len(toolResults) > 0 {
		alignmentScore := s.toolAlignment(llmOutput, toolResults)
		if alignmentScore < 1.0 {
			penalty := (1.0 - alignmentScore) * 0.4
			result.Score -= penalty
			result.Factors = append(result.Factors,
				fmt.Sprintf("工具结果对齐度 %.2f，扣 %.2f", alignmentScore, penalty))
			if alignmentScore < 0.5 {
				result.Warnings = append(result.Warnings, "LLM 回答与工具返回结果显著不一致")
			}
		}
	} else {
		// 没有工具结果验证，输出纯基于模型知识，扣分。
		result.Score -= 0.15
		result.Factors = append(result.Factors, "无工具验证，依赖模型知识，扣 0.15")
	}

	// 因子 3：源文档对齐。
	if len(sourceDocs) > 0 {
		docScore := s.sourceAlignment(llmOutput, sourceDocs)
		if docScore < 1.0 {
			penalty := (1.0 - docScore) * 0.3
			result.Score -= penalty
			result.Factors = append(result.Factors,
				fmt.Sprintf("源文档对齐度 %.2f，扣 %.2f", docScore, penalty))
		}
	}

	// 因子 4：输出质量（长度、结构化、自我矛盾）。
	qualityPenalty := s.evaluateOutputQuality(llmOutput)
	if qualityPenalty > 0 {
		result.Score -= qualityPenalty
		result.Factors = append(result.Factors,
			fmt.Sprintf("输出质量问题，扣 %.2f", qualityPenalty))
	}

	// 归一化，确保不破底。
	if result.Score < 0.0 {
		result.Score = 0.0
	}
	if result.Score > 1.0 {
		result.Score = 1.0
	}

	// 判定级别。
	switch {
	case result.Score >= float64(ConfidenceHigh):
		result.Level = LevelHigh
	case result.Score >= float64(ConfidenceMedium):
		result.Level = LevelMedium
	default:
		result.Level = LevelLow
	}

	result.NeedVerify = result.Score < float64(ConfidenceMedium)
	return result
}

// SelfConsistency 多次采样一致性评估（近似语义熵）。
// 对同一 query 运行 N 次采样，比较输出多样性。
func (s *ConfidenceService) SelfConsistency(
	runSingle func(ctx context.Context) (string, error),
	n int,
) (float64, error) {
	if n < 1 {
		n = 3
	}

	results := make([]string, n)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			res, err := runSingle(context.Background())
			mu.Lock()
			if err != nil && firstErr == nil {
				firstErr = err
			}
			results[idx] = res
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if firstErr != nil {
		return 0, firstErr
	}

	// Jaccard 相似度的平均值 = 一致性分数。
	var totalSim float64
	var count int
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			totalSim += s.jaccardSimilarity(results[i], results[j])
			count++
		}
	}

	if count == 0 {
		return 1.0, nil
	}
	return totalSim / float64(count), nil
}

// InjectConfidenceNote 生成带置信度标记的回复前缀。
func (s *ConfidenceService) InjectConfidenceNote(result *ConfidenceResult) string {
	if result.Score >= float64(ConfidenceHigh) {
		return "" // 不打断用户
	}
	if result.Level == LevelMedium {
		return fmt.Sprintf("[置信度：%.0f%%，仅供参考] ", result.Score*100)
	}
	return fmt.Sprintf("[低置信度：%.0f%%，建议验证：%s] ",
		result.Score*100, strings.Join(result.Warnings, "；"))
}

// ---- 启发式检测 ----

// detectUncertainty 检测不确定语言并返回扣分值。
func (s *ConfidenceService) detectUncertainty(output string) (float64, []string) {
	markers := []string{
		"可能", "也许", "大概", "应该", "或许",
		"我不确定", "不太确定", "推测", "估计",
		"probably", "maybe", "perhaps", "likely",
		"I think", "I believe", "不确定",
	}

	lower := strings.ToLower(output)
	var found []string
	for _, m := range markers {
		if strings.Contains(lower, strings.ToLower(m)) {
			found = append(found, m)
		}
	}

	if len(found) > 0 {
		penalty := math.Min(float64(len(found))*0.08, 0.3)
		return penalty, found
	}
	return 0, nil
}

// toolAlignment 检查 LLM 输出与工具结果的对齐程度。
// 股票工具结果额外做数值精度校验（价格、涨跌幅等）。
func (s *ConfidenceService) toolAlignment(output string, toolResults []*schema.Message) float64 {
	if len(toolResults) == 0 {
		return 1.0
	}

	// 通用文本片段对齐。
	totalSnippets := 0
	matchedSnippets := 0

	for _, tr := range toolResults {
		snippets := extractKeySnippets(tr.Content, 40)
		for _, snip := range snippets {
			totalSnippets++
			if strings.Contains(strings.ToLower(output), strings.ToLower(snip)) {
				matchedSnippets++
			}
		}

		// 股票专项：提取关键数值，做精确匹配。
		// 每个数值权重 ×3，因为股票数据中数值准确性最关键。
		if s.isStockToolResult(tr.Content) {
			stockNums := s.extractStockNumbers(tr.Content)
			for _, sn := range stockNums {
				totalSnippets++
				if strings.Contains(output, sn) {
					matchedSnippets += 3 // 股票数值匹配权重更高
				}
			}
		}
	}

	if totalSnippets == 0 {
		return 1.0
	}
	return safeDiv(float64(matchedSnippets), float64(totalSnippets))
}

// isStockToolResult 判断工具结果是否包含股票行情数据。
func (s *ConfidenceService) isStockToolResult(content string) bool {
	return strings.Contains(content, "实时行情") ||
		strings.Contains(content, "K线数据")
}

// extractStockNumbers 从股票工具结果中提取关键数值（价格、涨跌幅等）。
func (s *ConfidenceService) extractStockNumbers(content string) []string {
	var nums []string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		// 现价: ¥1255.67
		if idx := strings.Index(line, "现价"); idx >= 0 {
			if n := extractPrice(line); n != "" {
				nums = append(nums, n)
			}
		}
		// 涨跌: ↓-15.43 (-1.21%)
		if strings.Contains(line, "涨跌") || strings.Contains(line, "区间涨跌") {
			nums = append(nums, strings.TrimSpace(line))
		}
	}
	return nums
}

// extractPrice 从"现价: ¥1255.67"中提取"1255.67"。
func extractPrice(line string) string {
	start := strings.Index(line, "¥")
	if start < 0 {
		return ""
	}
	rest := line[start+len("¥"):]
	// 取到空格或行尾。
	end := strings.IndexAny(rest, " \t\n|")
	if end < 0 {
		end = len(rest)
	}
	price := strings.TrimSpace(rest[:end])
	// 精度归一：模型可能省去末尾0，API 可能有 .00
	// 统一去掉末尾无意义的0
	price = strings.TrimRight(price, "0")
	price = strings.TrimRight(price, ".")
	return price
}

// sourceAlignment 检查 LLM 输出与源文档的对齐程度。
func (s *ConfidenceService) sourceAlignment(output string, sources []string) float64 {
	totalSources := 0
	matchedSources := 0

	for _, src := range sources {
		snippets := extractKeySnippets(src, 30)
		matchCount := 0
		for _, snip := range snippets {
			if strings.Contains(strings.ToLower(output), strings.ToLower(snip)) {
				matchCount++
			}
		}
		totalSources++
		if len(snippets) > 0 && float64(matchCount)/float64(len(snippets)) >= 0.3 {
			matchedSources++
		}
	}

	if totalSources == 0 {
		return 1.0
	}
	return safeDiv(float64(matchedSources), float64(totalSources))
}

// evaluateOutputQuality 输出质量评估（长度、结构、自我矛盾）。
func (s *ConfidenceService) evaluateOutputQuality(output string) float64 {
	var penalty float64

	// 极短输出（无实质内容）。
	if utf8.RuneCountInString(strings.TrimSpace(output)) < 10 {
		penalty += 0.2
	}

	// 纯道歉/拒绝回答模式。
	refusalPatterns := []string{"无法", "不能", "抱歉", "对不起", "I cannot", "I'm unable"}
	lower := strings.ToLower(output)
	for _, p := range refusalPatterns {
		if strings.Contains(lower, strings.ToLower(p)) {
			penalty += 0.1
			break
		}
	}

	return math.Min(penalty, 0.3)
}

// jaccardSimilarity Jaccard 词级相似度。
func (s *ConfidenceService) jaccardSimilarity(a, b string) float64 {
	wordsA := strings.Fields(a)
	wordsB := strings.Fields(b)

	setA := make(map[string]bool, len(wordsA))
	for _, w := range wordsA {
		setA[strings.ToLower(w)] = true
	}
	setB := make(map[string]bool, len(wordsB))
	for _, w := range wordsB {
		setB[strings.ToLower(w)] = true
	}

	inter := 0
	for w := range setA {
		if setB[w] {
			inter++
		}
	}

	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 1.0
	}
	return float64(inter) / float64(union)
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
