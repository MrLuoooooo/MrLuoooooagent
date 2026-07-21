package esretriever

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v8"
)

// ESRRetriever 用 ES kNN 做向量检索，可选混合 kNN+BM25+RRF。
type ESRRetriever struct {
	client         *elasticsearch.Client
	embedder       embedding.Embedder
	indexName      string
	topK           int
	candidateTopK  int
	scoreThreshold float64
	hybridEnabled  bool
}

// NewESRetriever 建一个 ES 向量检索器。
// hybridEnabled=true 时走 kNN+BM25+RRF 多路融合。
func NewESRetriever(client *elasticsearch.Client, emb embedding.Embedder, indexName string, topK int, candidateTopK int, scoreThreshold float64, hybridEnabled bool) retriever.Retriever {
	return &ESRRetriever{
		client:         client,
		embedder:       emb,
		indexName:      indexName,
		topK:           topK,
		candidateTopK:  candidateTopK,
		scoreThreshold: scoreThreshold,
		hybridEnabled:  hybridEnabled,
	}
}

// Retrieve 把 query 转成向量，从 ES 里搜 topK 条最相似的文档。
// 向量失效（stub/embedding 不可用）时自动回退为 BM25 文本搜索。
func (r *ESRRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	if r.client == nil {
		return nil, fmt.Errorf("retriever: ES client is nil")
	}
	options := retriever.GetCommonOptions(&retriever.Options{
		TopK: &r.topK,
	}, opts...)

	vectors, err := r.embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("retriever: embed query: %w", err)
	}
	if len(vectors) == 0 {
		return nil, fmt.Errorf("retriever: empty embedding result")
	}
	queryVec := vectors[0]

	topK := r.topK
	if options.TopK != nil {
		topK = *options.TopK
	}

	// 检测 stub 向量（[1,0,0,...]）→ 回退为 BM25 文本搜索。
	isStub := len(queryVec) >= 3 && queryVec[0] == 1.0 && queryVec[1] == 0.0 && queryVec[2] == 0.0

	// 检索只搜 child chunks，排除 parent（parent 用于上下文注入非检索匹配）。
	// 向后兼容：旧文档无 chunk_type 的也不排除。
	childTypeFilter := map[string]any{
		"bool": map[string]any{
			"must_not": []map[string]any{
				{"term": map[string]any{"meta_data.chunk_type": "parent"}},
			},
		},
	}

	var searchBody map[string]any
	if isStub {
		searchBody = map[string]any{
			"query": map[string]any{
				"match": map[string]any{"content": query},
			},
			"size": topK,
		}
		if options.ScoreThreshold != nil && *options.ScoreThreshold > 0 {
			searchBody["min_score"] = *options.ScoreThreshold
		}
	} else if r.hybridEnabled {
		// 混合检索：kNN + BM25，ES 8.9+ 原生 RRF 融合排序。
		searchBody = map[string]any{
			"knn": map[string]any{
				"field":          "embedding",
				"query_vector":   queryVec,
				"k":              r.candidateTopK,
				"num_candidates": r.candidateTopK * 10,
				"filter":         childTypeFilter,
			},
			"query": map[string]any{
				"match": map[string]any{"content": query},
			},
			"rank": map[string]any{
				"rrf": map[string]any{
					"window_size": 60,
				},
			},
			"size": topK,
		}
		if r.scoreThreshold > 0 {
			searchBody["min_score"] = r.scoreThreshold
		}
	} else {
		searchBody = map[string]any{
			"knn": map[string]any{
				"field":          "embedding",
				"query_vector":   queryVec,
				"k":              topK,
				"num_candidates": topK * 10,
				"filter":         childTypeFilter,
			},
		}
		if options.ScoreThreshold != nil {
			searchBody["knn"].(map[string]any)["min_score"] = *options.ScoreThreshold
		}
	}
	searchBody["_source"] = []string{"content", "meta_data", "created_at"}

	bodyBytes, _ := json.Marshal(searchBody)

	resp, err := r.client.Search(
		r.client.Search.WithContext(ctx),
		r.client.Search.WithIndex(r.indexName),
		r.client.Search.WithBody(bytes.NewReader(bodyBytes)),
	)
	if err != nil {
		return nil, fmt.Errorf("retriever: es search: %w", err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, fmt.Errorf("retriever: es error: %s", resp.String())
	}

	var esResp esSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("retriever: decode response: %w", err)
	}

	docs := make([]*schema.Document, 0, len(esResp.Hits.Hits))
	for _, hit := range esResp.Hits.Hits {
		doc := &schema.Document{
			ID:       hit.ID,
			Content:  hit.Source.Content,
			MetaData: hit.Source.MetaData,
		}
		if doc.MetaData == nil {
			doc.MetaData = make(map[string]any)
		}
		doc.MetaData["_score"] = hit.Score
		docs = append(docs, doc)
	}

	return docs, nil
}

type esSearchResponse struct {
	Hits struct {
		Hits []esHit `json:"hits"`
	} `json:"hits"`
}

type esHit struct {
	ID     string  `json:"_id"`
	Score  float64 `json:"_score"`
	Source esDoc   `json:"_source"`
}

type esDoc struct {
	Content   string         `json:"content"`
	MetaData  map[string]any `json:"meta_data"`
	CreatedAt string         `json:"created_at"`
}

var _ retriever.Retriever = (*ESRRetriever)(nil)
