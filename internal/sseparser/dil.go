package sseparser

import (
	"encoding/json"
	"strings"
	"unicode"

	chatgpt_types "aurora/typings/chatgpt"
)

// ModelDILFallback 返回 ChatGPT 为不支持 GenUI/DIL 的客户端提供的 Markdown。
func ModelDILFallback(response chatgpt_types.ChatGPTResponse) string {
	modelDIL := response.Message.Metadata.ModelDILV2
	if modelDIL == nil {
		return ""
	}
	return modelDIL.FallbackMarkdown
}

// NormalizeAssistantText 把上游私有引用标记和 GenUI/DIL 降级为普通 Markdown。
func NormalizeAssistantText(text string, citeAlts map[string]string) string {
	return DegradeDILToMarkdown(ReplaceCiteMarkers(text, citeAlts))
}

// DegradeDILToMarkdown 对无法运行 GenUI 的文本客户端做语法级降级。
//
// 规则不依赖组件名：删除任意 JSX/XML 标签并保留成对标签内的静态文本；
// 删除 DIL 指令和动态表达式；保留 Markdown、自动链接以及代码块中的原文。
// 这里只解析文本，从不执行表达式。
func DegradeDILToMarkdown(text string) string {
	clean, _ := degradeDIL(text, true)
	return clean
}

// DILStreamPipeline 在分片边界暂存未闭合的标签或表达式，避免半个 DIL
// 标签先泄漏给客户端。完整普通文本会立即放行。
type DILStreamPipeline struct {
	raw     string
	emitted string
}

// Feed 追加原始增量并返回当前新增的安全 Markdown。
func (p *DILStreamPipeline) Feed(delta string) string {
	if delta != "" {
		p.raw += delta
	}
	clean, _ := degradeDIL(p.raw, false)
	if !strings.HasPrefix(clean, p.emitted) {
		// 降级结果必须单调增长；遇到未知语法时宁可等待流结束，也不重复文本。
		return ""
	}
	out := clean[len(p.emitted):]
	p.emitted = clean
	return out
}

// Flush 在流结束时处理剩余内容。残缺的 DIL 结构会被丢弃，普通文本保留。
func (p *DILStreamPipeline) Flush() string {
	clean, _ := degradeDIL(p.raw, true)
	if !strings.HasPrefix(clean, p.emitted) {
		return ""
	}
	out := clean[len(p.emitted):]
	p.raw = ""
	p.emitted = clean
	return out
}

func degradeDIL(text string, final bool) (string, bool) {
	var out strings.Builder
	out.Grow(len(text))

	inlineTicks := 0
	fenceTicks := 0
	fenceMarker := byte(0)
	lineStart := true

	for i := 0; i < len(text); {
		if text[i] == '`' || (text[i] == '~' && lineStart) {
			marker := text[i]
			run := repeatedByteLen(text, i, marker)
			out.WriteString(text[i : i+run])
			if fenceTicks != 0 {
				if marker == fenceMarker && lineStart && run >= fenceTicks {
					fenceTicks = 0
					fenceMarker = 0
				}
			} else if marker == '`' && inlineTicks != 0 {
				if run == inlineTicks {
					inlineTicks = 0
				}
			} else if lineStart && run >= 3 {
				fenceTicks = run
				fenceMarker = marker
			} else if marker == '`' {
				inlineTicks = run
			}
			i += run
			lineStart = false
			continue
		}

		if fenceTicks != 0 || inlineTicks != 0 {
			out.WriteByte(text[i])
			lineStart = text[i] == '\n'
			i++
			continue
		}

		switch text[i] {
		case '<':
			if !looksLikeTagPrefix(text[i:]) {
				out.WriteByte(text[i])
				lineStart = false
				i++
				continue
			}
			end, ok := scanAngleToken(text, i)
			if !ok {
				if !final && looksLikeTagPrefix(text[i:]) {
					return out.String(), false
				}
				// 流结束时丢弃看起来像组件的残片；普通比较符原样保留。
				if final && looksLikeTagPrefix(text[i:]) {
					return out.String(), true
				}
				out.WriteByte(text[i])
				lineStart = false
				i++
				continue
			}
			token := text[i:end]
			if isMarkupTag(token) {
				i = end
				continue
			}
			out.WriteString(token)
			lineStart = strings.HasSuffix(token, "\n")
			i = end
			continue

		case '{':
			end, ok := scanBalancedBraces(text, i)
			if !ok {
				if !final && looksLikeDILExpressionPrefix(text[i:]) {
					return out.String(), false
				}
				if final && looksLikeDILExpressionPrefix(text[i:]) {
					return out.String(), true
				}
				out.WriteByte(text[i])
				lineStart = false
				i++
				continue
			}
			expression := text[i+1 : end-1]
			if replacement, drop := dilExpressionFallback(expression); drop {
				out.WriteString(replacement)
				lineStart = strings.HasSuffix(replacement, "\n")
				i = end
				continue
			}
			out.WriteString(text[i:end])
			lineStart = strings.HasSuffix(text[i:end], "\n")
			i = end
			continue
		}

		out.WriteByte(text[i])
		lineStart = text[i] == '\n'
		i++
	}

	return out.String(), true
}

