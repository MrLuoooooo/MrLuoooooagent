package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type WebFetchTool struct {
	client  *http.Client
	enabled bool
}

func NewWebFetchTool(enabled bool) *WebFetchTool {
	return &WebFetchTool{
		client:  &http.Client{Timeout: 15 * time.Second},
		enabled: enabled,
	}
}

func (t *WebFetchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "web_fetch",
		Desc: "抓取网页内容并提取文本。适用于获取公告、新闻、财报等公开网页信息。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"url": {
				Type:     schema.String,
				Desc:     "目标网页 URL",
				Required: true,
			},
			"max_chars": {
				Type:     schema.Integer,
				Desc:     "最大返回字符数，默认 3000",
				Required: false,
			},
		}),
	}, nil
}

func (t *WebFetchTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	if !t.enabled {
		return "web_fetch 工具未启用。请检查配置 search.enabled。", nil
	}
	var args struct {
		URL      string `json:"url"`
		MaxChars int    `json:"max_chars"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("web_fetch: invalid args: %w", err)
	}
	if args.URL == "" {
		return "", fmt.Errorf("web_fetch: url 不能为空")
	}
	if args.MaxChars <= 0 {
		args.MaxChars = 3000
	}

	req, err := http.NewRequestWithContext(ctx, "GET", args.URL, nil)
	if err != nil {
		return fmt.Sprintf("抓取失败: %v", err), nil
	}
	req.Header.Set("User-Agent", "GoAgentPro/1.0")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Sprintf("抓取失败: %v", err), nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, int64(args.MaxChars*2)))
	text := stripHTML(string(body))

	if len(text) > args.MaxChars {
		text = text[:args.MaxChars] + "..."
	}
	return fmt.Sprintf("## 网页内容 (%s)\n\n%s", args.URL, text), nil
}

func stripHTML(s string) string {
	var b strings.Builder
	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			b.WriteRune(' ')
			continue
		}
		if !inTag {
			b.WriteRune(r)
		}
	}
	out := b.String()
	// Collapse whitespace.
	parts := strings.Fields(out)
	return strings.Join(parts, " ")
}

type CalculatorTool struct{}

func (t *CalculatorTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "calculator",
		Desc: "安全计算数学表达式。支持 + - * / % ^ sqrt abs round。示例: 100*(1+0.05)^5",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"expression": {
				Type:     schema.String,
				Desc:     "数学表达式，如 '100*1.05^5' 或 'sqrt(144)+10'",
				Required: true,
			},
		}),
	}, nil
}

func (t *CalculatorTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Expression string `json:"expression"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("calculator: %w", err)
	}
	if args.Expression == "" {
		return "", fmt.Errorf("calculator: expression 不能为空")
	}

	result, err := evalSafe(args.Expression)
	if err != nil {
		return fmt.Sprintf("计算出错: %v", err), nil
	}
	return fmt.Sprintf("%s = %s", args.Expression, formatNum(result)), nil
}

func evalSafe(expr string) (float64, error) {
	expr = strings.ReplaceAll(expr, " ", "")
	expr = strings.ReplaceAll(expr, "^", "**")
	expr = strings.ReplaceAll(expr, "sqrt", "Sqrt")
	expr = strings.ReplaceAll(expr, "abs", "Abs")
	expr = strings.ReplaceAll(expr, "round", "Round")
	expr = strings.ReplaceAll(expr, "pi", strconv.FormatFloat(math.Pi, 'f', 10, 64))
	expr = strings.ReplaceAll(expr, "e", strconv.FormatFloat(math.E, 'f', 10, 64))

	// Only allow safe characters.
	for _, r := range expr {
		if !strings.ContainsRune("0123456789.+-*/%()SqrtAbsRound,", r) {
			return 0, fmt.Errorf("不允许的字符: %c", r)
		}
	}

	return evalSimple(expr)
}

func evalSimple(expr string) (float64, error) {
	// Simple recursive descent: parse term (+-) → factor (*/%) → atom
	idx := 0
	return parseAddSub(expr, &idx)
}

