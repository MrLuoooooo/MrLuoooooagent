package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/chunker"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// CtxFileName is the context key for passing the original file name
// through the ingestion chain without changing the Eino chain type signature.
type ctxKey string

const CtxFileName ctxKey = "ingest_file_name"

// NewDocumentIngestionChain 文档摄入流水线：切分 → embedding → 写入向量库，返回文档 ID。
func NewDocumentIngestionChain(
	emb embedding.Embedder,
	idx indexer.Indexer,
	chunkSize int,
	chunkOverlap int,
) (compose.Runnable[[]byte, []string], error) {

	chain := compose.NewChain[[]byte, []string]()

	chain.AppendLambda(compose.InvokableLambda(
		func(ctx context.Context, data []byte) ([]string, error) {
			if len(data) == 0 {
				return nil, fmt.Errorf("empty file content")
			}

			text := string(data)
			fileName, _ := ctx.Value(CtxFileName).(string)

			// 1. Chunk with semantic boundary detection (paragraph + sentence).
			results := chunker.ChunkSemantic(text, chunkSize, chunkOverlap)
			if len(results) == 0 {
				return nil, fmt.Errorf("no content after chunking")
			}

			// 2. Create documents with IDs. All chunks share a parent document_id.
			parentID := uuid.New().String()
			now := time.Now().UTC()
			docs := make([]*schema.Document, len(results))
			for i, r := range results {
				docs[i] = &schema.Document{
					ID:      uuid.New().String(),
					Content: r.Text,
					MetaData: map[string]any{
						"document_id": parentID,
						"chunk_index": i,
						"title":       fileName,
						"section":     r.Section,
						"char_count":  len([]rune(r.Text)),
						"created_at":  now.Format(time.RFC3339),
					},
				}
			}

			// 3. Embed all chunks.
			texts := make([]string, len(docs))
			for i, d := range docs {
				texts[i] = d.Content
			}
			vectors, err := emb.EmbedStrings(ctx, texts)
			if err != nil {
				return nil, fmt.Errorf("document chain: embed: %w", err)
			}

			// Store vectors in document metadata (indexer will use them).
			for i := range docs {
				if docs[i].MetaData == nil {
					docs[i].MetaData = make(map[string]any)
				}
				// indexer may use this; some implementations ignore metadata vectors.
				docs[i].MetaData["vector"] = vectors[i]
			}

			// 4. 写入向量库，返回分块 ID。
			chunkIDs, err := idx.Store(ctx, docs)
			if err != nil {
				return nil, fmt.Errorf("document chain: index: %w", err)
			}

			// Return parent document ID as the first element, followed by chunk IDs.
			return append([]string{parentID}, chunkIDs...), nil
		},
	))

	return chain.Compile(context.Background())
}
