package esindexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v8"
)

// ElasticsearchIndexer 把文档向量写入 ES。
type ElasticsearchIndexer struct {
	client    *elasticsearch.Client
	indexName string
	embedder  embedding.Embedder
}

// NewElasticsearchIndexer 连 ES 并在库里没有索引时自动建表。
func NewElasticsearchIndexer(
	client *elasticsearch.Client,
	emb embedding.Embedder,
	indexName string,
	embeddingDim int,
) indexer.Indexer {
	// Auto-create index if missing.
	if err := ensureIndex(client, indexName, embeddingDim); err != nil {
		log.Printf("[esindexer] ensure index warning: %v", err)
	}

	return &ElasticsearchIndexer{
		client:    client,
		indexName: indexName,
		embedder:  emb,
	}
}

// Store 批量写文档到 ES。如果文档元数据里已经有向量就直接用，省一次 embedding。
func (e *ElasticsearchIndexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) ([]string, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	// Use pre-computed vectors from document metadata if available,
	// avoiding redundant embedding computation (the pipeline may have already embedded).
	var embeddings [][]float64
	needEmbed := false
	allHaveVectors := true
	for _, doc := range docs {
		_, ok := doc.MetaData["vector"]
		if !ok {
			allHaveVectors = false
			break
		}
	}
	if allHaveVectors {
		embeddings = make([][]float64, len(docs))
		for i, doc := range docs {
			embeddings[i] = doc.MetaData["vector"].([]float64)
		}
	} else {
		needEmbed = true
	}

	if needEmbed {
		texts := make([]string, len(docs))
		for i, doc := range docs {
			texts[i] = doc.Content
		}
		var err error
		embeddings, err = e.embedder.EmbedStrings(ctx, texts)
		if err != nil {
			return nil, fmt.Errorf("generate embeddings: %w", err)
		}
	}

	var ids []string
	var bulkBody bytes.Buffer
	for i, doc := range docs {
		docID := doc.ID
		if docID == "" {
			docID = fmt.Sprintf("doc_%d", time.Now().UnixNano())
		}
		ids = append(ids, docID)

		meta := map[string]any{
			"index": map[string]any{
				"_index": e.indexName,
				"_id":    docID,
			},
		}
		metaJSON, _ := json.Marshal(meta)
		bulkBody.Write(metaJSON)
		bulkBody.WriteByte('\n')

		docBody := map[string]any{
			"content":    doc.Content,
			"embedding":  embeddings[i],
			"meta_data":  doc.MetaData,
			"created_at": time.Now().UTC().Format(time.RFC3339),
		}
		docJSON, _ := json.Marshal(docBody)
		bulkBody.Write(docJSON)
		bulkBody.WriteByte('\n')
	}

	res, err := e.client.Bulk(
		bytes.NewReader(bulkBody.Bytes()),
		e.client.Bulk.WithContext(ctx),
	)
	if err != nil {
		return nil, fmt.Errorf("bulk request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("bulk failed: %s", string(body))
	}

	return ids, nil
}

// ensureIndex 在库里没有索引时建表，带上 dense_vector mapping。
func ensureIndex(client *elasticsearch.Client, indexName string, dim int) error {
	res, err := client.Indices.Exists([]string{indexName})
	if err != nil {
		return fmt.Errorf("check index: %w", err)
	}
	if res.StatusCode == 200 {
		res.Body.Close()
		return nil
	}
	res.Body.Close()

	mapping := map[string]any{
		"mappings": map[string]any{
			"properties": map[string]any{
				"content": map[string]any{"type": "text"},
				"embedding": map[string]any{
					"type": "dense_vector", "dims": dim,
					"index": true, "similarity": "cosine",
				},
				"meta_data": map[string]any{"type": "object", "dynamic": true},
				"created_at": map[string]any{"type": "date"},
			},
		},
	}

	mappingJSON, _ := json.Marshal(mapping)
	createRes, err := client.Indices.Create(
		indexName,
		client.Indices.Create.WithBody(strings.NewReader(string(mappingJSON))),
	)
	if err != nil {
		return fmt.Errorf("create index: %w", err)
	}
	defer createRes.Body.Close()

	if createRes.IsError() {
		body, _ := io.ReadAll(createRes.Body)
		return fmt.Errorf("create index failed: %s", string(body))
	}

	return nil
}

// DeleteByDocumentID 删掉某个文档的所有向量分块。
func (e *ElasticsearchIndexer) DeleteByDocumentID(ctx context.Context, docID string) error {
	query := fmt.Sprintf(`{"query":{"term":{"meta_data.document_id":"%s"}}}`, docID)
	res, err := e.client.DeleteByQuery(
		[]string{e.indexName},
		strings.NewReader(query),
		e.client.DeleteByQuery.WithContext(ctx),
		e.client.DeleteByQuery.WithRefresh(true),
	)
	if err != nil {
		return fmt.Errorf("delete by query: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("delete by query failed: %s", string(body))
	}
	return nil
}

var _ indexer.Indexer = (*ElasticsearchIndexer)(nil)
