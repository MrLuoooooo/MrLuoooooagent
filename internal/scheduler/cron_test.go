package scheduler

import (
	"testing"

	"github.com/MrLuoooooo/MrLuoooooagent/internal/model"
)

func TestParseApprovalBlock(t *testing.T) {
	input := `操作类型: 写入文件�?/D/goagentpro/output.md
风险等级: �?理由: 需要保存分析报告`
	result := parseApprovalBlock(input)
	if result["操作类型"] != "写入文件�?/D/goagentpro/output.md" {
		t.Errorf("操作类型 = %q", result["操作类型"])
	}
	if result["风险等级"] != "�? {
		t.Errorf("风险等级 = %q", result["风险等级"])
	}
	if result["理由"] != "需要保存分析报�? {
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
	input := "这是没有冒号的行\n这也是没有冒号的�?
	result := parseApprovalBlock(input)
	if len(result) != 0 {
		t.Errorf("expected empty map for no-colon input, got %d entries", len(result))
	}
}

func TestParseApprovalBlock_IgnoresInvalidLines(t *testing.T) {
	input := "操作类型: 测试\n不是键值对\n风险等级: �?
	result := parseApprovalBlock(input)
	if result["操作类型"] != "测试" {
		t.Errorf("操作类型 = %q", result["操作类型"])
	}
	if result["风险等级"] != "�? {
		t.Errorf("风险等级 = %q", result["风险等级"])
	}
	if _, ok := result["不是键值对"]; ok {
		t.Error("non-key-value line should be ignored")
	}
}

func TestApprovalPattern_FullBlock(t *testing.T) {
	output := `这是一些分析内�?..

[NEEDS_APPROVAL]
操作类型: 删除临时文件
风险等级: �?理由: 清理磁盘空间
[/NEEDS_APPROVAL]

继续输出更多内容。`

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
	output := "普通分析内容，没有审批�?
	matches := approvalPattern.FindAllStringSubmatch(output, -1)
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestApprovalPattern_MultipleBlocks(t *testing.T) {
	output := `[NEEDS_APPROVAL]
操作类型: 任务1
风险等级: �?理由: 理由1
[/NEEDS_APPROVAL]
中间内容
[NEEDS_APPROVAL]
操作类型: 任务2
风险等级: �?理由: 理由2
[/NEEDS_APPROVAL]`

	matches := approvalPattern.FindAllStringSubmatch(output, -1)
	if len(matches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matches))
	}
	if matches[0][1] != "操作类型: 任务1\n风险等级: 低\n理由: 理由1\n" {
		t.Errorf("block 1 = %q", matches[0][1])
	}
	if matches[1][1] != "操作类型: 任务2\n风险等级: 高\n理由: 理由2\n" {
		t.Errorf("block 2 = %q", matches[1][1])
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
