package handler

import (
	"fmt"
	"io"
	"strings"

	"aurora/httpclient/bogdanfinn"
	"aurora/internal/accounts"
	"aurora/internal/chatgpt"
	"aurora/internal/config"
	officialtypes "aurora/typings/official"
	chatgptrequestconverter "aurora/conversion/requests/chatgpt"

	"github.com/gin-gonic/gin"
)

type AudioHandler struct {
	accountPool *accounts.Pool
	cfg         *config.Config
}

func NewAudioHandler(pool *accounts.Pool, cfg *config.Config) *AudioHandler {
	return &AudioHandler{accountPool: pool, cfg: cfg}
}

// ── TTS ──

var ttsFmtMap = map[string]string{
	"mp3":  "mp3",
	"opus": "opus",
	"aac":  "aac",
	"flac": "aac",
	"wav":  "aac",
	"pcm":  "aac",
}

var ttsTypeMap = map[string]string{
	"mp3":  "audio/mpeg",
	"opus": "audio/ogg",
	"aac":  "audio/aac",
}

var ttsVoiceMap = map[string]string{
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

func (h *AudioHandler) TTS(c *gin.Context) {
	var original_request officialtypes.TTSAPIRequest
	err := c.BindJSON(&original_request)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    err.Error(),
		}})
		return
	}
	if original_request.Input == "" {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Missing required parameter: input",
			"type":    "invalid_request_error",
			"param":   "input",
			"code":    "missing_required_parameter",
		}})
		return
	}

	account, _, err := resolveAccount(c, h.accountPool, h.cfg, true)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "authorization_error",
			"param":   "Authorization",
			"code":    400,
		}})
		return
	}
	if account == nil || account.Token == "" {
		c.JSON(400, gin.H{"error": "TTS requires a logged-in ChatGPT access token."})
		return
	}
	if !account.Type.Satisfies(accounts.CapTTS) {
		c.JSON(403, gin.H{"error": "TTS requires a logged-in ChatGPT account."})
		return
	}

	proxyUrl := account.Proxy
	client := setupClientWithProxy(proxyUrl)

	// Convert the chat request to a ChatGPT request
	translated_request := chatgptrequestconverter.ConvertTTSAPIRequest(original_request.Input)
	clientState := chatgpt.NewChatClientState()
	clientState.ConversationID = translated_request.ConversationID
	clientState.ParentMessageID = translated_request.ParentMessageID

	response, wsConn, _, status, err := conversationClientOrder(&client, account, translated_request, proxyUrl, false, clientState, h.accountPool)
	if err != nil {
		c.JSON(status, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "request_conversion_error",
			"param":   "model",
			"code":    "request_conversion_error",
		}})
		return
	}
	defer response.Body.Close()
	if chatgpt.Handle_request_error(c, response) {
		if wsConn != nil {
			wsConn.Close()
		}
		return
	}

	msgId, convId := chatgpt.HandlerTTS(response, original_request.Input)
	if msgId == "" || convId == "" {
		c.JSON(500, gin.H{"error": "failed to get TTS message id"})
		return
	}
	defer chatgpt.RemoveConversation(client, account, convId, proxyUrl)

	format := ttsFmtMap[original_request.Format]
	if format == "" {
		format = "aac"
	}
	voice := ttsVoiceMap[original_request.Voice]
	if voice == "" {
		voice = "cove"
	}
	data, status, err := chatgpt.GetTTS(client, account, msgId, convId, voice, format, proxyUrl)
	if err != nil {
		c.JSON(status, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "synthesize_request_error",
			"param":   nil,
			"code":    status,
		}})
		return
	}
	c.Data(200, ttsTypeMap[format], data)
}

// ── Audio Transcriptions ──

var transcriptionsSupportedFormats = map[string]string{
	"json":         "application/json",
	"text":         "text/plain",
	"verbose_json": "application/json",
}

func (h *AudioHandler) Transcriptions(c *gin.Context) {
	h.handleTranscription(c, false)
}

