package sseparser

import (
	"encoding/json"
	"fmt"
	"strings"

	chatgpt_types "aurora/typings/chatgpt"
	official_types "aurora/typings/official"
)

// ── SSE 数据解析 ──

// DataPayloads 从 SSE 行中提取所有 data: 载荷。
func DataPayloads(line string) []string {
	var payloads []string
	for _, part := range strings.Split(strings.TrimRight(line, "\r\n"), "\n") {
		part = strings.TrimSpace(part)
		if !strings.HasPrefix(part, "data:") {
			continue
		}
		payloads = append(payloads, SplitDataPayloads(strings.TrimSpace(strings.TrimPrefix(part, "data:")))...)
	}
	return payloads
}

// SplitDataPayloads 分割拼接的 SSE data 载荷。
func SplitDataPayloads(payload string) []string {
	var payloads []string
	for {
		payload = strings.TrimSpace(payload)
		if payload == "" {
			return payloads
		}
		if strings.HasPrefix(payload, "data:") {
			payload = strings.TrimSpace(strings.TrimPrefix(payload, "data:"))
			continue
		}
		if strings.HasPrefix(payload, "[DONE]") {
			payloads = append(payloads, "[DONE]")
			payload = payload[len("[DONE]"):]
			continue
		}

		reader := strings.NewReader(payload)
		decoder := json.NewDecoder(reader)
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err == nil {
			payloads = append(payloads, string(raw))
			payload = payload[decoder.InputOffset():]
			continue
		}

		next := strings.Index(payload, "data:")
		if next < 0 {
			return payloads
		}
		if first := strings.TrimSpace(payload[:next]); first != "" {
			payloads = append(payloads, first)
		}
		payload = payload[next:]
	}
}

// EventName 从 SSE 行中提取 event 名称。
func EventName(line string) (string, bool) {
	for _, part := range strings.Split(strings.TrimRight(line, "\r\n"), "\n") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "event:") {
			return strings.TrimSpace(strings.TrimPrefix(part, "event:")), true
		}
	}
	return "", false
}

// ── Stream Handoff ──

// HandoffTopicFromPayload 从 SSE 载荷中提取 stream handoff topic ID。
func HandoffTopicFromPayload(payload string, currentEvent string) (string, bool) {
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return "", false
	}
	eventType, _ := raw["type"].(string)
	if eventType == "stream_handoff" {
		if topicID := handoffTopicFromEvent(raw); topicID != "" {
			return topicID, true
		}
		return "", true
	}
	if eventType == "server_ste_metadata" || currentEvent == "server_ste_metadata" {
		if topicID := handoffTopicFromMetadata(raw); topicID != "" {
			return topicID, true
		}
		return "", eventType == "server_ste_metadata"
	}
	if eventType == "resume_conversation_token" {
		return "", true
	}
	return "", false
}

func handoffTopicFromEvent(raw map[string]interface{}) string {
	options, ok := raw["options"].([]interface{})
	if !ok {
		return ""
	}
	for _, optionValue := range options {
		option, ok := optionValue.(map[string]interface{})
		if !ok {
			continue
		}
		optionType, _ := option["type"].(string)
		if optionType != "subscribe_ws_topic" {
			continue
		}
		topicID, _ := option["topic_id"].(string)
		return topicID
	}
	return ""
}

func handoffTopicFromMetadata(raw map[string]interface{}) string {
	if turnExchangeID, _ := raw["turn_exchange_id"].(string); turnExchangeID != "" {
		return "conversation-turn-" + turnExchangeID
	}
	metadata, ok := raw["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	if turnExchangeID, _ := metadata["turn_exchange_id"].(string); turnExchangeID != "" {
		return "conversation-turn-" + turnExchangeID
	}
	return ""
}

// ── Chat Completion Chunk 解析 ──

