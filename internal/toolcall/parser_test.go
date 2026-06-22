package toolcall

import (
	"encoding/json"
	"strings"
	"testing"

	"aurora/typings/official"
)

func TestParserBasicSingleCall(t *testing.T) {
	p := NewParser()
	text, calls := p.Feed("I will read the file. <tool_call>{\"name\":\"read\",\"arguments\":{\"filePath\":\"/tmp/x\"}}</tool_call>")
	if text != "I will read the file. " {
		t.Fatalf("text = %q", text)
	}
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if calls[0].Function.Name != "read" || calls[0].Function.Arguments != `{"filePath":"/tmp/x"}` {
		t.Fatalf("call = %#v", calls[0])
	}
	if !strings.HasPrefix(calls[0].ID, "call_") {
		t.Fatalf("id = %q, want call_ prefix", calls[0].ID)
	}
}

func TestParserStreamingChunked(t *testing.T) {
	p := NewParser()
	// 模型分段发出
	segments := []string{
		"让我先",
		"查一下文件:",
		"<tool_call>",
		`{"name": "bash",`,
		` "arguments": {"command": "ls"}}`,
		"</tool_call>",
		"  ",
	}
	var allText string
	var allCalls []official.ToolCall
	for _, s := range segments {
		txt, calls := p.Feed(s)
		allText += txt
		allCalls = append(allCalls, calls...)
	}
	if allText != "让我先查一下文件:  " {
		t.Fatalf("concatenated text = %q", allText)
	}
	if len(allCalls) != 1 {
		t.Fatalf("calls = %d", len(allCalls))
	}
	if allCalls[0].Function.Name != "bash" {
		t.Fatalf("name = %q", allCalls[0].Function.Name)
	}
}

func TestParserMultipleCalls(t *testing.T) {
	p := NewParser()
	in := `<tool_call>{"name":"a","arguments":{"x":1}}</tool_call><tool_call>{"name":"b","arguments":{"y":2}}</tool_call>`
	_, calls := p.Feed(in)
	if len(calls) != 2 {
		t.Fatalf("calls = %d, want 2", len(calls))
	}
	if calls[0].Function.Name != "a" || calls[1].Function.Name != "b" {
		t.Fatalf("names = [%q, %q]", calls[0].Function.Name, calls[1].Function.Name)
	}
}

func TestParserNormalizesTagVariants(t *testing.T) {
	cases := []struct {
		open, close string
	}{
		{"<tool_calls>", "</tool_calls>"},
		{"<tool_call>", "</tool_call>"},
		{"<tool call>", "</tool call>"},
		{"<Tool_Call>", "</Tool_Call>"},
	}
	for _, c := range cases {
		p := NewParser()
		in := "hi " + c.open + `{"name":"x","arguments":{}}` + c.close
		text, calls := p.Feed(in)
		if text != "hi " {
			t.Fatalf("text = %q for %q", text, c.open)
		}
		if len(calls) != 1 {
			t.Fatalf("calls = %d for %q", len(calls), c.open)
		}
	}
}

func TestParserFlushRecoversUnclosedTag(t *testing.T) {
	p := NewParser()
	_, _ = p.Feed("前缀文本<tool_call>{\"name\":\"a\",\"arguments\":{\"x\":1}}")
	text, calls := p.Flush()
	if text != "" {
		t.Fatalf("flush text = %q, want empty", text)
	}
	if len(calls) != 1 || calls[0].Function.Name != "a" {
		t.Fatalf("flush calls = %#v", calls)
	}
}

func TestParserFlushEmitsRawOnUnparseable(t *testing.T) {
	p := NewParser()
	text1, _ := p.Feed("hello<tool_call>{garbage")
	// 第一次 Feed 已经把 "hello" 作为正常文本吐出来
	if text1 != "hello" {
		t.Fatalf("first text = %q, want hello", text1)
	}
	text, calls := p.Flush()
	if len(calls) != 0 {
		t.Fatalf("calls = %#v, want none", calls)
	}
	// 解析失败时把 <tool_call> 标签也回吐,避免吞掉用户可见文本
	if !strings.Contains(text, "{garbage") {
		t.Fatalf("flush text = %q, want contains {garbage", text)
	}
	if !strings.Contains(text, "<tool_call>") {
		t.Fatalf("flush text = %q, want contains <tool_call> tag", text)
	}
}

