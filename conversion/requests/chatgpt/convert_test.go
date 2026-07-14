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
	req := official.APIRequest{
		Model: "gpt-5",
		Tools: []official.Tool{
			{Type: "function", Function: official.ToolFunction{Name: "bash"}},
		},
		Messages: []official.APIMessage{
			official.NewTextMessage("user", "Working directory: /home/x\nlist"),
		},
	}
	out := testConvert(t, req)
	last := out.Messages[len(out.Messages)-1]
	if last.Author.Role != "user" {
		t.Fatalf("last role = %q, want user (nudge)", last.Author.Role)
	}
	lastText, _ := last.Content.Parts[0].(string)
	if !strings.Contains(lastText, "READ CAREFULLY") {
		t.Fatalf("nudge missing READ CAREFULLY: %s", lastText)
	}
	if !strings.Contains(lastText, "working directory: /home/x") {
		t.Fatalf("nudge missing wd: %s", lastText)
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
	// 找到 tool 消息
	var toolMsg string
	for _, m := range out.Messages {
		if m.Author.Role == "tool" {
			text, _ := m.Content.Parts[0].(string)
			toolMsg = text
		}
	}
	if !strings.Contains(toolMsg, "Resultado da ferramenta bash") {
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
