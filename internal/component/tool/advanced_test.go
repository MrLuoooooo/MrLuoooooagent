package tool

import (
	"context"
	"encoding/json"
	"testing"

	stockdb "github.com/MrLuoooooo/MrLuoooooagent/internal/stock/db"
)

// ── Calculator ──────────────────────────────────────────

func TestCalculator_Basic(t *testing.T) {
	ct := &CalculatorTool{}
	cases := []struct {
		expr    string
		want    string
		wantErr bool
	}{
		{"1+2", "1+2 = 3", false},
		{"10-3", "10-3 = 7", false},
		{"4*5", "4*5 = 20", false},
		{"20/4", "20/4 = 5", false},
		{"10%3", "10%3 = 1", false},
		{"2^3", "2^3 = 8", false},
		{"(1+2)*3", "(1+2)*3 = 9", false},
		{"sqrt(144)", "sqrt(144) = 12", false},
		{"abs(-5)", "abs(-5) = 5", false},
		{"round(3.14159,2)", "round(3.14159,2) = 3.1400", false},
		{"100*(1+0.05)^5", "100*(1+0.05)^5 = 127.6282", false},
	}
	for _, tc := range cases {
		args, _ := json.Marshal(map[string]string{"expression": tc.expr})
		got, err := ct.InvokableRun(context.Background(), string(args))
		if tc.wantErr {
			if err == nil {
				t.Errorf("%s: expected error", tc.expr)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.expr, err)
			continue
		}
		if got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.expr, got, tc.want)
		}
	}
}

func TestCalculator_EmptyExpression(t *testing.T) {
	ct := &CalculatorTool{}
	args, _ := json.Marshal(map[string]string{"expression": ""})
	_, err := ct.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Error("expected error for empty expression")
	}
}

func TestCalculator_InvalidChars(t *testing.T) {
	ct := &CalculatorTool{}
	// SQL injection attempt
	args, _ := json.Marshal(map[string]string{"expression": "1; DROP TABLE users"})
	result, err := ct.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 检查返回的是错误提示而非执行结果
	if result == "" {
		t.Error("result should not be empty")
	}
}

func TestCalculator_DivisionByZero(t *testing.T) {
	ct := &CalculatorTool{}
	args, _ := json.Marshal(map[string]string{"expression": "1/0"})
	result, err := ct.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// InvokableRun 把除零错误包装成字符串返回
	if result == "" {
		t.Error("result should contain error message")
	}
}

// ── JSONTool ────────────────────────────────────────────

func TestJSONTool_Format(t *testing.T) {
	jt := &JSONTool{}
	args, _ := json.Marshal(map[string]string{
		"json_str": `{"name":"test","value":42}`,
	})
	result, err := jt.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("result should not be empty")
	}
}

func TestJSONTool_PathQuery(t *testing.T) {
	jt := &JSONTool{}
	args, _ := json.Marshal(map[string]string{
		"json_str": `{"data":{"items":[{"name":"first"},{"name":"second"}]}}`,
		"path":     "data.items[0].name",
	})
	result, err := jt.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `"first"` {
		t.Errorf("got %q, want \"first\"", result)
	}
}

func TestJSONTool_InvalidJSON(t *testing.T) {
	jt := &JSONTool{}
	args, _ := json.Marshal(map[string]string{
		"json_str": `{invalid json}`,
	})
	result, err := jt.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 错误包装成字符串返回
	if result == "" {
		t.Error("result should contain error message")
	}
}

// ── TextTools ───────────────────────────────────────────

func TestTextTools_Count(t *testing.T) {
	tt := &TextTools{}
	args, _ := json.Marshal(map[string]string{
		"text":      "hello world\nline two",
		"operation": "count",
	})
	result, err := tt.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("result should not be empty")
	}
}

func TestTextTools_Lines(t *testing.T) {
	tt := &TextTools{}
	input := "a\nb\nc"
	args, _ := json.Marshal(map[string]string{
		"text":      input,
		"operation": "lines",
	})
	result, err := tt.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != input {
		t.Errorf("got %q, want %q", result, input)
	}
}

