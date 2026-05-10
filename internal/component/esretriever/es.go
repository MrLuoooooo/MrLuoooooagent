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

// ESRRetriever implements the Eino Retriever interface backed by Elasticsearch kNN.
type ESRRetriever struct {
	client    *elasticsearch.Client
	embedder  embedding.Embedder
	indexName string
	topK      int
}

// NewESRetriever creates a new Elasticsearch-backed Retriever.
func NewESRetriever(client *elasticsearch.Client, emb embedding.Embedder, indexName string, topK int) retriever.Retriever {
	return &ESRRetriever{
		client:    client,
		embedder:  emb,
		indexName: indexName,
		topK:      topK,
	}
}

// Retrieve implements retriever.Retriever.
func (r *ESRRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
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

	searchBody := map[string]any{
		"knn": map[string]any{
			"field":          "embedding",
			"query_vector":   queryVec,
			"k":              topK,
			"num_candidates": topK * 10,
		},
		"_source": []string{"content", "meta_data", "created_at"},
	}

	if options.ScoreThreshold != nil {
		searchBody["knn"].(map[string]any)["min_score"] = *options.ScoreThreshold
	}

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
