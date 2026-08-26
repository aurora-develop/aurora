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

full := "\ue200cite\ue202turn543019search0\ue202turn543019search1\ue201"
	seg1 := "\ue200cite\ue202turn543019search0\ue202turn543"      // 对象初始 matched_text(截断)

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

// 验证流式 hold-back 管道: 标记跨帧切分 + alt 晚到,不再透传乱码/丢失链接。
func TestCiteStreamPipelineSplitMarker(t *testing.T) {
	state := &sseparser.PatchState{}
	pipeline := &sseparser.CiteStreamPipeline{}

	alt := "([inflection.ai](https://inflection.ai/?utm_source=chatgpt.com))"

	// 帧1: 正文含半截标记(有头无尾) —— 应整体暂存,不输出乱码
	out1 := pipeline.Feed(state.CiteAlts, "定位为 Personal Intelligence partner。citeturn543019search0turn543")
	if strings.ContainsRune(out1, '') {
		t.Fatalf("frame1 leaked unclosed marker: %q", out1)
	}
	if out1 != "定位为 Personal Intelligence partner。" {
		t.Fatalf("frame1 = %q", out1)
	}

	// 帧2: 补齐正文 + matched_text append
	out2 := pipeline.Feed(state.CiteAlts, "019search1")
	if out2 != "" {
		t.Fatalf("frame2 should hold (marker complete but no alt yet), got %q", out2)
	}

	// patch: matched_text 到齐
	frameMatched := `{"p":"/message/metadata/content_references/0/matched_text","o":"append","v":"citeturn543019search0turn543"}`
	parseConversationEvent(frameMatched, state, "auto")
	frameMatched2 := `{"p":"/message/metadata/content_references/0/matched_text","o":"append","v":"019search1"}`
	parseConversationEvent(frameMatched2, state, "auto")

	// 帧3: alt 到达 → 暂存的完整标记应替换为链接输出
	frameAlt := `{"v":[
		{"p":"/message/metadata/content_references/0/alt","o":"replace","v":"([inflection.ai](https://inflection.ai/?utm_source=chatgpt.com))"},
		{"p":"/message/content/parts/0","o":"append","v":""}
	]}`
	if _, ok := parseConversationEvent(frameAlt, state, "auto"); !ok {
		t.Fatal("frameAlt should parse")
	}
	// 帧3 的正文补丁(闭合符)也进入管道; alt 已在映射中 → 暂存标记应替换输出
	out3 := pipeline.Feed(state.CiteAlts, string([]rune{0xE201}))
	if !strings.Contains(out3, alt) {
		t.Fatalf("alt not flushed after arrival, got %q; map=%#v", out3, state.CiteAlts)
	}
	if strings.ContainsRune(out3, '') {
		t.Fatalf("marker residue in flush: %q", out3)
	}
}

// 验证 Flush: 流结束时无 alt 的标记被删除(不残留私有区字符)。
func TestCiteStreamPipelineFlushNoAlt(t *testing.T) {
	state := &sseparser.PatchState{}
	pipeline := &sseparser.CiteStreamPipeline{}

	marker := string([]rune{0xE200}) + "citeturn999search0" + string([]rune{0xE201})
	out1 := pipeline.Feed(state.CiteAlts, "前文" + marker + "尾")
	// 无 alt → 从标记起点暂存; 标记前的正文照常放行
	if out1 != "前文" {
		t.Fatalf("head before marker should flush, got %q", out1)
	}
	// 流结束: Flush 删除无 alt 的标记,保留剩余正文
	flushed := pipeline.Flush(state.CiteAlts)
	if flushed != "尾" {
		t.Fatalf("flush = %q, want 尾", flushed)
	}
}

// 验证 SplitCiteHold: 正常文本直接放行。
func TestSplitCiteHoldPlainText(t *testing.T) {
	flush, remain := sseparser.SplitCiteHold("普通文本没有标记", nil)
	if flush != "普通文本没有标记" || remain != "" {
		t.Fatalf("plain text split wrong: flush=%q remain=%q", flush, remain)
	}
}

// 验证 ReplaceCiteMarkers: 未闭合残缺标记直接删除。
func TestReplaceCiteMarkersUnclosed(t *testing.T) {
	got := sseparser.ReplaceCiteMarkers("开头citeturn1search0没有闭合", nil)
	if got != "开头" {
		t.Fatalf("unclosed marker not dropped: %q", got)
	}
}

// 验证无 alt 标记的类型感知兜底: entity 卡片保留显示名,其余删除。
func TestMarkerFallbackEntity(t *testing.T) {
	E200 := string([]rune{0xE200})
	E201 := string([]rune{0xE201})
	E202 := string([]rune{0xE202})

	// entity 人物卡片 -> 保留显示名
	text := "创始团队包括 " + E200 + "entity" + E202 + "[\"people\",\"Mustafa Suleyman\",\"co-founder of DeepMind\"]" + E201 + " 等"
	got := sseparser.ReplaceCiteMarkers(text, nil)
	if got != "创始团队包括 Mustafa Suleyman 等" {
		t.Fatalf("entity fallback = %q", got)
	}

	// cite 引用残留(无 alt) -> 整体删除
	text2 := "前文" + E200 + "cite" + E202 + "turn0search1" + E202 + "turn0search7" + E201 + "后文"
	got2 := sseparser.ReplaceCiteMarkers(text2, nil)
	if got2 != "前文后文" {
		t.Fatalf("cite no-alt = %q", got2)
	}

	// image_group 指令 -> 删除
	text3 := E200 + "image_group" + E202 + `{"layout":"bento","query":["logo"]}` + E201
	got3 := sseparser.ReplaceCiteMarkers(text3, nil)
	if got3 != "" {
		t.Fatalf("image_group should drop, got %q", got3)
	}
}

// 验证第三种批量帧形态: {"o":"patch","v":[...]} (省略顶层 p)。
// 真实抓包中 image_group 首帧即此形态,之前被整体丢弃导致正文缺头。
func TestBatchPatchFrameWithoutPath(t *testing.T) {
	state := &sseparser.PatchState{}

	frame := `{"o":"patch","v":[
		{"p":"/message/content/parts/0","o":"append","v":"开头文本"},
		{"p":"/message/status","o":"replace","v":"in_progress"}
	]}`
	ev, ok := parseConversationEvent(frame, state, "auto")
	if !ok {
		t.Fatal("o-patch-no-p frame should parse")
	}
	got, _ := state.Response.Message.Content.Parts[0].(string)
	if got != "开头文本" {
		t.Fatalf("accumulated = %q", got)
	}
	_ = ev
}