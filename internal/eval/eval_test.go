package eval

import (
	"testing"
)

// TestRetrievalEval 跑标注测试集上的 retrieval 评估。
// 需要实际的 retriever 实例——这是一个模板，实际使用时注入你的 retriever。
func TestRetrievalEval(t *testing.T) {
	labels, err := LoadTestData("testdata/retrieval_labels.json")
	if err != nil {
		t.Skipf("skip retrieval eval: %v", err)
	}

	// 替换为你的实际 retriever。
	stubRetriever := func(query string) []string {
		return []string{"doc_1"} // stub
	}

	metrics := CalculateRetrievalMetrics(labels, stubRetriever)
	report := PrintRetrievalReport(metrics)
	t.Log(report)

	if metrics.AvgRecall < 0.5 {
		t.Errorf("recall too low: %.2f, expected >= 0.5", metrics.AvgRecall)
	}
}

// TestAgentE2E 跑 Agent 端到端测试。
func TestAgentE2E(t *testing.T) {
	labels, err := LoadTestData("testdata/retrieval_labels.json")
	if err != nil {
		t.Skipf("skip agent E2E: %v", err)
	}

	// 替换为你的实际 agent runner。
	stubRunner := func(input string) string {
		return "GoAgent 支持 PDF、DOCX、XLSX、PPTX、TXT 格式"
	}

	results := RunAgentE2E(t, labels, stubRunner)
	t.Logf("Agent E2E: %d/%d passed, %d failed", results.Passed, results.Total, results.Failed)
}

// TestSampleData 验证示例数据格式正确。
func TestSampleData(t *testing.T) {
	samples := GenerateSampleTestData()
	if len(samples) != 3 {
		t.Errorf("expected 3 samples, got %d", len(samples))
	}
	for i, s := range samples {
		if s.Query == "" {
			t.Errorf("sample %d: empty query", i)
		}
		if len(s.DocIDs) == 0 {
			t.Errorf("sample %d: empty doc_ids", i)
		}
	}
}

// TestStockEval 股票置信度专项评估。
func TestStockEval(t *testing.T) {
	cases := GenerateStockEvalCases()
	passed, total := RunStockEval(t, cases)
	t.Logf("stock eval: %d/%d passed", passed, total)
	if passed < 3 {
		t.Errorf("stock eval: only %d/%d passed, expected >= 3", passed, total)
	}
}
