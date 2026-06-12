package store

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/embedding"
	"github.com/elastic/go-elasticsearch/v8"
	"go.uber.org/zap"
)

// MemoryMeta 是 store 层对外暴露的记忆元数据，不含向量。
type MemoryMeta struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Keywords  []string  `json:"keywords"`
	Source    string    `json:"source"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Version   int       `json:"version"`
}

// esMemDoc 是 ES 内部文档，比 MemoryMeta 多 embedding 字段。
type esMemDoc struct {
	MemoryID  string    `json:"memory_id"`
	UserID    string    `json:"user_id"`
	Type      string    `json:"type"`
	Content   string    `json:"content"`
	Keywords  []string  `json:"keywords"`
	Source    string    `json:"source"`
	Status    string    `json:"status"`
	Embedding []float64 `json:"embedding"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Version   int       `json:"version"`
}

// searchHits 通用 ES 搜索结果解析结构。
type searchHits struct {
	Hits struct {
		Hits []struct {
			ID     string   `json:"_id"`
			Score  float64  `json:"_score"`
			Source esMemDoc `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// ESMemoryStore ES 持久化用户记忆，支持向量检索和乐观锁。
type ESMemoryStore struct {
	client    *elasticsearch.Client
	indexName string
	embedder  embedding.Embedder
	logger    *zap.Logger
}

// NewESMemoryStore 连 ES，自动建索引（含 dense_vector mapping）。
func NewESMemoryStore(
	client *elasticsearch.Client,
	indexName string,
	embedder embedding.Embedder,
	logger *zap.Logger,
	embeddingDim int,
) (*ESMemoryStore, error) {
	s := &ESMemoryStore{
		client:    client,
		indexName: indexName,
		embedder:  embedder,
		logger:    logger,
	}
	var lastErr error
	for i := 0; i < 15; i++ {
		if err := s.ensureIndex(context.Background(), embeddingDim); err != nil {
			lastErr = err
			logger.Warn("es memory index not ready, retrying...", zap.Int("attempt", i+1), zap.Error(err))
			time.Sleep(2 * time.Second)
			continue
		}
		return s, nil
	}
	return nil, fmt.Errorf("ensure memory index after 15 retries: %w", lastErr)
}

func (s *ESMemoryStore) ensureIndex(ctx context.Context, dim int) error {
	exists, err := s.client.Indices.Exists([]string{s.indexName})
	if err != nil {
		return err
	}
	if exists.StatusCode == 404 {
		mapping := fmt.Sprintf(`{"mappings":{"properties":{`+
			`"memory_id":{"type":"keyword"},`+
			`"user_id":{"type":"keyword"},`+
			`"type":{"type":"keyword"},`+
			`"content":{"type":"text","analyzer":"standard"},`+
			`"keywords":{"type":"text","analyzer":"standard"},`+
			`"source":{"type":"keyword"},`+
			`"status":{"type":"keyword"},`+
			`"embedding":{"type":"dense_vector","dims":%d,"index":true,"similarity":"cosine"},`+
			`"created_at":{"type":"date"},`+
			`"updated_at":{"type":"date"},`+
			`"version":{"type":"integer"}`+
			`}}}`, dim)
		res, err := s.client.Indices.Create(s.indexName, s.client.Indices.Create.WithBody(strings.NewReader(mapping)))
		if err != nil {
			return err
		}
		defer res.Body.Close()
		if res.IsError() {
			return fmt.Errorf("create memory index %s: %s", s.indexName, res.String())
		}
	}
	return nil
}

// Index 计算向量后写入一条记忆。
func (s *ESMemoryStore) Index(ctx context.Context, meta MemoryMeta) error {
	vectors, err := s.embedder.EmbedStrings(ctx, []string{meta.Content})
	if err != nil {
		return fmt.Errorf("embed memory content: %w", err)
	}

	now := time.Now().UTC()
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = now
	}
	meta.UpdatedAt = now

	doc := esMemDoc{
		MemoryID:  meta.ID,
		UserID:    meta.UserID,
		Type:      meta.Type,
		Content:   meta.Content,
		Keywords:  meta.Keywords,
		Source:    meta.Source,
		Status:    meta.Status,
		Embedding: vectors[0],
		CreatedAt: meta.CreatedAt,
		UpdatedAt: meta.UpdatedAt,
		Version:   meta.Version,
	}

	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("marshal memory doc: %w", err)
	}

	res, err := s.client.Index(s.indexName, bytes.NewReader(body),
		s.client.Index.WithDocumentID(meta.ID),
		s.client.Index.WithRefresh("wait_for"),
	)
	if err != nil {
		s.logger.Error("es index memory", zap.String("mem_id", meta.ID), zap.Error(err))
		return fmt.Errorf("index memory: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		s.logger.Error("es index memory error", zap.String("mem_id", meta.ID), zap.String("resp", res.String()))
		return fmt.Errorf("index memory error: %s", res.String())
	}
	return nil
}

// Search 向量检索 + userID + status=active 过滤。
func (s *ESMemoryStore) Search(ctx context.Context, userID, query string, topK int) ([]MemoryMeta, error) {
	vectors, err := s.embedder.EmbedStrings(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed search query: %w", err)
	}

	searchBody := map[string]any{
		"knn": map[string]any{
			"field":          "embedding",
			"query_vector":   vectors[0],
			"k":              topK,
			"num_candidates": topK * 10,
			"filter": []map[string]any{
				{"term": map[string]any{"user_id": userID}},
				{"term": map[string]any{"status": "active"}},
			},
		},
	}

	bodyBytes, _ := json.Marshal(searchBody)
	resp, err := s.client.Search(
		s.client.Search.WithContext(ctx),
		s.client.Search.WithIndex(s.indexName),
		s.client.Search.WithBody(bytes.NewReader(bodyBytes)),
	)
	if err != nil {
		return nil, fmt.Errorf("search memories: %w", err)
	}
	defer resp.Body.Close()
	if resp.IsError() {
		return nil, fmt.Errorf("search memories error: %s", resp.String())
	}

	var esResp searchHits
	if err := json.NewDecoder(resp.Body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	result := make([]MemoryMeta, len(esResp.Hits.Hits))
	for i, hit := range esResp.Hits.Hits {
		result[i] = docToMeta(hit.Source)
	}
	return result, nil
}

// Supersede 原子替换：写新记忆 ∧ 标记旧记忆为 superseded（版本乐观锁）。
func (s *ESMemoryStore) Supersede(ctx context.Context, userID, oldID string, oldVersion int, newMeta MemoryMeta) error {
	if err := s.Index(ctx, newMeta); err != nil {
		return fmt.Errorf("supersede: index new: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	scriptSource := `ctx._source.status = 'superseded'; ctx._source.updated_at = params.now`
	params := map[string]any{"now": now}

	scriptBody := map[string]any{
		"script": map[string]any{
			"source": scriptSource,
			"lang":   "painless",
			"params": params,
		},
	}

	if oldVersion > 0 {
		scriptBody["script"].(map[string]any)["source"] =
			`if (ctx._source.version != params.expected) { throw new IllegalArgumentException('version conflict') } ctx._source.status = 'superseded'; ctx._source.updated_at = params.now`
		scriptBody["script"].(map[string]any)["params"].(map[string]any)["expected"] = oldVersion
	}

	bodyBytes, _ := json.Marshal(scriptBody)
	res, err := s.client.Update(s.indexName, oldID, bytes.NewReader(bodyBytes),
		s.client.Update.WithRefresh("wait_for"),
	)
	if err != nil {
		return fmt.Errorf("supersede: update old: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		if res.StatusCode == 404 {
			s.logger.Warn("supersede: old memory not found", zap.String("old_id", oldID))
			return nil
		}
		return fmt.Errorf("supersede: update old: %s", res.String())
	}
	return nil
}

// FindByKeyword 按关键词 + userID + active 查已有记忆（去重用）。
func (s *ESMemoryStore) FindByKeyword(ctx context.Context, userID, keyword string) ([]MemoryMeta, error) {
	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": []map[string]any{
					{"term": map[string]any{"user_id": userID}},
					{"term": map[string]any{"status": "active"}},
					{"match": map[string]any{"keywords": keyword}},
				},
			},
		},
		"size": 5,
	}
	return s.executeSearch(ctx, query)
}

// List 列某用户全部 active 记忆，最新在前。
func (s *ESMemoryStore) List(ctx context.Context, userID string) ([]MemoryMeta, error) {
	query := map[string]any{
		"query": map[string]any{
			"bool": map[string]any{
				"must": []map[string]any{
					{"term": map[string]any{"user_id": userID}},
					{"term": map[string]any{"status": "active"}},
				},
			},
		},
		"sort": []map[string]any{{"updated_at": map[string]string{"order": "desc"}}},
		"size": 1000,
	}
	return s.executeSearch(ctx, query)
}

// Delete 删一条记忆。
func (s *ESMemoryStore) Delete(ctx context.Context, id string) error {
	res, err := s.client.Delete(s.indexName, id,
		s.client.Delete.WithRefresh("wait_for"),
	)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() && res.StatusCode != 404 {
		return fmt.Errorf("delete memory error: %s", res.String())
	}
	return nil
}

// DeleteAll 清空某用户全部记忆。
func (s *ESMemoryStore) DeleteAll(ctx context.Context, userID string) error {
	query := map[string]any{
		"query": map[string]any{
			"term": map[string]any{"user_id": userID},
		},
	}
	queryBody, _ := json.Marshal(query)
	res, err := s.client.DeleteByQuery(
		[]string{s.indexName},
		strings.NewReader(string(queryBody)),
		s.client.DeleteByQuery.WithRefresh(true),
	)
	if err != nil {
		return fmt.Errorf("delete all memories: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return fmt.Errorf("delete all memories error: %s", res.String())
	}
	return nil
}

// ---- 内部方法 ----

func (s *ESMemoryStore) executeSearch(ctx context.Context, query map[string]any) ([]MemoryMeta, error) {
	queryBody, _ := json.Marshal(query)
	res, err := s.client.Search(
		s.client.Search.WithIndex(s.indexName),
		s.client.Search.WithBody(strings.NewReader(string(queryBody))),
	)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer res.Body.Close()
	if res.IsError() {
		return nil, fmt.Errorf("search error: %s", res.String())
	}

	var esResp searchHits
	if err := json.NewDecoder(res.Body).Decode(&esResp); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	result := make([]MemoryMeta, len(esResp.Hits.Hits))
	for i, hit := range esResp.Hits.Hits {
		result[i] = docToMeta(hit.Source)
	}
	return result, nil
}

func docToMeta(doc esMemDoc) MemoryMeta {
	return MemoryMeta{
		ID:        doc.MemoryID,
		UserID:    doc.UserID,
		Type:      doc.Type,
		Content:   doc.Content,
		Keywords:  doc.Keywords,
		Source:    doc.Source,
		Status:    doc.Status,
		CreatedAt: doc.CreatedAt,
		UpdatedAt: doc.UpdatedAt,
		Version:   doc.Version,
	}
}
