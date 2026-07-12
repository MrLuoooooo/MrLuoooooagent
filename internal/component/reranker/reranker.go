package reranker

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Reranker 对候选文档按与 query 的相关度重排序。
type Reranker interface {
	Rerank(ctx context.Context, query string, docs []*schema.Document, topN int) ([]*schema.Document, error)
}

// LLMReranker 用 LLM 对每篇文档打分后重排。
// 每篇文档独立调用一次评分 prompt，N 通常为 20-30，可控。
type LLMReranker struct {
	chatModel  model.ChatModel
	maxRetries int
}

// NewLLMReranker 建一个 LLM 重排器。
// chatModel 用于逐文档评分；传 nil 则禁用重排。
func NewLLMReranker(cm model.ChatModel) *LLMReranker {
	if cm == nil {
		return nil
	}
	return &LLMReranker{
		chatModel:  cm,
		maxRetries: 1,
	}
}

// Rerank 逐文档评分，按分数降序取 topN。
func (r *LLMReranker) Rerank(ctx context.Context, query string, docs []*schema.Document, topN int) ([]*schema.Document, error) {
	if r == nil || r.chatModel == nil || len(docs) == 0 {
		return docs, nil
	}
	if topN <= 0 || topN > len(docs) {
		topN = len(docs)
	}

	type scored struct {
		doc   *schema.Document
		score float64
	}

	scoredDocs := make([]scored, 0, len(docs))
	for i, doc := range docs {
		score, err := r.scoreDoc(ctx, query, doc.Content)
		if err != nil {
			// 单个打分失败不阻塞整体，给 0 分
			score = 0
			_ = i // suppress unused
		}
		scoredDocs = append(scoredDocs, scored{doc: doc, score: score})
	}

	sort.Slice(scoredDocs, func(i, j int) bool {
		return scoredDocs[i].score > scoredDocs[j].score
	})

	result := make([]*schema.Document, topN)
	for i := 0; i < topN; i++ {
		result[i] = scoredDocs[i].doc
	}
	return result, nil
}

func (r *LLMReranker) scoreDoc(ctx context.Context, query, text string) (float64, error) {
	prompt := fmt.Sprintf(
		`Rate how relevant the following document is to the query on a scale of 0 to 10.
10 = directly answers the query, 0 = completely irrelevant.
Return ONLY a number (0-10), nothing else.

Query: %s

Document: %s

Relevance score (0-10):`, query, text)

	messages := []*schema.Message{
		{Role: schema.User, Content: prompt},
	}

	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		resp, err := r.chatModel.Generate(ctx, messages)
		if err != nil {
			if attempt == r.maxRetries {
				return 0, fmt.Errorf("reranker: generate: %w", err)
			}
			continue
		}

		raw := strings.TrimSpace(resp.Content)
		// 提取第一个数字
		score, err := parseFirstNumber(raw)
		if err != nil {
			if attempt == r.maxRetries {
				return 0, nil // default to 0 on parse failure
			}
			continue
		}
		return score / 10.0, nil // normalize to 0-1
	}
	return 0, nil
}

func parseFirstNumber(s string) (float64, error) {
	// 找到第一个连续的数字串（含小数点）
	var numStr string
	inNum := false
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '.' {
			numStr += string(r)
			inNum = true
		} else if inNum {
			break // 数字串结束
		}
	}
	if numStr == "" {
		return 0, fmt.Errorf("no number found in %q", s)
	}
	return strconv.ParseFloat(numStr, 64)
}
