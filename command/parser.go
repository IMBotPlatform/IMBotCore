package command

import (
	"strings"
)

// ParseResult 承载文本命令解析后的结构化结果。
type ParseResult struct {
	IsCommand   bool     // 是否检测到命令前缀
	Tokens      []string // 解析后的命令及参数 token（包含命令本身）
	Raw         string   // 原始输入文本
	ArgumentRaw string   // 去除命令后的原始参数串
}

// Parser 解析企业微信文本内容，判定是否命令并拆分 token。
type Parser struct {
	Prefix string // 命令前缀，默认 "/"
}

// NewParser 创建带默认前缀的解析器。
func NewParser() Parser {
	return Parser{Prefix: "/"}
}

// Parse 将文本拆解为命令 token。规则参考 Telegram Message.IsCommand。
func (p Parser) Parse(text string) ParseResult {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ParseResult{Raw: text}
	}

	prefix := p.Prefix
	if prefix == "" {
		prefix = "/"
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return ParseResult{Raw: text}
	}
	first := fields[0]
	if !strings.HasPrefix(first, prefix) || len(first) <= len(prefix) {
		return ParseResult{Raw: text}
	}

	commandToken := strings.TrimPrefix(first, prefix)
	if idx := strings.IndexRune(commandToken, '@'); idx >= 0 {
		commandToken = commandToken[:idx]
	}
	if commandToken == "" {
		return ParseResult{Raw: text}
	}

	tokens := make([]string, 0, len(fields))
	tokens = append(tokens, commandToken)
	if len(fields) > 1 {
		tokens = append(tokens, fields[1:]...)
	}

	argumentRaw := ""
	if len(fields) > 1 {
		argumentRaw = strings.TrimSpace(strings.TrimPrefix(trimmed, first))
	}

	return ParseResult{
		IsCommand:   true,
		Tokens:      tokens,
		Raw:         text,
		ArgumentRaw: argumentRaw,
	}
}
