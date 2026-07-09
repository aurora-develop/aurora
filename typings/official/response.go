package official

import "encoding/json"

type ChatCompletionChunk struct {
	ID             string                 `json:"id"`
	Object         string                 `json:"object"`
	Created        int64                  `json:"created"`
	Model          string                 `json:"model"`
	Choices        []Choices              `json:"choices"`
	ConversationID string                 `json:"conversation_id,omitempty"`
	Sentinel       map[string]interface{} `json:"sentinel,omitempty"`
	Usage          *StreamUsage           `json:"usage,omitempty"`
}

// StreamUsage 是流式结束时的 usage 信息(仅当 stream_options.include_usage=true)。
type StreamUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
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
	Content          string          `json:"content,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	Role             string          `json:"role,omitempty"`
	ToolCalls        []ToolCallDelta `json:"tool_calls,omitempty"`
}

// ToolCallDelta 是 OpenAI 协议里 delta.tool_calls 元素的最小形态:
// 流式响应中 name / arguments 按"先 name 后 arguments"分块发出。
type ToolCallDelta struct {
	Index    int             `json:"index"`
	ID       string          `json:"id,omitempty"`
	Type     string          `json:"type,omitempty"`
	Function ToolCallFuncDelta `json:"function"`
}

type ToolCallFuncDelta struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

// ToolCall 是非流式响应 message.tool_calls 元素的完整形态。
type ToolCall struct {
	Index    int          `json:"index"`
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolCallFunc `json:"function"`
}

type ToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
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

// NewReasoningChunk 生成流式 reasoning_content 增量，对齐 OpenAI o1/o3-mini 系列模型。
func NewReasoningChunk(text string, model string) ChatCompletionChunk {
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
					ReasoningContent: text,
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

func StopChunkWithConversation(reason string, model string, conversationID string) ChatCompletionChunk {
	chunk := StopChunk(reason, model)
	chunk.ConversationID = conversationID
	return chunk
}

// NewToolCallChunk 生成流式 tool_call 增量:OpenAI 协议要求按 index 顺序
// 发出多块 —— name 段先到(携带 id/type/name),arguments 段后续追加。
func NewToolCallChunk(model string, deltas ...ToolCallDelta) ChatCompletionChunk {
	if model == "" {
		model = "auto"
	}
	return ChatCompletionChunk{
		ID:      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:  "chat.completion.chunk",
		Created: 0,
		Model:   model,
		Choices: []Choices{{Index: 0, Delta: Delta{ToolCalls: deltas}}},
	}
}

// NewToolCallStopChunk 生成 finish_reason=tool_calls 的尾块。
func NewToolCallStopChunk(model string, conversationID string) ChatCompletionChunk {
	chunk := StopChunk("tool_calls", model)
	if conversationID != "" {
		chunk.ConversationID = conversationID
	}
	return chunk
}

type ChatCompletion struct {
	ID             string                   `json:"id"`
	Object         string                   `json:"object"`
	Created        int64                    `json:"created"`
	Model          string                   `json:"model"`
	Usage          usage                    `json:"usage"`
	Choices        []Choice                 `json:"choices"`
	ConversationID string                   `json:"conversation_id,omitempty"`
	Sentinel       []map[string]interface{} `json:"sentinel,omitempty"`
}
type Msg struct {
	Role             string     `json:"role"`
	Content          string     `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
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
	return NewChatCompletionWithMetadata(full_test, input_tokens, output_tokens, model, "", nil)
}

func NewChatCompletionWithMetadata(full_test string, input_tokens, output_tokens int, model string, conversationID string, sentinel []map[string]interface{}) ChatCompletion {
	return NewChatCompletionWithMetadataAndReasoning(full_test, "", input_tokens, output_tokens, model, conversationID, sentinel)
}

// NewChatCompletionWithMetadataAndReasoning 构造非流式响应,可同时返回 reasoning_content。
func NewChatCompletionWithMetadataAndReasoning(full_test string, reasoningContent string, input_tokens, output_tokens int, model string, conversationID string, sentinel []map[string]interface{}) ChatCompletion {
	return NewChatCompletionWithToolCalls(full_test, reasoningContent, nil, input_tokens, output_tokens, model, conversationID, sentinel)
}

// NewChatCompletionWithToolCalls 构造非流式响应,可同时携带 reasoning_content、文本与 tool_calls。
// 当 toolCalls 非空时,Content 设为 nil(对齐 OpenAI:有 tool_calls 时 content 可为 null);
// finish_reason 自动设为 "tool_calls"。
func NewChatCompletionWithToolCalls(fullText string, reasoningContent string, toolCalls []ToolCall, inputTokens, outputTokens int, model, conversationID string, sentinel []map[string]interface{}) ChatCompletion {
	if model == "" {
		model = "auto"
	}
	finishReason := "stop"
	var contentPtr *string
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
		fullText = ""
	} else {
		contentPtr = &fullText
	}
	return ChatCompletion{
		ID:             "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:         "chat.completion",
		Created:        int64(0),
		Model:          model,
		ConversationID: conversationID,
		Sentinel:       sentinel,
		Usage: usage{
			PromptTokens:     inputTokens,
			CompletionTokens: outputTokens,
			TotalTokens:      inputTokens + outputTokens,
		},
		Choices: []Choice{
			{
				Message: Msg{
					Content:          derefString(contentPtr),
					ReasoningContent: reasoningContent,
					Role:             "assistant",
					ToolCalls:        toolCalls,
				},
				Index:        0,
				FinishReason: finishReason,
			},
		},
	}
}

func derefString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
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
	AccessToken  string `json:"accessToken"`
}

// GetAccessToken returns the access token field.
func (s *OpenAIAccessTokenWithSession) GetAccessToken() string {
	return s.AccessToken
}

// GetSessionToken returns the session token field.
func (s *OpenAIAccessTokenWithSession) GetSessionToken() string {
	return s.SessionToken
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

// ImageEditResponse 与 ImageGenerationResponse 同构,
// 用于 /v1/images/edits 接口,保持 OpenAI 官方响应兼容。
type ImageEditResponse = ImageGenerationResponse

func NewImageEditResponse(data []ImageGenerationData) ImageEditResponse {
	return ImageEditResponse{
		Created: 0,
		Data:    data,
	}
}

// ImageVariationResponse 与 ImageGenerationResponse 同构,
// 用于 /v1/images/variations 接口,保持 OpenAI 官方响应兼容。
type ImageVariationResponse = ImageGenerationResponse

func NewImageVariationResponse(data []ImageGenerationData) ImageVariationResponse {
	return ImageVariationResponse{
		Created: 0,
		Data:    data,
	}
}

// ── Audio Transcriptions / Translations ──

// TranscriptionResponse 对齐 OpenAI 官方 /v1/audio/transcriptions JSON 响应。
type TranscriptionResponse struct {
	Text string `json:"text"`
}

// VerboseTranscriptionResponse 对齐 /v1/audio/transcriptions?response_format=verbose_json。
type VerboseTranscriptionResponse struct {
	Task     string                 `json:"task"`
	Language string                 `json:"language"`
	Duration float64                `json:"duration"`
	Text     string                 `json:"text"`
	Segments []TranscriptionSegment `json:"segments"`
	Words    []TranscriptionWord    `json:"words"`
}

type TranscriptionSegment struct {
	ID               int     `json:"id"`
	Seek             int     `json:"seek"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	Tokens           []int   `json:"tokens"`
	Temperature      float64 `json:"temperature"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
}

type TranscriptionWord struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}
