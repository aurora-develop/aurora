package chatgpt

import (
	"os"
	"time"

	"github.com/google/uuid"
)

type chatgpt_message struct {
	ID         uuid.UUID              `json:"id"`
	Author     chatgpt_author         `json:"author"`
	CreateTime float64                `json:"create_time,omitempty"`
	Content    chatgpt_content        `json:"content"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

type chatgpt_content struct {
	ContentType string        `json:"content_type"`
	Parts       []interface{} `json:"parts"`
}

type chatgpt_author struct {
	Role string `json:"role"`
}

type ChatGPTRequest struct {
	Action                           string                 `json:"action"`
	Messages                         []chatgpt_message      `json:"messages"`
	ParentMessageID                  string                 `json:"parent_message_id"`
	ConversationID                   string                 `json:"conversation_id,omitempty"`
	Model                            string                 `json:"model"`
	ClientPrepareState               string                 `json:"client_prepare_state,omitempty"`
	TimezoneOffsetMin                int                    `json:"timezone_offset_min"`
	Timezone                         string                 `json:"timezone"`
	ConversationMode                 map[string]string      `json:"conversation_mode"`
	EnableMessageFollowups           bool                   `json:"enable_message_followups,omitempty"`
	SystemHints                      []string               `json:"system_hints"`
	SupportsBuffering                bool                   `json:"supports_buffering,omitempty"`
	SupportedEncodings               []string               `json:"supported_encodings,omitempty"`
	ClientContextualInfo             map[string]interface{} `json:"client_contextual_info,omitempty"`
	Suggestions                      []interface{}          `json:"suggestions,omitempty"`
	HistoryAndTrainingDisabled       bool                   `json:"history_and_training_disabled"`
	ParagenCotSummaryDisplayOverride string                 `json:"paragen_cot_summary_display_override"`
	ForceParallelSwitch              string                 `json:"force_parallel_switch"`
	ThinkingEffort                   string                 `json:"thinking_effort"`
	ForceRateLimit                   bool                   `json:"force_rate_limit,omitempty"`
	ResetRateLimits                  bool                   `json:"reset_rate_limits,omitempty"`
	ForceUseSse                      bool                   `json:"force_use_sse,omitempty"`
}

func NewChatGPTRequest() ChatGPTRequest {
	disable_history := os.Getenv("ENABLE_HISTORY") != "true"
	return ChatGPTRequest{
		Action:                     "next",
		ParentMessageID:            "client-created-root",
		Model:                      "auto",
		HistoryAndTrainingDisabled: disable_history,
		TimezoneOffsetMin:          -480,
		Timezone:                   "Asia/Shanghai",
		ConversationMode:           map[string]string{"kind": "primary_assistant"},
		SystemHints:                []string{},
		ParagenCotSummaryDisplayOverride: "allow",
		ForceParallelSwitch:              "auto",
		ThinkingEffort:                   "standard",
	}
}

func (c *ChatGPTRequest) AddMessage(role string, content string) {
	c.Messages = append(c.Messages, chatgpt_message{
		ID:         uuid.New(),
		Author:     chatgpt_author{Role: role},
		CreateTime: messageCreateTime(),
		Content:    chatgpt_content{ContentType: "text", Parts: []interface{}{content}},
		Metadata:   defaultMessageMetadata(),
	})
}

func (c *ChatGPTRequest) AddMultimodalMessage(role string, parts []interface{}, metadata map[string]interface{}) {
	contentType := "text"
	if len(parts) > 1 || (len(parts) == 1 && !isStringPart(parts[0])) {
		contentType = "multimodal_text"
	}
	c.Messages = append(c.Messages, chatgpt_message{
		ID:         uuid.New(),
		Author:     chatgpt_author{Role: role},
		CreateTime: messageCreateTime(),
		Content:    chatgpt_content{ContentType: contentType, Parts: parts},
		Metadata:   mergeMessageMetadata(metadata),
	})
}

func (c *ChatGPTRequest) AddAssistantMessage(input string) {
	var msg = chatgpt_message{
		ID:         uuid.New(),
		Author:     chatgpt_author{Role: "assistant"},
		CreateTime: messageCreateTime(),
		Content:    chatgpt_content{ContentType: "text", Parts: []interface{}{input}},
		Metadata:   defaultMessageMetadata(),
	}
	c.Messages = append(c.Messages, msg)
}

func isStringPart(part interface{}) bool {
	_, ok := part.(string)
	return ok
}

func messageCreateTime() float64 {
	return float64(time.Now().UnixMilli()) / 1000.0
}

func defaultMessageMetadata() map[string]interface{} {
	return map[string]interface{}{
		"developer_mode_connector_ids": []interface{}{},
		"selected_sources":             []interface{}{},
		"selected_github_repos":        []interface{}{},
		"selected_all_github_repos":    false,
		"serialization_metadata":       map[string]interface{}{"custom_symbol_offsets": []interface{}{}},
	}
}

func mergeMessageMetadata(metadata map[string]interface{}) map[string]interface{} {
	merged := defaultMessageMetadata()
	for key, value := range metadata {
		merged[key] = value
	}
	return merged
}
