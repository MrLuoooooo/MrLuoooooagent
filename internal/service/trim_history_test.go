package service

import (
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func TestEstimateTokens(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Errorf("empty = %d, want 0", got)
	}
	if got := EstimateTokens("一二三四五六七八九十"); got != 6 { // 10 runes * 10/16
		t.Errorf("10 CJK runes = %d, want 6", got)
	}
	if got := EstimateTokens("a"); got != 1 {
		t.Errorf("1 rune = %d, want >=1", got)
	}
}

func TestTrimHistoryByToken_NoTrimNeeded(t *testing.T) {
	msgs := []*schema.Message{
		{Role: schema.User, Content: "短消息"},
		{Role: schema.Assistant, Content: "短回复"},
	}
	called := false
	out := trimHistoryByToken(msgs, 10000, func([]*schema.Message) string {
		called = true
		return ""
	})
	if called {
		t.Error("summarize should not be called when within budget")
	}
	if len(out) != 2 {
		t.Errorf("messages should be returned as-is, got %d", len(out))
	}
}

func TestTrimHistoryByToken_TrimsOldKeepsRecent(t *testing.T) {
	big := strings.Repeat("长", 5000) // ≈3125 tokens per message（5000*10/16），单条加上小消息就超 3000 预算
	msgs := []*schema.Message{
		{Role: schema.User, Content: big}, // 老消息，装不下
		{Role: schema.Assistant, Content: big},
		{Role: schema.User, Content: "最近问题"},
		{Role: schema.Assistant, Content: "最近回复"},
	}
	summarized := 0
	out := trimHistoryByToken(msgs, 3000, func(old []*schema.Message) string {
		summarized = len(old)
		return "## 摘要：之前的对话做了 A 和 B"
	})
	// 预算 3000：两条大消息共 4000 放不下 → 前两条被裁，摘要 + 最近两条
	if summarized != 2 {
		t.Errorf("expected 2 old messages summarized, got %d", summarized)
	}
	if len(out) != 3 {
		t.Fatalf("want 3 messages (summary + 2 recent), got %d", len(out))
	}
	if !strings.Contains(out[0].Content, "摘要") {
		t.Errorf("first message should be summary, got: %s", out[0].Content)
	}
	if out[1].Content != "最近问题" || out[2].Content != "最近回复" {
		t.Errorf("recent messages order broken: %+v", out)
	}
}

func TestTrimHistoryByToken_AllTooBig(t *testing.T) {
	// 单条消息就超预算：从尾部累计放不下任何一条 → 全部交给 summarize
	msgs := []*schema.Message{
		{Role: schema.User, Content: strings.Repeat("大", 3200)},
		{Role: schema.Assistant, Content: strings.Repeat("大", 3200)},
	}
	out := trimHistoryByToken(msgs, 1000, func(old []*schema.Message) string {
		if len(old) != 2 {
			t.Errorf("expected all 2 summarized, got %d", len(old))
		}
		return "摘要"
	})
	if len(out) != 1 || out[0].Content != "摘要" {
		t.Errorf("want summary only, got %+v", out)
	}
}

func TestTrimHistoryByToken_SummaryFailure(t *testing.T) {
	msgs := []*schema.Message{
		{Role: schema.User, Content: strings.Repeat("长", 3200)},
		{Role: schema.User, Content: "最近"},
	}
	// summarize 返回空（LLM 失败）：宁可没摘要也不给占位符
	out := trimHistoryByToken(msgs, 1000, func([]*schema.Message) string { return "" })
	if len(out) != 1 || out[0].Content != "最近" {
		t.Errorf("want recent only without placeholder, got %+v", out)
	}
}