// ChunkFromRaw 从原始 JSON map 中提取 ChatCompletionChunk。
func ChunkFromRaw(raw map[string]interface{}, model string) (official_types.ChatCompletionChunk, bool) {
	choices, ok := raw["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return official_types.ChatCompletionChunk{}, false
	}
	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		return official_types.ChatCompletionChunk{}, false
	}
	delta, ok := choice["delta"].(map[string]interface{})
	if !ok {
		return official_types.ChatCompletionChunk{}, false
	}

	text, _ := delta["content"].(string)
	chunk := official_types.NewChatCompletionChunk(text, model)
	if id, ok := raw["id"].(string); ok && id != "" {
		chunk.ID = id
	}
	if object, ok := raw["object"].(string); ok && object != "" {
		chunk.Object = object
	}
	if created, ok := NumberToInt64(raw["created"]); ok {
		chunk.Created = created
	}
	if upstreamModel, ok := raw["model"].(string); ok && upstreamModel != "" {
		chunk.Model = upstreamModel
	}
	if role, ok := delta["role"].(string); ok && role != "" {
		chunk.Choices[0].Delta.Role = role
	}
	if finishReason, ok := choice["finish_reason"].(string); ok && finishReason != "" {
		chunk.Choices[0].FinishReason = finishReason
	}
	if conversationID, ok := raw["conversation_id"].(string); ok && conversationID != "" {
		chunk.ConversationID = conversationID
	}
	if sentinel, ok := raw["sentinel"].(map[string]interface{}); ok {
		chunk.Sentinel = sentinel
	}
	return chunk, true
}

// ── Chunk 辅助函数 ──

// ChunkContent 获取 chunk 的第一个 choice 的 content。
func ChunkContent(chunk official_types.ChatCompletionChunk) string {
	if len(chunk.Choices) == 0 {
		return ""
	}
	return chunk.Choices[0].Delta.Content
}

// ChunkRole 获取 chunk 的第一个 choice 的 role。
func ChunkRole(chunk official_types.ChatCompletionChunk) string {
	if len(chunk.Choices) == 0 {
		return ""
	}
	return chunk.Choices[0].Delta.Role
}

// ChunkFinishReason 获取 chunk 的第一个 choice 的 finish_reason。
func ChunkFinishReason(chunk official_types.ChatCompletionChunk) string {
	if len(chunk.Choices) == 0 || chunk.Choices[0].FinishReason == nil {
		return ""
	}
	if reason, ok := chunk.Choices[0].FinishReason.(string); ok {
		return reason
	}
	return fmt.Sprint(chunk.Choices[0].FinishReason)
}

// ── Channel 提取 ──

// ChannelFromValue 从任意嵌套结构中提取 channel 字段。
func ChannelFromValue(value interface{}) string {
	switch item := value.(type) {
	case map[string]interface{}:
		if channel, _ := item["channel"].(string); channel != "" {
			return channel
		}
		if delta, ok := item["delta"].(map[string]interface{}); ok {
			if channel, _ := delta["channel"].(string); channel != "" {
				return channel
			}
		}
		if choices, ok := item["choices"].([]interface{}); ok {
			for _, choiceValue := range choices {
				choice, ok := choiceValue.(map[string]interface{})
				if !ok {
					continue
				}
				if channel, _ := choice["channel"].(string); channel != "" {
					return channel
				}
				if delta, ok := choice["delta"].(map[string]interface{}); ok {
					if channel, _ := delta["channel"].(string); channel != "" {
						return channel
					}
				}
			}
		}
		if message, ok := item["message"].(map[string]interface{}); ok {
			if channel := ChannelFromValue(message); channel != "" {
				return channel
			}
		}
		if nested, ok := item["v"].(map[string]interface{}); ok {
			if channel := ChannelFromValue(nested); channel != "" {
				return channel
			}
		}
	}
	return ""
}

// ── 数值转换 ──

// NumberToInt64 把 interface{} 类型的数值转换为 int64。
func NumberToInt64(value interface{}) (int64, bool) {
	switch item := value.(type) {
	case float64:
		return int64(item), true
	case int64:
		return item, true
	case int:
		return int64(item), true
	default:
		return 0, false
	}
}

// ── Response 解析 ──

