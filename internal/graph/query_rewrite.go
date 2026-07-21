package graph

import (
	"context"
	"strings"
	"unicode"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// rewriteQueryIfNeeded 在用户输入口语化/有错别字/过短时，调 LLM 归一为书面提问。
// 返回空字符串表示不需要改写。
func rewriteQueryIfNeeded(ctx context.Context, chatModel model.ChatModel, query string) string {
	if !needsRewrite(query) {
		return ""
	}

	msg, err := chatModel.Generate(ctx, []*schema.Message{
		{Role: schema.System, Content: rewriteSystemPrompt},
		{Role: schema.User, Content: "改写以下用户输入为规范的书面提问：\n" + query},
	})
	if err != nil || msg == nil {
		return "" // 改写失败不影响主流程
	}
	rewritten := strings.TrimSpace(msg.Content)
	if rewritten == "" || rewritten == query {
		return ""
	}
	return rewritten
}

// needsRewrite 判断是否需要对 query 做归一化。
// 触发条件：字符数少、口语词、错别字、纯缩写。
func needsRewrite(query string) bool {
	runes := []rune(query)
	if len(runes) < 5 {
		return false // 太短，改写无意义（如"茅台"）
	}

	cjkCount := 0
	for _, r := range runes {
		if unicode.Is(unicode.Han, r) {
			cjkCount++
		}
	}

	// 含中文 + <20 字 → 但排除已包含明确信息（数字/年份）和消歧词的查询
	if cjkCount > 0 && len(runes) < 20 {
		return !alreadySpecific(query, runes)
	}

	// 包含口语特征词
	oralWords := []string{"咋", "嘛", "呗", "嘞", "咯", "哒", "啊", "吧", "呢", "哇",
		"多儿", "啥子", "咋了", "多小", "躲少"}
	for _, w := range oralWords {
		if strings.Contains(query, w) {
			return true
		}
	}

	return false
}

const rewriteSystemPrompt = `你是查询改写助手。只做以下操作，不添加额外信息：
1. 将口语转为书面语（"咋退钱"→"如何申请退款"）
2. 纠正明显的拼写/输入错误（"今念"→"今年"，"躲少"→"多少"）
3. 补充省略的主语和宾语（"茅台怎么样"→"贵州茅台（600519）的股价和基本面怎么样"）
4. 将含糊提问明确化（"看一下"→"展示以下信息"）

规则：
- 只返回改写后的一句话，不要解释
- 如果用户的问法已经是规范的书面提问，直接原样返回
- 不要修改用户的投资意图或风险取向`

// alreadySpecific 判断 query 是否已经包含明确的金融查询要素，不需要改写。
// 有数字 + 有实体名（中文词 ≥2 字）→ 已是具体提问。
func alreadySpecific(query string, runes []rune) bool {
	hasDigit := false
	hasEntity := false
	wordLen := 0
	for _, r := range runes {
		if r >= '0' && r <= '9' {
			hasDigit = true
		}
		if unicode.Is(unicode.Han, r) {
			wordLen++
		} else {
			if wordLen >= 2 {
				hasEntity = true
			}
			wordLen = 0
		}
	}
	if wordLen >= 2 {
		hasEntity = true
	}
	return hasDigit && hasEntity
}
