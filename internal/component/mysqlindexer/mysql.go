package mysqlindexer

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/components/indexer"
	eino_schema "github.com/cloudwego/eino/schema"
	"gorm.io/gorm"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/service"
	"github.com/MrLuoooooo/MrLuoooooagent/internal/store"
)

// MySQLIndexer 把 chunk 行写入 MySQL documents 表，替代 ES 向量写入。
// FULLTEXT 检索不需要向量，元数据里的 vector 字段直接忽略。
// 同时实现 service.VectorDeleter：注入层对 indexer.Indexer 做类型断言即可
// 拿到删除能力，ES / MySQL 两条路走同一套 fx 装配（LSP）。
type MySQLIndexer struct {
	db *gorm.DB
}

// NewMySQLIndexer 建一个 MySQL chunk 写入器，满足 eino indexer.Indexer。
func NewMySQLIndexer(db *gorm.DB) indexer.Indexer {
	return &MySQLIndexer{db: db}
}

// Store 批量写入 chunk 行。chunk 元数据（document_id / chunk_type / title）
// 由摄入管线（pipeline.NewDocumentIngestionChain）放进 MetaData，此处直接落列。
func (m *MySQLIndexer) Store(ctx context.Context, docs []*eino_schema.Document, _ ...indexer.Option) ([]string, error) {
	if len(docs) == 0 {
		return nil, nil
	}

	now := time.Now()
	rows := make([]store.DocumentChunk, 0, len(docs))
	ids := make([]string, 0, len(docs))
	for _, doc := range docs {
		id := doc.ID
		if id == "" {
			id = fmt.Sprintf("chunk_%d_%d", now.UnixNano(), len(ids))
		}
		ids = append(ids, id)
		rows = append(rows, store.DocumentChunk{
			ID:        id,
			DocID:     metaString(doc.MetaData, "document_id"),
			ChunkType: metaString(doc.MetaData, "chunk_type"),
			Title:     metaString(doc.MetaData, "title"),
			Content:   doc.Content,
			CreatedAt: now,
		})
	}

	if err := m.db.WithContext(ctx).CreateInBatches(&rows, 100).Error; err != nil {
		return nil, fmt.Errorf("mysql indexer store: %w", err)
	}
	return ids, nil
}

// DeleteByDocumentID 删掉某父文档的全部 chunk 行，满足 service.VectorDeleter。
func (m *MySQLIndexer) DeleteByDocumentID(ctx context.Context, docID string) error {
	return store.DeleteDocumentChunks(m.db.WithContext(ctx), docID)
}

// metaString 从 eino 文档元数据里安全取字符串。
func metaString(meta map[string]any, key string) string {
	if meta == nil {
		return ""
	}
	v, _ := meta[key].(string)
	return v
}

var _ indexer.Indexer = (*MySQLIndexer)(nil)
var _ service.VectorDeleter = (*MySQLIndexer)(nil)