func (h *AudioHandler) Translations(c *gin.Context) {
	h.handleTranscription(c, true)
}

func (h *AudioHandler) handleTranscription(c *gin.Context, isTranslation bool) {
	contentType := strings.Split(c.GetHeader("Content-Type"), ";")[0]
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Request must be multipart/form-data",
			"type":    "invalid_request_error",
			"param":   "Content-Type",
			"code":    "invalid_content_type",
		}})
		return
	}

	if err := c.Request.ParseMultipartForm(50 << 20); err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Failed to parse multipart form: " + err.Error(),
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    "parse_error",
		}})
		return
	}

	model := strings.TrimSpace(c.Request.FormValue("model"))
	language := strings.TrimSpace(c.Request.FormValue("language"))
	prompt := c.Request.FormValue("prompt")
	responseFormat := strings.TrimSpace(c.Request.FormValue("response_format"))

	if model == "" {
		model = "whisper-1"
	}
	if responseFormat == "" {
		responseFormat = "json"
	}

	respContentType, formatOK := transcriptionsSupportedFormats[responseFormat]
	if !formatOK {
		c.JSON(400, gin.H{"error": gin.H{
			"message": fmt.Sprintf("Unsupported response_format: %s. Supported values: json, text, verbose_json", responseFormat),
			"type":    "invalid_request_error",
			"param":   "response_format",
			"code":    "invalid_response_format",
		}})
		return
	}

	if len(prompt) > 1000 {
		prompt = prompt[:1000]
	}

	formFile, fileHeader, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Missing required multipart field: file",
			"type":    "invalid_request_error",
			"param":   "file",
			"code":    "missing_required_parameter",
		}})
		return
	}
	defer formFile.Close()

	audioData, err := io.ReadAll(formFile)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "invalid_request_error",
			"param":   "file",
			"code":    "file_read_error",
		}})
		return
	}
	if len(audioData) == 0 {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Uploaded audio file is empty",
			"type":    "invalid_request_error",
			"param":   "file",
			"code":    "empty_file",
		}})
		return
	}

	account, _, err := resolveAccount(c, h.accountPool, h.cfg, true)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "authorization_error",
			"param":   "Authorization",
			"code":    400,
		}})
		return
	}
	if account == nil || account.Token == "" {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Audio transcription requires a logged-in ChatGPT access token.",
			"type":    "invalid_request_error",
			"param":   "file",
			"code":    "missing_access_token",
		}})
		return
	}
	if !account.Type.Satisfies(accounts.CapTranscribe) {
		c.JSON(403, gin.H{"error": "Audio transcription requires a logged-in ChatGPT account."})
		return
	}

	proxyUrl := account.Proxy
	mimeType := fileHeader.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "audio/mpeg"
	}

	// 使用账号绑定的 Client(有指纹 + 代理);不存在则新建
	client, ok := account.Client.(*bogdanfinn.TlsClient)
	if !ok || client == nil {
		client = bogdanfinn.NewStdClient()
		client.SetCookies("https://chatgpt.com", chatgpt.BasicCookies)
		if proxyUrl != "" {
			client.SetProxy(proxyUrl)
		}
	}

	text, status, err := chatgpt.TranscribeAudio(client, account, proxyUrl, audioData, fileHeader.Filename, mimeType, language)
	if err != nil {
		c.JSON(status, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "transcription_error",
			"param":   nil,
			"code":    "transcription_error",
		}})
		return
	}

	switch responseFormat {
	case "json":
		c.JSON(200, officialtypes.TranscriptionResponse{Text: text})
	case "text":
		c.Data(200, "text/plain; charset=utf-8", []byte(text))
	case "verbose_json":
		c.JSON(200, officialtypes.VerboseTranscriptionResponse{
			Task:     "transcribe",
			Language: language,
			Duration: 0,
			Text:     text,
			Segments: []officialtypes.TranscriptionSegment{},
			Words:    []officialtypes.TranscriptionWord{},
		})
	default:
		c.Data(200, respContentType, []byte(text))
	}
}