func TestParserDoesNotEmitRawJSONMidStream(t *testing.T) {
	p := NewParser()
	// 模型直接输出 JSON,没包 <tool_call> —— 中间 chunk 应该被暂存,等下一个 chunk
	// 判断是否有闭合标签(避免把未完成的 JSON 提前吐给用户)
	text, _ := p.Feed(`{"name": "a", "arguments":`)
	if text != "" {
		t.Fatalf("text = %q, want empty (mid-stream JSON 等待闭合)", text)
	}
	// 中途又来一段普通文本 —— 确认前半截 JSON 仍被 hold 住
	text, _ = p.Feed(` unfinished`)
	if text != "" {
		t.Fatalf("text = %q, want empty", text)
	}
	// 流结束 → Flush 把 hold 住的 raw 当文本回吐
	text, calls := p.Flush()
	if len(calls) != 0 {
		t.Fatalf("calls = %#v, want none (不完整 JSON 无法解析)", calls)
	}
	if text == "" {
		t.Fatalf("text should fall back to raw emission on flush")
	}
}

func TestParserFlushTreatsBareJSONAsToolCall(t *testing.T) {
	p := NewParser()
	// 模型没写 <tool_call> 标签,但 buffer 是一段完整的 JSON
	text, _ := p.Feed(`{"name": "a", "arguments": {}}`)
	if text != "" {
		t.Fatalf("text = %q, want empty (hold 住 raw JSON)", text)
	}
	// 流结束 → bare JSON 被当作 tool_call
	_, calls := p.Flush()
	if len(calls) != 1 || calls[0].Function.Name != "a" {
		t.Fatalf("flush calls = %#v, want one tool_call", calls)
	}
}

func TestFixLoneBackslashesWindowsPath(t *testing.T) {
	in := `{"filePath":"G:\src\file.py"}`
	out := fixLoneBackslashes(in)
	// \s 不是合法 JSON 转义 → 被双倍;Go json 接受 \f,\b 等控制字符,所以保留
	if !strings.Contains(out, `G:\\src`) {
		t.Fatalf("output should escape lone \\s, got %q", out)
	}
}

func TestFixLoneBackslashesPreservesValidEscapes(t *testing.T) {
	in := `{"a":"line1\nline2","b":"quote: \"x\""}`
	out := fixLoneBackslashes(in)
	// 合法转义保留
	if !strings.Contains(out, `line1\nline2`) {
		t.Fatalf("lost \\n: %q", out)
	}
	if !strings.Contains(out, `\"x\"`) {
		t.Fatalf("lost \\\": %q", out)
	}
}

func TestFixLoneBackslashesRegex(t *testing.T) {
	// \d 不是合法 JSON 转义 → 修正
	in := `{"pattern":"\d+"}`
	out := fixLoneBackslashes(in)
	if !strings.Contains(out, `\\d+`) {
		t.Fatalf("output = %q", out)
	}
}

func TestRobustJSONRepairsWindowsPath(t *testing.T) {
	// Go json 接受 \f 解析为 form-feed 字符(0x0C),所以最终 path 会损失 "f" 字母
	// —— 这是  同源行为:仅修复 *真正* 会被 Go json 拒绝的 \s 等。
	in := `{"filePath":"G:\src\file.py","ok":true}`
	v, ok := robustJSON(in)
	if !ok {
		t.Fatalf("robustJSON should produce valid json")
	}
	// 把对象重新 marshal 出去,确认原始 \s 已被双倍(即出现 \\s)。
	// 不查 "含 \s":会与 "\\s" 误匹配;改用 \s 前面不是 \ 的模式。
	str, _ := json.Marshal(v)
	s := string(str)
	for i := 0; i < len(s)-1; i++ {
		if s[i] == '\\' && s[i+1] == 's' {
			if i > 0 && s[i-1] == '\\' {
				continue // \s 已被修正为 \\s,这是正确结果
			}
			t.Fatalf("found lone \\s at offset %d: %s", i, s)
		}
	}
}

func TestRobustJSONTruncatesUnbalanced(t *testing.T) {
	in := `{"a": 1, "b": "unterminated` // 没有 } 和 "
	v, ok := robustJSON(in)
	if ok {
		t.Fatalf("robustJSON should fail on truly broken input, got %#v", v)
	}
}

func TestRecoverFromTextExtractsToolCall(t *testing.T) {
	tools := []official.Tool{
		{Type: "function", Function: official.ToolFunction{Name: "bash", Parameters: json.RawMessage(`{"properties":{"command":{"type":"string"}}}`)}},
	}
	// 模型绕过标签,直接输出 JSON
	text := `some text {"name":"read","arguments":{"filePath":"/tmp/x"}} more text`
	calls := RecoverFromText(text, tools)
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1", len(calls))
	}
	if calls[0].Function.Name != "read" {
		t.Fatalf("name = %q", calls[0].Function.Name)
	}
}

