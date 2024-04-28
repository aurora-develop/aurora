package duckgo

import (
	officialtypes "aurora/typings/official"
	"strings"
)

func ConvertAPIRequest(api_request officialtypes.APIRequest) ApiRequest {
	duckgo_request := NewApiRequest(api_request.Model)
	if strings.HasPrefix(duckgo_request.Model, "gpt-3.5") {
		duckgo_request.Model = GPT3
	}
	if strings.HasPrefix(duckgo_request.Model, "claude") {
		duckgo_request.Model = Claude
	}
	content := ""
	for _, apiMessage := range api_request.Messages {
		if apiMessage.Role == "user" {
			content += "user:" + apiMessage.Content + "\r\n"
		}
		if apiMessage.Role == "system" {
			content += "system:" + apiMessage.Content + "\r\n"
		}
		if apiMessage.Role == "assistant" {
			content += "assistant:" + apiMessage.Content + "\r\n"
		}
	}
	duckgo_request.AddMessage("user", content)
	return duckgo_request
}
