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

			// 1. Chunk with Parent/Child separation (production grade).
			cfg := chunker.DefaultChunkConfig
			if chunkSize > 0 {
				cfg.ChildTokens = chunkSize
			}
			if chunkOverlap > 0 {
				cfg.Overlap = chunkOverlap
			}
			results, err := chunker.ChunkWithParent(text, cfg)
			if err != nil {
				return nil, fmt.Errorf("chunk: %w", err)
			}
			if len(results) == 0 {
				return nil, fmt.Errorf("no content after chunking")
			}

			// 2. Create documents with IDs.
			// Child chunks → embedding for vector search; tagged with parent_id.
			// Parent chunks → larger context blocks; also stored for retrieval-time expansion.
			parentDocID := uuid.New().String()
			now := time.Now().UTC()
			docs := make([]*schema.Document, len(results))
			for i, r := range results {
				meta := map[string]any{
					"document_id": parentDocID,
					"chunk_index": i,
					"title":       fileName,
					"section":     r.Section,
					"char_count":  len([]rune(r.Text)),
					"token_count": r.TokenCnt,
					"chunk_type":  r.ChunkType,
					"created_at":  now.Format(time.RFC3339),
				}
				if r.ParentID != "" {
					meta["parent_id"] = r.ParentID
				}
				docs[i] = &schema.Document{
					ID:      uuid.New().String(),
					Content: r.Text,
					MetaData: meta,
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
			return append([]string{parentDocID}, chunkIDs...), nil
		},
	))

	return chain.Compile(context.Background())
}
