package chatgpt

import (
	"freechatgpt/typings"
	chatgpt_types "freechatgpt/typings/chatgpt"
	official_types "freechatgpt/typings/official"
	"strings"
)

func ConvertToString(chatgpt_response *chatgpt_types.ChatGPTResponse, previous_text *typings.StringStruct, role bool) string {
	translated_response := official_types.NewChatCompletionChunk(strings.Replace(chatgpt_response.Message.Content.Parts[0].(string), previous_text.Text, "", 1))
	if role {
		translated_response.Choices[0].Delta.Role = chatgpt_response.Message.Author.Role
	} else if translated_response.Choices[0].Delta.Content == "" || (strings.HasPrefix(chatgpt_response.Message.Metadata.ModelSlug, "gpt-4") && translated_response.Choices[0].Delta.Content == "【") {
		return translated_response.Choices[0].Delta.Content
	}
	previous_text.Text = chatgpt_response.Message.Content.Parts[0].(string)
	return "data: " + translated_response.String() + "\n\n"
}
