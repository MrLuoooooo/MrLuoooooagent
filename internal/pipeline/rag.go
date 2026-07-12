package pipeline

import (
	"context"
	"fmt"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/component/reranker"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
)

// NewRAGChain — 检索(candidateTopK) → 可选重排(topK) → 拼接上下文 → ChatModel 生成回答。
// reranker 为 nil 时跳过重排；candidateTopK≤0 时默认取 topK。
func NewRAGChain(
	rd retriever.Retriever,
	tmpl prompt.ChatTemplate,
	cm model.ChatModel,
	rr reranker.Reranker,
	topK int,
	candidateTopK int,
) (compose.Runnable[string, *schema.Message], error) {

	if candidateTopK <= 0 {
		candidateTopK = topK
	}

	chain := compose.NewChain[string, *schema.Message]()

	// Node 1: Retrieve(候选数) → 可选重排(最终数) → 拼 context。
	chain.AppendLambda(compose.InvokableLambda(func(ctx context.Context, query string) (map[string]any, error) {
		retrieveK := candidateTopK
		docs, err := rd.Retrieve(ctx, query, retriever.WithTopK(retrieveK))
		if err != nil {
			return nil, fmt.Errorf("rag chain: retrieve: %w", err)
		}

		if rr != nil && len(docs) > topK {
			var rerankErr error
			docs, rerankErr = rr.Rerank(ctx, query, docs, topK)
			if rerankErr != nil {
				return nil, fmt.Errorf("rag chain: rerank: %w", rerankErr)
			}
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
			"基于以下上下文回答问题。\n"+
				"- 如果上下文中没有相关信息，请明确说【未找到相关信息】，不要编造。\n"+
				"- 回答时请引用来源编号，格式：[1] [2]。\n"+
				"- 上下文按相关度从高到低排列。\n\n"+
				"上下文：\n{context}\n\n问题：{query}",
		),
	)
}