func parseAddSub(s string, idx *int) (float64, error) {
	left, err := parseMulDiv(s, idx)
	if err != nil {
		return 0, err
	}
	for *idx < len(s) {
		op := s[*idx]
		if op != '+' && op != '-' {
			break
		}
		*idx++
		right, err := parseMulDiv(s, idx)
		if err != nil {
			return 0, err
		}
		if op == '+' {
			left += right
		} else {
			left -= right
		}
	}
	return left, nil
}

func parseMulDiv(s string, idx *int) (float64, error) {
	left, err := parsePower(s, idx)
	if err != nil {
		return 0, err
	}
	for *idx < len(s) {
		op := s[*idx]
		if op != '*' && op != '/' && op != '%' {
			break
		}
		*idx++
		right, err := parsePower(s, idx)
		if err != nil {
			return 0, err
		}
		switch op {
		case '*':
			left *= right
		case '/':
			if right == 0 {
				return 0, fmt.Errorf("除零")
			}
			left /= right
		case '%':
			left = math.Mod(left, right)
		}
	}
	return left, nil
}

func parsePower(s string, idx *int) (float64, error) {
	left, err := parseAtom(s, idx)
	if err != nil {
		return 0, err
	}
	for *idx < len(s) && s[*idx] == '*' && *idx+1 < len(s) && s[*idx+1] == '*' {
		*idx += 2
		right, err := parseAtom(s, idx)
		if err != nil {
			return 0, err
		}
		left = math.Pow(left, right)
	}
	return left, nil
}

func parseAtom(s string, idx *int) (float64, error) {
	if *idx >= len(s) {
		return 0, fmt.Errorf("表达式不完整")
	}

	// Unary minus.
	if s[*idx] == '-' {
		*idx++
		v, err := parseAtom(s, idx)
		return -v, err
	}

	// Parentheses.
	if s[*idx] == '(' {
		*idx++
		v, err := parseAddSub(s, idx)
		if err != nil {
			return 0, err
		}
		if *idx >= len(s) || s[*idx] != ')' {
			return 0, fmt.Errorf("缺少右括号")
		}
		*idx++
		return v, nil
	}

	// Function calls.
	if *idx+4 <= len(s) && s[*idx:*idx+4] == "Sqrt" {
		*idx += 4
		if *idx >= len(s) || s[*idx] != '(' {
			return 0, fmt.Errorf("Sqrt 需要括号")
		}
		*idx++
		v, err := parseAddSub(s, idx)
		if err != nil {
			return 0, err
		}
		if *idx >= len(s) || s[*idx] != ')' {
			return 0, fmt.Errorf("缺少右括号")
		}
		*idx++
		return math.Sqrt(v), nil
	}
	if *idx+3 <= len(s) && s[*idx:*idx+3] == "Abs" {
		*idx += 3
		if *idx >= len(s) || s[*idx] != '(' {
			return 0, fmt.Errorf("Abs 需要括号")
		}
		*idx++
		v, err := parseAddSub(s, idx)
		if err != nil {
			return 0, err
		}
		if *idx >= len(s) || s[*idx] != ')' {
			return 0, fmt.Errorf("缺少右括号")
		}
		*idx++
		return math.Abs(v), nil
	}
	if *idx+5 <= len(s) && s[*idx:*idx+5] == "Round" {
		*idx += 5
		if *idx >= len(s) || s[*idx] != '(' {
			return 0, fmt.Errorf("Round 需要括号")
		}
		*idx++
		v, err := parseAddSub(s, idx)
		if err != nil {
			return 0, err
		}
		// Optional second argument.
		decimals := 0.0
		if *idx < len(s) && s[*idx] == ',' {
			*idx++
			decimals, err = parseAddSub(s, idx)
			if err != nil {
				return 0, err
			}
		}
		if *idx >= len(s) || s[*idx] != ')' {
			return 0, fmt.Errorf("缺少右括号")
		}
		*idx++
		pow := math.Pow(10, decimals)
		return math.Round(v*pow) / pow, nil
	}

	// Number.
	start := *idx
	for *idx < len(s) && (s[*idx] >= '0' && s[*idx] <= '9' || s[*idx] == '.') {
		*idx++
	}
	if start == *idx {
		return 0, fmt.Errorf("期望数字，得到 '%c'", s[*idx])
	}
	return strconv.ParseFloat(s[start:*idx], 64)
}

