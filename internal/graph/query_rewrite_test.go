package graph

import (
	"testing"
)

func TestNeedsRewrite_Short(t *testing.T) {
	// 4 个字太短，不改写
	if needsRewrite("茅台") {
		t.Fatal("should not rewrite very short query")
	}
	if needsRewrite("你好") {
		t.Fatal("should not rewrite greeting")
	}
}

func TestNeedsRewrite_Oral(t *testing.T) {
	if !needsRewrite("茅台咋样啊能不能搞") {
		t.Fatal("should rewrite oral query")
	}
}

func TestNeedsRewrite_Typos(t *testing.T) {
	if !needsRewrite("贵州茅台的今念增长率是躲少嘞") {
		t.Fatal("should rewrite typo-heavy query")
	}
}

func TestNeedsRewrite_Formal(t *testing.T) {
	// 规范的书面提问不需要改写
	if needsRewrite("贵州茅台2025年第四季度营收同比增长率是多少") {
		t.Fatal("should not rewrite formal query")
	}
}

func TestNeedsRewrite_English(t *testing.T) {
	// 纯英文不会触发中文口语改写（cjkCount==0），设计如此
	if needsRewrite("how is the market doing today?") {
		t.Fatal("should not rewrite pure English query")
	}
}
