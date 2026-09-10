package chatgpt

import (
	"aurora/internal/accounts"
	chatgpt_types "aurora/typings/chatgpt"
	"aurora/typings/official"
	"encoding/json"
	"strings"
	"testing"
)

var testAccount = accounts.NewAccount("test", accounts.TypeNoAuth, "")

func testConvert(t *testing.T, req official.APIRequest) chatgpt_types.ChatGPTRequest {
	t.Helper()
	return ConvertAPIRequest(req, testAccount, "", nil)
}

func TestConvertAPIRequestNoToolsNoInjection(t *testing.T) {
	req := official.APIRequest{
		Model:    "gpt-5",
		Messages: []official.APIMessage{official.NewTextMessage("user", "hi")},
	}
	out := testConvert(t, req)
	if len(out.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(out.Messages))
	}
	if out.Messages[0].Author.Role != "user" {
		t.Fatalf("role = %q", out.Messages[0].Author.Role)
	}
}

func TestConvertAPIRequestRoutesThinkingAliasThroughReasonHint(t *testing.T) {
	for _, model := range []string{"gpt-5-6-t-mini", "gpt-5-6-thinking"} {
		t.Run(model, func(t *testing.T) {
			out := testConvert(t, official.APIRequest{
				Model:    model,
				Messages: []official.APIMessage{official.NewTextMessage("user", "hi")},
			})

			if out.Model != "auto" {
				t.Fatalf("Model = %q, want auto", out.Model)
			}
			if len(out.SystemHints) != 1 || out.SystemHints[0] != "reason" {
				t.Fatalf("SystemHints = %#v, want [reason]", out.SystemHints)
			}
			metadata := out.Messages[0].Metadata
			hints, ok := metadata["system_hints"].([]string)
			if !ok || len(hints) != 1 || hints[0] != "reason" {
				t.Fatalf("message system_hints = %#v, want [reason]", metadata["system_hints"])
			}
		})
	}
}

func TestConvertAPIRequestKeepsExplicitModelWithoutReasonHint(t *testing.T) {
	out := testConvert(t, official.APIRequest{
		Model:    "gpt-5-6-pro",
		Messages: []official.APIMessage{official.NewTextMessage("user", "hi")},
	})

	if out.Model != "gpt-5-6-pro" {
		t.Fatalf("Model = %q, want gpt-5-6-pro", out.Model)
	}
	if len(out.SystemHints) != 0 {
		t.Fatalf("SystemHints = %#v, want empty", out.SystemHints)
	}
	if _, ok := out.Messages[0].Metadata["system_hints"]; ok {
		t.Fatalf("message unexpectedly includes system_hints: %#v", out.Messages[0].Metadata)
	}
}

func TestConvertAPIRequestMapsReasoningEffortToWebEnum(t *testing.T) {
	tests := []struct {
		name   string
		effort string
		want   string
	}{
		{name: "default", effort: "", want: "standard"},
		{name: "minimal", effort: "minimal", want: "standard"},
		{name: "low", effort: "low", want: "standard"},
		{name: "medium", effort: "medium", want: "extended"},
		{name: "standard", effort: "standard", want: "standard"},
		{name: "extended", effort: "extended", want: "extended"},
		{name: "high", effort: "high", want: "max"},
		{name: "xhigh", effort: "xhigh", want: "max"},
		{name: "max", effort: "max", want: "max"},
		{name: "unknown", effort: "turbo", want: "standard"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := testConvert(t, official.APIRequest{
				Model:           "gpt-5",
				ReasoningEffort: tt.effort,
				Messages:        []official.APIMessage{official.NewTextMessage("user", "hi")},
			})
			if out.ThinkingEffort != tt.want {
				t.Fatalf("ThinkingEffort = %q, want %q", out.ThinkingEffort, tt.want)
			}
		})
	}
}

