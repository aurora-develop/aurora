package sseparser

import (
	"encoding/json"
	"html"
	neturl "net/url"
	"strconv"
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
	var tagStack []dilTagFrame

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
				if tag, ok := parseMarkupTag(token); ok {
					degradeMarkupTag(&out, &tagStack, tag)
				}
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
	if final {
		closeDILTagFrames(&out, tagStack, 0)
	}

	return out.String(), true
}

type dilTagAttribute struct {
	literal string
	static  bool
}

type dilMarkupTag struct {
	name        string
	closing     bool
	selfClosing bool
	attributes  map[string]dilTagAttribute
}

type dilTagFrame struct {
	name            string
	linkDestination string
	fallbackLabel   string
	contentStart    int
	openedLink      bool
	preserveIfEmpty bool
}

func degradeMarkupTag(out *strings.Builder, stack *[]dilTagFrame, tag dilMarkupTag) {
	if tag.closing {
		for i := len(*stack) - 1; i >= 0; i-- {
			if (*stack)[i].name != tag.name {
				continue
			}
			closeDILTagFrames(out, (*stack)[i:], 0)
			*stack = (*stack)[:i]
			return
		}
		return
	}

	destination, hasLinkAttribute := tagLinkDestination(tag.attributes)
	label := tagLinkLabel(tag.attributes)
	if tag.selfClosing {
		switch {
		case destination != "":
			out.WriteString(markdownLink(label, destination))
		case hasLinkAttribute && label != "":
			out.WriteString(escapeMarkdownLabel(label))
		}
		return
	}

	frame := dilTagFrame{
		name:            tag.name,
		fallbackLabel:   label,
		contentStart:    out.Len(),
		preserveIfEmpty: hasLinkAttribute && label != "",
	}
	if destination != "" {
		out.WriteByte('[')
		frame.linkDestination = destination
		frame.contentStart = out.Len()
		frame.openedLink = true
	}
	*stack = append(*stack, frame)
}

func closeDILTagFrames(out *strings.Builder, frames []dilTagFrame, start int) {
	for i := len(frames) - 1; i >= start; i-- {
		frame := frames[i]
		if frame.openedLink {
			if out.Len() == frame.contentStart {
				label := frame.fallbackLabel
				if label == "" {
					label = frame.linkDestination
				}
				out.WriteString(escapeMarkdownLabel(label))
			}
			out.WriteString("](")
			out.WriteString(escapeMarkdownDestination(frame.linkDestination))
			out.WriteByte(')')
			continue
		}
		if frame.preserveIfEmpty && out.Len() == frame.contentStart {
			out.WriteString(escapeMarkdownLabel(frame.fallbackLabel))
		}
	}
}

func parseMarkupTag(token string) (dilMarkupTag, bool) {
	var tag dilMarkupTag
	if len(token) < 3 || token[0] != '<' || token[len(token)-1] != '>' {
		return tag, false
	}
	inner := strings.TrimSpace(token[1 : len(token)-1])
	if inner == "" || strings.HasPrefix(inner, "!") || strings.HasPrefix(inner, "?") {
		return tag, false
	}
	if inner[0] == '/' {
		tag.closing = true
		inner = strings.TrimSpace(inner[1:])
	}
	if strings.HasSuffix(inner, "/") {
		tag.selfClosing = true
		inner = strings.TrimSpace(inner[:len(inner)-1])
	}
	if inner == "" || !isTagNameStart(rune(inner[0])) {
		return tag, false
	}
	nameEnd := 1
	for nameEnd < len(inner) && isTagNameContinue(rune(inner[nameEnd])) {
		nameEnd++
	}
	tag.name = strings.ToLower(inner[:nameEnd])
	if tag.closing {
		return tag, true
	}
	tag.attributes = parseTagAttributes(inner[nameEnd:])
	return tag, true
}

