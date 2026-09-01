package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// mysqlMemory 长期记忆表模型（P1：去 ES 依赖，关键词检索，省 embedding）。
// keywords 以 JSON 数组字符串存储，检索用 LIKE 匹配——个人规模够用。
type mysqlMemory struct {
	ID          string     `gorm:"column:id;primaryKey;size:64"`
	UserID      string     `gorm:"column:user_id;size:64;index:idx_mem_user_status,priority:1"`
	Type        string     `gorm:"column:type;size:32;not null"`
	Content     string     `gorm:"column:content;type:text"`
	Keywords    string     `gorm:"column:keywords;type:json;default:null"`
	Source      string     `gorm:"column:source;size:128;not null;default:''"`
	Status      string     `gorm:"column:status;size:32;not null;default:'active';index:idx_mem_user_status,priority:2"`
	Version     int        `gorm:"column:version;not null;default:1"`
	MemoryLayer string     `gorm:"column:memory_layer;size:8;not null;default:''"`
	Confidence  float64    `gorm:"column:confidence;not null;default:0.8"`
	ValidUntil  *time.Time `gorm:"column:valid_until;default:null"` // 零值存 NULL
	CreatedAt   time.Time  `gorm:"column:created_at;not null"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;not null"`
}

func (mysqlMemory) TableName() string { return "memories" }

// MySQLMemoryStore 长期记忆 MySQL 实现，满足 service.MemoryStore 接口（不改接口签名）。
type MySQLMemoryStore struct {
	db *gorm.DB
}

// NewMySQLMemoryStore —
func NewMySQLMemoryStore(db *gorm.DB) *MySQLMemoryStore {
	return &MySQLMemoryStore{db: db}
}

// Index 写入/更新一条记忆（按主键 upsert）。
func (s *MySQLMemoryStore) Index(_ context.Context, meta MemoryMeta) error {
	row, err := memoryMetaToRow(meta)
	if err != nil {
		return err
	}
	if err := s.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(row).Error; err != nil {
		return fmt.Errorf("mysql index memory: %w", err)
	}
	return nil
}

// Search 关键词检索：content 或 keywords LIKE 匹配，按置信度+新鲜度排序。
func (s *MySQLMemoryStore) Search(_ context.Context, userID, query string, topK int) ([]MemoryMeta, error) {
	pattern := "%" + likeEscape(query) + "%"
	var rows []mysqlMemory
	err := s.db.Where(
		"user_id = ? AND status = ? AND (content LIKE ? OR keywords LIKE ?)",
		userID, "active", pattern, pattern,
	).Order("confidence DESC, updated_at DESC").Limit(topK).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("mysql search memories: %w", err)
	}
	return memoryRowsToMetas(rows), nil
}

// Supersede 事务内原子替换：版本乐观锁标旧为 superseded，再写新记忆。
func (s *MySQLMemoryStore) Supersede(_ context.Context, userID, oldID string, oldVersion int, newMeta MemoryMeta) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		q := tx.Model(&mysqlMemory{}).
			Where("id = ? AND user_id = ?", oldID, userID)
		if oldVersion > 0 {
			q = q.Where("version = ?", oldVersion)
		}
		res := q.Updates(map[string]any{"status": "superseded", "updated_at": time.Now().UTC()})
		if res.Error != nil {
			return fmt.Errorf("supersede mark old: %w", res.Error)
		}
		if oldVersion > 0 && res.RowsAffected == 0 {
			return fmt.Errorf("supersede version conflict: memory %s expected version %d", oldID, oldVersion)
		}
		row, err := memoryMetaToRow(newMeta)
		if err != nil {
			return err
		}
		if err := tx.Create(row).Error; err != nil {
			return fmt.Errorf("supersede insert new: %w", err)
		}
		return nil
	})
}

