package tool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStockRealtimeTool_Info(t *testing.T) {
	st := NewStockRealtimeTool(t.TempDir())
	info, err := st.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "get_stock_realtime" {
		t.Errorf("name = %q", info.Name)
	}
}

func TestStockRealtimeTool_ReadFileSuccess(t *testing.T) {
	dir := t.TempDir()
	// Create mock data file
	stockDir := filepath.Join(dir, "realtime")
	os.MkdirAll(stockDir, 0755)
	now := time.Now()
	data := stockRealtime{
		Code: "sh600519", Name: "贵州茅台", Price: 1888.00,
		Open: 1880.00, High: 1900.00, Low: 1870.00,
		PreClose: 1870.00, Change: 18.00, ChangeRate: 0.96,
		Volume: 1000000, Amount: 1.888e9, Timestamp: now, Source: "sina",
	}
	fileData, _ := json.Marshal(data)
	os.WriteFile(filepath.Join(stockDir, "sh600519.json"), fileData, 0644)

	st := &StockRealtimeTool{dataDir: dir}
	result, err := st.InvokableRun(context.Background(), `{"codes": "sh600519"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(result, "贵州茅台") {
		t.Errorf("result should contain stock name, got: %s", result)
	}
	if !strings.Contains(result, "1888.00") {
		t.Errorf("result should contain price, got: %s", result)
	}
}

func TestStockRealtimeTool_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	st := &StockRealtimeTool{dataDir: dir}
	result, err := st.InvokableRun(context.Background(), `{"codes": "sh600519"}`)
	if err != nil {
		t.Fatalf("InvokableRun should not return error on missing data, got: %v", err)
	}
	// Should return a graceful message, not error
	if !strings.Contains(result, "无可用") {
		t.Errorf("expected graceful message, got: %s", result)
	}
}

func TestStockRealtimeTool_EmptyCodes(t *testing.T) {
	dir := t.TempDir()
	st := &StockRealtimeTool{dataDir: dir}
	_, err := st.InvokableRun(context.Background(), `{"codes": ""}`)
	if err == nil {
		t.Fatal("expected error for empty codes")
	}
}

func TestStockRealtimeTool_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	st := &StockRealtimeTool{dataDir: dir}
	_, err := st.InvokableRun(context.Background(), `not json`)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestStockRealtimeTool_MultipleCodes(t *testing.T) {
	dir := t.TempDir()
	stockDir := filepath.Join(dir, "realtime")
	os.MkdirAll(stockDir, 0755)
	// Create one valid and one missing
	data1 := stockRealtime{
		Code: "sh600519", Name: "茅台", Price: 1888.00,
		Change: 10.0, ChangeRate: 0.5, PreClose: 1878.00,
		Volume: 1000, Amount: 1e8, Timestamp: time.Now(), Source: "sina",
		Open: 1880, High: 1900, Low: 1870,
	}
	d1, _ := json.Marshal(data1)
	os.WriteFile(filepath.Join(stockDir, "sh600519.json"), d1, 0644)

	st := &StockRealtimeTool{dataDir: dir}
	result, err := st.InvokableRun(context.Background(), `{"codes": "sh600519,sz000001"}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(result, "茅台") {
		t.Errorf("should contain '茅台', got: %s", result)
	}
}

func TestStockKLineTool_Info(t *testing.T) {
	sk := NewStockKLineTool(t.TempDir())
	info, err := sk.Info(context.Background())
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.Name != "get_stock_kline" {
		t.Errorf("name = %q", info.Name)
	}
}

func TestStockKLineTool_ReadSuccess(t *testing.T) {
	dir := t.TempDir()
	histDir := filepath.Join(dir, "historical")
	os.MkdirAll(histDir, 0755)
	klines := []stockKLine{
		{Code: "sh600519", Time: "2026-01-01", Open: 1800, Close: 1850, High: 1860, Low: 1790, Volume: 100000, Amount: 1e8, Timestamp: time.Now()},
		{Code: "sh600519", Time: "2026-01-02", Open: 1850, Close: 1880, High: 1890, Low: 1840, Volume: 120000, Amount: 1.2e8, Timestamp: time.Now()},
		{Code: "sh600519", Time: "2026-01-03", Open: 1880, Close: 1900, High: 1910, Low: 1870, Volume: 150000, Amount: 1.5e8, Timestamp: time.Now()},
	}
	d, _ := json.Marshal(klines)
	os.WriteFile(filepath.Join(histDir, "sh600519_day.json"), d, 0644)

	sk := &StockKLineTool{dataDir: dir}
	result, err := sk.InvokableRun(context.Background(), `{"code": "sh600519", "period": "day", "limit": 5}`)
	if err != nil {
		t.Fatalf("InvokableRun: %v", err)
	}
	if !strings.Contains(result, "1850") {
		t.Errorf("result should contain close price, got: %s", result)
	}
}

func TestStockKLineTool_EmptyCode(t *testing.T) {
	sk := NewStockKLineTool(t.TempDir())
	result, err := sk.InvokableRun(context.Background(), `{"code": "", "period": "day"}`)
	if err == nil {
		t.Fatal("expected error for empty code")
	}
	_ = result
}

func TestStockKLineTool_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	sk := &StockKLineTool{dataDir: dir}
	result, err := sk.InvokableRun(context.Background(), `{"code": "sh600519", "period": "day"}`)
	if err != nil {
		t.Fatalf("InvokableRun should not return error on missing file: %v", err)
	}
	// Should return graceful message
	if !strings.Contains(result, "暂不可用") {
		t.Errorf("expected graceful message, got: %s", result)
	}
}
