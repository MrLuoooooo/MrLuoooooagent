package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"go.uber.org/zap"
)

// DocumentMeta stores metadata about uploaded documents.
type DocumentMeta struct {
	ID         string `json:"document_id"`
	Filename   string `json:"filename"`
	ChunkCount int    `json:"chunk_count"`
	CreatedAt  string `json:"created_at"`
	Content    string `json:"content,omitempty"`
}

// esDocRecord is the ES document stored in the doc index.
type esDocRecord struct {
	DocumentID string `json:"document_id"`
	Filename   string `json:"filename"`
	ChunkCount int    `json:"chunk_count"`
	CreatedAt  string `json:"created_at"`
	Content    string `json:"content"`
}

// ESDocumentStore persists document metadata in Elasticsearch.
type ESDocumentStore struct {
	client    *elasticsearch.Client
	docIndex  string
	logger    *zap.Logger
}

// NewESDocumentStore 连 ES 存文档元数据。
func NewESDocumentStore(client *elasticsearch.Client, docIndex string, logger *zap.Logger) (*ESDocumentStore, error) {
	s := &ESDocumentStore{
		client:   client,
		docIndex: docIndex,
		logger:   logger,
	}
	// Retry connecting to ES.
	var lastErr error
	for i := 0; i < 2; i++ {
		if err := s.ensureIndex(context.Background()); err != nil {
			lastErr = err
			logger.Warn("es doc index not ready, retrying...", zap.Int("attempt", i+1), zap.Error(err))
			time.Sleep(2 * time.Second)
			continue
		}
		return s, nil
	}
	return nil, fmt.Errorf("ensure doc index after 15 retries: %w", lastErr)
}

func (s *ESDocumentStore) ensureIndex(ctx context.Context) error {
	exists, err := s.client.Indices.Exists([]string{s.docIndex})
	if err != nil {
		return err
	}
	if exists.StatusCode == 404 {
		mapping := `{"mappings":{"properties":{` +
			`"document_id":{"type":"keyword"},` +
			`"filename":{"type":"text","analyzer":"standard"},` +
			`"chunk_count":{"type":"integer"},` +
			`"created_at":{"type":"date"},` +
			`"content":{"type":"text","analyzer":"standard"}` +
			`}}}`
		res, err := s.client.Indices.Create(s.docIndex, s.client.Indices.Create.WithBody(strings.NewReader(mapping)))
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.IsError() {
			return fmt.Errorf("create doc index %s: %s", s.docIndex, res.String())
		}
	}
	return nil
}

// Save stores document metadata. Auto-recreates index on 404.
func (s *ESDocumentStore) Save(ctx context.Context, doc DocumentMeta) error {
	record := esDocRecord{
		DocumentID: doc.ID,
		Filename:   doc.Filename,
		ChunkCount: doc.ChunkCount,
		CreatedAt:  doc.CreatedAt,
		Content:    doc.Content,
	}
	body, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal doc: %w", err)
	}

	res, err := s.client.Index(s.docIndex, bytes.NewReader(body),
		s.client.Index.WithDocumentID(doc.ID),
		s.client.Index.WithRefresh("wait_for"),
	)
	if err != nil {
		s.logger.Error("es index document", zap.String("doc_id", doc.ID), zap.Error(err))
		return fmt.Errorf("index document: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		if res.StatusCode == 404 {
			if err := s.ensureIndex(ctx); err != nil {
				s.logger.Error("es recreate doc index", zap.Error(err))
				return fmt.Errorf("recreate doc index: %w", err)
			}
			return s.Save(ctx, doc)
		}
		s.logger.Error("es index document error", zap.String("doc_id", doc.ID), zap.String("resp", res.String()))
		return fmt.Errorf("index document error: %s", res.String())
	}
	return nil
}

// Delete removes a document by ID.
func (s *ESDocumentStore) Delete(ctx context.Context, id string) error {
	res, err := s.client.Delete(s.docIndex, id,
		s.client.Delete.WithRefresh("wait_for"),
	)
	if err != nil {
		s.logger.Error("es delete document", zap.String("doc_id", id), zap.Error(err))
		return fmt.Errorf("delete document: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() && res.StatusCode != 404 {
		s.logger.Error("es delete document error", zap.String("doc_id", id), zap.String("resp", res.String()))
		return fmt.Errorf("delete document error: %s", res.String())
	}
	return nil
}

// List 列全部文档，最新在前。
// Auto-recreates index on 404 and returns empty list.
func (s *ESDocumentStore) List(ctx context.Context) ([]DocumentMeta, error) {
	query := `{"query":{"match_all":{}},"sort":[{"created_at":{"order":"desc"}}],"size":1000}`
	res, err := s.client.Search(
		s.client.Search.WithIndex(s.docIndex),
		s.client.Search.WithBody(strings.NewReader(query)),
	)
	if err != nil {
		s.logger.Error("es search documents", zap.Error(err))
		return nil, fmt.Errorf("search documents: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		if res.StatusCode == 404 {
			if err := s.ensureIndex(ctx); err != nil {
				s.logger.Error("es recreate doc index", zap.Error(err))
				return nil, fmt.Errorf("recreate doc index: %w", err)
			}
			return nil, nil
		}
		s.logger.Error("es search documents error", zap.String("resp", res.String()))
		return nil, fmt.Errorf("search documents error: %s", res.String())
	}

	var searchResp struct {
		Hits struct {
			Hits []struct {
				Source esDocRecord `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decode documents: %w", err)
	}

	result := make([]DocumentMeta, len(searchResp.Hits.Hits))
	for i, hit := range searchResp.Hits.Hits {
		result[i] = DocumentMeta{
			ID:         hit.Source.DocumentID,
			Filename:   hit.Source.Filename,
			ChunkCount: hit.Source.ChunkCount,
			CreatedAt:  hit.Source.CreatedAt,
			Content:    hit.Source.Content,
		}
	}
	return result, nil
}
