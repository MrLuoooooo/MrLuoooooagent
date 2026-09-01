package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
	ConversationID string        `json:"conversation_id"`
	Role           string        `json:"role"`
	Content        string        `json:"content"`
	ToolCalls      []toolCallDoc `json:"tool_calls,omitempty"`
	CreatedAt      time.Time     `json:"created_at"`
	Order          int64         `json:"order"`
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
	client    *elasticsearch.Client
	convIndex string
	msgIndex  string
	logger    *zap.Logger
}

// NewESConversationStore 连 ES 存会话，自动建索引。
func NewESConversationStore(client *elasticsearch.Client, convIndex, msgIndex string, logger *zap.Logger) (*ESConversationStore, error) {
	s := &ESConversationStore{
		client:    client,
		convIndex: convIndex,
		msgIndex:  msgIndex,
		logger:    logger,
	}
	var lastErr error
	for i := 0; i < 2; i++ {
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

// Save 追加消息到会话，更新元数据。
func (s *ESConversationStore) Save(ctx context.Context, conversationID string, msgs []*schema.Message) error {
	if len(msgs) == 0 {
		return nil
	}

	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, msg := range msgs {
		doc := esMsgDoc{
			ConversationID: conversationID,
			Role:           string(msg.Role),
			Content:        msg.Content,
			CreatedAt:      time.Now().UTC(),
			Order:          time.Now().UnixNano(),
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
		if err := enc.Encode(map[string]interface{}{"index": map[string]string{"_index": s.msgIndex}}); err != nil {
			return fmt.Errorf("encode bulk meta: %w", err)
		}
		if err := enc.Encode(doc); err != nil {
			return fmt.Errorf("encode message doc: %w", err)
		}
	}

	res, err := s.client.Bulk(
		bytes.NewReader(buf.Bytes()),
		s.client.Bulk.WithRefresh("wait_for"),
	)
	if err != nil {
		s.logger.Error("es bulk save messages", zap.String("conv_id", conversationID), zap.Error(err))
		return fmt.Errorf("bulk index messages: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		s.logger.Error("es bulk save error", zap.String("conv_id", conversationID), zap.String("resp", res.String()))
		return fmt.Errorf("bulk index error: %s", res.String())
	}

	if err := s.updateConvCount(ctx, conversationID); err != nil {
		s.logger.Warn("update conversation count", zap.String("id", conversationID), zap.Error(err))
	}

	return nil
}

// esTermQuery builds a safe ES term query for the given field/value.
func esTermQuery(field, value string) string {
	q := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				field: value,
			},
		},
	}
	b, _ := json.Marshal(q)
	return string(b)
}

func (s *ESConversationStore) updateConvCount(ctx context.Context, conversationID string) error {
	query := esTermQuery("conversation_id", conversationID)
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

	update := map[string]interface{}{
		"doc": map[string]interface{}{
			"message_count": countResp.Count,
			"updated_at":    time.Now().UTC().Format(time.RFC3339),
		},
	}
	updateBody, _ := json.Marshal(update)
	upRes, err := s.client.Update(
		s.convIndex,
		conversationID,
		strings.NewReader(string(updateBody)),
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
	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal conversation doc: %w", err)
	}
	res, err := s.client.Index(s.convIndex, bytes.NewReader(body),
		s.client.Index.WithDocumentID(id),
		s.client.Index.WithRefresh("wait_for"),
	)
	if err != nil {
		s.logger.Error("es index conversation", zap.String("conv_id", id), zap.Error(err))
		return fmt.Errorf("index conversation: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		s.logger.Error("es index conversation error", zap.String("conv_id", id), zap.String("resp", res.String()))
		return fmt.Errorf("index conversation error: %s", res.String())
	}
	return nil
}

// Exists 判断会话元数据是否存在。404 视为不存在而非错误（业务正常态）。
func (s *ESConversationStore) Exists(ctx context.Context, id string) (bool, error) {
	res, err := s.client.Get(s.convIndex, id, s.client.Get.WithContext(ctx))
	if err != nil {
		return false, fmt.Errorf("get conversation: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		if res.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, fmt.Errorf("get conversation error: %s", res.String())
	}
	return true, nil
}

// UpdateTitle updates a conversation's title and updated_at timestamp.
func (s *ESConversationStore) UpdateTitle(ctx context.Context, id, title string) error {
	update := map[string]interface{}{
		"doc": map[string]interface{}{
			"title":      title,
			"updated_at": time.Now().UTC().Format(time.RFC3339),
		},
	}
	updateBody, _ := json.Marshal(update)
	upRes, err := s.client.Update(s.convIndex, id, strings.NewReader(string(updateBody)))
	if err != nil {
		return fmt.Errorf("update conversation title: %w", err)
	}
	defer upRes.Body.Close()
	if upRes.IsError() && upRes.StatusCode != 404 {
		return fmt.Errorf("update conversation title error: %s", upRes.String())
	}
	return nil
}

// List 列全部会话元数据，最新在前。
func (s *ESConversationStore) List(ctx context.Context) ([]ConversationMeta, error) {
	query := `{"query":{"match_all":{}},"sort":[{"updated_at":{"order":"desc"}}],"size":1000}`
	res, err := s.client.Search(
		s.client.Search.WithIndex(s.convIndex),
		s.client.Search.WithBody(strings.NewReader(query)),
	)
	if err != nil {
		s.logger.Error("es search conversations", zap.Error(err))
		return nil, fmt.Errorf("search conversations: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		s.logger.Error("es search conversations error", zap.String("resp", res.String()))
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

// Load 取会话消息，按时间升序。
func (s *ESConversationStore) Load(ctx context.Context, conversationID string) ([]*schema.Message, error) {
	query := map[string]interface{}{
		"query": map[string]interface{}{
			"term": map[string]interface{}{
				"conversation_id": conversationID,
			},
		},
		"sort": []map[string]interface{}{
			{"order": map[string]string{"order": "asc"}},
		},
		"size": 1000,
	}
	queryBody, _ := json.Marshal(query)
	res, err := s.client.Search(
		s.client.Search.WithIndex(s.msgIndex),
		s.client.Search.WithBody(strings.NewReader(string(queryBody))),
	)
	if err != nil {
		s.logger.Error("es search messages", zap.String("conv_id", conversationID), zap.Error(err))
		return nil, fmt.Errorf("search messages: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		s.logger.Error("es search messages error", zap.String("conv_id", conversationID), zap.String("resp", res.String()))
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

// Delete 删会话及其全部消息。
func (s *ESConversationStore) Delete(ctx context.Context, conversationID string) error {
	delRes, err := s.client.Delete(s.convIndex, conversationID,
		s.client.Delete.WithRefresh("wait_for"),
	)
	if err != nil {
		s.logger.Error("es delete conversation", zap.String("conv_id", conversationID), zap.Error(err))
		return fmt.Errorf("delete conversation: %w", err)
	}
	defer delRes.Body.Close()

	query := esTermQuery("conversation_id", conversationID)
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

// DeleteAll 清空全部会话及其消息。
func (s *ESConversationStore) DeleteAll(ctx context.Context) error {
	matchAll := `{"query":{"match_all":{}}}`
	for _, idx := range []string{s.convIndex, s.msgIndex} {
		res, err := s.client.DeleteByQuery(
			[]string{idx},
			strings.NewReader(matchAll),
			s.client.DeleteByQuery.WithRefresh(true),
		)
		if err != nil {
			return fmt.Errorf("delete all from %s: %w", idx, err)
		}
		defer res.Body.Close()
		if res.IsError() {
			return fmt.Errorf("delete all from %s error: %s", idx, res.String())
		}
	}
	return nil
}

var convCounter int64

// NewConversationID 生成唯一会话 ID。
func NewConversationID() string {
	convCounter++
	return fmt.Sprintf("conv_%d_%d", time.Now().UnixNano(), convCounter)
}
