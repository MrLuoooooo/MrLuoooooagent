package eval

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ContrastiveTriplet 对比学习三元组：query, positive(相关), negative(不相关)。
// 用于下游 embedding 模型微调（如 OpenAI fine-tuning、SBERT 训练）。
type ContrastiveTriplet struct {
	Query    string `json:"query"`
	Positive string `json:"positive"` // 相关 chunk 内容
	Negative string `json:"negative"` // 不相关 chunk 内容（hard negative）
	Score    int    `json:"score"`    // 标注置信度 1-5
}

// GenerateContrastiveTriplets 基于真实标注集生成对比学习数据。
// positive 来自真实标注，negative 来自语料库中的"近似但不匹配"片段（hard negative）。
//
// corpusFunc: 接受 query 字符串，返回候选 chunk 列表（用于找 hard negative）。
func GenerateContrastiveTriplets(
	labels []RealQueryLabel,
	corpusFunc func(query string) []string,
) []ContrastiveTriplet {
	var triplets []ContrastiveTriplet

	for _, l := range labels {
		candidates := corpusFunc(l.Query)
		positive := findPositive(l, candidates)
		if positive == "" {
			continue
		}

		// hard negative：候选 chunk 中不含 must_contain_keywords 的那些
		var negatives []string
		for _, c := range candidates {
			if containsAny(c, l.MustContainKeywords) {
				continue
			}
			negatives = append(negatives, c)
		}
		if len(negatives) == 0 {
			continue
		}

		// 每个 query 取前 3 个 hard negative
		maxNeg := 3
		if len(negatives) > maxNeg {
			negatives = negatives[:maxNeg]
		}
		for _, neg := range negatives {
			triplets = append(triplets, ContrastiveTriplet{
				Query:    l.Query,
				Positive: positive,
				Negative: neg,
				Score:    4, // hard negative 价值高
			})
		}
	}
	return triplets
}

// findPositive 找到第一个含全部 must_contain_keywords 的候选。
func findPositive(l RealQueryLabel, candidates []string) string {
	for _, c := range candidates {
		matched := 0
		for _, kw := range l.MustContainKeywords {
			if strings.Contains(c, kw) {
				matched++
			}
		}
		if matched == len(l.MustContainKeywords) {
			return c
		}
	}
	return ""
}

func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// ExportTripletsJSONL 把三元组导出为 JSONL，每行一个 JSON 对象。
// 供下游训练脚本（Python/PyTorch）直接读取。
func ExportTripletsJSONL(triplets []ContrastiveTriplet, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create jsonl: %w", err)
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	for _, t := range triplets {
		if err := enc.Encode(t); err != nil {
			return fmt.Errorf("encode: %w", err)
		}
	}
	return nil
}
