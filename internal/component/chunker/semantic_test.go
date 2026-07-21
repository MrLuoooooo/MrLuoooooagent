package chunker

import (
	"strings"
	"testing"
)

// ——— CountTokens ———

func TestCountTokens_Chinese(t *testing.T) {
	n := CountTokens("贵州茅台2025年营收一千二百亿")
	if n < 8 || n > 20 {
		t.Fatalf("expected 8-20 tokens for Chinese text, got %d", n)
	}
}

func TestCountTokens_English(t *testing.T) {
	n := CountTokens("Kweichow Moutai reported revenue growth")
	// ~6 words, typical tokenizer emits 8-12 tokens
	if n < 5 || n > 20 {
		t.Fatalf("expected 5-20 tokens for English text, got %d", n)
	}
}

func TestCountTokens_Empty(t *testing.T) {
	if CountTokens("") != 0 {
		t.Fatal("expected 0 tokens for empty string")
	}
}

func TestCountTokens_Reasonable(t *testing.T) {
	// Token count should be roughly proportional to string length
	short := CountTokens("hello")
	long := CountTokens("hello world this is a longer sentence with more words")
	if short >= long {
		t.Fatalf("longer text should have more tokens: short=%d long=%d", short, long)
	}
}

// ——— splitSentences ———

func TestSplitSentences_Basic(t *testing.T) {
	parts := splitSentences("今天天气很好。明天可能要下雨。")
	if len(parts) != 2 {
		t.Fatalf("expected 2 sentences, got %d: %v", len(parts), parts)
	}
}

func TestSplitSentences_Decimal(t *testing.T) {
	text := "营收为1,234.56亿元，同比增长。"
	parts := splitSentences(text)
	// 1.56 should NOT break the sentence
	if len(parts) != 1 {
		t.Fatalf("expected 1 sentence (decimal preserved), got %d: %v", len(parts), parts)
	}
	if !strings.Contains(parts[0], "1,234.56") {
		t.Fatalf("decimal was broken: %s", parts[0])
	}
}

func TestSplitSentences_Abbrev(t *testing.T) {
	text := "Inc. and Corp. revenue grew. Next quarter."
	parts := splitSentences(text)
	if len(parts) != 2 {
		t.Fatalf("expected 2 sentences (Inc./Corp. preserved), got %d: %v", len(parts), parts)
	}
}

func TestSplitSentences_Mixed(t *testing.T) {
	text := "营收增长12.5%。Revenue grew by 3.2%. 这是第三句。"
	parts := splitSentences(text)
	// 。 breaks, . after 3.2 should be preserved (digits both sides)
	if len(parts) != 3 {
		t.Fatalf("expected 3 sentences, got %d: %v", len(parts), parts)
	}
}

// ——— splitBlocks ———

func TestSplitBlocks_CodeFence(t *testing.T) {
	text := "hello\n```go\nfunc main() {}\n```\nworld"
	blocks := splitBlocks(text)
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks (para, code, para), got %d", len(blocks))
	}
	if blocks[0].typ != blockParagraph || blocks[1].typ != blockCode || blocks[2].typ != blockParagraph {
		t.Fatalf("wrong block types: %v", blocks)
	}
}

func TestSplitBlocks_Table(t *testing.T) {
	text := "intro\n| A | B |\n|---|---|\n| 1 | 2 |\nfooter"
	blocks := splitBlocks(text)
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks (para, table, para), got %d", len(blocks))
	}
	if blocks[0].typ != blockParagraph || blocks[1].typ != blockTable || blocks[2].typ != blockParagraph {
		t.Fatalf("wrong block types: %v", blocks)
	}
	if !strings.Contains(blocks[1].text, "|---") {
		t.Fatalf("table separator lost: %s", blocks[1].text)
	}
}

// ——— Parent/Child ———

