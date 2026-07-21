package chunker

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

// ChunkResult 单个切片及其元信息。
type ChunkResult struct {
	ID        string // chunk 唯一 ID（parent 用此 ID 被 children 引用）
	Text      string // 切片文本（含 section 前缀）
	Section   string // 所属章节/标题（可为空）
	ChunkType string // "child" / "parent"
	ParentID  string // 父 chunk 的 ID，仅 child 有值
	TokenCnt  int    // token 估算数
}

// ChunkConfig 切片参数。
type ChunkConfig struct {
	ChildTokens  int // child chunk 目标 token 数（默认 256）
	ParentTokens int // parent chunk 目标 token 数（默认 1024）
	Overlap      int // 相邻 child 的重叠句数
}

// DefaultChunkConfig 生产推荐值。
var DefaultChunkConfig = ChunkConfig{
	ChildTokens:  256,
	ParentTokens: 1024,
	Overlap:      2,
}

// ChunkSemantic 兼容旧接口：单层切片（child only），参数为字符数。
// 内部将字符数估算为 token 数（中文≈1:1，英文≈0.5:1），
// 调用方应逐步迁移至 ChunkWithParent。
func ChunkSemantic(text string, size int, overlap int) []ChunkResult {
	// 兼容：size 为字符数，转换为近似 token 数
	approxTokens := size
	if HasCJK(text) {
		approxTokens = size // 中文 1 字≈1 token
	} else {
		approxTokens = size / 2 // 英文 1 字符≈0.5 token
	}
	if approxTokens < 64 {
		approxTokens = 64
	}
	results, _ := ChunkWithParent(text, ChunkConfig{
		ChildTokens:  approxTokens,
		ParentTokens: 0, // 不产生 parent
		Overlap:      overlap,
	})
	return results
}

// ChunkWithParent 生产级切片：child 小块做 embedding 检索，parent 大块做上下文注入。
// 特殊块（表格/代码）保留整体结构不被句子拆散。
// ParentTokens=0 时不生成 parent chunk。
func ChunkWithParent(text string, cfg ChunkConfig) ([]ChunkResult, error) {
	if cfg.ChildTokens <= 0 {
		cfg.ChildTokens = DefaultChunkConfig.ChildTokens
	}
	noParents := cfg.ParentTokens == 0
	if cfg.ParentTokens <= 0 && !noParents {
		cfg.ParentTokens = DefaultChunkConfig.ParentTokens
	}
	if cfg.Overlap < 0 {
		cfg.Overlap = 0
	}

	// 1. 将文本拆为逻辑块（段落 / 表格 / 代码块）
	blocks := splitBlocks(text)

	// 2. 在每个 block 内按句子拆（表格/代码块保留整体）
	var items []chunkItem
	currentSection := ""
	for _, block := range blocks {
		if block.typ == blockTable || block.typ == blockCode {
			// 特殊块保留整体结构
			if sec := detectSection(strings.SplitN(block.text, "\n", 2)[0]); sec != "" {
				currentSection = sec
			}
			items = append(items, chunkItem{
				text:     block.text,
				section:  currentSection,
				isAtomic: true, // 不可拆分
			})
			continue
		}
		// 段落按句子拆
		trimmed := strings.TrimSpace(block.text)
		if trimmed == "" {
			continue
		}
		if sec := detectSection(trimmed); sec != "" {
			currentSection = sec
		}
		parts := splitSentences(trimmed)
		for _, s := range parts {
			if s := strings.TrimSpace(s); s != "" {
				items = append(items, chunkItem{
					text:    s,
					section: currentSection,
				})
			}
		}
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("chunker: no content after splitting")
	}

	// 3. 合并 items 为 child chunks（按 token 数，保留 overlap）
	children := assembleChunks(items, cfg.ChildTokens, cfg.Overlap, "child")

	// 4. 产生 parent chunks（合并相邻 child，按 token 数）
	if noParents {
		return children, nil
	}
	parents := buildParents(children, cfg.ParentTokens)

	return append(children, parents...), nil
}

// ——— chunkItem：内部用的切分单元 ———

type chunkItem struct {
	text     string
	section  string
	isAtomic bool // 不可进一步拆分的块（表格/代码）
}

// ——— block 类型 ———

type blockType int

const (
	blockParagraph blockType = iota
	blockTable
	blockCode
)

type textBlock struct {
	text string
	typ  blockType
}

