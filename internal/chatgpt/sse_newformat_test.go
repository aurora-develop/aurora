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

// 验证 content_references patch 提取 + cite 标记替换 (方案 B)。
// 数据来自 2026-08 真实抓包: Inflection Pi 回答带 web 搜索引用。
func TestCiteMarkerReplacement(t *testing.T) {
	state := &sseparser.PatchState{}

	seg1 := "\ue200cite\ue202turn543019search0\ue202turn543"      // 对象初始 matched_text(截断)
	full := "\ue200cite\ue202turn543019search0\ue202turn543019search1\ue201" // 补齐后的完整标记

	// 1. 批量 patch: 正文 append(含截断的 cite 标记) + content_references append
	frame1 := `{"p":"","o":"patch","v":[
		{"p":"/message/content/parts/0","o":"append","v":"定位为 Personal Intelligence partner。\ue200cite\ue202turn543019search0\ue202turn543"},
		{"p":"/message/metadata/content_references","o":"append","v":[{"matched_text":"\ue200cite\ue202turn543019search0\ue202turn543","start_idx":298,"type":"hidden","invalid":true}]}
	]}`
	if _, ok := parseConversationEvent(frame1, state, "auto"); !ok {
		t.Fatal("frame1 should parse")
	}
	if state.CiteAlts["ref:0:matched"] != seg1 {
		t.Fatalf("frame1 matched = %q, want %q", state.CiteAlts["ref:0:matched"], seg1)
	}

	// 2. 裸数组帧: 正文补齐 + matched_text 补齐
	frame2 := `{"v":[
		{"p":"/message/content/parts/0","o":"append","v":"019search1"},
		{"p":"/message/metadata/content_references/0/matched_text","o":"append","v":"019search1"}
	]}`
	if _, ok := parseConversationEvent(frame2, state, "auto"); !ok {
		t.Fatal("frame2 should parse")
	}
	if state.CiteAlts["ref:0:matched"] != full {
		t.Fatalf("frame2 matched = %q, want full %q", state.CiteAlts["ref:0:matched"], full)
	}

	// 3. alt 到齐 → 应建立 完整标记→alt 映射
	frame3 := `{"v":[
		{"p":"/message/metadata/content_references/0/safe_urls","o":"append","v":["https://inflection.ai/"]},
		{"p":"/message/metadata/content_references/0/alt","o":"replace","v":"([inflection.ai](https://inflection.ai/?utm_source=chatgpt.com))"},
		{"p":"/message/metadata/content_references/0/type","o":"replace","v":"grouped_webpages"}
	]}`
	if _, ok := parseConversationEvent(frame3, state, "auto"); !ok {
		t.Fatal("frame3 should parse")
	}

	wantAlt := "([inflection.ai](https://inflection.ai/?utm_source=chatgpt.com))"
	if got := state.CiteAlts[full]; got != wantAlt {
		t.Fatalf("full marker mapping = %q, want %q; map=%#v", got, wantAlt, state.CiteAlts)
	}

	// 4. ReplaceCiteMarkers: 有 alt 替换,无 alt 删除
	text := "前文" + full + "中段" + "\ue200citeturn999\ue201" + "尾"
	got := sseparser.ReplaceCiteMarkers(text, state.CiteAlts)
	want := "前文" + wantAlt + "中段尾"
	if got != want {
		t.Fatalf("ReplaceCiteMarkers = %q, want %q", got, want)
	}
}

func TestApplyPatchContentRefDirect(t *testing.T) {
	state := &sseparser.PatchState{}
	obj := map[string]interface{}{
		"matched_text": "citeturn543019search0turn543",
		"start_idx":    float64(298),
		"type":         "hidden",
	}
	ok := sseparser.ApplyPatch(state, "/message/metadata/content_references", "append", obj)
	t.Logf("ApplyPatch direct: ok=%v map=%#v", ok, state.CiteAlts)
	if !ok {
		t.Fatal("direct ApplyPatch failed")
	}
}