func repeatedByteLen(text string, start int, target byte) int {
	end := start
	for end < len(text) && text[end] == target {
		end++
	}
	return end - start
}

func scanAngleToken(text string, start int) (int, bool) {
	quote := byte(0)
	escaped := false
	braceDepth := 0
	for i := start + 1; i < len(text); i++ {
		ch := text[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"', '`':
			quote = ch
		case '{':
			braceDepth++
		case '}':
			if braceDepth > 0 {
				braceDepth--
			}
		case '>':
			if braceDepth == 0 {
				return i + 1, true
			}
		}
	}
	return 0, false
}

func looksLikeTagPrefix(text string) bool {
	if text == "<" {
		return true
	}
	if len(text) < 2 || text[0] != '<' {
		return false
	}
	i := 1
	if text[i] == '/' {
		i++
		if i >= len(text) {
			return true
		}
	}
	return isTagNameStart(rune(text[i]))
}

func isMarkupTag(token string) bool {
	if len(token) < 3 || token[0] != '<' || token[len(token)-1] != '>' {
		return false
	}
	inner := strings.TrimSpace(token[1 : len(token)-1])
	if inner == "" {
		return false
	}
	if strings.HasPrefix(inner, "!--") || strings.HasPrefix(inner, "!") || strings.HasPrefix(inner, "?") {
		return true
	}
	if !strings.ContainsAny(inner, " \t\r\n") &&
		(strings.Contains(inner, "://") || strings.HasPrefix(strings.ToLower(inner), "mailto:")) {
		return false
	}
	if inner[0] == '/' {
		inner = strings.TrimSpace(inner[1:])
	}
	if inner == "" || !isTagNameStart(rune(inner[0])) {
		return false
	}
	nameEnd := 1
	for nameEnd < len(inner) && isTagNameContinue(rune(inner[nameEnd])) {
		nameEnd++
	}
	if nameEnd == len(inner) {
		return true
	}
	next := inner[nameEnd]
	return unicode.IsSpace(rune(next)) || next == '/'
}

func isTagNameStart(r rune) bool {
	return unicode.IsLetter(r) || r == '_'
}

func isTagNameContinue(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == ':' || r == '.'
}

func scanBalancedBraces(text string, start int) (int, bool) {
	depth := 0
	quote := byte(0)
	escaped := false
	for i := start; i < len(text); i++ {
		ch := text[i]
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == quote {
				quote = 0
			}
			continue
		}
		switch ch {
		case '\'', '"', '`':
			quote = ch
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1, true
			}
		}
	}
	return 0, false
}

func looksLikeDILExpressionPrefix(text string) bool {
	if text == "{" {
		return true
	}
	if len(text) < 2 || text[0] != '{' {
		return false
	}
	trimmed := strings.TrimSpace(text[1:])
	if trimmed == "" {
		return true
	}
	switch trimmed[0] {
	case '@', '#', ':', '/':
		return true
	}
	return strings.Contains(trimmed, "GenUI.") || strings.Contains(trimmed, "DIL.") ||
		strings.Contains(trimmed, "=>") || strings.Contains(trimmed, "turn")
}

func dilExpressionFallback(expression string) (string, bool) {
	trimmed := strings.TrimSpace(expression)
	if trimmed == "" {
		return "", false
	}
	switch trimmed[0] {
	case '@', '#', ':', '/':
		return "", true
	}

	// JSX 中常见的 {"静态文字"} 不应丢失。
	if trimmed[0] == '"' {
		var literal string
		if json.Unmarshal([]byte(trimmed), &literal) == nil {
			return literal, true
		}
	}

	if looksLikeDynamicExpression(trimmed) {
		return "", true
	}
	return "", false
}

func looksLikeDynamicExpression(expression string) bool {
	if strings.Contains(expression, "GenUI.") || strings.Contains(expression, "DIL.") ||
		strings.Contains(expression, "=>") {
		return true
	}
	if strings.HasPrefix(expression, "turn") && isIdentifierPath(expression) {
		return true
	}
	if strings.Contains(expression, "[") && strings.Contains(expression, "]") && containsLetter(expression) {
		return true
	}
	if strings.Contains(expression, "(") && strings.Contains(expression, ")") &&
		containsLetter(expression) && strings.ContainsAny(expression, ".[:") {
		return true
	}
	return strings.Contains(expression, ".") && isIdentifierPath(expression)
}

func isIdentifierPath(expression string) bool {
	if expression == "" {
		return false
	}
	for _, r := range expression {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '$' || r == '.' {
			continue
		}
		return false
	}
	return containsLetter(expression)
}

func containsLetter(text string) bool {
	for _, r := range text {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}