func TestChunkWithParent_Linkage(t *testing.T) {
	text := strings.Repeat("测试句子。", 50) // 50 sentences
	results, err := ChunkWithParent(text, DefaultChunkConfig)
	if err != nil {
		t.Fatal(err)
	}

	var children, parents []ChunkResult
	for _, r := range results {
		switch r.ChunkType {
		case "child":
			children = append(children, r)
		case "parent":
			parents = append(parents, r)
		}
	}

	if len(children) == 0 {
		t.Fatal("expected at least 1 child")
	}
	if len(parents) == 0 {
		t.Fatal("expected at least 1 parent")
	}

	// Every child must have a non-empty ParentID
	for i, c := range children {
		if c.ParentID == "" {
			t.Fatalf("child[%d] has empty ParentID", i)
		}
	}

	// Every parent must have an EMPTY ParentID (parent has no parent)
	for i, p := range parents {
		if p.ParentID != "" {
			t.Fatalf("parent[%d] should have empty ParentID, got %q", i, p.ParentID)
		}
	}

	// Every child's ParentID must correspond to an existing parent.ID
	parentIDs := make(map[string]bool)
	for _, p := range parents {
		if p.ID == "" {
			t.Fatal("parent has empty ID")
		}
		parentIDs[p.ID] = true
	}
	for _, c := range children {
		if !parentIDs[c.ParentID] {
			t.Fatalf("child has orphan ParentID: %q (child text starts: %.30s)", c.ParentID, c.Text)
		}
	}
}

// ——— ChunkSemantic backward compat ———

func TestChunkSemantic_BackwardCompat(t *testing.T) {
	text := strings.Repeat("测试文本内容。", 30)
	results := ChunkSemantic(text, 200, 1)
	if len(results) == 0 {
		t.Fatal("expected at least 1 chunk")
	}
	// All results should be children (no parents with ParentTokens=0)
	for _, r := range results {
		if r.ChunkType != "child" {
			t.Fatalf("ChunkSemantic should only return children, got %q", r.ChunkType)
		}
		if r.ParentID != "" {
			t.Fatalf("ChunkSemantic children should have empty ParentID (no parent produced)")
		}
	}
}

func TestChunkSemantic_Empty(t *testing.T) {
	results := ChunkSemantic("", 200, 1)
	if len(results) != 0 {
		t.Fatalf("expected 0 chunks for empty text, got %d", len(results))
	}
}

// ——— Section detection ———

func TestDetectSection_Markdown(t *testing.T) {
	sec := detectSection("# 财务分析")
	if sec != "财务分析" {
		t.Fatalf("expected '财务分析', got %q", sec)
	}
}

func TestDetectSection_ChineseNum(t *testing.T) {
	sec := detectSection("一、营收概况")
	if sec != "一、营收概况" {
		t.Fatalf("expected '一、营收概况', got %q", sec)
	}
}

func TestDetectSection_Chapter(t *testing.T) {
	sec := detectSection("第三章 风险提示")
	if sec != "第三章 风险提示" {
		t.Fatalf("expected '第三章 风险提示', got %q", sec)
	}
}

// ——— Context header ———

func TestPrefixWithSection(t *testing.T) {
	result := prefixWithSection("chunk内容", "财务分析")
	if !strings.HasPrefix(result, "[财务分析]") {
		t.Fatalf("section prefix missing: %s", result)
	}
	if !strings.Contains(result, "chunk内容") {
		t.Fatalf("content lost: %s", result)
	}
}

// ——— Overlap ———

func TestAssembleChunks_Overlap(t *testing.T) {
	items := make([]chunkItem, 20)
	for i := range items {
		items[i] = chunkItem{
			text:    "测试句子内容。",
			section: "",
		}
	}
	results := assembleChunks(items, 30, 2, "child")
	if len(results) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(results))
	}
	// Check that consecutive chunks overlap
	for i := 1; i < len(results); i++ {
		prev := results[i-1].Text
		curr := results[i].Text
		// The last sentence of prev should appear in curr
		prevParts := splitSentences(prev)
		if len(prevParts) < 2 {
			continue
		}
		lastSent := prevParts[len(prevParts)-1]
		if !strings.Contains(curr, lastSent) {
			t.Fatalf("overlap missing between chunk[%d] and chunk[%d]:\n  last of prev: %s\n  curr: %s", i-1, i, lastSent, curr)
		}
	}
}
