package chunker

import (
	"strings"
	"unicode"
)

// ChunkResult 单个片段及其元信息。
type ChunkResult struct {
	Text    string // 切片文本
	Section string // 所属章节/标题（可为空）
}

// ChunkSemantic 按段落+句子边界切片，优先在语义边界断开。
// size 为目标字符数，overlap 为相邻切片的句重叠数。
// 返回切片列表，每个切片附带其所在章节名。
func ChunkSemantic(text string, size int, overlap int) []ChunkResult {
	if size <= 0 {
		size = 500
	}
	if overlap >= size {
		overlap = size / 10
	}

	runes := []rune(text)
	if len(runes) <= size {
		section := detectSection(text)
		return []ChunkResult{{Text: strings.TrimSpace(text), Section: section}}
	}

	// 1. 按段落拆
	paragraphs := strings.Split(text, "\n\n")

	// 2. 每个段落按句子边界拆
	var sentences []string
	currentSection := ""
	for _, para := range paragraphs {
		trimmed := strings.TrimSpace(para)
		if trimmed == "" {
			continue
		}
		if sec := detectSection(trimmed); sec != "" {
			currentSection = sec
		}
		parts := splitSentences(trimmed)
		for _, s := range parts {
			if s := strings.TrimSpace(s); s != "" {
				sentences = append(sentences, s)
			}
		}
	}

	// 3. 合并短句到 size 附近，保留 section 信息
	var results []ChunkResult
	var buf strings.Builder
	bufSec := ""
	var bufLen int

	flush := func() {
		text := strings.TrimSpace(buf.String())
		if text != "" {
			results = append(results, ChunkResult{Text: text, Section: bufSec})
		}
		buf.Reset()
		bufLen = 0
	}

	// 记录 last flush 时的句子数，用于 overlap 回退
	var flushedSentences []string
	for i, s := range sentences {
		sLen := len([]rune(s))

		if bufLen > 0 && bufLen+sLen > size {
			flush()
			flushedSentences = nil
		}

		if bufLen > 0 {
			buf.WriteString("\n")
			bufLen++
		}
		buf.WriteString(s)
		bufLen += sLen
		flushedSentences = append(flushedSentences, s)
		if bufSec == "" {
			bufSec = currentSection
		}
		// take last overlap sentences
		if len(flushedSentences) > overlap {
			flushedSentences = flushedSentences[1:]
		}
		_ = i // suppress unused
	}
	flush()

	return results
}

// splitSentences 按中英文句子边界拆文本。
func splitSentences(text string) []string {
	var result []string
	runes := []rune(text)
	start := 0
	for i, r := range runes {
		if isSentenceEnd(r) {
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

func isSentenceEnd(r rune) bool {
	switch r {
	case '。', '！', '？', '；',
		'.', '!', '?', ';':
		return true
	case '\n':
		return true
	}
	return false
}

// detectSection 检测文本是否为章节/标题行。
// 支持：# / ## / 一、/ 1. / 第X章 等格式。
func detectSection(line string) string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return ""
	}

	// Markdown heading
	if strings.HasPrefix(trimmed, "#") {
		return strings.TrimLeft(trimmed, "# ")
	}

	// 中文序号标题
	prefixes := []string{"一、", "二、", "三、", "四、", "五、", "六、", "七、", "八、", "九、", "十、",
		"第", "第一部分", "第二部分", "第三部分",
		"第一章", "第二章", "第三章", "第四章", "第五章",
		"第一节", "第二节", "第三节"}
	for _, p := range prefixes {
		if strings.HasPrefix(trimmed, p) {
			return trimmed
		}
	}

	// 数字序号（短标题，全行 < 40 字符）
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
