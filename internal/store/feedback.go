package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
	"github.com/elastic/go-elasticsearch/v8"
	"go.uber.org/zap"
)

// esFeedbackDoc ES 内部反馈文档。
type esFeedbackDoc struct {
	FeedbackID     string `json:"feedback_id"`
	ConversationID string `json:"conversation_id"`
	MessageIndex   int    `json:"message_index"`
	Type           string `json:"type"`
	Rating         int    `json:"rating"`
	CorrectAnswer  string `json:"correct_answer"`
	Comment        string `json:"comment"`
	SourceQuery    string `json:"source_query"`
	SourceAnswer   string `json:"source_answer"`
	CreatedAt      string `json:"created_at"`
}

// ESFeedbackStore ES 持久化反馈。
type ESFeedbackStore struct {
	client    *elasticsearch.Client
	indexName string
	logger    *zap.Logger
}

// NewESFeedbackStore —
func NewESFeedbackStore(client *elasticsearch.Client, indexName string, logger *zap.Logger) (*ESFeedbackStore, error) {
	s := &ESFeedbackStore{client: client, indexName: indexName, logger: logger}
	var lastErr error
	for i := 0; i < 15; i++ {
		if err := s.ensureIndex(context.Background()); err != nil {
			lastErr = err
			logger.Warn("es feedback index not ready, retrying...", zap.Int("attempt", i+1), zap.Error(err))
			time.Sleep(2 * time.Second)
			continue
		}
		return s, nil
	}
	return nil, fmt.Errorf("ensure feedback index after 15 retries: %w", lastErr)
}

func (s *ESFeedbackStore) ensureIndex(ctx context.Context) error {
	exists, err := s.client.Indices.Exists([]string{s.indexName})
	if err != nil {
		return err
	}
	if exists.StatusCode == 404 {
		mapping := `{"mappings":{"properties":{` +
			`"feedback_id":{"type":"keyword"},` +
			`"conversation_id":{"type":"keyword"},` +
			`"message_index":{"type":"integer"},` +
			`"type":{"type":"keyword"},` +
			`"rating":{"type":"integer"},` +
			`"correct_answer":{"type":"text"},` +
			`"comment":{"type":"text"},` +
			`"source_query":{"type":"text"},` +
			`"source_answer":{"type":"text"},` +
			`"created_at":{"type":"date"}` +
			`}}}`
		res, err := s.client.Indices.Create(s.indexName, s.client.Indices.Create.WithBody(strings.NewReader(mapping)))
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.IsError() {
			return fmt.Errorf("create feedback index: %s", res.String())
		}
	}
	return nil
}

// Save 存一条反馈。
func (s *ESFeedbackStore) Save(ctx context.Context, item *model.FeedbackItem) error {
	doc := esFeedbackDoc{
		FeedbackID:     item.ID,
		ConversationID: item.ConversationID,
		MessageIndex:   item.MessageIndex,
		Type:           string(item.Type),
		Rating:         item.Rating,
		CorrectAnswer:  item.CorrectAnswer,
		Comment:        item.Comment,
		SourceQuery:    item.SourceQuery,
		SourceAnswer:   item.SourceAnswer,
		CreatedAt:      item.CreatedAt.Format(time.RFC3339),
	}
	body, _ := json.Marshal(doc)
	res, err := s.client.Index(s.indexName, bytes.NewReader(body),
		s.client.Index.WithDocumentID(item.ID),
		s.client.Index.WithRefresh("wait_for"),
	)
	if err != nil {
		return fmt.Errorf("index feedback: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("index feedback error: %s", res.String())
	}
	return nil
}

// List 按 conversation_id 查询。
func (s *ESFeedbackStore) List(ctx context.Context, conversationID string) ([]*model.FeedbackItem, error) {
	query := fmt.Sprintf(`{"query":{"term":{"conversation_id":"%s"}},"sort":[{"created_at":"desc"}],"size":100}`, conversationID)
	return s.search(ctx, query)
}

// ListRecent 最近 N 条（全量）。
func (s *ESFeedbackStore) ListRecent(ctx context.Context, limit int) ([]*model.FeedbackItem, error) {
	query := fmt.Sprintf(`{"query":{"match_all":{}},"sort":[{"created_at":"desc"}],"size":%d}`, limit)
	return s.search(ctx, query)
}

func (s *ESFeedbackStore) search(ctx context.Context, query string) ([]*model.FeedbackItem, error) {
	res, err := s.client.Search(
		s.client.Search.WithIndex(s.indexName),
		s.client.Search.WithBody(strings.NewReader(query)),
	)
	if err != nil {
		return nil, fmt.Errorf("search feedback: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("search feedback error: %s", res.String())
	}

	var esResp struct {
		Hits struct {
			Hits []struct {
				Source esFeedbackDoc `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("decode feedback: %w", err)
	}

	result := make([]*model.FeedbackItem, len(esResp.Hits.Hits))
	for i, hit := range esResp.Hits.Hits {
		ct, _ := time.Parse(time.RFC3339, hit.Source.CreatedAt)
		result[i] = &model.FeedbackItem{
			ID:             hit.Source.FeedbackID,
			ConversationID: hit.Source.ConversationID,
			MessageIndex:   hit.Source.MessageIndex,
			Type:           model.FeedbackType(hit.Source.Type),
			Rating:         hit.Source.Rating,
			CorrectAnswer:  hit.Source.CorrectAnswer,
			Comment:        hit.Source.Comment,
			SourceQuery:    hit.Source.SourceQuery,
			SourceAnswer:   hit.Source.SourceAnswer,
			CreatedAt:      ct,
		}
	}
	return result, nil
}