func TestConvertAPIRequestInjectsToolInstructions(t *testing.T) {
	req := official.APIRequest{
		Model: "gpt-5",
		Tools: []official.Tool{
			{Type: "function", Function: official.ToolFunction{
				Name:        "bash",
				Description: "Run a shell command",
				Parameters:  json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}`),
			}},
		},
		Messages: []official.APIMessage{
			official.NewTextMessage("user", "list files"),
		},
	}
	out := testConvert(t, req)
	if len(out.Messages) < 2 {
		t.Fatalf("messages = %d, want ≥ 2 (system + user + nudge)", len(out.Messages))
	}
	// 头部应该是 system 消息,包含工具说明
	first := out.Messages[0]
	if first.Author.Role != "system" {
		t.Fatalf("first role = %q, want system", first.Author.Role)
	}
	firstText, _ := first.Content.Parts[0].(string)
	for _, want := range []string{"bash", "Run a shell command", "TOOL CALLING FORMAT", "<tool_call>"} {
		if !strings.Contains(firstText, want) {
			t.Errorf("system message missing %q", want)
		}
	}
}

func TestConvertAPIRequestAppendsFinalNudgeForUserTurn(t *testing.T) {
	// FinalNudge removed: capability folded into the <tool_calling_protocol> block.
	// This test now verifies: no trailing nudge message, protocol tag wrapping,
	// and the user's own system prompt staying first.
	req := official.APIRequest{
		Model: "gpt-5",
		Tools: []official.Tool{
			{Type: "function", Function: official.ToolFunction{Name: "bash"}},
		},
		Messages: []official.APIMessage{
			official.NewTextMessage("system", "You are a helpful assistant."),
			official.NewTextMessage("user", "list files in /home/x"),
		},
	}
	out := testConvert(t, req)
	last := out.Messages[len(out.Messages)-1]
	if last.Author.Role != "user" {
		t.Fatalf("last role = %q, want user", last.Author.Role)
	}
	lastText, _ := last.Content.Parts[0].(string)
	if !strings.Contains(lastText, "list files in /home/x") {
		t.Fatalf("last message should be original user text, got: %s", lastText)
	}
	if strings.Contains(lastText, "READ CAREFULLY") {
		t.Fatal("nudge should not be appended to the user message")
	}
	first := out.Messages[0]
	firstText, _ := first.Content.Parts[0].(string)
	if !strings.HasPrefix(firstText, "You are a helpful assistant.") {
		t.Fatalf("user system prompt should come first, got: %s", firstText[:60])
	}
	if !strings.Contains(firstText, "<tool_calling_protocol>") {
		t.Fatal("protocol block should be wrapped in <tool_calling_protocol>")
	}
}

func TestConvertAPIRequestHandlesToolResult(t *testing.T) {
	req := official.APIRequest{
		Model: "gpt-5",
		Tools: []official.Tool{
			{Type: "function", Function: official.ToolFunction{Name: "bash"}},
		},
		Messages: []official.APIMessage{
			{Role: "assistant", Content: official.MessageContent{TextValue: ""}, ToolCalls: []official.ToolCallRef{{ID: "c1", Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: "bash", Arguments: `{"command":"ls"}`}}}},
			{Role: "tool", ToolCallID: "c1", Name: "bash", Content: official.MessageContent{TextValue: "file1.py\nfile2.py"}},
		},
	}
	out := testConvert(t, req)
	var toolMsg string
	for _, m := range out.Messages {
		if m.Author.Role != "user" || len(m.Content.Parts) == 0 {
			continue
		}
		text, _ := m.Content.Parts[0].(string)
		if strings.Contains(text, "[tool result of") {
			toolMsg = text
			break
		}
	}
	if !strings.Contains(toolMsg, "[tool result of bash]") {
		t.Fatalf("tool message missing  prefix: %q", toolMsg)
	}
	if !strings.Contains(toolMsg, "file1.py") {
		t.Fatalf("tool message missing content: %q", toolMsg)
	}
}

func TestConvertAPIRequestSerializesHistoryToolCalls(t *testing.T) {
	req := official.APIRequest{
		Model: "gpt-5",
		Tools: []official.Tool{
			{Type: "function", Function: official.ToolFunction{Name: "bash"}},
		},
		Messages: []official.APIMessage{
			{Role: "user", Content: official.MessageContent{TextValue: "list"}},
			{Role: "assistant", Content: official.MessageContent{TextValue: ""}, ToolCalls: []official.ToolCallRef{{ID: "c1", Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: "bash", Arguments: `{"command":"ls"}`}}}},
		},
	}
	out := testConvert(t, req)
	// 找到 assistant 消息,确认 <tool_call> 标签已序列化
	var found bool
	for _, m := range out.Messages {
		if m.Author.Role == "assistant" {
			parts := m.Content.Parts
			for _, p := range parts {
				if s, ok := p.(string); ok && strings.Contains(s, "<tool_call>") {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("assistant message missing <tool_call> serialization: %#v", out.Messages)
	}
}

func TestConvertAPIRequestForcedToolChoice(t *testing.T) {
	choice := &official.ToolChoice{Type: "function", Function: &official.ToolChoiceFunction{Name: "bash"}}
	req := official.APIRequest{
		Model:      "gpt-5",
		Tools:      []official.Tool{{Type: "function", Function: official.ToolFunction{Name: "bash"}}},
		ToolChoice: choice,
		Messages:   []official.APIMessage{official.NewTextMessage("user", "x")},
	}
	out := testConvert(t, req)
	text, _ := out.Messages[0].Content.Parts[0].(string)
	if !strings.Contains(text, `MUST call the tool "bash"`) {
		t.Fatalf("missing forced-call line: %s", text)
	}
}

func TestConvertAPIRequestToolChoiceNoneStripsProtocol(t *testing.T) {
	// tool_choice=none + tools:仍要教模型协议(否则它不知道 "none" 是什么意思),
	// 但要追加 "DISABLED tool calling" 警告
	req := official.APIRequest{
		Model:      "gpt-5",
		Tools:      []official.Tool{{Type: "function", Function: official.ToolFunction{Name: "bash"}}},
		ToolChoice: &official.ToolChoice{Type: "none"},
		Messages:   []official.APIMessage{official.NewTextMessage("user", "just answer in text")},
	}
	out := testConvert(t, req)
	text, _ := out.Messages[0].Content.Parts[0].(string)
	if !strings.Contains(text, "DISABLED tool calling") {
		t.Fatalf("missing none-warning: %s", text)
	}
}
