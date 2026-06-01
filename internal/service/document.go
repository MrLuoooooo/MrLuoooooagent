package service

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/pipeline"
	"go.uber.org/zap"
)

// DocMeta holds upload-level metadata.
type DocMeta struct {
	ID         string
	Filename   string
	ChunkCount int
	CreatedAt  string
	Content    string
}

// DocStore is the persistence interface for document metadata.
type DocStore interface {
	Save(ctx context.Context, doc DocMeta) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]DocMeta, error)
}

// VectorDeleter deletes vectors by parent document ID.
// This decouples DocumentService from specific indexer implementations.
type VectorDeleter interface {
	DeleteByDocumentID(ctx context.Context, docID string) error
}

// DocumentService decouples the document handler from the Eino Runnable.
type DocumentService struct {
	ingestionChain compose.Runnable[[]byte, []string]
	docStore       DocStore
	vectorDeleter  VectorDeleter
	logger         *zap.Logger
}

// NewDocumentService —
func NewDocumentService(
	ingestionChain compose.Runnable[[]byte, []string],
	docStore DocStore,
	vectorDeleter VectorDeleter,
	logger *zap.Logger,
) *DocumentService {
	return &DocumentService{
		ingestionChain: ingestionChain,
		docStore:       docStore,
		vectorDeleter:  vectorDeleter,
		logger:         logger,
	}
}

// Ingest processes raw file bytes through the ingestion pipeline and persists metadata.
// Returns [parentID, chunkID1, chunkID2, ...] where parentID is the first element.
func (s *DocumentService) Ingest(ctx context.Context, data []byte, filename string) ([]string, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty file")
	}

	// Extract text based on file format
	text, err := pipeline.ExtractText(data, filename)
	if err != nil {
		s.logger.Warn("text extraction warning", zap.String("file", filename), zap.Error(err))
		// If extraction fails, try to process raw data anyway
		text = string(data)
	}

	ids, err := s.ingestionChain.Invoke(ctx, []byte(text))
	if err != nil {
		s.logger.Error("document ingestion failed", zap.Error(err))
		return nil, err
	}

	// ids[0] is the parent document ID (generated in pipeline).
	parentID := ids[0]
	chunkCount := len(ids) - 1 // exclude parent ID

	// Persist document metadata in ES.
	if s.docStore != nil {
		doc := DocMeta{
			ID:         parentID,
			Filename:   filename,
			ChunkCount: chunkCount,
			CreatedAt:  time.Now().UTC().Format(time.RFC3339),
			Content:    string(data),
		}
		if err := s.docStore.Save(ctx, doc); err != nil {
			s.logger.Warn("persist doc meta failed", zap.Error(err))
		}
	}

	return ids, nil
}

// DeleteDocument removes document metadata AND its vector chunks.
func (s *DocumentService) DeleteDocument(ctx context.Context, id string) error {
	// Delete vector chunks first (best effort — metadata deletion takes priority).
	if s.vectorDeleter != nil {
		if err := s.vectorDeleter.DeleteByDocumentID(ctx, id); err != nil {
			s.logger.Warn("delete document vectors failed", zap.String("doc_id", id), zap.Error(err))
		}
	}

	if s.docStore != nil {
		return s.docStore.Delete(ctx, id)
	}
	return nil
}

// ListDocuments 列全部文档。
func (s *DocumentService) ListDocuments(ctx context.Context) ([]DocMeta, error) {
	if s.docStore != nil {
		return s.docStore.List(ctx)
	}
	return nil, nil
}
