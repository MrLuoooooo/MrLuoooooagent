package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AgentToolDemoService —— @agenttool 注解演示（仅用于代码生成测试）。
type AgentToolDemoService struct {
	prefix string
}

// NewAgentToolDemoService ——
func NewAgentToolDemoService() *AgentToolDemoService {
	return &AgentToolDemoService{prefix: "[Agent] "}
}

// @agenttool name="demo_echo" desc="回显消息，添加Agent前缀。测试string返回类型。"
// @param message string 要回显的消息 required
func (s *AgentToolDemoService) Echo(ctx context.Context, message string) (string, error) {
	return s.prefix + message, nil
}

// @agenttool name="demo_count_words" desc="统计文本中单词数量。测试int返回类型。"
// @param text string 要统计的文本 required
func (s *AgentToolDemoService) CountWords(ctx context.Context, text string) (int, error) {
	return len(strings.Fields(text)), nil
}

// @agenttool name="demo_match" desc="检查文本是否包含关键词。测试bool返回类型。"
// @param text string 要检查的文本 required
// @param keyword string 关键词 required
func (s *AgentToolDemoService) Match(ctx context.Context, text string, keyword string) (bool, error) {
	return strings.Contains(strings.ToLower(text), strings.ToLower(keyword)), nil
}

// @agenttool name="demo_split" desc="按分隔符切分字符串。测试[]string数组返回。"
// @param text string 要切分的文本 required
// @param sep string 分隔符
func (s *AgentToolDemoService) Split(ctx context.Context, text string, sep string) ([]string, error) {
	if sep == "" {
		sep = ","
	}
	return strings.Split(text, sep), nil
}

// @agenttool name="demo_log" desc="记录一条日志。测试仅error返回（无返回值）。"
// @param message string 日志内容 required
// @param level string 日志级别 (info/warn/error)
func (s *AgentToolDemoService) Log(ctx context.Context, message string, level string) error {
	entry := map[string]string{
		"message": s.prefix + message,
		"level":   level,
	}
	b, _ := json.Marshal(entry)
	fmt.Println(string(b))
	return nil
}