// FindByKeyword 按关键词查 active 记忆（去重用），最新在前。
func (s *MySQLMemoryStore) FindByKeyword(_ context.Context, userID, keyword string) ([]MemoryMeta, error) {
	var rows []mysqlMemory
	err := s.db.Where(
		"user_id = ? AND status = ? AND keywords LIKE ?",
		userID, "active", "%"+likeEscape(keyword)+"%",
	).Order("updated_at DESC").Limit(5).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("mysql find memory by keyword: %w", err)
	}
	return memoryRowsToMetas(rows), nil
}

// List 列某用户全部 active 记忆，最新在前。
func (s *MySQLMemoryStore) List(_ context.Context, userID string) ([]MemoryMeta, error) {
	var rows []mysqlMemory
	err := s.db.Where("user_id = ? AND status = ?", userID, "active").
		Order("updated_at DESC").Limit(1000).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("mysql list memories: %w", err)
	}
	return memoryRowsToMetas(rows), nil
}

// Delete 删一条记忆。
func (s *MySQLMemoryStore) Delete(_ context.Context, id string) error {
	if err := s.db.Delete(&mysqlMemory{}, "id = ?", id).Error; err != nil {
		return fmt.Errorf("mysql delete memory: %w", err)
	}
	return nil
}

// DeleteAll 清空某用户全部记忆。
func (s *MySQLMemoryStore) DeleteAll(_ context.Context, userID string) error {
	if err := s.db.Delete(&mysqlMemory{}, "user_id = ?", userID).Error; err != nil {
		return fmt.Errorf("mysql delete all memories: %w", err)
	}
	return nil
}

// ——— 内部转换 ———

func memoryMetaToRow(meta MemoryMeta) (*mysqlMemory, error) {
	kw, err := json.Marshal(meta.Keywords)
	if err != nil {
		return nil, fmt.Errorf("marshal memory keywords: %w", err)
	}
	now := time.Now().UTC()
	if meta.CreatedAt.IsZero() {
		meta.CreatedAt = now
	}
	if meta.UpdatedAt.IsZero() {
		meta.UpdatedAt = now
	}
	var validUntil *time.Time
	if !meta.ValidUntil.IsZero() {
		vu := meta.ValidUntil
		validUntil = &vu
	}
	return &mysqlMemory{
		ID:          meta.ID,
		UserID:      meta.UserID,
		Type:        meta.Type,
		Content:     meta.Content,
		Keywords:    string(kw),
		Source:      meta.Source,
		Status:      meta.Status,
		Version:     meta.Version,
		MemoryLayer: meta.MemoryLayer,
		Confidence:  meta.Confidence,
		ValidUntil:  validUntil,
		CreatedAt:   meta.CreatedAt,
		UpdatedAt:   meta.UpdatedAt,
	}, nil
}

func memoryRowToMeta(row mysqlMemory) MemoryMeta {
	var keywords []string
	if row.Keywords != "" {
		_ = json.Unmarshal([]byte(row.Keywords), &keywords)
	}
	meta := MemoryMeta{
		ID:          row.ID,
		UserID:      row.UserID,
		Type:        row.Type,
		Content:     row.Content,
		Keywords:    keywords,
		Source:      row.Source,
		Status:      row.Status,
		Version:     row.Version,
		MemoryLayer: row.MemoryLayer,
		Confidence:  row.Confidence,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
	if row.ValidUntil != nil {
		meta.ValidUntil = *row.ValidUntil
	}
	return meta
}

func memoryRowsToMetas(rows []mysqlMemory) []MemoryMeta {
	metas := make([]MemoryMeta, len(rows))
	for i, r := range rows {
		metas[i] = memoryRowToMeta(r)
	}
	return metas
}

// likeEscape 转义 LIKE 通配符，防止用户输入中的 %/_ 破坏匹配语义。
func likeEscape(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\', '%', '_':
			out = append(out, '\\', s[i])
		default:
			out = append(out, s[i])
		}
	}
	return string(out)
}
