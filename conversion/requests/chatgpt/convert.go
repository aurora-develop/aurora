package chatgpt

import (
	"aurora/internal/tokens"
	chatgpt_types "aurora/typings/chatgpt"
	official_types "aurora/typings/official"
)

func ConvertAPIRequest(api_request official_types.APIRequest, secret *tokens.Secret, proxy string) chatgpt_types.ChatGPTRequest {
	chatgpt_request := chatgpt_types.NewChatGPTRequest()

	// Model is passed directly to upstream; default to "auto" if not provided
	model := api_request.Model
	if model == "" {
		model = "auto"
	}
	chatgpt_request.Model = model

	if api_request.PluginIDs != nil {
		chatgpt_request.PluginIDs = api_request.PluginIDs
		chatgpt_request.Model = "gpt-4-plugins"
	}
	for _, api_message := range api_request.Messages {
		if api_message.Role == "system" {
			api_message.Role = "critic"
		}
		chatgpt_request.AddMessage(api_message.Role, api_message.Content)
	}
	return chatgpt_request
}

func ConvertTTSAPIRequest(input string) chatgpt_types.ChatGPTRequest {
	chatgpt_request := chatgpt_types.NewChatGPTRequest()
	chatgpt_request.HistoryAndTrainingDisabled = false
	chatgpt_request.AddAssistantMessage(input)
	return chatgpt_request
}
