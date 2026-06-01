package scheduler

import (
	"testing"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
)

func TestParseApprovalBlock(t *testing.T) {
	input := "操作类型: 写入文件到 /D/goagentpro/output.md\n风险等级: 低\n理由: 需要保存分析报告"
	result := parseApprovalBlock(input)
	if result["操作类型"] != "写入文件到 /D/goagentpro/output.md" {
		t.Errorf("操作类型 = %q", result["操作类型"])
	}
	if result["风险等级"] != "低" {
		t.Errorf("风险等级 = %q", result["风险等级"])
	}
	if result["理由"] != "需要保存分析报告" {
		t.Errorf("理由 = %q", result["理由"])
	}
}

func TestParseApprovalBlock_EmptyInput(t *testing.T) {
	result := parseApprovalBlock("")
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestParseApprovalBlock_NoColons(t *testing.T) {
	input := "This line has no colon.\nNeither does this."
	result := parseApprovalBlock(input)
	if len(result) != 0 {
		t.Errorf("expected empty map for no-colon input, got %d entries", len(result))
	}
}

func TestParseApprovalBlock_IgnoresInvalidLines(t *testing.T) {
	input := "操作类型: 测试\nnot a kv pair\n风险等级: 中"
	result := parseApprovalBlock(input)
	if result["操作类型"] != "测试" {
		t.Errorf("操作类型 = %q", result["操作类型"])
	}
	if result["风险等级"] != "中" {
		t.Errorf("风险等级 = %q", result["风险等级"])
	}
	if _, ok := result["not a kv pair"]; ok {
		t.Error("non-key-value line should be ignored")
	}
}

func TestApprovalPattern_FullBlock(t *testing.T) {
	output := "some analysis...\n\n[NEEDS_APPROVAL]\n操作类型: 删除临时文件\n风险等级: 中\n理由: 清理磁盘空间\n[/NEEDS_APPROVAL]\n\nmore content."
	matches := approvalPattern.FindAllStringSubmatch(output, -1)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	inner := matches[0][1]
	if inner == "" {
		t.Fatal("inner block is empty")
	}
}

func TestApprovalPattern_NoApprovalBlock(t *testing.T) {
	output := "normal analysis without approval block"
	matches := approvalPattern.FindAllStringSubmatch(output, -1)
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestApprovalPattern_MultipleBlocks(t *testing.T) {
	output := "[NEEDS_APPROVAL]\n操作类型: 任务1\n风险等级: 低\n理由: 理由1\n[/NEEDS_APPROVAL]\n中间内容\n[NEEDS_APPROVAL]\n操作类型: 任务2\n风险等级: 高\n理由: 理由2\n[/NEEDS_APPROVAL]"
	matches := approvalPattern.FindAllStringSubmatch(output, -1)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
}

func TestApprovalStatusConstants(t *testing.T) {
	if model.ApprovalPending != "pending" {
		t.Errorf("ApprovalPending = %q", model.ApprovalPending)
	}
	if model.ApprovalAccepted != "accepted" {
		t.Errorf("ApprovalAccepted = %q", model.ApprovalAccepted)
	}
	if model.ApprovalRejected != "rejected" {
		t.Errorf("ApprovalRejected = %q", model.ApprovalRejected)
	}
	if model.ApprovalExpired != "expired" {
		t.Errorf("ApprovalExpired = %q", model.ApprovalExpired)
	}
}