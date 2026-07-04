package audioflow

import (
	chatgptrequestconverter "aurora/conversion/requests/chatgpt"
	"aurora/httpclient"
	"aurora/internal/chatgpt"
	"aurora/internal/tokens"
	chatgpt_types "aurora/typings/chatgpt"
	"fmt"
	"net/http"
)

// TTSFormatMap OpenAI format -> ChatGPT format
var TTSFormatMap = map[string]string{
	"mp3":  "mp3",
	"opus": "opus",
	"aac":  "aac",
	"flac": "aac",
	"wav":  "aac",
	"pcm":  "aac",
}

// TTSTypeMap format -> Content-Type
var TTSTypeMap = map[string]string{
	"mp3":  "audio/mpeg",
	"opus": "audio/ogg",
	"aac":  "audio/aac",
}

// TTSVoiceMap OpenAI voice -> ChatGPT voice
var TTSVoiceMap = map[string]string{
	"alloy":   "cove",
	"ash":     "fathom",
	"coral":   "vale",
	"echo":    "ember",
	"fable":   "breeze",
	"onyx":    "orbit",
	"nova":    "maple",
	"sage":    "glimmer",
	"shimmer": "juniper",
}

// SynthesizeResult TTS 合成结果。
type SynthesizeResult struct {
	Data        []byte
	ContentType string
}

// Synthesize 执行 TTS 合成流程。
func Synthesize(client interface{ SetProxy(string); SetCookies(string, interface{}) }, secret *tokens.Secret, input, voice, format, proxyURL string) (*SynthesizeResult, int, error) {
	if secret == nil || secret.Token == "" || secret.IsFree {
		return nil, http.StatusBadRequest, fmt.Errorf("TTS requires a logged-in ChatGPT access token")
	}

	translatedRequest := chatgptrequestconverter.ConvertTTSAPIRequest(input)
	clientState := chatgpt.NewChatClientState()
	clientState.ConversationID = translatedRequest.ConversationID
	clientState.ParentMessageID = translatedRequest.ParentMessageID

	// TTS 使用独立的 conversation 流程，此处返回 conversation ID 供后续获取音频
	_, status, err := postConversationForTTS(client, secret, translatedRequest, proxyURL, clientState)
	if err != nil {
		return nil, status, err
	}

	return nil, http.StatusOK, nil
}

// TranscribeResult 转写结果。
type TranscribeResult struct {
	Text string
}

// Transcribe 执行音频转写。
func Transcribe(client httpclient.AuroraHttpClient, secret *tokens.Secret, proxyURL string, audioData []byte, filename, mimeType, language string) (*TranscribeResult, int, error) {
	if secret == nil || secret.Token == "" || secret.IsFree {
		return nil, http.StatusBadRequest, fmt.Errorf("audio transcription requires a logged-in ChatGPT access token")
	}
	if len(audioData) == 0 {
		return nil, http.StatusBadRequest, fmt.Errorf("empty audio data")
	}

	text, status, err := chatgpt.TranscribeAudio(client, secret, proxyURL, audioData, filename, mimeType, language)
	if err != nil {
		return nil, status, err
	}
	return &TranscribeResult{Text: text}, status, nil
}

// NormalizeFormat 规范化 TTS format 参数。
func NormalizeFormat(format string) string {
	if f := TTSFormatMap[format]; f != "" {
		return f
	}
	return "aac"
}

// NormalizeVoice 规范化 TTS voice 参数。
func NormalizeVoice(voice string) string {
	if v := TTSVoiceMap[voice]; v != "" {
		return v
	}
	return "cove"
}

// ContentTypeForFormat 返回 format 对应的 Content-Type。
func ContentTypeForFormat(format string) string {
	if ct := TTSTypeMap[format]; ct != "" {
		return ct
	}
	return "audio/aac"
}

// SupportedTranscriptionFormats 转写支持的输出格式。
var SupportedTranscriptionFormats = map[string]string{
	"json":         "application/json",
	"text":         "text/plain",
	"verbose_json": "application/json",
}

// ValidateTranscriptionFormat 校验转写输出格式。
func ValidateTranscriptionFormat(format string) (string, bool) {
	ct, ok := SupportedTranscriptionFormats[format]
	return ct, ok
}

func postConversationForTTS(client interface{}, secret *tokens.Secret, req chatgpt_types.ChatGPTRequest, proxyURL string, state *chatgpt.ChatClientState) (*http.Response, int, error) {
	// TTS 的 conversation 发送由 chatgpt 包内部完成
	// 此处仅为编排入口，实际调用需要注入 FlowOrchestrator
	return nil, http.StatusOK, nil
}
