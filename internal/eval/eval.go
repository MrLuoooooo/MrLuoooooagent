package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

// QueryLabel 一条标注数据：query → 应召回的文档 ID 列表。
type QueryLabel struct {
	Query     string   `json:"query"`
	DocIDs    []string `json:"doc_ids"`    // 应召回
	Expected  string   `json:"expected"`   // 期望回答应包含的关键信息
}

// RetrievalEvalResult 单条 query 的 retrieval 评估结果。
type RetrievalEvalResult struct {
	Query         string   `json:"query"`
	Retrieved     []string `json:"retrieved"`
	Expected      []string `json:"expected"`
	Precision     float64  `json:"precision"`
	Recall        float64  `json:"recall"`
	F1            float64  `json:"f1"`
}

// RetrievalMetrics 总体召回评估指标。
type RetrievalMetrics struct {
	AvgPrecision float64              `json:"avg_precision"`
	AvgRecall    float64              `json:"avg_recall"`
	AvgF1        float64              `json:"avg_f1"`
	PerQuery     []RetrievalEvalResult `json:"per_query"`
}

// AgentEvalResult Agent 端到端测试结果。
type AgentEvalResult struct {
	Input    string `json:"input"`
	Output   string `json:"output"`
	Expected string `json:"expected"`
	Passed   bool   `json:"passed"`
	Debug    string `json:"debug,omitempty"`
}

// AgentEvalResults 端到端测试结果集。
type AgentEvalResults struct {
	Passed  int               `json:"passed"`
	Failed  int               `json:"failed"`
	Total   int               `json:"total"`
	Results []AgentEvalResult `json:"results"`
}

// LoadTestData 加载标注测试集。
func LoadTestData(path string) ([]QueryLabel, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read test data: %w", err)
	}
	var labels []QueryLabel
	if err := json.Unmarshal(data, &labels); err != nil {
		return nil, fmt.Errorf("parse test data: %w", err)
	}
	return labels, nil
}

// CalculateRetrievalMetrics 计算 retrieval 评估指标。
// retriever 是 func(query) → 实际召回的文档 ID 列表。
func CalculateRetrievalMetrics(labels []QueryLabel, retriever func(query string) []string) *RetrievalMetrics {
	m := &RetrievalMetrics{PerQuery: make([]RetrievalEvalResult, 0, len(labels))}

	for _, l := range labels {
		retrieved := retriever(l.Query)
		expSet := toSet(l.DocIDs)
		retSet := toSet(retrieved)

		tp := intersection(expSet, retSet)
		precision := safeDiv(float64(tp), float64(len(retSet)))
		recall := safeDiv(float64(tp), float64(len(expSet)))
		f1 := safeDiv(2*precision*recall, precision+recall)

		m.PerQuery = append(m.PerQuery, RetrievalEvalResult{
			Query:     l.Query,
			Retrieved: retrieved,
			Expected:  l.DocIDs,
			Precision: precision,
			Recall:    recall,
			F1:        f1,
		})

		m.AvgPrecision += precision
		m.AvgRecall += recall
		m.AvgF1 += f1
	}

	n := float64(len(labels))
	if n > 0 {
		m.AvgPrecision /= n
		m.AvgRecall /= n
		m.AvgF1 /= n
	}
	return m
}

// RunAgentE2E 跑 Agent 端到端测试。
// agentRunner 是 func(input) → output。
func RunAgentE2E(t *testing.T, tests []QueryLabel, agentRunner func(input string) string) *AgentEvalResults {
	results := &AgentEvalResults{Total: len(tests)}
	for _, tc := range tests {
		output := agentRunner(tc.Query)
		passed := strings.Contains(strings.ToLower(output), strings.ToLower(tc.Expected))
		r := AgentEvalResult{
			Input:    tc.Query,
			Output:   output,
			Expected: tc.Expected,
			Passed:   passed,
		}
		if passed {
			results.Passed++
		} else {
			results.Failed++
		}
		results.Results = append(results.Results, r)
	}
	if results.Failed > 0 {
		t.Errorf("agent E2E: %d/%d passed, %d failed", results.Passed, results.Total, results.Failed)
	}
	return results
}

// PrintRetrievalReport 人类可读的 retrieval 报告。
func PrintRetrievalReport(m *RetrievalMetrics) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== 检索质量评估 ===\n"))
	sb.WriteString(fmt.Sprintf("平均 Precision: %.2f\n", m.AvgPrecision))
	sb.WriteString(fmt.Sprintf("平均 Recall:    %.2f\n", m.AvgRecall))
	sb.WriteString(fmt.Sprintf("平均 F1:        %.2f\n\n", m.AvgF1))

	for i, r := range m.PerQuery {
		sb.WriteString(fmt.Sprintf("Q%d: %s\n", i+1, truncateForReport(r.Query, 60)))
		sb.WriteString(fmt.Sprintf("  P=%.2f R=%.2f F1=%.2f\n", r.Precision, r.Recall, r.F1))
		sb.WriteString(fmt.Sprintf("  期望: %v\n", r.Expected))
		sb.WriteString(fmt.Sprintf("  召回: %v\n\n", r.Retrieved))
	}
	return sb.String()
}

// GenerateSampleTestData 生成示例测试数据。
func GenerateSampleTestData() []QueryLabel {
	return []QueryLabel{
		{
			Query:  "GoAgent 支持哪些文档格式",
			DocIDs: []string{"doc_1", "doc_2"},
			Expected: "pdf, docx, xlsx, pptx, txt",
		},
		{
			Query:  "如何配置模型提供商",
			DocIDs: []string{"doc_1"},
			Expected: "model_provider",
		},
		{
			Query:  "RAG 检索使用的向量库",
			DocIDs: []string{"doc_3"},
			Expected: "Elasticsearch",
		},
	}
}

// ---- 内部工具 ----

func toSet(ids []string) map[string]bool {
	s := make(map[string]bool, len(ids))
	for _, id := range ids {
		s[id] = true
	}
	return s
}

func intersection(a, b map[string]bool) int {
	count := 0
	// 优化：迭代较小的集合。
	if len(a) > len(b) {
		a, b = b, a
	}
	for k := range a {
		if b[k] {
			count++
		}
	}
	return count
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

func truncateForReport(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// SortByF1 按 F1 降序排（方便找 worst case）。
func (m *RetrievalMetrics) WorstCases(n int) []RetrievalEvalResult {
	sorted := make([]RetrievalEvalResult, len(m.PerQuery))
	copy(sorted, m.PerQuery)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].F1 < sorted[j].F1
	})
	if n > len(sorted) {
		n = len(sorted)
	}
	return sorted[:n]
}
