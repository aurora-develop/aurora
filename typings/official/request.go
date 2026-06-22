package official

import (
	"encoding/json"
	"fmt"
	"strings"
)

type APIRequest struct {
	Messages         []APIMessage `json:"messages"`
	Stream           bool         `json:"stream"`
	Model            string       `json:"model"`
	ArtifactDelivery string       `json:"artifact_delivery,omitempty"`
	// 工具调用协议(对齐 OpenAI):
	// - Tools:      客户端声明的可调用工具列表
	// - ToolChoice: 强制 / 允许 / 禁止模型调用工具
	// - ParallelToolCalls: 是否允许同一轮发起多个 tool_call(默认 true)
	Tools              []Tool      `json:"tools,omitempty"`
	ToolChoice         *ToolChoice `json:"tool_choice,omitempty"`
	ParallelToolCalls  *bool       `json:"parallel_tool_calls,omitempty"`

	// ── 标准生成参数 ──
	Temperature         *float64   `json:"temperature,omitempty"`
	TopP                *float64   `json:"top_p,omitempty"`
	N                   *int       `json:"n,omitempty"`
	Stop                *StopParam `json:"stop,omitempty"`
	MaxTokens           *int       `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int       `json:"max_completion_tokens,omitempty"`
	PresencePenalty     *float64   `json:"presence_penalty,omitempty"`
	FrequencyPenalty    *float64   `json:"frequency_penalty,omitempty"`
	LogitBias           map[int]int `json:"logit_bias,omitempty"`
	Seed                *int       `json:"seed,omitempty"`

	// ── 扩展参数 ──
	ResponseFormat  *ResponseFormat  `json:"response_format,omitempty"`
	ReasoningEffort string           `json:"reasoning_effort,omitempty"`
	StreamOptions   *StreamOptions   `json:"stream_options,omitempty"`
	User            string           `json:"user,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	Store           *bool            `json:"store,omitempty"`
}

// StopParam 接受 string 或 []string。
// OpenAI 协议: stop 可以是单个字符串或最多 4 个字符串的数组。
type StopParam struct {
	Values []string
}

// UnmarshalJSON 同时接受 string 和 []string 两种形态。
func (s *StopParam) UnmarshalJSON(data []byte) error {
	var single string
	if err := json.Unmarshal(data, &single); err == nil {
		s.Values = []string{single}
		return nil
	}
	var multi []string
	if err := json.Unmarshal(data, &multi); err == nil {
		s.Values = multi
		return nil
	}
	return fmt.Errorf("stop must be a string or array of strings")
}

// ResponseFormat 对齐 OpenAI response_format 参数。
type ResponseFormat struct {
	Type       string          `json:"type"` // "text" | "json_object" | "json_schema"
	JSONSchema json.RawMessage `json:"json_schema,omitempty"`
}

// StreamOptions 对齐 OpenAI stream_options 参数。
type StreamOptions struct {
	IncludeUsage bool `json:"include_usage,omitempty"`
}

// Tool 对齐 OpenAI 的 tools[*] 项。
// Type 固定为 "function";Function 描述具体函数。
type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

// ToolFunction 描述一个函数工具的 name / description / JSON schema 参数。
type ToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	// Strict / Cache 暂不实现 —— OpenAI 协议可选字段,ChatGPT Web 不消费
}

// ToolChoice 取值:
//   - nil       : 模型自行决定
//   - "auto"    : 模型自行决定(显式)
//   - "none"    : 禁止调用工具
//   - "any"     : 强制至少调用一个
//   - &ToolChoice{Type: "function", Function: {Name: "X"}} : 强制调用 X
type ToolChoice struct {
	Type     string             `json:"type"`
	Function *ToolChoiceFunction `json:"function,omitempty"`
}

type ToolChoiceFunction struct {
	Name string `json:"name"`
}

