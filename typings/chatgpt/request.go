package chatgpt

import (
	"os"

	"github.com/google/uuid"
)

type chatgpt_message struct {
	ID       uuid.UUID              `json:"id"`
	Author   chatgpt_author         `json:"author"`
	Content  chatgpt_content        `json:"content"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type chatgpt_content struct {
	ContentType string        `json:"content_type"`
	Parts       []interface{} `json:"parts"`
}

type chatgpt_author struct {
	Role string `json:"role"`
}

type ChatGPTRequest struct {
	Action                     string            `json:"action"`
	Messages                   []chatgpt_message `json:"messages"`
	ParentMessageID            string            `json:"parent_message_id"`
	ConversationID             string            `json:"conversation_id,omitempty"`
	Model                      string            `json:"model"`
	TimezoneOffsetMin          int               `json:"timezone_offset_min"`
	Suggestions                []interface{}     `json:"suggestions"`
	HistoryAndTrainingDisabled bool              `json:"history_and_training_disabled"`
	ForceRateLimit             bool              `json:"force_rate_limit"`
	ResetRateLimits            bool              `json:"reset_rate_limits"`
	ForceUseSse                bool              `json:"force_use_sse"`
}

func NewChatGPTRequest() ChatGPTRequest {
	disable_history := os.Getenv("ENABLE_HISTORY") != "true"
	return ChatGPTRequest{
		Action:                     "next",
		ParentMessageID:            uuid.NewString(),
		Model:                      "auto",
		HistoryAndTrainingDisabled: disable_history,
		ForceUseSse:                true,
		TimezoneOffsetMin:          -480,
	}
}

func (c *ChatGPTRequest) AddMessage(role string, content string) {
	c.Messages = append(c.Messages, chatgpt_message{
		ID:      uuid.New(),
		Author:  chatgpt_author{Role: role},
		Content: chatgpt_content{ContentType: "text", Parts: []interface{}{content}},
	})
}

func (c *ChatGPTRequest) AddMultimodalMessage(role string, parts []interface{}, metadata map[string]interface{}) {
	contentType := "text"
	if len(parts) > 1 || (len(parts) == 1 && !isStringPart(parts[0])) {
		contentType = "multimodal_text"
	}
	c.Messages = append(c.Messages, chatgpt_message{
		ID:       uuid.New(),
		Author:   chatgpt_author{Role: role},
		Content:  chatgpt_content{ContentType: contentType, Parts: parts},
		Metadata: metadata,
	})
}

func (c *ChatGPTRequest) AddAssistantMessage(input string) {
	var msg = chatgpt_message{
		ID:      uuid.New(),
		Author:  chatgpt_author{Role: "assistant"},
		Content: chatgpt_content{ContentType: "text", Parts: []interface{}{input}},
	}
	c.Messages = append(c.Messages, msg)
}

func isStringPart(part interface{}) bool {
	_, ok := part.(string)
	return ok
}
