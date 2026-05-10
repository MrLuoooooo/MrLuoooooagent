package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/embedding"
	"github.com/cloudwego/eino/components/indexer"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
)

// NewDocumentIngestionChain builds a Runnable[[]byte, []string] that:
//   1. Treats the byte input as raw text
//   2. Chunks the text into segments
//   3. Embeds each chunk
//   4. Stores in the vector index
//   5. Returns the assigned document IDs
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

			// 1. Chunk text.
			chunks := chunkText(text, chunkSize, chunkOverlap)
			if len(chunks) == 0 {
				return nil, fmt.Errorf("no content after chunking")
			}

			// 2. Create documents with IDs.
			now := time.Now().UTC()
			docs := make([]*schema.Document, len(chunks))
			for i, chunk := range chunks {
				docs[i] = &schema.Document{
					ID:      uuid.New().String(),
					Content: chunk,
					MetaData: map[string]any{
						"chunk_index": i,
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

			// 4. Index — store returns assigned IDs.
			ids, err := idx.Store(ctx, docs)
			if err != nil {
				return nil, fmt.Errorf("document chain: index: %w", err)
			}

			return ids, nil
		},
	))

	return chain.Compile(context.Background())
}

// chunkText splits text into overlapping chunks of approximately chunkSize characters.
func chunkText(text string, size, overlap int) []string {
	if size <= 0 {
		size = 500
	}
	if overlap >= size {
		overlap = size / 10
	}

	runes := []rune(text)
	if len(runes) <= size {
		return []string{text}
	}

	var chunks []string
	start := 0
	for start < len(runes) {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[start:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		start += size - overlap
	}

	return chunks
}
