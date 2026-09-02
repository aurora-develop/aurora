package official

import (
	"encoding/json"
	"time"
)

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
	ID               string                `json:"id"`
	Object           string                `json:"object"`
	CreatedAt        int64                 `json:"created_at"`
	Status           string                `json:"status"`
	Model            string                `json:"model"`
	Output           []ResponsesOutputItem `json:"output"`
	OutputText       string                `json:"output_text"`
	Usage            ResponsesUsage        `json:"usage"`
	ReasoningContent string                `json:"reasoning_content,omitempty"`
	// MsSinceStart / MsTTFT 是流式响应的耗时信息（毫秒），嵌入 response.completed 事件。
	MsSinceStart     int64                 `json:"ms_since_start,omitempty"`
	MsTTFT           int64                 `json:"ms_ttft,omitempty"`
}

type ResponsesTextDeltaEvent struct {
	Type         string `json:"type"` // "response.output_text.delta"
	ItemID       string `json:"item_id,omitempty"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
}

func (e ResponsesTextDeltaEvent) String() string {
	b, _ := json.Marshal(e)
	return string(b)
}

type ResponsesCreatedEvent struct {
	Type     string            `json:"type"`
	Response ResponsesResponse `json:"response"`
}

type ResponsesCompletedEvent struct {
	Type     string            `json:"type"`
	Response ResponsesResponse `json:"response"`
}

func NewResponsesResponse(text, reasoning string, inputTokens, outputTokens, reasoningTokens int, cachedTokens, cacheWriteTokens int, model string) ResponsesResponse {
	if model == "" {
		model = "auto"
	}
	resp := ResponsesResponse{
		ID:               "resp_QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		Object:           "response",
		CreatedAt:        time.Now().Unix(),
		Status:           "completed",
		Model:            model,
		OutputText:       text,
		ReasoningContent: reasoning,
		Usage: ResponsesUsage{
			InputTokens: inputTokens,
			InputTokensDetails: ResponsesInputTokensDetails{
				CachedTokens:     cachedTokens,
				CacheWriteTokens: cacheWriteTokens,
			},
			OutputTokens: outputTokens,
			OutputTokensDetails: ResponsesOutputTokensDetails{
				ReasoningTokens: reasoningTokens,
			},
			TotalTokens: inputTokens + outputTokens,
		},
		Output: []ResponsesOutputItem{
			{
				ID:     "msg_QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
				Type:   "message",
				Status: "completed",
				Role:   "assistant",
				Content: []ResponsesContentPart{
					{Type: "output_text", Text: text},
				},
			},
		},
	}
	if reasoning != "" {
		resp.Output = append([]ResponsesOutputItem{
			{
				ID:     "rs_QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
				Type:   "reasoning",
				Status: "completed",
				Content: []ResponsesContentPart{
					{Type: "reasoning_text", Text: reasoning},
				},
			},
		}, resp.Output...)
	}
	return resp
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

// ── OpenAI Responses API 扩展类型（对齐 openai-node responses.ts） ──

// ResponsesUsage 对齐 OpenAI ResponseUsage。
type ResponsesUsage struct {
	InputTokens         int                        `json:"input_tokens"`
	InputTokensDetails  ResponsesInputTokensDetails  `json:"input_tokens_details"`
	OutputTokens        int                        `json:"output_tokens"`
	OutputTokensDetails ResponsesOutputTokensDetails `json:"output_tokens_details"`
	TotalTokens         int                        `json:"total_tokens"`
}

type ResponsesInputTokensDetails struct {
	CachedTokens     int `json:"cached_tokens"`
	CacheWriteTokens int `json:"cache_write_tokens"`
}

type ResponsesOutputTokensDetails struct {
	ReasoningTokens int `json:"reasoning_tokens"`
}

// ResponsesOutputItem 对齐 OpenAI ResponseOutputItem 的最小形态。
type ResponsesOutputItem struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"` // "message" | "reasoning"
	Status  string                 `json:"status,omitempty"`
	Role    string                 `json:"role,omitempty"`
	Content []ResponsesContentPart `json:"content"`
}

type ResponsesContentPart struct {
	Type string `json:"type"` // "output_text" | "reasoning_text"
	Text string `json:"text"`
}

// ResponsesReasoningItem 用于最终 output 数组里的思维链项。
type ResponsesReasoningItem struct {
	ID      string                 `json:"id"`
	Type    string                 `json:"type"` // "reasoning"
	Status  string                 `json:"status"`
	Content []ResponsesContentPart `json:"content"`
}

// ResponsesReasoningDeltaEvent 流式思维链事件。
type ResponsesReasoningDeltaEvent struct {
	Type         string `json:"type"` // "response.reasoning_text.delta"
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Delta        string `json:"delta"`
}

func (e ResponsesReasoningDeltaEvent) String() string {
	b, _ := json.Marshal(e)
	return string(b)
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