// UnmarshalJSON 同时接受字符串("auto"/"none"/"any")和对象两种形态,
// 兼容 OpenAI 协议里 tool_choice 字段的字符串简写。
func (t *ToolChoice) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		t.Type = s
		t.Function = nil
		return nil
	}
	type alias ToolChoice
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*t = ToolChoice(a)
	return nil
}

// ForcedFunctionName 当 tool_choice 强制某个具名工具时返回它的名字;
// 否则返回 ""(auto / none / any / nil)。
func (t *ToolChoice) ForcedFunctionName() string {
	if t == nil {
		return ""
	}
	if t.Type == "function" && t.Function != nil {
		return t.Function.Name
	}
	return ""
}

// IsForcedNone 报告 tool_choice 是否显式禁止调用工具。
func (t *ToolChoice) IsForcedNone() bool {
	return t != nil && t.Type == "none"
}

// ToolCallRef 出现在 assistant 历史消息的 tool_calls 字段里,
// 用于多轮对话时回放工具调用上下文。
type ToolCallRef struct {
	Index    int    `json:"index"`
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type APIMessage struct {
	Role        string           `json:"role"`
	Content     MessageContent   `json:"content"`
	Attachments []FileAttachment `json:"attachments,omitempty"`
	// ToolCalls 在 role=assistant 的消息中携带该轮发起的工具调用列表。
	// 用于多轮对话时把模型"之前"发起的调用回放进 prompt。
	ToolCalls []ToolCallRef `json:"tool_calls,omitempty"`
	// ToolCallID 在 role=tool 的消息中携带对应的 tool_call.id,
	// 配合 Content 一起作为工具执行结果回传。
	ToolCallID string `json:"tool_call_id,omitempty"`
	// Name 在 role=tool 时携带工具名(some clients use this).
	Name string `json:"name,omitempty"`
}

// HasToolCalls 报告该消息是否带有 tool_calls(仅 assistant 消息会用到)。
func (m APIMessage) HasToolCalls() bool {
	return len(m.ToolCalls) > 0
}

// IsToolResult 报告该消息是否是 tool 执行结果(role=tool / function)。
func (m APIMessage) IsToolResult() bool {
	return m.Role == "tool" || m.Role == "function"
}

type MessageContent struct {
	TextValue string
	Parts     []MessageContentPart
}

type MessageContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *ImageURLDetail `json:"image_url,omitempty"`
	FileID   string          `json:"file_id,omitempty"`
	FileName string          `json:"filename,omitempty"`
	Name     string          `json:"name,omitempty"`
	MimeType string          `json:"mime_type,omitempty"`
	MIMEType string          `json:"mimeType,omitempty"`
	Size     int64           `json:"size,omitempty"`
	Width    int             `json:"width,omitempty"`
	Height   int             `json:"height,omitempty"`
	File     *FileAttachment `json:"file,omitempty"`
}

type ImageURLDetail struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