// splitBlocks 把原始文本拆为段落、表格、代码块。
func splitBlocks(text string) []textBlock {
	lines := strings.Split(text, "\n")
	var blocks []textBlock
	var buf strings.Builder
	inCode := false
	inTable := false
	flush := func(typ blockType) {
		s := strings.TrimSpace(buf.String())
		if s != "" {
			blocks = append(blocks, textBlock{text: s, typ: typ})
		}
		buf.Reset()
	}
	currentType := blockParagraph

	for _, line := range lines {
		isFence := DetectCodeFence(line)
		isTableLine := DetectTableRow(line) || DetectTableSeparator(line)

		switch {
		case isFence && !inCode:
			flush(currentType)
			buf.WriteString(line)
			inCode = true
			currentType = blockCode
		case isFence && inCode:
			buf.WriteString("\n")
			buf.WriteString(line)
			flush(blockCode)
			inCode = false
			currentType = blockParagraph
		case inCode:
			buf.WriteString("\n")
			buf.WriteString(line)
		case isTableLine && !inTable:
			flush(currentType)
			buf.WriteString(line)
			inTable = true
			currentType = blockTable
		case isTableLine && inTable:
			buf.WriteString("\n")
			buf.WriteString(line)
		case inTable && !isTableLine:
			// 表格结束
			flush(blockTable)
			inTable = false
			currentType = blockParagraph
			buf.WriteString(line)
		default:
			if buf.Len() > 0 && currentType == blockParagraph {
				buf.WriteString("\n")
			}
			buf.WriteString(line)
		}
	}
	if inTable {
		flush(blockTable)
	} else {
		flush(currentType)
	}
	return blocks
}

// ——— child chunk 组装 ———

func assembleChunks(items []chunkItem, targetTokens int, overlap int, chunkType string) []ChunkResult {
	var results []ChunkResult
	var buf strings.Builder
	bufSec := ""
	var bufTokens int
	var sentenceBuf []string

	flush := func() {
		text := strings.TrimSpace(buf.String())
		if text == "" {
			return
		}
		prefixed := prefixWithSection(text, bufSec)
		results = append(results, ChunkResult{
			Text:      prefixed,
			Section:   bufSec,
			ChunkType: chunkType,
			TokenCnt:  CountTokens(prefixed),
		})
		buf.Reset()
		bufTokens = 0
	}

	rebuildBufFrom := func(sents []string) {
		for _, s := range sents {
			if bufTokens > 0 {
				buf.WriteString("\n")
				bufTokens += CountTokens("\n")
			}
			buf.WriteString(s)
			bufTokens += CountTokens(s)
		}
	}

	for _, item := range items {
		tok := CountTokens(item.text)
		if item.isAtomic {
			// 原子块（表格/代码）：如果当前 buf 有内容先 flush，再单独成块
			if bufTokens > 0 {
				flush()
				sentenceBuf = nil
			}
			prefixed := prefixWithSection(item.text, item.section)
			results = append(results, ChunkResult{
				Text:      prefixed,
				Section:   item.section,
				ChunkType: chunkType,
				TokenCnt:  CountTokens(prefixed),
			})
			continue
		}

		if bufTokens > 0 && bufTokens+tok > targetTokens {
			if overlap > 0 && len(sentenceBuf) > 0 {
				start := len(sentenceBuf) - overlap
				if start < 0 {
					start = 0
				}
				retained := sentenceBuf[start:]
				flush()
				rebuildBufFrom(retained)
				sentenceBuf = append([]string{}, retained...)
			} else {
				flush()
				sentenceBuf = nil
			}
		}

		if bufTokens > 0 {
			buf.WriteString("\n")
			bufTokens += CountTokens("\n")
		}
		buf.WriteString(item.text)
		bufTokens += tok
		sentenceBuf = append(sentenceBuf, item.text)
		if bufSec == "" {
			bufSec = item.section
		}
		if len(sentenceBuf) > overlap {
			sentenceBuf = sentenceBuf[1:]
		}
	}
	flush()
	return results
}

// ——— parent chunk 构建 ———

func buildParents(children []ChunkResult, targetTokens int) []ChunkResult {
	if len(children) == 0 {
		return nil
	}

	var parents []ChunkResult
	var buf strings.Builder
	var bufTokens int
	parentIdx := 0
	currentParentID := uuid.New().String()

	for i := range children {
		tok := children[i].TokenCnt
		children[i].ParentID = currentParentID

		if bufTokens > 0 && bufTokens+tok > targetTokens {
			text := strings.TrimSpace(buf.String())
			parents = append(parents, ChunkResult{
				ID:        currentParentID,
				Text:      text,
				Section:   children[i-1].Section,
				ChunkType: "parent",
				TokenCnt:  CountTokens(text),
			})
			parentIdx++
			currentParentID = uuid.New().String()
			buf.Reset()
			bufTokens = 0
		}

		if bufTokens > 0 {
			buf.WriteString("\n\n")
			bufTokens += CountTokens("\n\n")
		}
		buf.WriteString(strings.TrimPrefix(children[i].Text, "["+children[i].Section+"] "))
		bufTokens += tok
	}

	if buf.Len() > 0 {
		text := strings.TrimSpace(buf.String())
		parents = append(parents, ChunkResult{
			ID:        currentParentID,
			Text:      text,
			Section:   children[len(children)-1].Section,
			ChunkType: "parent",
			TokenCnt:  CountTokens(text),
		})
	}

	return parents
}

