package chatgpt

import (
	"strings"

	"aurora/typings"
	chatgpt_types "aurora/typings/chatgpt"
	official_types "aurora/typings/official"
)

func ConvertToString(chatgpt_response *chatgpt_types.ChatGPTResponse, previous_text *typings.StringStruct, role bool, model string) string {
	currentText := firstTextPart(chatgpt_response.Message.Content.Parts)
	// delta 计算: 仅当 previous 是 current 的前缀时才发尾部增量。
	// 旧实现 strings.Replace(current, previous, "", 1) 会删除 previous 在 current 中
	// 的"任意首次出现"—— 当某个非累积帧(工具消息/整消息重写)重置了 previous_text,
	// 后续全文又恰好在中段包含该字符串时,会把前半截正文整体吞掉(内容从中间开始)。
	var deltaText string
	if previous_text.Text == "" {
		deltaText = currentText
	} else if strings.HasPrefix(currentText, previous_text.Text) {
		deltaText = currentText[len(previous_text.Text):]
	} else {
		// 不匹配: 内容被上游重写(非累积帧),整段重发。
		// 注意不能用 strings.Contains 找中段匹配后裁剪 —— 那会把首次出现处之前的
		// 正文整体吞掉(输出从半截开始)。宁可整段重发,不能丢内容。
		deltaText = currentText
	}
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