func TestRecoverFromTextShellFallback(t *testing.T) {
	tools := []official.Tool{
		{Type: "function", Function: official.ToolFunction{Name: "bash", Parameters: json.RawMessage(`{"properties":{"command":{"type":"string"}}}`)}},
	}
	// 模型输出沙箱格式
	text := `{"cmd": ["powershell", "-Command", "Get-Process"]}`
	calls := RecoverFromText(text, tools)
	if len(calls) != 1 {
		t.Fatalf("calls = %d", len(calls))
	}
	if calls[0].Function.Name != "bash" {
		t.Fatalf("name = %q", calls[0].Function.Name)
	}
	if !strings.Contains(calls[0].Function.Arguments, "Get-Process") {
		t.Fatalf("args = %q", calls[0].Function.Arguments)
	}
}

func TestRecoverFromTextDeduplicates(t *testing.T) {
	tools := []official.Tool{
		{Type: "function", Function: official.ToolFunction{Name: "x", Parameters: json.RawMessage(`{}`)}},
	}
	text := `{"name":"x","arguments":{"a":1}} {"name":"x","arguments":{"a":1}}`
	calls := RecoverFromText(text, tools)
	if len(calls) != 1 {
		t.Fatalf("calls = %d, want 1 dedup", len(calls))
	}
}

