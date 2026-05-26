package prompt

import (
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

// NewRAGTemplate 返回标准 RAG 提示词模板。
// Variables: {context}, {query}
func NewRAGTemplate() prompt.ChatTemplate {
	return prompt.FromMessages(schema.FString,
		schema.SystemMessage(systemRAG),
	)
}
