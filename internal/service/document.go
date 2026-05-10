package service

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"go.uber.org/zap"
)

// DocumentService decouples the document handler from the Eino Runnable.
type DocumentService struct {
	ingestionChain compose.Runnable[[]byte, []string]
	logger         *zap.Logger
}

// NewDocumentService creates a DocumentService.
func NewDocumentService(
	ingestionChain compose.Runnable[[]byte, []string],
	logger *zap.Logger,
) *DocumentService {
	return &DocumentService{ingestionChain: ingestionChain, logger: logger}
}

// Ingest processes raw file bytes through the ingestion pipeline.
func (s *DocumentService) Ingest(ctx context.Context, data []byte) ([]string, error) {
	if len(data) == 0 {
		return nil, nil
	}
	ids, err := s.ingestionChain.Invoke(ctx, data)
	if err != nil {
		s.logger.Error("document ingestion failed", zap.Error(err))
		return nil, err
	}
	return ids, nil
}
