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
	for _, apiMessage := range api_request.Messages {
		duckgo_request.AddMessage(apiMessage.Role, apiMessage.Content)
	}
	return duckgo_request
}
