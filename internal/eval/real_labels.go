package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// RealQueryLabel 真实标注：query → 应答中必须包含的关键词集合。
// 生产级标注集不依赖具体的 ES doc_id（UUID 是运行时生成，无法预先标注），
// 而是通过"答案应包含的关键词"反向校验 RAG 检索是否召回了正确的 chunk。
type RealQueryLabel struct {
	Query               string   `json:"query"`
	ExpectedAnswer      string   `json:"expected_answer"`
	MustContainKeywords []string `json:"must_contain_keywords"`
	Category            string   `json:"category"`     // finance_factual/finance_analytical/macro/user_profile/sentiment
	Difficulty          string   `json:"difficulty"`   // easy/medium/hard
}

// RealRetrievalEvalResult 单条 query 的真实评估结果。
type RealRetrievalEvalResult struct {
	Query         string   `json:"query"`
	Retrieved     []string `json:"retrieved"`
	KeywordHits   int      `json:"keyword_hits"`
	KeywordTotal  int      `json:"keyword_total"`
	HitRate       float64  `json:"hit_rate"`
	Passed        bool     `json:"passed"`
}

// RealRetrievalMetrics 真实标注评估汇总。
type RealRetrievalMetrics struct {
	TotalQueries    int                     `json:"total_queries"`
	PassedQueries   int                     `json:"passed_queries"`
	PassRate        float64                 `json:"pass_rate"`
	AvgHitRate      float64                 `json:"avg_hit_rate"`
	PerQuery        []RealRetrievalEvalResult `json:"per_query"`
	ByCategory      map[string]int          `json:"by_category"`
}

// LoadRealLabels 从 JSON 文件加载真实标注集。
func LoadRealLabels(path string) ([]RealQueryLabel, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read real labels: %w", err)
	}
	var labels []RealQueryLabel
	if err := json.Unmarshal(b, &labels); err != nil {
		return nil, fmt.Errorf("parse real labels: %w", err)
	}
	return labels, nil
}

// EvaluateRealLabels 用真实标注集评估 retriever。
// retriever 函数返回的每个 doc 用 content 字符串，框架无关。
func EvaluateRealLabels(labels []RealQueryLabel, retriever func(query string) []string) RealRetrievalMetrics {
	m := RealRetrievalMetrics{
		TotalQueries: len(labels),
		ByCategory:   make(map[string]int),
	}

	totalHitRate := 0.0

	for _, l := range labels {
		docs := retriever(l.Query)
		result := RealRetrievalEvalResult{
			Query:        l.Query,
			Retrieved:    docs,
			KeywordTotal: len(l.MustContainKeywords),
		}
		for _, kw := range l.MustContainKeywords {
			for _, d := range docs {
				if strings.Contains(d, kw) {
					result.KeywordHits++
					break
				}
			}
		}
		if result.KeywordTotal > 0 {
			result.HitRate = float64(result.KeywordHits) / float64(result.KeywordTotal)
		}
		result.Passed = result.HitRate >= 0.5 // 至少一半关键词命中

		if result.Passed {
			m.PassedQueries++
		}
		totalHitRate += result.HitRate
		m.ByCategory[l.Category]++
		m.PerQuery = append(m.PerQuery, result)
	}

	if len(labels) > 0 {
		m.PassRate = float64(m.PassedQueries) / float64(len(labels))
		m.AvgHitRate = totalHitRate / float64(len(labels))
	}
	return m
}
