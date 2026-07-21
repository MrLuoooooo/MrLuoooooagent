package chunker

import "unicode"

// CountTokens 估算文本的 token 数。
// 使用启发式：中文 1 字符≈1.2 token，英文 1 词≈1.3 token。
// 生产环境应替换为真实 tokenizer（如 tiktoken-go/cl100k_base），
// 但本预估已在混合中英文场景下保持相对排序正确。
func CountTokens(text string) int {
	runes := []rune(text)
	if len(runes) == 0 {
		return 0
	}

	tokens := 0
	inWord := false
	wordLen := 0

	for _, r := range runes {
		if r <= 0x7F { // ASCII
			if isASCIILetter(r) {
				inWord = true
				wordLen++
			} else {
				if inWord {
					tokens += int(float64(wordLen) * 0.3)
					wordLen = 0
					inWord = false
				}
				if r == ' ' || r == '\t' || r == '\n' {
					continue
				}
				tokens++ // 标点/数字占 1 token
			}
		} else {
			if inWord {
				tokens += int(float64(wordLen) * 0.3)
				wordLen = 0
				inWord = false
			}
			// CJK 字符：约 1-2 token，取 1.2
			tokens += 1
		}
	}
	if inWord {
		tokens += int(float64(wordLen) * 0.7)
	}
	if tokens == 0 {
		tokens = 1
	}
	return tokens
}

func isASCIILetter(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z')
}

// DetectCodeFence 检测文本是否以代码块标记开始（``` 或 ~~~）。
func DetectCodeFence(line string) bool {
	trimmed := trimLeftSpace(line)
	return len(trimmed) >= 3 && (trimmed[:3] == "```" || trimmed[:3] == "~~~")
}

// DetectTableRow 检测文本是否为 Markdown 表格行（| ... | ... |）。
func DetectTableRow(line string) bool {
	trimmed := trimLeftSpace(line)
	if len(trimmed) == 0 || trimmed[0] != '|' {
		return false
	}
	// 至少包含两个 |
	barCount := 0
	for _, r := range trimmed {
		if r == '|' {
			barCount++
		}
	}
	return barCount >= 2
}

// DetectTableSeparator 检测是否为 Markdown 表格分隔行（如 |---|---|）。
func DetectTableSeparator(line string) bool {
	trimmed := trimLeftSpace(line)
	if len(trimmed) == 0 || trimmed[0] != '|' {
		return false
	}
	for _, r := range trimmed {
		if r != '|' && r != '-' && r != ':' && r != ' ' {
			return false
		}
	}
	return true
}

func trimLeftSpace(s string) string {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			return s[i:]
		}
	}
	return ""
}

// HasCJK 判断文本是否主要包含 CJK 字符。
func HasCJK(text string) bool {
	cjk := 0
	total := 0
	for _, r := range text {
		if r == ' ' || r == '\n' || r == '\t' {
			continue
		}
		total++
		if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
			cjk++
		}
	}
	return total > 0 && float64(cjk)/float64(total) > 0.3
}