func TestShellCmdToString(t *testing.T) {
	cases := []struct {
		in   any
		want string
	}{
		{"ls -la", "ls -la"},
		{[]any{"bash", "-c", "ls"}, "ls"},
		{[]any{"powershell", "-Command", "Get-Process"}, "Get-Process"},
		{[]any{"pwsh", "-Command", "A", "B"}, "A B"},
		{[]any{"ls", "-la"}, "ls -la"},
		{[]string{"ls", "-la"}, "ls -la"},
	}
	for _, c := range cases {
		if got := cmdToString(c.in); got != c.want {
			t.Errorf("cmdToString(%#v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestResolveShellTool(t *testing.T) {
	tools := []official.Tool{
		{Type: "function", Function: official.ToolFunction{Name: "glob"}},
		{Type: "function", Function: official.ToolFunction{Name: "bash", Parameters: json.RawMessage(`{"properties":{"command":{"type":"string"}}}`)}},
	}
	name, param := ResolveShellTool(tools)
	if name != "bash" || param != "command" {
		t.Fatalf("got (%q, %q)", name, param)
	}
}

func TestStreamToToolCallDeltas(t *testing.T) {
	calls := []official.ToolCall{
		{ID: "c1", Type: "function", Function: official.ToolCallFunc{Name: "x", Arguments: `{"a":1}`}},
	}
	deltas := StreamToToolCallDeltas(calls)
	if len(deltas) != 2 {
		t.Fatalf("deltas = %d, want 2 (name + args)", len(deltas))
	}
	if deltas[0][0].Function.Name != "x" || deltas[0][0].ID != "c1" {
		t.Fatalf("name delta = %#v", deltas[0])
	}
	if deltas[1][0].Function.Arguments != `{"a":1}` {
		t.Fatalf("args delta = %#v", deltas[1])
	}
	if deltas[0][0].Index != 0 || deltas[1][0].Index != 0 {
		t.Fatalf("index wrong: %+v", deltas)
	}
}

func TestSerializeForHistory(t *testing.T) {
	refs := []official.ToolCallRef{
		{Index: 0, ID: "c1", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "read", Arguments: `{"filePath":"/x"}`}},
		{Index: 1, ID: "c2", Function: struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"`
		}{Name: "bash", Arguments: `{"command":"ls"}`}},
	}
	out := SerializeForHistory(refs)
	if !strings.Contains(out, `<tool_call>{"name": "read", "arguments": {"filePath":"/x"}}</tool_call>`) {
		t.Fatalf("missing first: %s", out)
	}
	if !strings.Contains(out, `<tool_call>{"name": "bash", "arguments": {"command":"ls"}}</tool_call>`) {
		t.Fatalf("missing second: %s", out)
	}
}

func TestBuildInstructionsEmpty(t *testing.T) {
	if got := BuildInstructions(nil, nil); got != "" {
		t.Fatalf("got = %q, want empty", got)
	}
}

func TestBuildInstructionsIncludesNameAndDescription(t *testing.T) {
	tools := []official.Tool{
		{Type: "function", Function: official.ToolFunction{
			Name:        "bash",
			Description: "Run a shell command",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"command":{"type":"string","description":"cmd to run"}},"required":["command"]}`),
		}},
	}
	got := BuildInstructions(tools, nil)
	for _, expect := range []string{"bash", "Run a shell command", "command", "required", "TOOL CALLING FORMAT"} {
		if !strings.Contains(got, expect) {
			t.Errorf("missing %q in:\n%s", expect, got)
		}
	}
}

func TestBuildInstructionsForcedChoice(t *testing.T) {
	tools := []official.Tool{
		{Type: "function", Function: official.ToolFunction{Name: "bash"}},
	}
	choice := &official.ToolChoice{Type: "function", Function: &official.ToolChoiceFunction{Name: "bash"}}
	got := BuildInstructions(tools, choice)
	if !strings.Contains(got, `MUST call the tool "bash"`) {
		t.Fatalf("missing forced-call line: %s", got)
	}
}

func TestBuildInstructionsForcedNone(t *testing.T) {
	tools := []official.Tool{
		{Type: "function", Function: official.ToolFunction{Name: "bash"}},
	}
	choice := &official.ToolChoice{Type: "none"}
	got := BuildInstructions(tools, choice)
	if !strings.Contains(got, "DISABLED tool calling") {
		t.Fatalf("missing none-warning: %s", got)
	}
}

func TestFirstToolCallExamplePicksBash(t *testing.T) {
	tools := []official.Tool{
		{Type: "function", Function: official.ToolFunction{Name: "glob"}},
		{Type: "function", Function: official.ToolFunction{Name: "bash"}},
	}
	ex := FirstToolCallExample(tools, "/tmp")
	if !strings.Contains(ex, `"bash"`) {
		t.Fatalf("ex = %q", ex)
	}
}

func TestFirstToolCallExamplePicksFirst(t *testing.T) {
	tools := []official.Tool{
		{Type: "function", Function: official.ToolFunction{Name: "magic_tool"}},
	}
	ex := FirstToolCallExample(tools, "")
	if !strings.Contains(ex, `"magic_tool"`) {
		t.Fatalf("ex = %q", ex)
	}
}

func TestExtractWorkingDir(t *testing.T) {
	msgs := []official.APIMessage{
		{Role: "user", Content: official.MessageContent{TextValue: "Working directory: /home/x"}},
		{Role: "user", Content: official.MessageContent{TextValue: "Working directory: /home/y"}},
	}
	if got := ExtractWorkingDir(msgs); got != "/home/x" {
		t.Fatalf("got %q, want /home/x", got)
	}
}

func TestFinalNudgeForToolResult(t *testing.T) {
	tools := []official.Tool{
		{Type: "function", Function: official.ToolFunction{Name: "bash"}},
	}
	msgs := []official.APIMessage{
		{Role: "user", Content: official.MessageContent{TextValue: "list"}},
		{Role: "assistant", Content: official.MessageContent{TextValue: ""}},
		{Role: "tool", Content: official.MessageContent{TextValue: "file1\nfile2"}},
	}
	got := FinalNudge(tools, msgs)
	if got == "" {
		t.Fatalf("got empty nudge")
	}
	if !strings.Contains(got, "ground truth") {
		t.Fatalf("missing ground-truth language: %s", got)
	}
}

func TestFinalNudgeForUserTurn(t *testing.T) {
	tools := []official.Tool{
		{Type: "function", Function: official.ToolFunction{Name: "bash"}},
	}
	msgs := []official.APIMessage{
		{Role: "user", Content: official.MessageContent{TextValue: "Working directory: /home/x\n列出文件"}},
	}
	got := FinalNudge(tools, msgs)
	if got == "" {
		t.Fatalf("got empty")
	}
	if !strings.Contains(got, "working directory: /home/x") {
		t.Fatalf("missing wd: %s", got)
	}
	if !strings.Contains(got, `"bash"`) {
		t.Fatalf("missing example: %s", got)
	}
}

func TestFinalNudgeEmptyForOtherRole(t *testing.T) {
	tools := []official.Tool{
		{Type: "function", Function: official.ToolFunction{Name: "bash"}},
	}
	msgs := []official.APIMessage{
		{Role: "assistant", Content: official.MessageContent{TextValue: "hi"}},
	}
	if got := FinalNudge(tools, msgs); got != "" {
		t.Fatalf("got = %q, want empty for non-tool/user last role", got)
	}
}
