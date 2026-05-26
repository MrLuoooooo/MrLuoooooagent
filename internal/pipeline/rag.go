package pipeline

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
)

// NewRAGChain — 检索相关文档、拼接上下文、模板填充、调用模型生成回答
func NewRAGChain(
	rd retriever.Retriever,
	tmpl prompt.ChatTemplate,
	cm model.ChatModel,
) (compose.Runnable[string, *schema.Message], error) {

	chain := compose.NewChain[string, *schema.Message]()

	// Node 1: Retrieve + build prompt variables.
	// A Lambda wraps the retriever so we preserve the original query alongside
	// the retrieved documents for downstream prompt formatting.
	chain.AppendLambda(compose.InvokableLambda(func(ctx context.Context, query string) (map[string]any, error) {
		docs, err := rd.Retrieve(ctx, query)
		if err != nil {
			return nil, fmt.Errorf("rag chain: retrieve: %w", err)
		}

		var contextStr string
		for i, doc := range docs {
			contextStr += fmt.Sprintf("[%d] %s\n", i+1, doc.Content)
		}

		return map[string]any{
			"query":   query,
			"context": contextStr,
		}, nil
	}))

	// Node 2: Format the chat template with prompt variables.
	chain.AppendChatTemplate(tmpl)

	// Node 3: Generate the final answer.
	chain.AppendChatModel(cm)

	return chain.Compile(context.Background())
}

// NewDefaultRAGTemplate — 标准 RAG prompt 模板
func NewDefaultRAGTemplate() prompt.ChatTemplate {
	return prompt.FromMessages(schema.FString,
		schema.SystemMessage(
			"Answer the question based on the following context.\n"+
				"Context:\n{context}\n\nQuestion: {query}",
		),
	)
}