// prefixWithSection 把章节信息拼到文本前面，形成 context header。
func prefixWithSection(text, section string) string {
	if section == "" {
		return text
	}
	return "[" + section + "] " + text
}

// splitSentences 按中英文句子边界拆文本（数字/缩写保护）。
func splitSentences(text string) []string {
	var result []string
	runes := []rune(text)
	start := 0
	for i := range runes {
		if isBreakPoint(runes, i) {
			s := strings.TrimSpace(string(runes[start : i+1]))
			if s != "" {
				result = append(result, s)
			}
			start = i + 1
		}
	}
	if start < len(runes) {
		s := strings.TrimSpace(string(runes[start:]))
		if s != "" {
			result = append(result, s)
		}
	}
	if len(result) == 0 {
		result = append(result, text)
	}
	return result
}

// isBreakPoint 判断 runes[idx] 是否为句子边界。
func isBreakPoint(runes []rune, idx int) bool {
	r := runes[idx]
	switch r {
	case '。', '！', '？', '；', '\n':
		return true
	case '!', '?':
		return isEnglishEnd(runes, idx)
	case '.':
		return isEnglishEnd(runes, idx)
	case ';':
		return false
	}
	return false
}

// isEnglishEnd 判断英文标点是否真正结束句子。
func isEnglishEnd(runes []rune, idx int) bool {
	if idx == len(runes)-1 {
		return true
	}
	prevIsDigit := idx > 0 && runes[idx-1] >= '0' && runes[idx-1] <= '9'
	nextIsDigit := runes[idx+1] >= '0' && runes[idx+1] <= '9'
	if prevIsDigit && nextIsDigit {
		return false
	}
	word := extractWordBefore(runes, idx)
	if isKnownAbbrev(word) {
		return false
	}
	j := idx + 1
	for j < len(runes) && (runes[j] == ' ' || runes[j] == '\t' || runes[j] == '\r') {
		j++
	}
	if j >= len(runes) {
		return true
	}
	next := runes[j]
	if (next >= 'a' && next <= 'z') || (next >= '0' && next <= '9') {
		return false
	}
	return true
}

func extractWordBefore(runes []rune, idx int) string {
	end := idx
	for end > 0 && !isLetter(runes[end-1]) {
		end--
	}
	start := end
	for start > 0 && isLetter(runes[start-1]) {
		start--
	}
	if start >= end {
		return ""
	}
	return strings.ToLower(string(runes[start:end]))
}

func isLetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

func isKnownAbbrev(word string) bool {
	switch word {
	case "inc", "corp", "ltd", "co",
		"etc", "et al", "e.g", "i.e", "vs",
		"dr", "mr", "mrs", "ms", "prof", "sr", "jr",
		"dept", "approx", "est", "govt",
		"jan", "feb", "mar", "apr", "jun", "jul", "aug", "sep", "oct", "nov", "dec",
		"a.m", "p.m":
		return true
	}
	return false
}

// detectSection 检测文本是否为章节/标题行。
func detectSection(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "#") {
		return strings.TrimLeft(trimmed, "# ")
	}
	prefixes := []string{"一、", "二、", "三、", "四、", "五、", "六、", "七、", "八、", "九、", "十、",
		"第", "第一部分", "第二部分", "第三部分",
		"第一章", "第二章", "第三章", "第四章", "第五章",
		"第一节", "第二节", "第三节"}
	for _, p := range prefixes {
		if strings.HasPrefix(trimmed, p) {
			return trimmed
		}
	}
	if len([]rune(trimmed)) < 40 {
		digitFound := false
		for _, r := range trimmed {
			if unicode.IsDigit(r) {
				digitFound = true
				break
			}
			if r == '.' || r == ')' || r == '、' {
				break
			}
		}
		if digitFound && (strings.Contains(trimmed, ".") || strings.Contains(trimmed, "、") || strings.Contains(trimmed, ")")) {
			return trimmed
		}
	}
	return ""
}
