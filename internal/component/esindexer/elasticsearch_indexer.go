package esindexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v8"
)

// ElasticsearchIndexer implements the Eino Indexer interface (v0.8.13) for Elasticsearch.
type ElasticsearchIndexer struct {
	client    *elasticsearch.Client
	indexName string
	embedder  embedding.Embedder
}

// NewElasticsearchIndexer creates a new Elasticsearch indexer.
// On creation, it ensures the ES index exists with the correct dense_vector mapping.
func NewElasticsearchIndexer(
	client *elasticsearch.Client,
	emb embedding.Embedder,
	indexName string,
	embeddingDim int,
) indexer.Indexer {
	// Auto-create index if missing.
	if err := ensureIndex(client, indexName, embeddingDim); err != nil {
		// Non-fatal: log only, actual writes will fail clearly.
		fmt.Printf("[esindexer] ensure index warning: %v\n", err)
	}

	return &ElasticsearchIndexer{
		client:    client,
		indexName: indexName,
		embedder:  emb,
	}
}

// Store implements indexer.Indexer.
func (e *ElasticsearchIndexer) Store(ctx context.Context, docs []*schema.Document, opts ...indexer.Option) ([]string, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	texts := make([]string, len(docs))
	for i, doc := range docs {
		texts[i] = doc.Content
	}

	embeddings, err := e.embedder.EmbedStrings(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("generate embeddings: %w", err)
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

// ensureIndex creates the ES index with a dense_vector mapping if it doesn't exist.
func ensureIndex(client *elasticsearch.Client, indexName string, dim int) error {
	res, err := client.Indices.Exists([]string{indexName})
	if err != nil {
		return fmt.Errorf("check index: %w", err)
	}
	if res.StatusCode == 200 {
		res.Body.Close()
		return nil // already exists
	}
	res.Body.Close()

	// Create index with vector mapping.
	mapping := map[string]any{
		"mappings": map[string]any{
			"properties": map[string]any{
				"content": map[string]any{
					"type": "text",
				},
				"embedding": map[string]any{
					"type":       "dense_vector",
					"dims":       dim,
					"index":      true,
					"similarity": "cosine",
				},
				"meta_data": map[string]any{
					"type":    "object",
					"dynamic": true,
				},
				"created_at": map[string]any{
					"type": "date",
				},
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

var _ indexer.Indexer = (*ElasticsearchIndexer)(nil)
