package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// MySQL 文档表模型：RAG 检索从 ES 向量迁移到 MySQL FULLTEXT(ngram) 的落地表。
// 与 ES 版（ESDocumentStore / esindexer）平级，通过实现 service 层接口参与注入，
// 业务代码不感知后端差异（LSP）。

// DocumentChunk documents 表的行模型：一个 RAG chunk 一行。
// chunk_type=child 参与全文检索，parent 仅作上下文来源，meta 行不存在于本表。
type DocumentChunk struct {
	ID        string    `gorm:"column:id;primaryKey;size:64"`
	DocID     string    `gorm:"column:doc_id;size:64;index:idx_documents_doc_id"`
	ChunkType string    `gorm:"column:chunk_type;size:16;index:idx_documents_chunk_type"`
	Title     string    `gorm:"column:title;size:512;not null;default:''"`
	Content   string    `gorm:"column:content;type:mediumtext"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
}

func (DocumentChunk) TableName() string { return "documents" }

// mysqlDocMeta document_metas 表的行模型：文档级元数据（对应 ES 的 doc index）。
// 单独建表而非塞进 documents，避免文档级内容污染 FULLTEXT 索引。
type mysqlDocMeta struct {
	ID         string `gorm:"column:id;primaryKey;size:64"` // 父文档 uuid，同 DocMeta.ID
	Filename   string `gorm:"column:filename;size:512;not null;default:''"`
	ChunkCount int    `gorm:"column:chunk_count;not null;default:0"`
	Content    string `gorm:"column:content;type:mediumtext"`
	CreatedAt  string `gorm:"column:created_at;size:32;not null;default:''"` // RFC3339，同 ES 行为
}

func (mysqlDocMeta) TableName() string { return "document_metas" }

// MySQLDocumentStore 文档元数据的 MySQL 实现，满足 service.DocStore。
type MySQLDocumentStore struct {
	db *gorm.DB
}

// NewMySQLDocumentStore 建一个 MySQL 文档元数据存储。
func NewMySQLDocumentStore(db *gorm.DB) *MySQLDocumentStore {
	return &MySQLDocumentStore{db: db}
}

// Save 保存文档元数据（upsert，重复摄入同名文档时覆盖计数）。
func (s *MySQLDocumentStore) Save(_ context.Context, doc DocumentMeta) error {
	row := mysqlDocMeta{
		ID:         doc.ID,
		Filename:   doc.Filename,
		ChunkCount: doc.ChunkCount,
		Content:    doc.Content,
		CreatedAt:  doc.CreatedAt,
	}
	if err := s.db.Save(&row).Error; err != nil {
		return fmt.Errorf("mysql doc meta save: %w", err)
	}
	return nil
}

// Delete 删除文档元数据行。chunk 行由 VectorDeleter（mysqlindexer）负责。
func (s *MySQLDocumentStore) Delete(_ context.Context, id string) error {
	if err := s.db.Delete(&mysqlDocMeta{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("mysql doc meta delete: %w", err)
	}
	return nil
}

// List 列全部文档元数据，最新在前，与 ES 版行为一致。
func (s *MySQLDocumentStore) List(_ context.Context) ([]DocumentMeta, error) {
	var rows []mysqlDocMeta
	if err := s.db.Order("created_at DESC").Limit(1000).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("mysql doc meta list: %w", err)
	}
	result := make([]DocumentMeta, len(rows))
	for i, r := range rows {
		result[i] = DocumentMeta{
			ID:         r.ID,
			Filename:   r.Filename,
			ChunkCount: r.ChunkCount,
			CreatedAt:  r.CreatedAt,
			Content:    r.Content,
		}
	}
	return result, nil
}

// DeleteDocumentChunks 删除某父文档的全部 chunk 行。
// mysqlindexer（service.VectorDeleter）与 mysqlretriever 共用此实现，删除逻辑单点维护。
func DeleteDocumentChunks(db *gorm.DB, docID string) error {
	if err := db.Where("doc_id = ?", docID).Delete(&DocumentChunk{}).Error; err != nil {
		return fmt.Errorf("mysql delete document chunks: %w", err)
	}
	return nil
}

// ensureFulltextIndexes 为 documents 建 ft_content FULLTEXT 索引。
// 仅 MySQL 方言执行（sqlite 测试库跳过）；ngram parser 要求 MySQL >= 8.0，
// 版本不足直接报错暴露，而不是静默退化成查不到结果。
func ensureFulltextIndexes(db *gorm.DB) error {
	if db.Dialector.Name() != "mysql" {
		return nil
	}
	var version string
	if err := db.Raw("SELECT VERSION()").Scan(&version).Error; err != nil {
		return fmt.Errorf("mysql select version: %w", err)
	}
	major, err := parseMajorVersion(version)
	if err != nil {
		return err
	}
	if major < 8 {
		return fmt.Errorf("mysql fulltext ngram parser requires >= 8.0, got %s", version)
	}

	var count int64
	if err := db.Raw(
		"SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'documents' AND index_name = 'ft_content'",
	).Scan(&count).Error; err != nil {
		return fmt.Errorf("mysql check fulltext index: %w", err)
	}
	if count > 0 {
		return nil
	}
	if err := db.Exec("CREATE FULLTEXT INDEX ft_content ON documents(content) WITH PARSER ngram").Error; err != nil {
		return fmt.Errorf("mysql create fulltext index: %w", err)
	}
	return nil
}

// parseMajorVersion 从 "8.0.36" / "8.4.2-log" 里取主版本号。
func parseMajorVersion(version string) (int, error) {
	majorStr, _, _ := strings.Cut(version, ".")
	major, err := strconv.Atoi(strings.TrimSpace(majorStr))
	if err != nil {
		return 0, fmt.Errorf("parse mysql version %q: %w", version, err)
	}
	return major, nil
}
