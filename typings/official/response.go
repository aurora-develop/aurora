package official

import "encoding/json"

type ChatCompletionChunk struct {
	ID      string    `json:"id"`
	Object  string    `json:"object"`
	Created int64     `json:"created"`
	Model   string    `json:"model"`
	Choices []Choices `json:"choices"`
}

func (chunk *ChatCompletionChunk) String() string {
	resp, _ := json.Marshal(chunk)
	return string(resp)
}

type Choices struct {
	Delta        Delta       `json:"delta"`
	Index        int         `json:"index"`
	FinishReason interface{} `json:"finish_reason"`
}

type Delta struct {
	Content string `json:"content,omitempty"`
	Role    string `json:"role,omitempty"`
}

func NewChatCompletionChunk(text string) ChatCompletionChunk {
	return ChatCompletionChunk{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   "gpt-3.5-turbo-0301",
		Choices: []Choices{
			{
				Index: 0,
				Delta: Delta{
					Content: text,
				},
				FinishReason: nil,
			},
		},
	}
}

func StopChunk(reason string) ChatCompletionChunk {
	return ChatCompletionChunk{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   "gpt-3.5-turbo-0125",
		Choices: []Choices{
			{
				Index:        0,
				FinishReason: reason,
			},
		},
	}
}

type ChatCompletion struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Usage   usage    `json:"usage"`
	Choices []Choice `json:"choices"`
}
type Msg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Choice struct {
	Index        int         `json:"index"`
	Message      Msg         `json:"message"`
	FinishReason interface{} `json:"finish_reason"`
}
type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func NewChatCompletion(full_test string, input_tokens, output_tokens int) ChatCompletion {
	return ChatCompletion{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion",
		Created: int64(0),
		Model:   "gpt-3.5-turbo-0125",
		Usage: usage{
			PromptTokens:     input_tokens,
			CompletionTokens: output_tokens,
			TotalTokens:      input_tokens + output_tokens,
		},
		Choices: []Choice{
			{
				Message: Msg{
					Content: full_test,
					Role:    "assistant",
				},
				Index: 0,
			},
		},
	}
}

type OpenAIAccessTokenWithSession struct {
	SessionToken string `json:"session_token"`
	AccessToken  string `json:"access_token"`
}

func NewOpenAISessionToken(session_token string, access_token string) *OpenAIAccessTokenWithSession {
	return &OpenAIAccessTokenWithSession{
		SessionToken: session_token,
		AccessToken:  access_token,
	}
}
