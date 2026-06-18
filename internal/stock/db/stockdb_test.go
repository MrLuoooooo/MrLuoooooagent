package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// newTestDB 创建内存 SQLite 测试数据库。
func newTestDB(t *testing.T) *SQLiteStockDB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	db.SetMaxOpenConns(1)
	s := &SQLiteStockDB{db: db}
	if err := s.migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// 插入测试数据
	_, err = db.Exec(`INSERT INTO stocks(code, name, industry, market_cap, pe, pb) VALUES
		('sh600519', '贵州茅台', '白酒', 22000, 25.5, 8.3),
		('sz000858', '五粮液', '白酒', 8000, 18.2, 5.6),
		('sh601398', '工商银行', '银行', 19000, 5.8, 0.6),
		('sz300750', '宁德时代', '新能源电池', 9500, 22.0, 6.5),
		('sh688981', '中芯国际', '半导体', 3500, 45.0, 3.2)`)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	s.count = 5
	return s
}

func TestSQLiteStockDB_Search(t *testing.T) {
	s := newTestDB(t)
	defer s.Close()

	t.Run("by name", func(t *testing.T) {
		r, err := s.Search("茅台", 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(r) != 1 || r[0].Code != "sh600519" {
			t.Fatalf("expected 贵州茅台, got %v", r)
		}
	})

	t.Run("by industry", func(t *testing.T) {
		r, err := s.Search("白酒", 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(r) != 2 {
			t.Fatalf("expected 2 白酒 stocks, got %d", len(r))
		}
	})

	t.Run("by code", func(t *testing.T) {
		r, err := s.Search("300750", 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(r) != 1 || r[0].Name != "宁德时代" {
			t.Fatal("expected 宁德时代")
		}
	})

	t.Run("no match", func(t *testing.T) {
		r, err := s.Search("不存在的股票", 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(r) != 0 {
			t.Fatalf("expected empty, got %d", len(r))
		}
	})

	t.Run("limit", func(t *testing.T) {
		r, err := s.Search("", 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(r) != 2 {
			t.Fatalf("expected 2, got %d", len(r))
		}
	})
}

func TestSQLiteStockDB_GetByCode(t *testing.T) {
	s := newTestDB(t)
	defer s.Close()

	r, err := s.GetByCode("sh600519")
	if err != nil {
		t.Fatal(err)
	}
	if r == nil || r.Name != "贵州茅台" || r.MarketCap != 22000 {
		t.Fatalf("unexpected: %+v", r)
	}

	r, err = s.GetByCode("sh999999")
	if err != nil {
		t.Fatal(err)
	}
	if r != nil {
		t.Fatal("expected nil for nonexistent code")
	}
}

func TestSQLiteStockDB_List(t *testing.T) {
	s := newTestDB(t)
	defer s.Close()

	t.Run("by industry", func(t *testing.T) {
		r, err := s.List(StockFilter{Industry: "白酒", Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(r) != 2 {
			t.Fatalf("expected 2, got %d", len(r))
		}
	})

	t.Run("by PE range", func(t *testing.T) {
		r, err := s.List(StockFilter{MaxPE: 10, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(r) != 1 || r[0].Code != "sh601398" {
			t.Fatalf("expected 工商银行, got %v", r)
		}
	})

	t.Run("by market cap", func(t *testing.T) {
		r, err := s.List(StockFilter{MinMarketCap: 10000, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(r) < 2 {
			t.Fatalf("expected ≥2 large-cap, got %d", len(r))
		}
	})

	t.Run("combined", func(t *testing.T) {
		r, err := s.List(StockFilter{
			Industry:  "白酒",
			MaxPE:    20,
			MinPB:    5,
			Limit:    10,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(r) != 1 || r[0].Code != "sz000858" {
			t.Fatalf("expected 五粮液, got %v", r)
		}
	})
}

func TestSQLiteStockDB_Count(t *testing.T) {
	s := newTestDB(t)
	defer s.Close()
	if s.Count() != 5 {
		t.Fatalf("expected 5, got %d", s.Count())
	}
}

func TestNormalizeCode(t *testing.T) {
	cases := []struct{ in, want string }{
		{"sh600519", "sh600519"},
		{"SH600519", "sh600519"},
		{"600519", "sh600519"},
		{"sz000001", "sz000001"},
		{"000001", "sz000001"},
		{"300750", "sz300750"},
		{"688981", "sh688981"},
		{"abc", "abc"},
	}
	for _, c := range cases {
		got := NormalizeCode(c.in)
		if got != c.want {
			t.Errorf("NormalizeCode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
