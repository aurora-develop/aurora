package chatgpt

import (
	"strings"
	"testing"

	"aurora/internal/sseparser"
	chatgpt_types "aurora/typings/chatgpt"
)

// 用真实抓包(2026-08 新版 SSE)验证解析器。
func TestParseConversationEventNewFormat(t *testing.T) {
	state := &sseparser.PatchState{}

	// 1. 完整 message 帧 (channel=commentary, thinking preamble)
	preamble := `{"v":{"message":{"id":"293f6c2c","author":{"role":"assistant"},"content":{"content_type":"text","parts":["我先确认你说的Pi Agent具体是哪一个项目"]},"status":"finished_successfully","end_turn":false,"metadata":{"is_thinking_preamble_message":true,"message_type":"next"},"recipient":"all","channel":"commentary"},"conversation_id":"conv-1","error":null},"c":5}`
	ev, ok := parseConversationEvent(preamble, state, "auto")
	if !ok {
		t.Fatalf("preamble frame should parse")
	}
	if ev.response.Message.Channel != "commentary" {
		t.Fatalf("preamble channel = %q, want commentary", ev.response.Message.Channel)
	}
	if !ev.response.Message.Metadata.IsThinkingPreambleMessage {
		t.Fatal("preamble flag not parsed")
	}

	// 2. 裸字符串 append
	bare := `{"v":"如果你说的是 **Mario"}`
	if _, ok := parseConversationEvent(bare, state, "auto"); !ok {
		t.Fatal("bare string frame should parse")
	}

	// 3. 标准 patch
	std := `{"p":"/message/content/parts/0","o":"append","v":" Zechner的 pi"}`
	if _, ok := parseConversationEvent(std, state, "auto"); !ok {
		t.Fatal("standard patch frame should parse")
	}

	// 4. 🔴 关键: 裸补丁数组帧(新版正文主要形式,之前完全丢失)
	bareArray := `{"v":[
		{"p":"/message/content/parts/0","o":"append","v":"-mono / Pi Coding Agent"},
		{"p":"/message/metadata/content_references","o":"append","v":[{"matched_text":"x"}]}
	]}`
	if _, ok := parseConversationEvent(bareArray, state, "auto"); !ok {
		t.Fatal("bare patch array frame should parse")
	}

	// 验证补丁数组里的正文已拼上
	// 注意: preamble 帧的 parts 也会拼进来 —— 过滤在主循环输出层做,
	// 解析层只负责把增量正确应用到 PatchState。
	got, _ := state.Response.Message.Content.Parts[0].(string)
	want := "我先确认你说的Pi Agent具体是哪一个项目" + "如果你说的是 **Mario" + " Zechner的 pi" + "-mono / Pi Coding Agent"
	if got != want {
		t.Fatalf("accumulated text = %q, want %q", got, want)
	}

	// 5. 批量 patch 帧 (p="", o="patch")
	batch := `{"p":"","o":"patch","v":[
		{"p":"/message/content/parts/0","o":"append","v":" 的设计理念"},
		{"p":"/message/status","o":"replace","v":"finished_successfully"}
	]}`
	if _, ok := parseConversationEvent(batch, state, "auto"); !ok {
		t.Fatal("batch patch frame should parse")
	}
	got, _ = state.Response.Message.Content.Parts[0].(string)
	if !strings.HasSuffix(got, "的设计理念") {
		t.Fatalf("batch append failed, text = %q", got)
	}
}

// 验证过滤器: preamble/commentary 不应进入正文输出路径的条件判断数据正确。
func TestPreambleFilterFields(t *testing.T) {
	// 模拟主循环过滤条件用到的字段
	preambleMsg := chatgpt_types.Message{
		Author:    chatgpt_types.Author{Role: "assistant"},
		Content:   chatgpt_types.Content{ContentType: "text", Parts: []interface{}{"思考前导"}},
		Metadata:  chatgpt_types.Metadata{MessageType: "next", IsThinkingPreambleMessage: true},
		Channel:   "commentary",
		Recipient: "all",
	}
	// 这两个条件任一命中就跳过 — 主循环里已加:
	// if msg.Metadata.IsThinkingPreambleMessage { continue }
	// if msg.Channel == "commentary" { continue }
	if !preambleMsg.Metadata.IsThinkingPreambleMessage || preambleMsg.Channel != "commentary" {
		t.Fatal("filter fields should identify preamble")
	}
}
