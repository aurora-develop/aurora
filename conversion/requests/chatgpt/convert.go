package chatgpt

import (
	"fmt"
	"strings"

	"aurora/internal/tokens"
	chatgpt_types "aurora/typings/chatgpt"
	official_types "aurora/typings/official"

	arkose "github.com/xqdoo00o/funcaptcha"
)

func ConvertAPIRequest(api_request official_types.APIRequest, secret *tokens.Secret, requireArk bool, proxy string) chatgpt_types.ChatGPTRequest {
	chatgpt_request := chatgpt_types.NewChatGPTRequest()
	var api_version int
	if secret.PUID == "" {
		api_request.Model = "gpt-3.5"
	}
	if strings.HasPrefix(api_request.Model, "gpt-3.5") {
		api_version = 3
		chatgpt_request.Model = "text-davinci-002-render-sha"
	} else if strings.HasPrefix(api_request.Model, "gpt-4") {
		api_version = 4
		chatgpt_request.Model = api_request.Model
		// Cover some models like gpt-4-32k
		if len(api_request.Model) >= 7 && api_request.Model[6] >= 48 && api_request.Model[6] <= 57 {
			chatgpt_request.Model = "gpt-4"
		}
	}
	if requireArk {
		token, err := arkose.GetOpenAIToken(api_version, secret.PUID, "", proxy)
		if err == nil {
			chatgpt_request.ArkoseToken = token
		} else {
			fmt.Println("Error getting Arkose token: ", err)
		}
	}

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

func RenewTokenForRequest(request *chatgpt_types.ChatGPTRequest, puid string, proxy string) {
	var api_version int
	if strings.HasPrefix(request.Model, "gpt-4") {
		api_version = 4
	} else {
		api_version = 3
	}
	token, err := arkose.GetOpenAIToken(api_version, puid, "", proxy)
	if err == nil {
		request.ArkoseToken = token
	} else {
		fmt.Println("Error getting Arkose token: ", err)
	}
}
