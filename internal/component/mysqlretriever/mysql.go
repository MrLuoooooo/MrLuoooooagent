package mysqlretriever

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"gorm.io/gorm"
)

// MySQLRetriever 用 MySQL FULLTEXT(ngram) 做全文检索，替代 ES 向量检索。
// 只搜 chunk_type='child'（与 ES 版过滤 parent 的语义一致），
// 召回后交给现有 LLM reranker 精排，检索精度由 reranker 兜底。
type MySQLRetriever struct {
	db   *gorm.DB
	topK int
}

// NewMySQLRetriever 建一个 MySQL FULLTEXT 检索器，满足 eino retriever.Retriever。
func NewMySQLRetriever(db *gorm.DB, topK int) retriever.Retriever {
	if topK <= 0 {
		topK = 5
	}
	return &MySQLRetriever{db: db, topK: topK}
}

// Retrieve 用 MATCH ... AGAINST(NATURAL LANGUAGE MODE) 搜 topK 条最相关 child chunk。
// 空结果返回空切片而非 error——「没检索到」是正常业务态，不是故障，
// 上游 RAG 链据此走拒答分支而非 500。
func (r *MySQLRetriever) Retrieve(ctx context.Context, query string, opts ...retriever.Option) ([]*schema.Document, error) {
	if r.db == nil {
		return nil, fmt.Errorf("retriever: mysql db is nil")
	}
	options := retriever.GetCommonOptions(&retriever.Options{TopK: &r.topK}, opts...)
	topK := r.topK
	if options.TopK != nil && *options.TopK > 0 {
		topK = *options.TopK
	}

	// MATCH 相关性得分与 cosine 不同量纲，不套用 score_threshold（那是向量相似度的阈值）。
	var rows []struct {
		ID        string    `gorm:"column:id"`
		DocID     string    `gorm:"column:doc_id"`
		ChunkType string    `gorm:"column:chunk_type"`
		Title     string    `gorm:"column:title"`
		Content   string    `gorm:"column:content"`
		CreatedAt time.Time `gorm:"column:created_at"`
		Score     float64   `gorm:"column:score"`
	}
	err := r.db.WithContext(ctx).Raw(`
		SELECT id, doc_id, chunk_type, title, content, created_at,
		       MATCH(content) AGAINST(? IN NATURAL LANGUAGE MODE) AS score
		FROM documents
		WHERE chunk_type = 'child'
		  AND MATCH(content) AGAINST(? IN NATURAL LANGUAGE MODE)
		ORDER BY score DESC
		LIMIT ?`,
		query, query, topK,
	).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("retriever: mysql fulltext: %w", err)
	}

	docs := make([]*schema.Document, 0, len(rows))
	for _, row := range rows {
		meta := map[string]any{
			"document_id": row.DocID,
			"chunk_type":  row.ChunkType,
			"title":       row.Title,
			"created_at":  row.CreatedAt.Format(time.RFC3339),
			"_score":      row.Score,
		}
		docs = append(docs, &schema.Document{
			ID:       row.ID,
			Content:  row.Content,
			MetaData: meta,
		})
	}
	return docs, nil
}

var _ retriever.Retriever = (*MySQLRetriever)(nil)
