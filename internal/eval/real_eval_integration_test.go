package eval

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRealLabelsEval_WithMock(t *testing.T) {
	path := filepath.Join("testdata", "real_labels.json")
	labels, err := LoadRealLabels(path)
	if err != nil {
		t.Fatalf("load real labels: %v", err)
	}
	if len(labels) != 5 {
		t.Fatalf("expected 5 real labels, got %d", len(labels))
	}

	// 模拟低质量 RAG：只能命中部分关键词
	lowQuality := func(q string) []string {
		return []string{"贵州茅台 2025 年 Q4 营收增长至 1,234.56 亿",
			"海天味业 毛利率 同比下降 2.3 个百分点",
			"消费者信心指数 CPI 同比变化 +0.5%"}
	}
	m := EvaluateRealLabels(labels, lowQuality)
	t.Logf("低质量 RAG: PassRate=%.0f%%, AvgHitRate=%.2f, Passed=%d/%d",
		m.PassRate*100, m.AvgHitRate, m.PassedQueries, m.TotalQueries)
	for _, r := range m.PerQuery {
		t.Logf("  %q: %d/%d (%.0f%%) %v",
			r.Query, r.KeywordHits, r.KeywordTotal, r.HitRate*100, r.Passed)
	}

	// 模拟高质量 RAG：能命中更多关键词
	highQuality := func(q string) []string {
		if strings.Contains(q, "茅台") {
			return []string{"贵州茅台 2025 年 Q4 营收 1,234.56 亿同比增长 15%"}
		}
		if strings.Contains(q, "海天") {
			return []string{"海天味业 毛利率 2025 年同比下降 2.3 个百分点"}
		}
		if strings.Contains(q, "CPI") {
			return []string{"国家统计局公布 3 月 CPI 同比 +0.5%"}
		}
		if strings.Contains(q, "投资风格") {
			return []string{"用户偏好右侧交易 保守型 不碰银行 地产"}
		}
		if strings.Contains(q, "市场情绪") {
			return []string{"今日成交量萎缩 主力出货 情绪偏空"}
		}
		return []string{}
	}
	m2 := EvaluateRealLabels(labels, highQuality)
	t.Logf("高质量 RAG: PassRate=%.0f%%, AvgHitRate=%.2f, Passed=%d/%d",
		m2.PassRate*100, m2.AvgHitRate, m2.PassedQueries, m2.TotalQueries)

	// 高质量 RAG 应全部通过
	if m2.PassRate < 0.8 {
		t.Errorf("high quality RAG should have >=80%% pass rate, got %.0f%%", m2.PassRate*100)
	}
}