type FileAttachment struct {
	ID            string `json:"id,omitempty"`
	FileID        string `json:"file_id,omitempty"`
	Name          string `json:"name,omitempty"`
	FileName      string `json:"file_name,omitempty"`
	Filename      string `json:"filename,omitempty"`
	MimeType      string `json:"mime_type,omitempty"`
	MIMEType      string `json:"mimeType,omitempty"`
	Size          int64  `json:"size,omitempty"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
	LibraryFileID string `json:"library_file_id,omitempty"`
	Source        string `json:"source,omitempty"`
}

func NewTextMessage(role, content string) APIMessage {
	return APIMessage{Role: role, Content: MessageContent{TextValue: content}}
}

func (c *MessageContent) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		c.TextValue = text
		c.Parts = nil
		return nil
	}

	var parts []MessageContentPart
	if err := json.Unmarshal(data, &parts); err == nil {
		c.TextValue = ""
		c.Parts = parts
		return nil
	}

	var part MessageContentPart
	if err := json.Unmarshal(data, &part); err == nil {
		c.TextValue = ""
		c.Parts = []MessageContentPart{part}
		return nil
	}
	return fmt.Errorf("invalid message content")
}

func (c MessageContent) MarshalJSON() ([]byte, error) {
	if len(c.Parts) > 0 {
		return json.Marshal(c.Parts)
	}
	return json.Marshal(c.TextValue)
}

func (c MessageContent) Text() string {
	if len(c.Parts) == 0 {
		return c.TextValue
	}
	var texts []string
	for _, part := range c.Parts {
		switch part.Type {
		case "text", "input_text", "output_text", "":
			if part.Text != "" {
				texts = append(texts, part.Text)
			}
		}
	}
	return strings.Join(texts, "")
}

func (c MessageContent) Files() []FileAttachment {
	var files []FileAttachment
	for _, part := range c.Parts {
		partType := strings.TrimSpace(part.Type)
		if partType == "image_url" && part.ImageURL != nil && part.ImageURL.URL != "" {
			files = append(files, FileAttachment{
				ID:       part.ImageURL.URL,
				FileID:   part.ImageURL.URL,
				Name:     guessImageFilename(part.ImageURL.URL),
				Filename: guessImageFilename(part.ImageURL.URL),
				MimeType: guessImageMime(part.ImageURL.URL),
				MIMEType: guessImageMime(part.ImageURL.URL),
				Source:   part.ImageURL.URL,
			})
			continue
		}
		if partType != "file" && partType != "input_file" && partType != "image" && partType != "input_image" {
			continue
		}
		if part.File != nil {
			files = append(files, *part.File)
			continue
		}
		fileID := strings.TrimSpace(part.FileID)
		if fileID == "" {
			continue
		}
		files = append(files, FileAttachment{
			ID:       fileID,
			FileID:   fileID,
			Name:     firstNonEmpty(part.Name, part.FileName),
			Filename: firstNonEmpty(part.FileName, part.Name),
			MimeType: firstNonEmpty(part.MimeType, part.MIMEType),
			MIMEType: firstNonEmpty(part.MIMEType, part.MimeType),
			Size:     part.Size,
			Width:    part.Width,
			Height:   part.Height,
		})
	}
	return files
}

func guessImageFilename(url string) string {
	if strings.HasPrefix(url, "data:") {
		return "image.png"
	}
	idx := strings.LastIndex(url, "/")
	if idx >= 0 && idx < len(url)-1 {
		name := url[idx+1:]
		if q := strings.Index(name, "?"); q >= 0 {
			name = name[:q]
		}
		if name != "" {
			return name
		}
	}
	return "image.png"
}

func guessImageMime(url string) string {
	if strings.HasPrefix(url, "data:") {
		end := strings.Index(url, ";")
		if end > 5 {
			return url[5:end]
		}
	}
	lower := strings.ToLower(url)
	switch {
	case strings.Contains(lower, ".png"):
		return "image/png"
	case strings.Contains(lower, ".jpg") || strings.Contains(lower, ".jpeg"):
		return "image/jpeg"
	case strings.Contains(lower, ".webp"):
		return "image/webp"
	case strings.Contains(lower, ".gif"):
		return "image/gif"
	default:
		return "image/png"
	}
}

func (m APIMessage) Text() string {
	return m.Content.Text()
}

func (m APIMessage) Files() []FileAttachment {
	files := m.Content.Files()
	files = append(files, m.Attachments...)
	return files
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type ResponsesAPIRequest struct {
	Model        string          `json:"model"`
	Input        json.RawMessage `json:"input"`
	Instructions json.RawMessage `json:"instructions"`
	Stream       bool            `json:"stream"`

	// 标准生成参数
	Temperature     *float64 `json:"temperature,omitempty"`
	TopP            *float64 `json:"top_p,omitempty"`
	MaxOutputTokens *int     `json:"max_output_tokens,omitempty"`

	// 扩展参数
	Text      *ResponseFormatText `json:"text,omitempty"`
	Reasoning *ReasoningConfig    `json:"reasoning,omitempty"`
	Store     *bool               `json:"store,omitempty"`
	User      string              `json:"user,omitempty"`
	Metadata  map[string]string   `json:"metadata,omitempty"`
}

// ResponseFormatText 对应 Responses API 的 text.query.format。
type ResponseFormatText struct {
	Format *ResponseFormat `json:"format,omitempty"`
}

// ReasoningConfig 对应 Responses API 的 reasoning 参数。
type ReasoningConfig struct {
	Effort string `json:"effort,omitempty"` // "low" | "medium" | "high"
}

type responseInputMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type responseInputContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func (r ResponsesAPIRequest) ToAPIRequest() (APIRequest, error) {
	apiRequest := APIRequest{
		Model:  r.Model,
		Stream: r.Stream,
	}

	// 透传 Responses 参数到 APIRequest
	apiRequest.Temperature = r.Temperature
	apiRequest.TopP = r.TopP
	apiRequest.MaxTokens = r.MaxOutputTokens
	apiRequest.MaxCompletionTokens = r.MaxOutputTokens
	apiRequest.Store = r.Store
	apiRequest.User = r.User
	apiRequest.Metadata = r.Metadata

	// reasoning.effort → reasoning_effort
	if r.Reasoning != nil {
		apiRequest.ReasoningEffort = r.Reasoning.Effort
	}

	// text.query.format → response_format
	if r.Text != nil && r.Text.Format != nil {
		apiRequest.ResponseFormat = r.Text.Format
	}

	if instruction := rawText(r.Instructions); instruction != "" {
		apiRequest.Messages = append(apiRequest.Messages, NewTextMessage("system", instruction))
	}

	inputMessages, err := responsesInputToMessages(r.Input)
	if err != nil {
		return apiRequest, err
	}
	apiRequest.Messages = append(apiRequest.Messages, inputMessages...)

	if len(apiRequest.Messages) == 0 {
		return apiRequest, fmt.Errorf("missing required parameter: input")
	}
	return apiRequest, nil
}

func rawText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	return strings.TrimSpace(string(raw))
}

func responsesInputToMessages(raw json.RawMessage) ([]APIMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []APIMessage{NewTextMessage("user", text)}, nil
	}

	var content MessageContent
	if err := json.Unmarshal(raw, &content); err == nil && (content.Text() != "" || len(content.Files()) > 0) {
		return []APIMessage{{Role: "user", Content: content}}, nil
	}

	var messages []responseInputMessage
	if err := json.Unmarshal(raw, &messages); err != nil {
		return nil, fmt.Errorf("invalid input")
	}

	result := make([]APIMessage, 0, len(messages))
	for _, message := range messages {
		role := message.Role
		if role == "" {
			role = "user"
		}
		content, err := responseContentToMessageContent(message.Content)
		if err != nil {
			content = MessageContent{TextValue: responsesContentToText(message.Content)}
		}
		if content.Text() == "" && len(content.Files()) == 0 {
			continue
		}
		result = append(result, APIMessage{Role: role, Content: content})
	}
	return result, nil
}

func responseContentToMessageContent(raw json.RawMessage) (MessageContent, error) {
	var content MessageContent
	if err := json.Unmarshal(raw, &content); err == nil {
		return content, nil
	}
	var parts []responseInputContent
	if err := json.Unmarshal(raw, &parts); err != nil {
		return content, err
	}
	messageParts := make([]MessageContentPart, 0, len(parts))
	for _, part := range parts {
		messageParts = append(messageParts, MessageContentPart{
			Type: part.Type,
			Text: part.Text,
		})
	}
	return MessageContent{Parts: messageParts}, nil
}

func responsesContentToText(raw json.RawMessage) string {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}

	var parts []responseInputContent
	if err := json.Unmarshal(raw, &parts); err == nil {
		texts := make([]string, 0, len(parts))
		for _, part := range parts {
			if part.Text != "" {
				texts = append(texts, part.Text)
			}
		}
		return strings.Join(texts, "\n")
	}
	return strings.TrimSpace(string(raw))
}

type OpenAISessionToken struct {
	SessionToken string `json:"session_token"`
}

type OpenAIRefreshToken struct {
	RefreshToken string `json:"refresh_token"`
}

type TTSAPIRequest struct {
	Input  string `json:"input"`
	Voice  string `json:"voice"`
	Format string `json:"response_format"`
}

type ImageGenerationRequest struct {
	Prompt         string `json:"prompt"`
	Model          string `json:"model"`
	N              int    `json:"n"`
	Size           string `json:"size"`
	Quality        string `json:"quality"`
	ResponseFormat string `json:"response_format"`
	// Stream 非 OpenAI 官方字段,但很多客户端会用它来请求增量输出。
	// 启用时,服务端以 SSE(image.generation.chunk)形式分片返回每张图片。
	Stream bool `json:"stream,omitempty"`
}

func (r ImageGenerationRequest) ToAPIRequest() APIRequest {
	model := r.Model
	if model == "" || strings.HasPrefix(model, "dall-e") {
		model = "gpt-image-2"
	}
	prompt := "Generate an image for this request. Return only the generated image, not a text description.\n\n" + r.Prompt
	return APIRequest{
		Model:    model,
		Messages: []APIMessage{NewTextMessage("user", prompt)},
	}
}

type ImageEditRequest struct {
	Prompt         string            `json:"prompt"`
	Model          string            `json:"model"`
	N              int               `json:"n"`
	Size           string            `json:"size"`
	ResponseFormat string            `json:"response_format"`
	Images         []ImageEditSource `json:"images,omitempty"`
	// Stream 非 OpenAI 官方字段,启用时按 SSE 流式返回。
	Stream bool `json:"stream,omitempty"`
}

type ImageEditSource struct {
	ImageURL string `json:"image_url"`
}

// ImageVariationRequest 对齐 OpenAI 官方 /v1/images/variations 请求:
// 仅基于用户提供的源图生成相似变体,prompt 可选;
// Images 字段沿用 ImageEditSource,语义上 image_url 指向源图(data: 或 http(s))。
type ImageVariationRequest struct {
	Prompt         string            `json:"prompt,omitempty"`
	Model          string            `json:"model"`
	N              int               `json:"n"`
	Size           string            `json:"size"`
	ResponseFormat string            `json:"response_format"`
	Images         []ImageEditSource `json:"images,omitempty"`
	// Stream 非 OpenAI 官方字段,启用时按 SSE 流式返回。
	Stream bool `json:"stream,omitempty"`
}

// TranscriptionsRequest 对齐 OpenAI 官方 /v1/audio/transcriptions 请求。
// multipart/form-data 形态。
type TranscriptionsRequest struct {
	File           string  `json:"file"`                      // 文件路径(框架处理)
	Model          string  `json:"model"`                     // 模型名(whisper-1 / gpt-4o-transcribe)
	Language       string  `json:"language,omitempty"`        // 语言 ISO 代码
	Prompt         string  `json:"prompt,omitempty"`          // 提示文本
	ResponseFormat string  `json:"response_format,omitempty"` // json / text / verbose_json / srt / vtt
	Temperature    float64 `json:"temperature,omitempty"`     // 采样温度
}

// TranslationsRequest 对齐 OpenAI 官方 /v1/audio/translations 请求。
// 与 TranscriptionsRequest 同结构,但语义上翻译为英文。
type TranslationsRequest struct {
	File           string  `json:"file"`
	Model          string  `json:"model"`
	Prompt         string  `json:"prompt,omitempty"`
	ResponseFormat string  `json:"response_format,omitempty"` // json / text / verbose_json / srt / vtt
	Temperature    float64 `json:"temperature,omitempty"`
}
