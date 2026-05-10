package store

// DocumentMeta stores metadata about uploaded documents.
// Used by the document handler list/delete endpoints.
type DocumentMeta struct {
	ID         string `json:"document_id"`
	Filename   string `json:"filename"`
	ChunkCount int    `json:"chunk_count"`
	CreatedAt  string `json:"created_at"`
}

// DocumentStore is a placeholder for document metadata persistence.
// Currently in-memory. ES-backed implementation pending.
type DocumentStore struct {
	docs map[string]DocumentMeta
}

// NewDocumentStore creates a DocumentStore.
func NewDocumentStore() *DocumentStore {
	return &DocumentStore{docs: make(map[string]DocumentMeta)}
}

// Save stores document metadata.
func (s *DocumentStore) Save(meta DocumentMeta) {
	s.docs[meta.ID] = meta
}

// Delete removes a document by ID.
func (s *DocumentStore) Delete(id string) {
	delete(s.docs, id)
}

// List returns all stored document metadata.
func (s *DocumentStore) List() []DocumentMeta {
	result := make([]DocumentMeta, 0, len(s.docs))
	for _, v := range s.docs {
		result = append(result, v)
	}
	return result
}
