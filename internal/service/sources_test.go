package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/callback"
)

func TestExtractSources_WebSearch(t *testing.T) {
	result := "直接答案: 茅台2025年营收超1700亿\n\n" +
		"1. 贵州茅台2025年年报\n" +
		"   2025年贵州茅台实现营业收入1708.99亿元\n" +
		"   URL: https://finance.sina.com.cn/stock/600519.shtml\n\n" +
		"2. 茅台2025年报解读（学术论文视角）\n" +
		"   从杜邦分析视角看茅台ROE\n" +
		"   URL: https://www.cnki.net/paper/maotai2025\n\n" +
		"3. 无链接条目\n" +
		"   摘要只有文字没有URL\n\n"
	refs := ExtractSources("web_search", `{"query":"茅台2025年报"}`, result)
	if len(refs) != 2 {
		t.Fatalf("want 2 sources, got %d: %+v", len(refs), refs)
	}
	if refs[0].Title != "贵州茅台2025年年报" || refs[0].URL != "https://finance.sina.com.cn/stock/600519.shtml" || refs[0].Kind != "web" {
		t.Errorf("ref0 mismatch: %+v", refs[0])
	}
	if refs[1].Title != "茅台2025年报解读（学术论文视角）" || refs[1].URL != "https://www.cnki.net/paper/maotai2025" {
		t.Errorf("ref1 mismatch: %+v", refs[1])
	}
}

func TestExtractSources_WebSearchEmpty(t *testing.T) {
	if refs := ExtractSources("web_search", "{}", "(无搜索结果)"); refs != nil {
		t.Errorf("empty result should yield nil, got %+v", refs)
	}
}

func TestExtractSources_WebSearchNonURL(t *testing.T) {
	result := "1. 标题\n   URL: not-a-real-url\n"
	if refs := ExtractSources("web_search", "{}", result); refs != nil {
		t.Errorf("non-URL entries should be filtered, got %+v", refs)
	}
}

func TestExtractSources_Knowledge(t *testing.T) {
	refs := ExtractSources("retrieve_knowledge", `{"query":"部署"}`, "根据知识库文档……")
	if len(refs) != 1 || refs[0].Kind != "knowledge" || refs[0].URL != "" {
		t.Fatalf("knowledge source mismatch: %+v", refs)
	}
	if refs := ExtractSources("retrieve_knowledge", "{}", "  "); refs != nil {
		t.Errorf("empty knowledge result should yield nil")
	}
}

func TestExtractSources_StockWithCode(t *testing.T) {
	// 工具名必须对齐 fx.go 注册的真实 Name：get_stock_realtime / get_stock_kline / get_financial_report
	refs := ExtractSources("get_stock_kline", `{"code":"sh600519"}`, `{"close":1520.5}`)
	if len(refs) != 1 {
		t.Fatalf("want 1 stock source, got %+v", refs)
	}
	if refs[0].URL != "https://quote.eastmoney.com/sh600519.html" || refs[0].Kind != "stock" {
		t.Errorf("stock source mismatch: %+v", refs[0])
	}
	if !strings.Contains(refs[0].Title, "SH600519") {
		t.Errorf("title should contain upper code: %s", refs[0].Title)
	}
	// 裸 6 位码（模型传参常不带前缀）也要能归一化出 URL
	bare := ExtractSources("get_stock_kline", `{"code":"600519"}`, "kline")
	if len(bare) != 1 || bare[0].URL != "https://quote.eastmoney.com/sh600519.html" {
		t.Errorf("bare code normalization mismatch: %+v", bare)
	}
}

func TestExtractSources_StockNoCode(t *testing.T) {
	// stock_list 不在 stockToolNames 里 → 走默认分支无来源；get_market_news 是通用资讯来源
	if refs := ExtractSources("stock_list", `{}`, `[{"code":"sh600519"}]`); refs != nil {
		t.Fatalf("stock_list should produce no source, got %+v", refs)
	}
	refs := ExtractSources("get_market_news", `{}`, "news list")
	if len(refs) != 1 || refs[0].Kind != "stock" {
		t.Fatalf("market news source mismatch: %+v", refs)
	}
	// 匹配 stock 工具但参数解析不出 code → 退化为无 URL 的通用行情来源
	fallback := ExtractSources("get_stock_kline", `{}`, "kline data")
	if len(fallback) != 1 || fallback[0].URL != "" || fallback[0].Kind != "stock" {
		t.Fatalf("no-code fallback mismatch: %+v", fallback)
	}
}

func TestExtractSources_OtherTools(t *testing.T) {
	for _, name := range []string{"bash", "read_file", "datetime"} {
		if refs := ExtractSources(name, "{}", "anything"); refs != nil {
			t.Errorf("tool %s should yield nil, got %+v", name, refs)
		}
	}
}

func TestCollectSources_DedupAcrossRecords(t *testing.T) {
	records := []callback.ToolRunRecord{
		{ToolName: "get_stock_realtime", Args: `{"code":"sh600519"}`, Result: `{"price":1520}`},
		{ToolName: "get_stock_kline", Args: `{"code":"sh600519"}`, Result: "kline"},
	}
	refs := CollectSources(records)
	// 同一股票两轮调用 → 同一来源，去重后只剩 1 条
	if len(refs) != 1 || refs[0].URL != "https://quote.eastmoney.com/sh600519.html" {
		t.Fatalf("dedup mismatch: %+v", refs)
	}
}

func TestCollectSources_CapsAtMaxSources(t *testing.T) {
	// 12 条不同股票记录 > maxSources=8，封顶后只保留先出现的 8 条
	records := make([]callback.ToolRunRecord, 0, 12)
	for i := range 12 {
		records = append(records, callback.ToolRunRecord{
			ToolName: "get_stock_realtime",
			Args:     fmt.Sprintf(`{"code":"sh60%04d"}`, i),
			Result:   "{}",
		})
	}
	refs := CollectSources(records)
	if len(refs) != 8 {
		t.Fatalf("want capped 8 sources, got %d", len(refs))
	}
	if refs[0].URL != "https://quote.eastmoney.com/sh600000.html" {
		t.Errorf("cap should keep earliest records, got %+v", refs[0])
	}
}

func TestCollectSources_SkipsEmptyPlaceholders(t *testing.T) {
	records := []callback.ToolRunRecord{
		{}, // 孤儿占位：callback 只收到 OnEnd 无 OnStart
		{ToolName: "retrieve_knowledge", Args: "{}", Result: "文档内容"},
	}
	refs := CollectSources(records)
	if len(refs) != 1 || refs[0].Kind != "knowledge" {
		t.Fatalf("placeholder skip mismatch: %+v", refs)
	}
	if out := CollectSources(nil); len(out) != 0 {
		t.Errorf("nil records should yield empty slice, got %+v", out)
	}
}
