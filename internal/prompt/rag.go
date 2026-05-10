package prompt

import (
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
)

// NewRAGTemplate returns a ChatTemplate with a standard RAG system prompt.
// Variables: {context}, {query}
func NewRAGTemplate() prompt.ChatTemplate {
	return prompt.FromMessages(schema.FString,
		schema.SystemMessage(systemRAG),
	)
}
