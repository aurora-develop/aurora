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
		return false
	}
	return true
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
