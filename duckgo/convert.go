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
	content := buildContent(&api_request)
	duckgo_request.AddMessage("user", content)
	return duckgo_request
}

func buildContent(api_request *officialtypes.APIRequest) string {
	var content strings.Builder
	for _, apiMessage := range api_request.Messages {
		role := apiMessage.Role
		if role == "user" || role == "system" || role == "assistant" {
			content.WriteString(role + ":" + apiMessage.Content + "\r\n")
		}
	}
	return content.String()
}