func parseTagAttributes(text string) map[string]dilTagAttribute {
	attributes := make(map[string]dilTagAttribute)
	for i := 0; i < len(text); {
		for i < len(text) && unicode.IsSpace(rune(text[i])) {
			i++
		}
		if i >= len(text) {
			break
		}
		nameStart := i
		for i < len(text) && !unicode.IsSpace(rune(text[i])) && text[i] != '=' {
			i++
		}
		if nameStart == i {
			i++
			continue
		}
		name := strings.ToLower(text[nameStart:i])
		for i < len(text) && unicode.IsSpace(rune(text[i])) {
			i++
		}
		if i >= len(text) || text[i] != '=' {
			attributes[name] = dilTagAttribute{}
			continue
		}
		i++
		for i < len(text) && unicode.IsSpace(rune(text[i])) {
			i++
		}
		value, end, static := parseTagAttributeValue(text, i)
		attributes[name] = dilTagAttribute{literal: value, static: static}
		i = end
	}
	return attributes
}

func parseTagAttributeValue(text string, start int) (string, int, bool) {
	if start >= len(text) {
		return "", start, false
	}
	if text[start] == '"' || text[start] == '\'' || text[start] == '`' {
		quote := text[start]
		for i := start + 1; i < len(text); i++ {
			if text[i] == '\\' {
				i++
				continue
			}
			if text[i] == quote {
				return decodeTagString(text[start : i+1]), i + 1, true
			}
		}
		return "", len(text), false
	}
	if text[start] == '{' {
		end, ok := scanBalancedBraces(text, start)
		if !ok {
			return "", len(text), false
		}
		expression := strings.TrimSpace(text[start+1 : end-1])
		if len(expression) >= 2 && (expression[0] == '"' || expression[0] == '\'' || expression[0] == '`') && expression[len(expression)-1] == expression[0] {
			return decodeTagString(expression), end, true
		}
		return "", end, false
	}
	end := start
	for end < len(text) && !unicode.IsSpace(rune(text[end])) {
		end++
	}
	return html.UnescapeString(text[start:end]), end, true
}

func decodeTagString(quoted string) string {
	if len(quoted) < 2 {
		return ""
	}
	if quoted[0] == '"' || quoted[0] == '`' {
		if decoded, err := strconv.Unquote(quoted); err == nil {
			return html.UnescapeString(decoded)
		}
	}
	inner := quoted[1 : len(quoted)-1]
	inner = strings.ReplaceAll(inner, `\\`, `\`)
	inner = strings.ReplaceAll(inner, `\'`, `'`)
	return html.UnescapeString(inner)
}

func tagLinkDestination(attributes map[string]dilTagAttribute) (string, bool) {
	for _, name := range []string{"href", "url"} {
		attribute, ok := attributes[name]
		if !ok {
			continue
		}
		if attribute.static && isSafeLinkDestination(attribute.literal) {
			return attribute.literal, true
		}
		return "", true
	}
	return "", false
}

func tagLinkLabel(attributes map[string]dilTagAttribute) string {
	for _, name := range []string{"title", "label"} {
		if attribute, ok := attributes[name]; ok && attribute.static {
			return strings.TrimSpace(attribute.literal)
		}
	}
	return ""
}

func isSafeLinkDestination(destination string) bool {
	if destination == "" || strings.ContainsAny(destination, "\r\n\x00") {
		return false
	}
	parsed, err := neturl.Parse(destination)
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https", "mailto", "tel":
		return true
	default:
		return false
	}
}

func markdownLink(label, destination string) string {
	if label == "" {
		label = destination
	}
	return "[" + escapeMarkdownLabel(label) + "](" + escapeMarkdownDestination(destination) + ")"
}

func escapeMarkdownLabel(label string) string {
	return strings.NewReplacer(`\`, `\\`, `[`, `\[`, `]`, `\]`).Replace(label)
}

func escapeMarkdownDestination(destination string) string {
	return strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`, " ", "%20").Replace(destination)
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