// IsUsableConversationResponse 检查 ChatGPTResponse 是否包含可用数据。
func IsUsableConversationResponse(response chatgpt_types.ChatGPTResponse) bool {
	return response.Error != nil ||
		response.Message.ID != "" ||
		response.Message.Author.Role != "" ||
		len(response.Message.Content.Parts) > 0 ||
		response.Message.EndTurn != nil
}

// ResponseFromValue 从 interface{} 中提取 ChatGPTResponse。
func ResponseFromValue(value interface{}) (chatgpt_types.ChatGPTResponse, bool) {
	if value == nil {
		return chatgpt_types.ChatGPTResponse{}, false
	}
	data, err := json.Marshal(value)
	if err != nil {
		return chatgpt_types.ChatGPTResponse{}, false
	}

	var response chatgpt_types.ChatGPTResponse
	if err := json.Unmarshal(data, &response); err == nil && IsUsableConversationResponse(response) {
		return response, true
	}

	var message chatgpt_types.Message
	if err := json.Unmarshal(data, &message); err == nil && (message.ID != "" || message.Author.Role != "" || len(message.Content.Parts) > 0 || message.EndTurn != nil) {
		response.Message = message
		return response, true
	}

	return chatgpt_types.ChatGPTResponse{}, false
}

// ── Sentinel 收集 ──

