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

func NewChatCompletionChunk(text string, model string) ChatCompletionChunk {
	if model == "" {
		model = "auto"
	}
	return ChatCompletionChunk{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   model,
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

func StopChunk(reason string, model string) ChatCompletionChunk {
	if model == "" {
		model = "auto"
	}
	return ChatCompletionChunk{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   model,
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

func NewChatCompletion(full_test string, input_tokens, output_tokens int, model string) ChatCompletion {
	if model == "" {
		model = "auto"
	}
	return ChatCompletion{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion",
		Created: int64(0),
		Model:   model,
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

type ResponsesResponse struct {
	ID         string            `json:"id"`
	Object     string            `json:"object"`
	CreatedAt  int64             `json:"created_at"`
	Status     string            `json:"status"`
	Model      string            `json:"model"`
	Output     []ResponsesOutput `json:"output"`
	OutputText string            `json:"output_text"`
	Usage      usage             `json:"usage"`
}

type ResponsesOutput struct {
	ID      string             `json:"id"`
	Type    string             `json:"type"`
	Status  string             `json:"status"`
	Role    string             `json:"role"`
	Content []ResponsesContent `json:"content"`
}

type ResponsesContent struct {
	Type        string        `json:"type"`
	Text        string        `json:"text"`
	Annotations []interface{} `json:"annotations"`
}

type ResponsesTextDeltaEvent struct {
	Type         string `json:"type"`
	Delta        string `json:"delta"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
}

type ResponsesCreatedEvent struct {
	Type     string            `json:"type"`
	Response ResponsesResponse `json:"response"`
}

type ResponsesCompletedEvent struct {
	Type     string            `json:"type"`
	Response ResponsesResponse `json:"response"`
}

func NewResponsesResponse(text string, inputTokens, outputTokens int, model string) ResponsesResponse {
	if model == "" {
		model = "auto"
	}
	return ResponsesResponse{
		ID:         "resp_QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:     "response",
		CreatedAt:  int64(0),
		Status:     "completed",
		Model:      model,
		OutputText: text,
		Usage: usage{
			PromptTokens:     inputTokens,
			CompletionTokens: outputTokens,
			TotalTokens:      inputTokens + outputTokens,
		},
		Output: []ResponsesOutput{
			{
				ID:     "msg_QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
				Type:   "message",
				Status: "completed",
				Role:   "assistant",
				Content: []ResponsesContent{
					{
						Type:        "output_text",
						Text:        text,
						Annotations: []interface{}{},
					},
				},
			},
		},
	}
}

func ResponsesTextDelta(text string) string {
	event := ResponsesTextDeltaEvent{
		Type:         "response.output_text.delta",
		Delta:        text,
		OutputIndex:  0,
		ContentIndex: 0,
	}
	resp, _ := json.Marshal(event)
	return string(resp)
}

func ResponsesCreated(response ResponsesResponse) string {
	response.Status = "in_progress"
	event := ResponsesCreatedEvent{
		Type:     "response.created",
		Response: response,
	}
	resp, _ := json.Marshal(event)
	return string(resp)
}

func ResponsesCompleted(response ResponsesResponse) string {
	event := ResponsesCompletedEvent{
		Type:     "response.completed",
		Response: response,
	}
	resp, _ := json.Marshal(event)
	return string(resp)
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

type ImageGenerationResponse struct {
	Created int64                 `json:"created"`
	Data    []ImageGenerationData `json:"data"`
}

type ImageGenerationData struct {
	URL           string `json:"url,omitempty"`
	B64JSON       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

func NewImageGenerationResponse(data []ImageGenerationData) ImageGenerationResponse {
	return ImageGenerationResponse{
		Created: 0,
		Data:    data,
	}
}
