package chatgpt

import (
	"os"

	"github.com/google/uuid"
)

type chatgpt_message struct {
	ID      uuid.UUID       `json:"id"`
	Author  chatgpt_author  `json:"author"`
	Content chatgpt_content `json:"content"`
}

type chatgpt_content struct {
	ContentType string   `json:"content_type"`
	Parts       []string `json:"parts"`
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
	ArkoseToken                string            `json:"arkose_token,omitempty"`
	PluginIDs                  []string          `json:"plugin_ids,omitempty"`
	ForceRateLimit             bool              `json:"force_rate_limit"`
	ResetRateLimits            bool              `json:"reset_rate_limits"`
	ForceUseSse                bool              `json:"force_use_sse"`
}

func NewChatGPTRequest() ChatGPTRequest {
	disable_history := os.Getenv("ENABLE_HISTORY") != "true"
	return ChatGPTRequest{
		Action:                     "next",
		ParentMessageID:            uuid.NewString(),
		Model:                      "text-davinci-002-render-sha",
		HistoryAndTrainingDisabled: disable_history,
		ForceUseSse:                true,
		TimezoneOffsetMin:          -480,
	}
}

func (c *ChatGPTRequest) AddMessage(role string, content string) {
	c.Messages = append(c.Messages, chatgpt_message{
		ID:      uuid.New(),
		Author:  chatgpt_author{Role: role},
		Content: chatgpt_content{ContentType: "text", Parts: []string{content}},
	})
}
