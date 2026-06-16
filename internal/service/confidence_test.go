package service

import (
	"context"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"
)

func TestConfidenceToolAlignment_StockData(t *testing.T) {
	svc := NewConfidenceService(zap.NewNop())

	// 模拟股票工具返回
	toolResults := []*schema.Message{
		{Role: schema.Tool, Content: "## 实时行情 (1只股票)\n\n### 贵州茅台 sh600519\n- **现价**: ¥1255.67  |  **涨跌**: ↓-15.43 (-1.21%)  |  **昨收**: ¥1271.10\n- 开盘 ¥1267.01  /  最高 ¥1267.88  /  最低 ¥1255.00\n- 成交量: 2345678股  /  成交额: ¥29.45亿  /  来源: sina\n- 更新时间: 2026-06-16 15:00:00"},
	}

	// Case 1: 模型正确引用了工具数据
	result := svc.Evaluate(context.Background(),
		"贵州茅台当前价格 ¥1255.67，较昨日下跌 1.21%，成交量 234 万股。",
		toolResults, nil)

	t.Logf("Case 1 score=%.2f level=%s factors=%v",
		result.Score, result.Level, result.Factors)
	if result.Score < 0.75 {
		t.Errorf("faithful response score should be high, got %.2f", result.Score)
	}

	// Case 2: 模型编造了错误价格
	result2 := svc.Evaluate(context.Background(),
		"贵州茅台现价约 1000 元，今日小幅波动。",
		toolResults, nil)

	t.Logf("Case 2 score=%.2f level=%s factors=%v",
		result2.Score, result2.Level, result2.Factors)
	if result2.Score >= result.Score - 0.05 {
		t.Errorf("hallucinated price should score significantly lower than faithful, got %.2f vs %.2f", result2.Score, result.Score)
	}

	// Case 3: 无工具结果，回退行为
	result3 := svc.Evaluate(context.Background(),
		"茅台大概在 1255 左右吧。", nil, nil)

	t.Logf("Case 3 score=%.2f level=%s factors=%v",
		result3.Score, result3.Level, result3.Factors)
	if result3.Level == LevelHigh {
		t.Errorf("uncertain language + no tools should reduce confidence")
	}
}

func TestConfidenceStockNumericalAlignment(t *testing.T) {
	svc := NewConfidenceService(zap.NewNop())

	// API 返回 1255.67
	toolResults := []*schema.Message{
		{Role: schema.Tool, Content: "## 实时行情\n\n### 贵州茅台 sh600519\n- **现价**: ¥1255.67"},
	}

	tests := []struct {
		name         string
		modelOutput  string
		expectHigh   bool
	}{
		{"精确匹配", "茅台现价 1255.67 元", true},
		{"去掉小数位", "茅台现价 1255 元", true}, // 1255 包含在 1255.67 中
		{"完全错误", "茅台现价 800 元", false},
		{"编造股票", "腾讯控股今日上涨 2%", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.Evaluate(context.Background(), tt.modelOutput, toolResults, nil)
			t.Logf("%s: score=%.2f level=%s", tt.name, result.Score, result.Level)
			if tt.expectHigh && result.Score < 0.6 {
				t.Errorf("expected high score, got %.2f", result.Score)
			}
			if !tt.expectHigh && result.Score >= 0.8 {
				t.Errorf("expected low score, got %.2f", result.Score)
			}
		})
	}
}

func TestExtractStockNumbers(t *testing.T) {
	svc := NewConfidenceService(zap.NewNop())
	content := "### 贵州茅台 sh600519\n- **现价**: ¥1255.67  |  **涨跌**: ↓-15.43 (-1.21%)"
	nums := svc.extractStockNumbers(content)

	if len(nums) != 2 {
		t.Errorf("expected 2 numbers (price + change), got %d: %v", len(nums), nums)
	}
	if !strings.Contains(nums[0], "1255") {
		t.Errorf("price should contain 1255, got %s", nums[0])
	}
}

func TestIsStockToolResult(t *testing.T) {
	svc := NewConfidenceService(zap.NewNop())
	if !svc.isStockToolResult("## 实时行情\n### 茅台 sh600519") {
		t.Error("should detect stock realtime data")
	}
	if !svc.isStockToolResult("# K线数据") {
		t.Error("should detect stock kline data")
	}
	if svc.isStockToolResult("这是一段普通文本") {
		t.Error("should not detect stock in plain text")
	}
}
