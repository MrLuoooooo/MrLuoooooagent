package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/elastic/go-elasticsearch/v8"
	"go.uber.org/zap"
)

// ConversationMeta holds basic conversation metadata.
type ConversationMeta struct {
	ID           string    `json:"conversation_id"`
	Title        string    `json:"title"`
	MessageCount int       `json:"message_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// esConvDoc is the ES document for a conversation metadata record.
type esConvDoc struct {
	ConversationID string    `json:"conversation_id"`
	Title          string    `json:"title"`
	MessageCount   int       `json:"message_count"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// esMsgDoc is the ES document for a single message.
type esMsgDoc struct {
	ConversationID string       `json:"conversation_id"`
	Role           string       `json:"role"`
	Content        string       `json:"content"`
	ToolCalls      []toolCallDoc `json:"tool_calls,omitempty"`
	CreatedAt      time.Time    `json:"created_at"`
	Order          int64        `json:"order"`
}

type toolCallDoc struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ESConversationStore persists conversation data in Elasticsearch.
type ESConversationStore struct {
	client        *elasticsearch.Client
	convIndex     string
	msgIndex      string
	logger        *zap.Logger
	msgSeqCounter int64
}

// NewESConversationStore creates an ES-backed conversation store.
func NewESConversationStore(client *elasticsearch.Client, convIndex, msgIndex string, logger *zap.Logger) (*ESConversationStore, error) {
	s := &ESConversationStore{
		client:    client,
		convIndex: convIndex,
		msgIndex:  msgIndex,
		logger:    logger,
	}
	// Retry connecting to ES since it may still be starting (Docker restart race)
	var lastErr error
	for i := 0; i < 15; i++ {
		if err := s.ensureIndices(context.Background()); err != nil {
			lastErr = err
			logger.Warn("es not ready, retrying...", zap.Int("attempt", i+1), zap.Error(err))
			time.Sleep(2 * time.Second)
			continue
		}
		return s, nil
	}
	return nil, fmt.Errorf("ensure conversation indices after 15 retries: %w", lastErr)
}

func (s *ESConversationStore) ensureIndices(ctx context.Context) error {
	for _, idx := range []string{s.convIndex, s.msgIndex} {
		exists, err := s.client.Indices.Exists([]string{idx})
		if err != nil {
			return err
		}
		if exists.StatusCode == 404 {
			mapping := `{"mappings":{"properties":{`
			if idx == s.convIndex {
				mapping += `"conversation_id":{"type":"keyword"},"title":{"type":"text","analyzer":"standard"},"message_count":{"type":"integer"},"created_at":{"type":"date"},"updated_at":{"type":"date"}`
			} else {
				mapping += `"conversation_id":{"type":"keyword"},"role":{"type":"keyword"},"content":{"type":"text","analyzer":"standard"},"tool_calls":{"type":"object","enabled":false},"created_at":{"type":"date"},"order":{"type":"long"}`
			}
			mapping += `}}}`
			res, err := s.client.Indices.Create(idx, s.client.Indices.Create.WithBody(strings.NewReader(mapping)))
			if err != nil {
				return err
			}
			defer res.Body.Close()
			if res.IsError() {
				return fmt.Errorf("create index %s: %s", idx, res.String())
			}
		}
	}
	return nil
}

// Save appends messages to a conversation and updates metadata.
func (s *ESConversationStore) Save(ctx context.Context, conversationID string, msgs []*schema.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	var buf bytes.Buffer
	for _, msg := range msgs {
		s.msgSeqCounter++
		doc := esMsgDoc{
			ConversationID: conversationID,
			Role:           string(msg.Role),
			Content:        msg.Content,
			CreatedAt:      time.Now().UTC(),
			Order:          s.msgSeqCounter,
		}
		if len(msg.ToolCalls) > 0 {
			doc.ToolCalls = make([]toolCallDoc, len(msg.ToolCalls))
			for i, tc := range msg.ToolCalls {
				doc.ToolCalls[i] = toolCallDoc{
					ID:   tc.ID,
					Type: tc.Type,
				}
				doc.ToolCalls[i].Function.Name = tc.Function.Name
				doc.ToolCalls[i].Function.Arguments = tc.Function.Arguments
			}
		}
		line, _ := json.Marshal(map[string]interface{}{"index": map[string]string{"_index": s.msgIndex}})
		buf.Write(line)
		buf.WriteByte('\n')
		body, _ := json.Marshal(doc)
		buf.Write(body)
		buf.WriteByte('\n')
	}

	res, err := s.client.Bulk(
		bytes.NewReader(buf.Bytes()),
		s.client.Bulk.WithRefresh("wait_for"),
	)
	if err != nil {
		return fmt.Errorf("bulk index messages: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("bulk index error: %s", res.String())
	}

	// Update conversation message count
	if err := s.updateConvCount(ctx, conversationID); err != nil {
		s.logger.Warn("update conversation count", zap.String("id", conversationID), zap.Error(err))
	}

	return nil
}

func (s *ESConversationStore) updateConvCount(ctx context.Context, conversationID string) error {
	// Count messages via search
	query := fmt.Sprintf(`{"query":{"term":{"conversation_id":"%s"}}}`, conversationID)
	res, err := s.client.Count(
		s.client.Count.WithIndex(s.msgIndex),
		s.client.Count.WithBody(strings.NewReader(query)),
	)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("count messages: %s", res.String())
	}
	var countResp struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(res.Body).Decode(&countResp); err != nil {
		return err
	}

	// Update conversation doc
	update := fmt.Sprintf(`{"doc":{"message_count":%d,"updated_at":"%s"}}`,
		countResp.Count, time.Now().UTC().Format(time.RFC3339))
	upRes, err := s.client.Update(
		s.convIndex,
		conversationID,
		strings.NewReader(update),
	)
	if err != nil {
		return err
	}
	defer upRes.Body.Close()
	if upRes.IsError() && upRes.StatusCode != 404 {
		return fmt.Errorf("update conv count: %s", upRes.String())
	}
	return nil
}

// Create registers a new conversation with metadata.
func (s *ESConversationStore) Create(ctx context.Context, id string, title string) error {
	doc := esConvDoc{
		ConversationID: id,
		Title:          title,
		MessageCount:   0,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	body, _ := json.Marshal(doc)
	res, err := s.client.Index(s.convIndex, bytes.NewReader(body),
		s.client.Index.WithDocumentID(id),
		s.client.Index.WithRefresh("wait_for"),
	)
	if err != nil {
		return fmt.Errorf("index conversation: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("index conversation error: %s", res.String())
	}
	return nil
}

// List returns metadata for all conversations, newest first.
func (s *ESConversationStore) List(ctx context.Context) ([]ConversationMeta, error) {
	query := `{"query":{"match_all":{}},"sort":[{"updated_at":{"order":"desc"}}],"size":1000}`
	res, err := s.client.Search(
		s.client.Search.WithIndex(s.convIndex),
		s.client.Search.WithBody(strings.NewReader(query)),
	)
	if err != nil {
		return nil, fmt.Errorf("search conversations: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("search conversations error: %s", res.String())
	}

	var searchResp struct {
		Hits struct {
			Hits []struct {
				Source esConvDoc `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decode conversations: %w", err)
	}

	result := make([]ConversationMeta, len(searchResp.Hits.Hits))
	for i, hit := range searchResp.Hits.Hits {
		result[i] = ConversationMeta{
			ID:           hit.Source.ConversationID,
			Title:        hit.Source.Title,
			MessageCount: hit.Source.MessageCount,
			CreatedAt:    hit.Source.CreatedAt,
			UpdatedAt:    hit.Source.UpdatedAt,
		}
	}
	return result, nil
}

// Load returns messages for a conversation, ordered by time ascending.
func (s *ESConversationStore) Load(ctx context.Context, conversationID string) ([]*schema.Message, error) {
	query := fmt.Sprintf(
		`{"query":{"term":{"conversation_id":"%s"}},"sort":[{"order":{"order":"asc"}}],"size":1000}`,
		conversationID,
	)
	res, err := s.client.Search(
		s.client.Search.WithIndex(s.msgIndex),
		s.client.Search.WithBody(strings.NewReader(query)),
	)
	if err != nil {
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("search messages error: %s", res.String())
	}

	var searchResp struct {
		Hits struct {
			Hits []struct {
				Source esMsgDoc `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decode messages: %w", err)
	}

	result := make([]*schema.Message, len(searchResp.Hits.Hits))
	for i, hit := range searchResp.Hits.Hits {
		msg := &schema.Message{
			Role:    schema.RoleType(hit.Source.Role),
			Content: hit.Source.Content,
		}
		if len(hit.Source.ToolCalls) > 0 {
			msg.ToolCalls = make([]schema.ToolCall, len(hit.Source.ToolCalls))
			for j, tc := range hit.Source.ToolCalls {
				msg.ToolCalls[j] = schema.ToolCall{
					ID:   tc.ID,
					Type: tc.Type,
					Function: schema.FunctionCall{
						Name:      tc.Function.Name,
						Arguments: tc.Function.Arguments,
					},
				}
			}
		}
		result[i] = msg
	}
	return result, nil
}

// Delete removes a conversation and all its messages.
func (s *ESConversationStore) Delete(ctx context.Context, conversationID string) error {
	// Delete conversation document
	delRes, err := s.client.Delete(s.convIndex, conversationID,
		s.client.Delete.WithRefresh("wait_for"),
	)
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	defer delRes.Body.Close()

	// Delete all messages for this conversation via delete_by_query
	query := fmt.Sprintf(`{"query":{"term":{"conversation_id":"%s"}}}`, conversationID)
	dbqRes, err := s.client.DeleteByQuery(
		[]string{s.msgIndex},
		strings.NewReader(query),
		s.client.DeleteByQuery.WithRefresh(true),
	)
	if err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}
	defer dbqRes.Body.Close()
	if dbqRes.IsError() {
		return fmt.Errorf("delete messages error: %s", dbqRes.String())
	}
	return nil
}

// Ensure conversation IDs are unique-ish at runtime.
var convCounter int64

// NewConversationID returns a unique conversation ID.
func NewConversationID() string {
	convCounter++
	return fmt.Sprintf("conv_%d_%d", time.Now().UnixNano(), convCounter)
}
