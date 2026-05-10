package model

// UploadDocumentResponse is the response from a document upload.
type UploadDocumentResponse struct {
	DocumentIDs []string `json:"document_ids"`
	ChunkCount  int      `json:"chunk_count"`
	Status      string   `json:"status"`
}

// DeleteDocumentResponse is the response from a document deletion.
type DeleteDocumentResponse struct {
	DocumentID string `json:"document_id"`
	Status     string `json:"status"`
}

// DocumentItem is a single document in list responses.
type DocumentItem struct {
	DocumentID  string `json:"document_id"`
	Content     string `json:"content"`
	ChunkCount  int    `json:"chunk_count"`
	CreatedAt   string `json:"created_at"`
}

// ListDocumentsResponse is the paginated list response.
type ListDocumentsResponse struct {
	Total     int            `json:"total"`
	Documents []DocumentItem `json:"documents"`
}
