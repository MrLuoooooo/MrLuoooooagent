package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	_ "modernc.org/sqlite"
)

// SQLiteStockDB StockDB 的 SQLite 实现。
// 纯 Go (modernc.org/sqlite)，无 CGO 依赖。
// 线程安全：读写操作由 RWMutex 保护。
type SQLiteStockDB struct {
	db    *sql.DB
	mu    sync.RWMutex
	count int
}

// NewSQLite 创建并初始化 SQLite 股票数据库。
// dataDir 是数据目录路径，数据库文件为 {dataDir}/stocks.db。
// 如果数据库不存在，调用者应在注册后调用 Refresh()。
func NewSQLite(dataDir string) (*SQLiteStockDB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("stock db: create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "stocks.db")
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("stock db: open: %w", err)
	}

	// 连接池：SQLite 单写者，连接数设为 1。
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	s := &SQLiteStockDB{db: db}

	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("stock db: migrate: %w", err)
	}

	s.count = s.loadCount()
	return s, nil
}

func (s *SQLiteStockDB) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS stocks (
			code       TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			industry   TEXT NOT NULL DEFAULT '',
			market_cap REAL NOT NULL DEFAULT 0,
			pe         REAL NOT NULL DEFAULT 0,
			pb         REAL NOT NULL DEFAULT 0
		);
		CREATE INDEX IF NOT EXISTS idx_stocks_name ON stocks(name);
		CREATE INDEX IF NOT EXISTS idx_stocks_industry ON stocks(industry);
	`)
	return err
}

func (s *SQLiteStockDB) loadCount() int {
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM stocks").Scan(&n); err != nil {
		return 0
	}
	return n
}

// ── StockDB 接口实现 ────────────────────────────

func (s *SQLiteStockDB) Search(keyword string, limit int) ([]StockBasic, error) {
	if limit <= 0 {
		limit = 15
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(
		`SELECT code, name, industry FROM stocks
		 WHERE code LIKE ? OR name LIKE ? OR industry LIKE ?
		 ORDER BY market_cap DESC
		 LIMIT ?`,
		"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%", limit,
	)
	if err != nil {
		return nil, fmt.Errorf("stock db: search: %w", err)
	}
	defer rows.Close()

	var result []StockBasic
	for rows.Next() {
		var r StockBasic
		if err := rows.Scan(&r.Code, &r.Name, &r.Industry); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *SQLiteStockDB) GetByCode(code string) (*StockBasic, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var r StockBasic
	err := s.db.QueryRow(
		`SELECT code, name, industry, market_cap, pe, pb FROM stocks WHERE code = ?`,
		code,
	).Scan(&r.Code, &r.Name, &r.Industry, &r.MarketCap, &r.PE, &r.PB)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stock db: get: %w", err)
	}
	return &r, nil
}

func (s *SQLiteStockDB) List(filter StockFilter) ([]StockBasic, error) {
	if filter.Limit <= 0 {
		filter.Limit = 100
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	qb := queryBuilder{query: "SELECT code, name, industry, market_cap, pe, pb FROM stocks WHERE 1=1"}
	qb.addIf(filter.Industry != "", "AND industry LIKE ?", "%"+filter.Industry+"%")
	qb.addIf(filter.MinMarketCap > 0, "AND market_cap >= ?", filter.MinMarketCap)
	qb.addIf(filter.MaxMarketCap > 0, "AND market_cap <= ?", filter.MaxMarketCap)
	qb.addIf(filter.MinPE > 0, "AND pe > 0 AND pe >= ?", filter.MinPE)
	qb.addIf(filter.MaxPE > 0, "AND pe > 0 AND pe <= ?", filter.MaxPE)
	qb.addIf(filter.MinPB > 0, "AND pb > 0 AND pb >= ?", filter.MinPB)
	qb.addIf(filter.MaxPB > 0, "AND pb > 0 AND pb <= ?", filter.MaxPB)
	qb.query += " ORDER BY market_cap DESC LIMIT ?"
	qb.args = append(qb.args, filter.Limit)

	rows, err := s.db.Query(qb.query, qb.args...)
	if err != nil {
		return nil, fmt.Errorf("stock db: list: %w", err)
	}
	defer rows.Close()

	var result []StockBasic
	for rows.Next() {
		var r StockBasic
		if err := rows.Scan(&r.Code, &r.Name, &r.Industry, &r.MarketCap, &r.PE, &r.PB); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *SQLiteStockDB) Refresh() error {
	stocks, err := fetchAllStocks()
	if err != nil {
		return fmt.Errorf("stock db: refresh fetch: %w", err)
	}
	if len(stocks) == 0 {
		return fmt.Errorf("stock db: refresh fetched 0 stocks")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("stock db: refresh tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM stocks"); err != nil {
		return fmt.Errorf("stock db: refresh delete: %w", err)
	}

	stmt, err := tx.Prepare("INSERT OR REPLACE INTO stocks(code, name, industry, market_cap, pe, pb) VALUES(?,?,?,?,?,?)")
	if err != nil {
		return fmt.Errorf("stock db: refresh prepare: %w", err)
	}
	defer stmt.Close()

	for _, st := range stocks {
		if _, err := stmt.Exec(st.Code, st.Name, st.Industry, st.MarketCap, st.PE, st.PB); err != nil {
			return fmt.Errorf("stock db: refresh insert %s: %w", st.Code, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("stock db: refresh commit: %w", err)
	}

	s.count = len(stocks)
	return nil
}

func (s *SQLiteStockDB) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.count
}

func (s *SQLiteStockDB) Close() error {
	return s.db.Close()
}

// ── queryBuilder ─────────────────────────────────

type queryBuilder struct {
	query string
	args  []interface{}
}

func (qb *queryBuilder) addIf(cond bool, clause string, arg interface{}) {
	if cond {
		qb.query += " " + clause
		qb.args = append(qb.args, arg)
	}
}

// ── code normalization ───────────────────────────

// NormalizeCode 将各种代码格式统一为 sh600519 / sz000001。
func NormalizeCode(raw string) string {
	raw = strings.TrimSpace(strings.ToUpper(raw))
	// 已是规范格式
	if (strings.HasPrefix(raw, "SH") || strings.HasPrefix(raw, "SZ")) && len(raw) == 8 {
		return strings.ToLower(raw)
	}
	// 纯数字：6开头 → sh, 0/3开头 → sz
	if len(raw) == 6 {
		for _, c := range raw {
			if c < '0' || c > '9' {
				return raw
			}
		}
		if raw[0] == '6' {
			return "sh" + raw
		}
		return "sz" + raw
	}
	return strings.ToLower(raw)
}
