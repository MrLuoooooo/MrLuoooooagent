// Package mask 提供审计与工具输出的敏感信息脱敏。
// 纯函数包：service 与 component/tool 共用（component 不得 import service，
// 共享脱敏逻辑必须下沉到此层）。
package mask

import "regexp"

// 敏感信息正则：审计日志脱敏用。顺序即应用顺序——
// 身份证必须在手机号之前（18 位身份证内嵌 11 位数字，先匹配长模式）。
var (
	maskIDCardRe = regexp.MustCompile(`\d{17}[\dXx]`)
	maskEmailRe  = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	maskSecretRe = regexp.MustCompile(`(?i)(sk-[A-Za-z0-9_\-]{8,}|Bearer\s+[A-Za-z0-9._\-]{8,})`)
	maskPhoneRe  = regexp.MustCompile(`1[3-9]\d{9}`)
)

// MaskSensitive 对手机号/邮箱/身份证/API Key 做保留首尾的掩码处理，
// 用于审计日志面（zap 工具记录等）。
// 注意取舍：会话库与 trace 保留原文——它们是功能数据（LLM 上下文回放、
// 调试复盘），掩码会破坏语义；合规脱敏的落点在日志。
func MaskSensitive(s string) string {
	if s == "" {
		return s
	}
	s = maskIDCardRe.ReplaceAllStringFunc(s, func(m string) string {
		return m[:6] + "********" + m[len(m)-4:]
	})
	s = maskEmailRe.ReplaceAllStringFunc(s, func(m string) string {
		at := indexByteFrom(m, '@')
		return m[:1] + "***" + m[at:]
	})
	s = maskSecretRe.ReplaceAllStringFunc(s, func(m string) string {
		return m[:minChars(5, len(m))] + "****"
	})
	s = maskPhoneRe.ReplaceAllStringFunc(s, func(m string) string {
		return m[:3] + "****" + m[len(m)-4:]
	})
	return s
}

func indexByteFrom(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return len(s)
}

func minChars(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// envValueRe 匹配配置文件中敏感键的值（PASSWORD/SECRET/TOKEN/KEY 等），
// 覆盖随机字符串类密钥——通用正则抓不到的由这里兜底。
var envValueRe = regexp.MustCompile(`(?im)^([A-Za-z0-9_]*(PASSWORD|SECRET|TOKEN|API_KEY|PRIVATE_KEY|ACCESS_KEY)[A-Za-z0-9_]*)(\s*[=:]\s*)(\S+)`)

// MaskConfigValues 对配置文件内容做键级脱敏：敏感键（含 PASSWORD/SECRET/
// TOKEN/KEY 等字样）的值整体掩码，其余行原样保留。供 read_file 读取
// .env/配置类敏感文件时使用——随机密码是通用正则抓不到的，必须按键名兜底。
func MaskConfigValues(s string) string {
	if s == "" {
		return s
	}
	return envValueRe.ReplaceAllString(s, "$1$3****")
}