func TestTextTools_Sort(t *testing.T) {
	tt := &TextTools{}
	args, _ := json.Marshal(map[string]string{
		"text":      "c\na\nb",
		"operation": "sort",
	})
	result, err := tt.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "a\nb\nc" {
		t.Errorf("got %q, want \"a\\nb\\nc\"", result)
	}
}

func TestTextTools_Unique(t *testing.T) {
	tt := &TextTools{}
	args, _ := json.Marshal(map[string]string{
		"text":      "a\nb\na\nc\nb",
		"operation": "unique",
	})
	result, err := tt.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "a\nb\nc" {
		t.Errorf("got %q, want \"a\\nb\\nc\"", result)
	}
}

func TestTextTools_Head(t *testing.T) {
	tt := &TextTools{}
	input := map[string]interface{}{
		"text":      "1\n2\n3\n4\n5",
		"operation": "head",
		"n":         float64(2),
	}
	args, _ := json.Marshal(input)
	result, err := tt.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "1\n2" {
		t.Errorf("got %q, want \"1\\n2\"", result)
	}
}

func TestTextTools_Tail(t *testing.T) {
	tt := &TextTools{}
	input := map[string]interface{}{
		"text":      "1\n2\n3\n4\n5",
		"operation": "tail",
		"n":         float64(2),
	}
	args, _ := json.Marshal(input)
	result, err := tt.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "4\n5" {
		t.Errorf("got %q, want \"4\\n5\"", result)
	}
}

func TestTextTools_InvalidOperation(t *testing.T) {
	tt := &TextTools{}
	args, _ := json.Marshal(map[string]string{
		"text":      "hello",
		"operation": "nonexistent",
	})
	result, err := tt.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("result should contain error message")
	}
}

// ── WebFetchTool ────────────────────────────────────────

func TestWebFetchTool_Disabled(t *testing.T) {
	wf := NewWebFetchTool(false)
	args, _ := json.Marshal(map[string]string{"url": "http://example.com"})
	result, err := wf.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("result should indicate tool is disabled")
	}
}

func TestWebFetchTool_EmptyURL(t *testing.T) {
	wf := NewWebFetchTool(true)
	args, _ := json.Marshal(map[string]string{"url": ""})
	_, err := wf.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

// ── StockSearchTool ─────────────────────────────────────

// stubStockDB is a minimal StockDB for tests.
type stubStockDB struct{}

func (s *stubStockDB) Search(keyword string, limit int) ([]stockdb.StockBasic, error) {
	if keyword == "茅台" {
		return []stockdb.StockBasic{
			{Code: "sh600519", Name: "贵州茅台", Industry: "白酒"},
		}, nil
	}
	return nil, nil
}
func (s *stubStockDB) GetByCode(code string) (*stockdb.StockBasic, error) { return nil, nil }
func (s *stubStockDB) List(filter stockdb.StockFilter) ([]stockdb.StockBasic, error) { return nil, nil }
func (s *stubStockDB) Refresh() error { return nil }
func (s *stubStockDB) Count() int { return 0 }

func TestStockSearchTool_Found(t *testing.T) {
	st := NewStockSearchTool(&stubStockDB{})
	args, _ := json.Marshal(map[string]string{"keyword": "茅台"})
	result, err := st.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("result should not be empty")
	}
}

func TestStockSearchTool_NotFound(t *testing.T) {
	st := NewStockSearchTool(&stubStockDB{})
	args, _ := json.Marshal(map[string]string{"keyword": "这个股票不存在"})
	result, err := st.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("result should not be empty")
	}
}

func TestStockSearchTool_EmptyKeyword(t *testing.T) {
	st := NewStockSearchTool(&stubStockDB{})
	args, _ := json.Marshal(map[string]string{"keyword": ""})
	_, err := st.InvokableRun(context.Background(), string(args))
	if err == nil {
		t.Error("expected error for empty keyword")
	}
}

// ── StockIndexTool ──────────────────────────────────────

func TestStockIndexTool_All(t *testing.T) {
	si := &StockIndexTool{}
	args, _ := json.Marshal(map[string]string{})
	result, err := si.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("result should not be empty")
	}
}

func TestStockIndexTool_Specific(t *testing.T) {
	si := &StockIndexTool{}
	args, _ := json.Marshal(map[string]string{"name": "上证"})
	result, err := si.InvokableRun(context.Background(), string(args))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("result should not be empty")
	}
}
