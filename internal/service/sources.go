package service

import (
	"encoding/json"
	"regexp"
	"strings"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/callback"
)

// SourceRef 与 model.SourceRef 解耦的 service 层来源描述。
// handler 负责映射到 SSE 事件；service 只负责提取逻辑，不碰传输结构。
type SourceRef struct {
	Title string
	URL   string
	Kind  string // web / knowledge / stock
}

// maxSources 单次回复的来源上限，防止工具狂刷撑爆前端。
const maxSources = 8

// webSearchEntryRe 匹配 web_search 工具结果里的 "N. 标题" 行。
var webSearchEntryRe = regexp.MustCompile(`(?m)^\s*\d+\.\s*(.+)$`)

// webSearchURLRe 匹配结果里的 "URL: xxx" 行。
var webSearchURLRe = regexp.MustCompile(`(?m)^\s*URL:\s*(\S+)\s*$`)

// stockCodeReInArgs 匹配带交易所前缀的 A 股代码（如 sh600519 / sz000001 / bj832000）。
// 与 semantic_cache.go 的 stockCodeRe 语义相近但用途不同，避免重名。
var stockCodeReInArgs = regexp.MustCompile(`(sh|sz|bj)\d{6}`)

// bareStockCodeRe 匹配裸 6 位代码（模型传参常不带前缀，如 {"code":"600519"}）。
var bareStockCodeRe = regexp.MustCompile(`(?:^|\D)(\d{6})(?:\D|$)`)

// normalizeStockCode 把代码归一化为带前缀形式（sh600519）。
// 裸 6 位码按代码段推断交易所：60/68→sh，00/30→sz，43/8/92→bj；推断不出返回空。
func normalizeStockCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if stockCodeReInArgs.MatchString(code) {
		return code
	}
	if m := bareStockCodeRe.FindStringSubmatch(code); m != nil {
		code = m[1]
	} else {
		return ""
	}
	switch {
	case strings.HasPrefix(code, "60"), strings.HasPrefix(code, "68"):
		return "sh" + code
	case strings.HasPrefix(code, "00"), strings.HasPrefix(code, "30"):
		return "sz" + code
	case strings.HasPrefix(code, "43"), strings.HasPrefix(code, "8"), strings.HasPrefix(code, "92"):
		return "bj" + code
	}
	return ""
}

// stockToolNames 会针对单只股票返回数据的工具（参数含 code，可生成行情页链接）。
// 名字必须与 fx.go 注册的工具 Name 严格一致。
var stockToolNames = map[string]bool{
	"get_stock_realtime":  true,
	"get_stock_kline":     true,
	"get_financial_report": true,
}

// ExtractSources 从一次工具调用的名称、参数、结果中提取引用来源。
// 纯函数：不落库、不发事件、不依赖外部状态，可单测。
//
// 覆盖四类工具：
//   - web_search：解析 "N. 标题 / URL: 链接" 格式化输出
//   - retrieve_knowledge：知识库来源（无 URL，前端渲染为内部引用徽标）
//   - get_stock_* / get_financial_report：行情数据源（东方财富行情页链接，代码从工具参数提取）
//   - get_market_news：市场资讯来源
//
// 其余工具返回 nil——不是所有工具都产生可引用来源。
func ExtractSources(toolName, toolArgs, toolResult string) []SourceRef {
	switch {
	case toolName == "web_search":
		return extractWebSearchSources(toolResult)
	case toolName == "retrieve_knowledge":
		if strings.TrimSpace(toolResult) == "" {
			return nil
		}
		return []SourceRef{{Title: "知识库文档检索", Kind: "knowledge"}}
	case stockToolNames[toolName]:
		return extractStockSource(toolArgs)
	case toolName == "get_market_news":
		return []SourceRef{{Title: "东方财富市场资讯", URL: "https://news.eastmoney.com", Kind: "stock"}}
	default:
		return nil
	}
}

// extractWebSearchSources 解析 web_search 的格式化输出。
// 输出格式（web_search.go 三个引擎统一）：
//
//	1. 标题
//	   摘要
//	   URL: https://example.com
func extractWebSearchSources(result string) []SourceRef {
	if strings.Contains(result, "(无搜索结果)") || strings.TrimSpace(result) == "" {
		return nil
	}
	titles := webSearchEntryRe.FindAllStringSubmatch(result, -1)
	urls := webSearchURLRe.FindAllStringSubmatch(result, -1)
	if len(titles) == 0 || len(urls) == 0 {
		return nil
	}
	var refs []SourceRef
	for i, t := range titles {
		if i >= len(urls) {
			break
		}
		url := strings.TrimSpace(urls[i][1])
		if !isHTTPURL(url) {
			continue
		}
		refs = append(refs, SourceRef{
			Title: strings.TrimSpace(t[1]),
			URL:   url,
			Kind:  "web",
		})
		if len(refs) >= maxSources {
			break
		}
	}
	return refs
}

// extractStockSource 从工具参数中提取股票代码，生成行情数据源引用。
// 参数形如 {"code":"sh600519","period":"day"}；解析不出代码时退化为
// 无 URL 的通用行情来源标注。
func extractStockSource(toolArgs string) []SourceRef {
	var args struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal([]byte(toolArgs), &args)
	code := args.Code
	if code == "" {
		// 参数里没有 code 字段（或为空）：从原始参数文本搜带前缀码，再退裸码
		code = stockCodeReInArgs.FindString(toolArgs)
		if code == "" {
			if m := bareStockCodeRe.FindStringSubmatch(toolArgs); m != nil {
				code = m[1]
			}
		}
	}
	if full := normalizeStockCode(code); full != "" {
		return []SourceRef{{
			Title: "东方财富行情 · " + strings.ToUpper(full),
			URL:   "https://quote.eastmoney.com/" + full + ".html",
			Kind:  "stock",
		}}
	}
	return []SourceRef{{Title: "股票行情数据（东方财富/新浪财经）", Kind: "stock"}}
}

// isHTTPURL 只接受 http/https 绝对链接，防止把工具输出里的杂串当 URL 渲染。
func isHTTPURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

// maxArgSnippet 传入 ExtractSources 的参数截断长度，防止巨型参数白白参与正则匹配。
const maxArgSnippet = 200

// CollectSources 从一次 Agent 执行产生的全部工具调用记录中提取可引用来源。
// 纯函数：编排 ExtractSources，统一做去重（kind|url|title）与 maxSources 封顶，
// 保证多轮工具调用不会撑爆前端"参考来源"区块。handler 只消费结果，不做合并。
func CollectSources(records []callback.ToolRunRecord) []SourceRef {
	seen := make(map[string]bool)
	out := make([]SourceRef, 0, maxSources)
	for _, rec := range records {
		if rec.ToolName == "" && rec.Result == "" {
			continue // 空占位记录（callback 只拿到孤儿 OnEnd）无来源价值
		}
		args := rec.Args
		if len(args) > maxArgSnippet {
			args = args[:maxArgSnippet]
		}
		for _, ref := range ExtractSources(rec.ToolName, args, rec.Result) {
			key := ref.Kind + "|" + ref.URL + "|" + ref.Title
			if seen[key] {
				continue
			}
			if len(out) >= maxSources {
				return out // 封顶：按工具执行顺序保留先出现的来源
			}
			seen[key] = true
			out = append(out, ref)
		}
	}
	return out
}
