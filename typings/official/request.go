package official

import (
	"encoding/json"
	"fmt"
	"strings"
)

type APIRequest struct {
	Messages  []api_message `json:"messages"`
	Stream    bool          `json:"stream"`
	Model     string        `json:"model"`
	PluginIDs []string      `json:"plugin_ids"`
}

type api_message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
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
		apiRequest.Messages = append(apiRequest.Messages, api_message{
			Role:    "system",
			Content: instruction,
		})
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

func responsesInputToMessages(raw json.RawMessage) ([]api_message, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return []api_message{{Role: "user", Content: text}}, nil
	}

	var messages []responseInputMessage
	if err := json.Unmarshal(raw, &messages); err != nil {
		return nil, fmt.Errorf("invalid input")
	}

	result := make([]api_message, 0, len(messages))
	for _, message := range messages {
		role := message.Role
		if role == "" {
			role = "user"
		}
		content := responsesContentToText(message.Content)
		if content == "" {
			continue
		}
		result = append(result, api_message{Role: role, Content: content})
	}
	return result, nil
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
	if model == "" || strings.HasPrefix(model, "gpt-image") || strings.HasPrefix(model, "dall-e") {
		model = "gpt-4-dalle"
	}
	prompt := "Generate an image for this request. Return only the generated image, not a text description.\n\n" + r.Prompt
	return APIRequest{
		Model: model,
		Messages: []api_message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
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