// SentinelsFromResponse 从 ChatGPTResponse 中提取所有 sentinel 事件。
func SentinelsFromResponse(response chatgpt_types.ChatGPTResponse) []map[string]interface{} {
	var raw map[string]interface{}
	data, err := json.Marshal(response)
	if err != nil {
		return nil
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	var sentinel []map[string]interface{}
	collectSentinelsFromValue(raw["sentinel"], &sentinel)
	collectSentinelsFromValue(raw["message"], &sentinel)
	return sentinel
}

func collectSentinelsFromValue(value interface{}, sentinel *[]map[string]interface{}) {
	switch item := value.(type) {
	case map[string]interface{}:
		if event, ok := item["event"].(string); ok && event != "" {
			*sentinel = append(*sentinel, item)
		}
		for _, nested := range item {
			collectSentinelsFromValue(nested, sentinel)
		}
	case []interface{}:
		for _, nested := range item {
			collectSentinelsFromValue(nested, sentinel)
		}
	}
}

// ── Conversation Patch ──

// PatchState 表示 conversation SSE 流的 patch 状态。
type PatchState struct {
	Response chatgpt_types.ChatGPTResponse
	Channel  string
	// CiteAlts 记录 cite 标记(matched_text) → alt Markdown 链接 的映射。
	// 新版 SSE (2026-08) 通过 /message/metadata/content_references patch 下发,
	// 用于把正文中的 cite...<ref> 标记替换为可读链接。
	CiteAlts map[string]string
	// nextRefIdx assigns the next index for content_references objects.
	nextRefIdx int
}

// EnsurePatchDefaults 确保 patch state 的默认值。
func EnsurePatchDefaults(state *PatchState) {
	if state.Response.Message.Author.Role == "" {
		state.Response.Message.Author.Role = "assistant"
	}
	if state.Response.Message.Recipient == "" {
		state.Response.Message.Recipient = "all"
	}
	if state.Response.Message.Content.ContentType == "" {
		state.Response.Message.Content.ContentType = "text"
	}
	if state.Response.Message.Content.Parts == nil {
		state.Response.Message.Content.Parts = []interface{}{""}
	}
	if state.Response.Message.Metadata.MessageType == "" {
		state.Response.Message.Metadata.MessageType = "next"
	}
	if state.CiteAlts == nil {
		state.CiteAlts = make(map[string]string)
	}
}

// ApplyPatch 应用一个 conversation patch 操作。
func ApplyPatch(state *PatchState, patchPath string, operation string, value interface{}) bool {
	EnsurePatchDefaults(state)
	switch {
	case patchPath == "/conversation_id":
		if text, ok := value.(string); ok {
			state.Response.ConversationID = text
		}
	case patchPath == "/message":
		if response, ok := ResponseFromValue(value); ok {
			if response.ConversationID != "" {
				state.Response.ConversationID = response.ConversationID
			}
			state.Response.Message = response.Message
		}
		if channel := ChannelFromValue(value); channel != "" {
			state.Channel = channel
		}
	case patchPath == "/message/id":
		if text, ok := value.(string); ok {
			state.Response.Message.ID = text
		}
	case patchPath == "/message/channel":
		if text, ok := value.(string); ok {
			state.Channel = text
		}
	case patchPath == "/message/author/role":
		if text, ok := value.(string); ok {
			state.Response.Message.Author.Role = text
		}
	case patchPath == "/message/recipient":
		if text, ok := value.(string); ok {
			state.Response.Message.Recipient = text
		}
	case patchPath == "/message/content/content_type":
		if text, ok := value.(string); ok {
			state.Response.Message.Content.ContentType = text
		}
	case patchPath == "/message/content/parts":
		if parts, ok := value.([]interface{}); ok {
			state.Response.Message.Content.Parts = parts
		}
	case strings.HasPrefix(patchPath, "/message/content/parts/0"):
		if text, ok := value.(string); ok {
			current, _ := state.Response.Message.Content.Parts[0].(string)
			if operation == "append" {
				text = current + text
			}
			state.Response.Message.Content.Parts[0] = text
		}
	case patchPath == "/message/metadata/message_type":
		if text, ok := value.(string); ok {
			state.Response.Message.Metadata.MessageType = text
		}
	case patchPath == "/message/metadata/model_slug":
		if text, ok := value.(string); ok {
			state.Response.Message.Metadata.ModelSlug = text
		}
	case patchPath == "/message/metadata/finish_details":
		if value == nil {
			state.Response.Message.Metadata.FinishDetails = nil
			break
		}
		data, err := json.Marshal(value)
		if err != nil {
			break
		}
		var finishDetails chatgpt_types.FinishDetails
		if json.Unmarshal(data, &finishDetails) == nil {
			state.Response.Message.Metadata.FinishDetails = &finishDetails
		}
	case patchPath == "/message/end_turn":
		state.Response.Message.EndTurn = value
	default:
		// 新版 SSE (2026-08): /message/metadata/content_references[/N][/field]
		// 携带 cite 标记的引用数据。我们只关心 matched_text(标记) 和 alt(替换链接),
		// 其余字段(safe_urls/type/items 等)忽略。
		if strings.HasPrefix(patchPath, "/message/metadata/content_references") {
			return applyContentReferencePatch(state, patchPath, operation, value)
		}
		return false
	}
	return true
}

// applyContentReferencePatch 处理 content_references 相关 patch,
// 提取 matched_text → alt 映射到 state.CiteAlts。
//
// 支持的路径形式:
//   /message/metadata/content_references            (append 整个引用对象)
//   /message/metadata/content_references/N          (append/replace 引用对象)
//   /message/metadata/content_references/N/alt      (replace alt 字符串)
//   /message/metadata/content_references/N/matched_text (append/replace 标记)
func applyContentReferencePatch(state *PatchState, patchPath string, operation string, value interface{}) bool {
	EnsurePatchDefaults(state)

	switch {
	case patchPath == "/message/metadata/content_references":
		// append 引用对象或对象数组: {"matched_text":"...", "alt":"..."}
		// 对象字段后续还会通过 /N/matched_text append 增量到达,
		// 所以这里同时记录 partial 值作为拼接起点。
		switch v := value.(type) {
		case map[string]interface{}:
			recordRefObject(state, v)
			return true
		case []interface{}:
			for _, item := range v {
				if obj, ok := item.(map[string]interface{}); ok {
					recordRefObject(state, obj)
				}
			}
			return true
		}
	case strings.HasSuffix(patchPath, "/matched_text"):
		idx := contentRefIndex(patchPath)
		if idx < 0 {
			return false
		}
		if text, ok := value.(string); ok {
			key := fmt.Sprintf("ref:%d:matched", idx)
			// matched_text 可能分多帧 append 到达,需要拼接
			if operation == "append" {
				state.CiteAlts[key] += text
			} else {
				state.CiteAlts[key] = text
			}
			// 如果 alt 已到齐,建立最终映射
			if alt := state.CiteAlts[fmt.Sprintf("ref:%d:alt", idx)]; alt != "" {
				state.CiteAlts[state.CiteAlts[key]] = alt
			}
			return true
		}
	case strings.HasSuffix(patchPath, "/alt"):
		idx := contentRefIndex(patchPath)
		if idx < 0 {
			return false
		}
		if text, ok := value.(string); ok && text != "" {
			state.CiteAlts[fmt.Sprintf("ref:%d:alt", idx)] = text
			// 如果 matched 已到齐,建立最终映射
			if matched := state.CiteAlts[fmt.Sprintf("ref:%d:matched", idx)]; matched != "" {
				state.CiteAlts[matched] = text
			}
			return true
		}
	default:
		// 整个引用对象的 append/replace: .../content_references/N 或带其他后缀的对象值
		if obj, ok := value.(map[string]interface{}); ok {
			recordRefObject(state, obj)
			return true
		}
	}
	return false
}

// recordRefObject 记录一个 content_references 引用对象:
// 1. matched_text 作为 ref:N:matched 的拼接起点
// 2. matched 和 alt 都非空时建立最终映射
func recordRefObject(state *PatchState, obj map[string]interface{}) {
	matched, _ := obj["matched_text"].(string)
	alt, _ := obj["alt"].(string)
	if matched != "" {
		idx := state.nextRefIdx
		state.nextRefIdx++
		state.CiteAlts[fmt.Sprintf("ref:%d:matched", idx)] = matched
	}
	if matched != "" && alt != "" {
		state.CiteAlts[matched] = alt
	}
}

// contentRefIndex 从路径中解析 content_references/N 的 N。
func contentRefIndex(path string) int {
	const prefix = "/message/metadata/content_references/"
	rest := strings.TrimPrefix(path, prefix)
	// rest 应该以数字开头
	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	if end == 0 {
		return -1
	}
	n := 0
	for _, c := range rest[:end] {
		n = n*10 + int(c-'0')
	}
	return n
}

// ReplaceCiteMarkers 把正文中的 cite/entity 等私有区标记转换为可读文本。
// 处理规则:
//   - 有 alt 映射的标记(搜索引用) -> 替换为 Markdown 链接;
//   - entity 标记(人物/公司卡片) -> 提取显示名(数组第 2 个元素);
//   - 其余无 alt 的标记(cite 残留/image_group 等) -> 删除;
//   - 未闭合的残缺标记(流式截断产生) -> 删除。
//
// 标记格式: typepayload... (私有区控制符包裹)。
func ReplaceCiteMarkers(text string, citeAlts map[string]string) string {
	if text == "" || !strings.ContainsRune(text, '') {
		return text
	}
	var b strings.Builder
	i := 0
	runes := []rune(text)
	for i < len(runes) {
		if runes[i] == '' {
			// 找结束符 
			j := i + 1
			for j < len(runes) && runes[j] != '' {
				j++
			}
			if j >= len(runes) {
				// 未闭合的残缺标记: 直接丢弃,不透传私有区乱码
				break
			}
			marker := string(runes[i : j+1])
			if alt, ok := citeAlts[marker]; ok && alt != "" {
				b.WriteString(alt)
			} else if fallback := markerFallback(marker); fallback != "" {
				b.WriteString(fallback)
			}
			// 其余无 alt 的标记丢弃
			i = j + 1
			continue
		}
		b.WriteRune(runes[i])
		i++
	}
	return b.String()
}

// markerFallback 为无 alt 的标记提供兜底文本。
// entity 卡片携带 ["类型","显示名","描述"] 数组,显示名对用户可读,应保留;
// 其他类型(cite 引用残留、image_group 图片指令等)内容是内部协议,删除。
func markerFallback(marker string) string {
	runes := []rune(marker)
	if len(runes) < 3 {
		return ""
	}
	inner := string(runes[1 : len(runes)-1]) // 去掉首尾控制符(按 rune,PUA 是多字节)
	if !strings.HasPrefix(inner, "entity") {
		return ""
	}
	payload := inner[len("entity"):]
	arrStart := strings.Index(payload, "[")
	arrEnd := strings.LastIndex(payload, "]")
	if arrStart < 0 || arrEnd <= arrStart {
		return ""
	}
	var arr []interface{}
	if json.Unmarshal([]byte(payload[arrStart:arrEnd+1]), &arr) != nil {
		return ""
	}
	if len(arr) >= 2 {
		if name, ok := arr[1].(string); ok && name != "" {
			return name
		}
	}
	return ""
}
// ── 流式 cite 处理管道 ──

// MaxCiteHoldBytes 暂存区字节上限: 超过后强制放行(未解析标记由 ReplaceCiteMarkers 删除),
// 兜底防止 alt 迟迟不到导致正文被无限期扣住。
const MaxCiteHoldBytes = 4096

// CiteStreamPipeline 流式输出时的 cite 标记 hold-back 缓冲。
//
// 新版 SSE (2026-08) 中 cite 标记会被切到两帧到达、且 alt 链接
// 晚于正文 1~2 帧。逐帧直接替换会导致:
//  1. 半截标记当帧透传(私有区乱码);
//  2. 完整但 alt 未到的标记被误删,链接永久丢失。
//
// 因此未解析完成的尾部必须暂存,等闭合且 alt 到达后再输出。
type CiteStreamPipeline struct {
	hold string
}

// Feed 追加一段原始增量,返回当前可安全输出的替换后文本。
// citeAlts 每次传入最新映射(alt 可能随新 patch 到达)。
func (p *CiteStreamPipeline) Feed(citeAlts map[string]string, delta string) string {
	if delta != "" {
		p.hold += delta
	}
	flush, remain := SplitCiteHold(p.hold, citeAlts)
	p.hold = remain
	if flush == "" {
		return ""
	}
	return ReplaceCiteMarkers(flush, citeAlts)
}

// Flush 流结束时冲刷剩余暂存: 有 alt 替换,无 alt 删除。
func (p *CiteStreamPipeline) Flush(citeAlts map[string]string) string {
	if p.hold == "" {
		return ""
	}
	text := ReplaceCiteMarkers(p.hold, citeAlts)
	p.hold = ""
	return text
}

// SplitCiteHold 把待发文本切成 (可立即处理部分, 继续暂存尾部)。
// 暂存规则:
//  1. 未闭合的 ... 区间: 从标记起点开始暂存;
//  2. 已闭合但 alt 未到的标记: 同样暂存(等待后续 alt patch);
//  3. 暂存量超过 MaxCiteHoldBytes 时整体放行(由 ReplaceCiteMarkers 删除残缺标记)。
func SplitCiteHold(text string, citeAlts map[string]string) (string, string) {
	runes := []rune(text)
	holdStart := -1
	for i := 0; i < len(runes); {
		if runes[i] != '' {
			i++
			continue
		}
		j := i + 1
		for j < len(runes) && runes[j] != '' {
			j++
		}
		if j >= len(runes) || citeAlts[string(runes[i:j+1])] == "" {
			holdStart = i
			break
		}
		i = j + 1
	}
	if holdStart < 0 {
		return text, ""
	}
	headBytes := len(string(runes[:holdStart]))
	if len(text)-headBytes > MaxCiteHoldBytes {
		return text, ""
	}
	return string(runes[:holdStart]), string(runes[holdStart:])
}

// NormalizeContentDelta 规范化 OpenAI content delta。
func NormalizeContentDelta(currentText string, incoming string) string {
	if incoming == "" {
		return ""
	}
	if currentText == "" {
		return incoming
	}
	if strings.HasPrefix(incoming, currentText) {
		return incoming[len(currentText):]
	}
	return incoming
}

// FirstStringPart 获取 parts 数组的第一个字符串元素。
func FirstStringPart(parts []interface{}) string {
	if len(parts) == 0 {
		return ""
	}
	text, _ := parts[0].(string)
	return text
}
