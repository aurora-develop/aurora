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
}

type APIMessage struct {
	Role        string           `json:"role"`
	Content     MessageContent   `json:"content"`
	Attachments []FileAttachment `json:"attachments,omitempty"`
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
}

type ImageEditSource struct {
	ImageURL string `json:"image_url"`
}
