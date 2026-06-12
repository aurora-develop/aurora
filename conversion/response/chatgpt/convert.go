package chatgpt

import (
	"strings"

	"aurora/typings"
	chatgpt_types "aurora/typings/chatgpt"
	official_types "aurora/typings/official"
)

func ConvertToString(chatgpt_response *chatgpt_types.ChatGPTResponse, previous_text *typings.StringStruct, role bool, model string) string {
	currentText := firstTextPart(chatgpt_response.Message.Content.Parts)
	deltaText := strings.Replace(currentText, previous_text.Text, "", 1)
	translated_response := official_types.NewChatCompletionChunk(deltaText, model)
	if role {
		translated_response.Choices[0].Delta.Role = chatgpt_response.Message.Author.Role
	} else if translated_response.Choices[0].Delta.Content == "" || translated_response.Choices[0].Delta.Content == "【" {
		return translated_response.Choices[0].Delta.Content
	}
	previous_text.Text = currentText
	return "data: " + translated_response.String() + "\n\n"
}

func firstTextPart(parts []interface{}) string {
	if len(parts) == 0 {
		return ""
	}
	text, _ := parts[0].(string)
	return text
}