func formatNum(v float64) string {
	if v == math.Trunc(v) {
		return fmt.Sprintf("%.0f", v)
	}
	return strconv.FormatFloat(v, 'f', 4, 64)
}

// ═══════════════════════════════════════════════════════════
// Tool: stock_search — search stocks by keyword
// ═══════════════════════════════════════════════════════════

type StockSearchTool struct{}

func (t *StockSearchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "stock_search",
		Desc: "根据关键字搜索A股股票代码。输入公司名/行业/概念，返回匹配的股票列表。数据来自内置股票库。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"keyword": {
				Type:     schema.String,
				Desc:     "搜索关键字，如 茅台、新能源、银行",
				Required: true,
			},
		}),
	}, nil
}

func (t *StockSearchTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Keyword string `json:"keyword"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("stock_search: %w", err)
	}
	if args.Keyword == "" {
		return "", fmt.Errorf("stock_search: keyword 不能为空")
	}

	results := searchStockDB(args.Keyword)
	if len(results) == 0 {
		return fmt.Sprintf("未找到匹配 '%s' 的股票。尝试用更宽泛的关键词或已知代码。", args.Keyword), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("## 搜索 '%s' (%d条结果)\n\n", args.Keyword, len(results)))
	b.WriteString("| 代码 | 名称 | 行业 |\n|------|------|------|\n")
	for _, r := range results {
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", r.code, r.name, r.industry))
	}
	return b.String(), nil
}

type stockEntry struct {
	code, name, industry string
}

func searchStockDB(keyword string) []stockEntry {
	kw := strings.ToLower(keyword)
	var results []stockEntry
	for _, s := range stockDB {
		if strings.Contains(strings.ToLower(s.name), kw) || strings.Contains(strings.ToLower(s.code), kw) || strings.Contains(strings.ToLower(s.industry), kw) {
			results = append(results, s)
			if len(results) >= 15 {
				break
			}
		}
	}
	return results
}

// Built-in stock database (common A-shares).
var stockDB = []stockEntry{
	{"sh600000", "浦发银行", "银行"},
	{"sh600016", "民生银行", "银行"},
	{"sh600036", "招商银行", "银行"},
	{"sh601398", "工商银行", "银行"},
	{"sh601939", "建设银行", "银行"},
	{"sh600519", "贵州茅台", "白酒"},
	{"sz000858", "五粮液", "白酒"},
	{"sz000568", "泸州老窖", "白酒"},
	{"sh600276", "恒瑞医药", "医药"},
	{"sz300760", "迈瑞医疗", "医疗器械"},
	{"sh600196", "复星医药", "医药"},
	{"sz002415", "海康威视", "安防"},
	{"sh600030", "中信证券", "券商"},
	{"sz300059", "东方财富", "互联网金融"},
	{"sh601318", "中国平安", "保险"},
	{"sh601628", "中国人寿", "保险"},
	{"sh600900", "长江电力", "电力"},
	{"sh601985", "中国核电", "核电"},
	{"sz300750", "宁德时代", "新能源电池"},
	{"sz002594", "比亚迪", "新能源汽车"},
	{"sh601012", "隆基绿能", "光伏"},
	{"sz300274", "阳光电源", "光伏逆变器"},
	{"sh600585", "海螺水泥", "建材"},
	{"sh601668", "中国建筑", "建筑"},
	{"sh600048", "保利发展", "房地产"},
	{"sz000002", "万科A", "房地产"},
	{"sh600809", "山西汾酒", "白酒"},
	{"sz000001", "平安银行", "银行"},
	{"sz000333", "美的集团", "家电"},
	{"sz000651", "格力电器", "家电"},
	{"sh601888", "中国中免", "免税"},
	{"sh600809", "山西汾酒", "白酒"},
	{"sz300124", "汇川技术", "工业自动化"},
	{"sh603259", "药明康德", "医药外包"},
	{"sh688981", "中芯国际", "半导体"},
	{"sz002371", "北方华创", "半导体设备"},
	{"sh600703", "三安光电", "LED芯片"},
	{"sh601899", "紫金矿业", "有色金属"},
	{"sz002460", "赣锋锂业", "锂矿"},
	{"sh601111", "中国国航", "航空"},
	{"sh600029", "南方航空", "航空"},
}

// ═══════════════════════════════════════════════════════════
// Tool: stock_index — get market index data
// ═══════════════════════════════════════════════════════════

type StockIndexTool struct{}

func (t *StockIndexTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "stock_index",
		Desc: "获取A股大盘指数概况。包括上证指数、深证成指、创业板指、沪深300等。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"name": {
				Type:     schema.String,
				Desc:     "指数名: 上证/深证/创业板/沪深300/科创50，不填则返回全部",
				Required: false,
			},
		}),
	}, nil
}

func (t *StockIndexTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Name string `json:"name"`
	}
	_ = json.Unmarshal([]byte(argsJSON), &args)

	var selected []indexInfo
	for _, idx := range marketIndices {
		if args.Name == "" || strings.Contains(idx.name, args.Name) {
			selected = append(selected, idx)
		}
	}

	var b strings.Builder
	b.WriteString("## 大盘指数\n\n")
	b.WriteString("指数数据通过 get_stock_realtime 工具实时获取。以下为参考信息：\n\n")
	b.WriteString("| 指数 | 代码 | 说明 |\n|------|------|------|\n")
	for _, idx := range selected {
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", idx.name, idx.code, idx.desc))
	}
	b.WriteString("\n> 使用 get_stock_realtime 查询实时点位，如 `get_stock_realtime sh000001`")
	return b.String(), nil
}

type indexInfo struct {
	code, name, desc string
}

var marketIndices = []indexInfo{
	{"sh000001", "上证指数", "上海证券交易所综合指数"},
	{"sz399001", "深证成指", "深圳证券交易所成份指数"},
	{"sz399006", "创业板指", "创业板市场指数"},
	{"sh000300", "沪深300", "沪深两市300只大盘蓝筹股"},
	{"sh000688", "科创50", "科创板50只成份股"},
	{"sh000016", "上证50", "沪市50只超级大盘股"},
	{"sz399005", "中小100", "中小板100只成份股"},
}

// ═══════════════════════════════════════════════════════════
// Tool: json_tool — parse and query JSON data
// ═══════════════════════════════════════════════════════════

type JSONTool struct{}

func (t *JSONTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "json_tool",
		Desc: "解析和查询JSON数据。支持提取字段、格式化、路径查询。用于处理API返回的复杂JSON。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"json_str": {
				Type:     schema.String,
				Desc:     "JSON 字符串",
				Required: true,
			},
			"path": {
				Type:     schema.String,
				Desc:     "点号分隔的字段路径，如 data.items[0].name，不填则格式化输出",
				Required: false,
			},
		}),
	}, nil
}

func (t *JSONTool) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		JSONStr string `json:"json_str"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("json_tool: %w", err)
	}

	var data interface{}
	if err := json.Unmarshal([]byte(args.JSONStr), &data); err != nil {
		return fmt.Sprintf("JSON 解析失败: %v", err), nil
	}

	if args.Path == "" {
		pretty, _ := json.MarshalIndent(data, "", "  ")
		return string(pretty), nil
	}

	// Simple path query.
	parts := strings.Split(args.Path, ".")
	current := data
	for _, part := range parts {
		// Handle array index: name[0]
		arrIdx := -1
		field := part
		if bidx := strings.Index(part, "["); bidx >= 0 {
			field = part[:bidx]
			if eidx := strings.Index(part[bidx:], "]"); eidx >= 0 {
				arrIdx, _ = strconv.Atoi(part[bidx+1 : bidx+eidx])
			}
		}
		m, ok := current.(map[string]interface{})
		if !ok {
			return fmt.Sprintf("路径 '%s' 在 '%s' 处不是对象", args.Path, field), nil
		}
		current = m[field]
		if arrIdx >= 0 {
			arr, ok := current.([]interface{})
			if !ok || arrIdx >= len(arr) {
				return fmt.Sprintf("路径 '%s': 索引 %d 越界", args.Path, arrIdx), nil
			}
			current = arr[arrIdx]
		}
	}

	pretty, _ := json.MarshalIndent(current, "", "  ")
	return string(pretty), nil
}

