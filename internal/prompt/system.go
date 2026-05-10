package prompt

// systemRAG is the default system prompt for the RAG pipeline.
// Variables: {context}, {query}
const systemRAG = `You are a helpful AI assistant. Answer the question based on the context below.
If the context does not contain enough information, say so.

Context:
{context}

Question: {query}`