// ═══════════════════════════════════════════════════════════
// Tool: text_tools — text operations
// ═══════════════════════════════════════════════════════════

type TextTools struct{}

func (t *TextTools) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "text_tools",
		Desc: "文本处理工具。支持统计字数、提取关键词、按行排序、去重等操作。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"text": {
				Type:     schema.String,
				Desc:     "输入文本",
				Required: true,
			},
			"operation": {
				Type:     schema.String,
				Desc:     "操作: count(统计)/lines(按行分割)/sort(排序)/unique(去重)/head(前N行)/tail(后N行)",
				Required: true,
			},
			"n": {
				Type:     schema.Integer,
				Desc:     "head/tail 的行数，默认10",
				Required: false,
			},
		}),
	}, nil
}

func (t *TextTools) InvokableRun(ctx context.Context, argsJSON string, opts ...tool.Option) (string, error) {
	var args struct {
		Text      string `json:"text"`
		Operation string `json:"operation"`
		N         int    `json:"n"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("text_tools: %w", err)
	}
	if args.N <= 0 {
		args.N = 10
	}

	switch args.Operation {
	case "count":
		lines := strings.Split(args.Text, "\n")
		chars := len([]rune(args.Text))
		words := len(strings.Fields(args.Text))
		return fmt.Sprintf("字数: %d | 词数: %d | 行数: %d", chars, words, len(lines)), nil

	case "lines":
		lines := strings.Split(args.Text, "\n")
		return strings.Join(lines, "\n"), nil

	case "sort":
		lines := strings.Split(args.Text, "\n")
		sortLines(lines)
		return strings.Join(lines, "\n"), nil

	case "unique":
		lines := strings.Split(args.Text, "\n")
		seen := make(map[string]bool)
		var result []string
		for _, l := range lines {
			if !seen[l] {
				seen[l] = true
				result = append(result, l)
			}
		}
		return strings.Join(result, "\n"), nil

	case "head":
		lines := strings.Split(args.Text, "\n")
		if args.N > len(lines) {
			args.N = len(lines)
		}
		return strings.Join(lines[:args.N], "\n"), nil

	case "tail":
		lines := strings.Split(args.Text, "\n")
		if args.N > len(lines) {
			args.N = len(lines)
		}
		return strings.Join(lines[len(lines)-args.N:], "\n"), nil

	default:
		return fmt.Sprintf("不支持的操作: %s。支持: count/lines/sort/unique/head/tail", args.Operation), nil
	}
}

func sortLines(lines []string) {
	sort.Strings(lines)
}

// ═══════════════════════════════════════════════════════════

var (
	_ tool.InvokableTool = (*WebFetchTool)(nil)
	_ tool.InvokableTool = (*CalculatorTool)(nil)
	_ tool.InvokableTool = (*StockSearchTool)(nil)
	_ tool.InvokableTool = (*StockIndexTool)(nil)
	_ tool.InvokableTool = (*JSONTool)(nil)
	_ tool.InvokableTool = (*TextTools)(nil)
)
