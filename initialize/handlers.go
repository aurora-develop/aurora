package initialize

import (
	chatgptrequestconverter "aurora/conversion/requests/chatgpt"
	"aurora/httpclient/bogdanfinn"
	"aurora/internal/chatgpt"
	"aurora/internal/proxys"
	"aurora/internal/tokens"
	chatgpt_types "aurora/typings/chatgpt"
	officialtypes "aurora/typings/official"
	"aurora/util"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type Handler struct {
	proxy *proxys.IProxy
	token *tokens.AccessToken
}

func writeChatCompletionStreamDone(c *gin.Context, stopSent bool, model string, conversationID string) {
	if !stopSent {
		finalLine := officialtypes.StopChunkWithConversation("stop", model, conversationID)
		c.Writer.WriteString("data: " + finalLine.String() + "\n\n")
		c.Writer.Flush()
	}
	c.Writer.WriteString("data: [DONE]\n\n")
	c.Writer.Flush()
}

func NewHandle(proxy *proxys.IProxy, token *tokens.AccessToken) *Handler {
	return &Handler{proxy: proxy, token: token}
}

// initTurnStileWithRetry 初始化 turnstile，当 paid token 返回 401 时自动禁用并轮询下一个
func (h *Handler) initTurnStileWithRetry(client **bogdanfinn.TlsClient, secret **tokens.Secret, proxyUrl string) (*chatgpt.TurnStile, int, error) {
	return h.initTurnStileWithRetryState(client, secret, proxyUrl, nil)
}

func (h *Handler) initTurnStileWithRetryState(client **bogdanfinn.TlsClient, secret **tokens.Secret, proxyUrl string, state *chatgpt.ChatClientState) (*chatgpt.TurnStile, int, error) {
	for {
		(*client).SetCookies("https://chatgpt.com", chatgpt.BasicCookies)
		turnStile, status, err := chatgpt.InitTurnStileWithState(*client, *secret, proxyUrl, state)
		if err == nil {
			return turnStile, status, nil
		}
		if status == http.StatusUnauthorized && *secret != nil && !(*secret).IsFree && (*secret).Token != "" {
			if !h.token.DisableSecret((*secret).Token) {
				return nil, status, err
			}
			newSecret := h.token.GetPaidSecret()
			if newSecret == nil || newSecret.Token == "" {
				return nil, status, err
			}
			*secret = newSecret
			*client = bogdanfinn.NewStdClient()
			continue
		}
		return nil, status, err
	}
}

func (h *Handler) postConversationGptClientOrder(client **bogdanfinn.TlsClient, secret **tokens.Secret, translatedRequest chatgpt_types.ChatGPTRequest, proxyUrl string, stream bool, state *chatgpt.ChatClientState) (*http.Response, *websocket.Conn, int, error) {
	if state != nil {
		state.ApplyToRequest(&translatedRequest)
	}
	turnTraceID := uuid.NewString()
	secretTokenBefore := ""
	if *secret != nil {
		secretTokenBefore = (*secret).Token
	}
	conduitToken, err := chatgpt.PrepareConversationConduitWithState(*client, translatedRequest, *secret, proxyUrl, turnTraceID, state)
	if err != nil {
		return nil, nil, http.StatusInternalServerError, err
	}

	turnStile, status, err := h.initTurnStileWithRetryState(client, secret, proxyUrl, state)
	if err != nil {
		return nil, nil, status, err
	}
	if *secret != nil && (*secret).Token != secretTokenBefore {
		conduitToken, err = chatgpt.PrepareConversationConduitWithState(*client, translatedRequest, *secret, proxyUrl, turnTraceID, state)
		if err != nil {
			return nil, nil, http.StatusInternalServerError, err
		}
	}

	var wsConn *websocket.Conn
	if stream {
		wsConn, err = chatgpt.DialChatWebsocketWithStateAndProxy(*client, *secret, state, proxyUrl)
		if err != nil {
			return nil, nil, http.StatusInternalServerError, err
		}
	}

	response, err := chatgpt.POSTconversationPreparedWithState(*client, translatedRequest, *secret, turnStile, proxyUrl, conduitToken, turnTraceID, state)
	if err != nil {
		if wsConn != nil {
			wsConn.Close()
		}
		return nil, nil, http.StatusInternalServerError, err
	}
	return response, wsConn, http.StatusOK, nil
}

func (h *Handler) refresh(c *gin.Context) {
	var refreshToken officialtypes.OpenAIRefreshToken
	err := c.BindJSON(&refreshToken)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    err.Error(),
		}})
		return
	}
	proxyUrl := h.proxy.GetProxyIP()
	client := bogdanfinn.NewStdClient()
	client.Client.GetCookieJar()
	openaiRefreshToken, status, err := chatgpt.GETTokenForRefreshToken(client, refreshToken.RefreshToken, proxyUrl)
	if err != nil {
		c.JSON(400, gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    err.Error(),
		})
		return
	}
	c.JSON(status, openaiRefreshToken)
}

func (h *Handler) InitBasicConfigForChatGPT() {
	proxy_url := h.proxy.GetProxyIP()
	client := bogdanfinn.NewStdClient()
	chatgpt.GetDpl(client, proxy_url)
	//cfStr, err := chatgpt.GetCf(proxy_url)
	//if err != nil {
	//	fmt.Println(err.Error())
	//}
	//chatgpt.BasicCookies = append(chatgpt.BasicCookies, &http.Cookie{Name: "cf_clearance", Value: cfStr, Domain: "https://chatgpt.com"})
}

func (h *Handler) session(c *gin.Context) {
	var sessionToken officialtypes.OpenAISessionToken
	err := c.BindJSON(&sessionToken)
	if err != nil {
		c.JSON(400, gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    err.Error(),
		})
		return
	}
	proxy_url := h.proxy.GetProxyIP()
	client := bogdanfinn.NewStdClient()
	openaiSessionToken, status, err := chatgpt.GETTokenForSessionToken(client, sessionToken.SessionToken, proxy_url)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    err.Error(),
		}})
		return
	}
	c.JSON(status, openaiSessionToken)
}

func optionsHandler(c *gin.Context) {
	// Set headers for CORS
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "POST")
	c.Header("Access-Control-Allow-Headers", "*")
	c.JSON(200, gin.H{
		"message": "pong",
	})
}

func (h *Handler) refresh_handler(c *gin.Context) {
	var refresh_token officialtypes.OpenAIRefreshToken
	err := c.BindJSON(&refresh_token)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    err.Error(),
		}})
		return
	}

	proxy_url := h.proxy.GetProxyIP()
	client := bogdanfinn.NewStdClient()
	openaiRefreshToken, status, err := chatgpt.GETTokenForRefreshToken(client, refresh_token.RefreshToken, proxy_url)
	if err != nil {
		c.JSON(400, gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    err.Error(),
		})
		return
	}
	c.JSON(status, openaiRefreshToken)
}

func (h *Handler) session_handler(c *gin.Context) {
	var session_token officialtypes.OpenAISessionToken
	err := c.BindJSON(&session_token)
	if err != nil {
		c.JSON(400, gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    err.Error(),
		})
		return
	}
	proxy_url := h.proxy.GetProxyIP()
	client := bogdanfinn.NewStdClient()
	openaiSessionToken, status, err := chatgpt.GETTokenForSessionToken(client, session_token.SessionToken, proxy_url)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    err.Error(),
		}})
		return
	}
	c.JSON(status, openaiSessionToken)
}

func (h *Handler) nightmare(c *gin.Context) {
	var original_request officialtypes.APIRequest
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
	if len(original_request.Messages) == 0 {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Missing required parameter: messages",
			"type":    "invalid_request_error",
			"param":   "messages",
			"code":    "missing_required_parameter",
		}})
		return
	}
	proxyUrl := h.proxy.GetProxyIP()
	input_tokens := countMessagesTokens(original_request.Messages)
	secret, status, err := h.secretFromAuthorization(c, original_requestHasFiles(original_request), false, proxyUrl)
	if err != nil {
		c.JSON(status, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "authorization_error",
			"param":   "Authorization",
			"code":    status,
		}})
		return
	}
	if secret == nil {
		c.JSON(400, gin.H{"error": "Not Account Found."})
		c.Abort()
		return
	}

	uid := uuid.NewString()
	client := bogdanfinn.NewStdClient()

	// Convert the chat request to a ChatGPT request
	translated_request := chatgptrequestconverter.ConvertAPIRequest(original_request, secret, proxyUrl, client)
	clientState := chatgpt.NewChatClientState()
	clientState.ConversationID = translated_request.ConversationID
	clientState.ParentMessageID = translated_request.ParentMessageID

	// Use the model from the original request, default to "auto"
	reqModel := original_request.Model
	if reqModel == "" {
		reqModel = "auto"
	}

	response, wsConn, status, err := h.postConversationGptClientOrder(&client, &secret, translated_request, proxyUrl, original_request.Stream, clientState)
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
			wsConn = nil
		}
		return
	}
	var full_response string
	var conversationID string
	var sentinel []map[string]interface{}
	var stopSent bool

	if os.Getenv("STREAM_MODE") == "false" {
		original_request.Stream = false
	}
	if original_request.Stream {
		c.Writer.Header().Set("Content-Type", "text/event-stream")
		c.Writer.Header().Set("Cache-Control", "no-cache")
		c.Writer.Header().Set("Connection", "keep-alive")
		c.Writer.Header().Set("X-Accel-Buffering", "no")
	}
	for i := 3; i > 0; i-- {
		var continue_info *chatgpt.ContinueInfo
		result := chatgpt.HandlerDetailedWithOptions(c, response, client, secret, uid, translated_request, original_request.Stream, reqModel, chatgpt.HandlerDetailedOptions{
			Websocket:        wsConn,
			ClientState:      clientState,
			ArtifactDelivery: original_request.ArtifactDelivery,
			ProxyURL:         proxyUrl,
		})
		wsConn = nil
		continue_info = result.Continue
		full_response += result.Text
		if result.ConversationID != "" {
			conversationID = result.ConversationID
		}
		sentinel = append(sentinel, result.Sentinel...)
		if result.StopSent {
			stopSent = true
		}
		parentMessageID := result.ParentMessageID
		if continue_info != nil {
			parentMessageID = continue_info.ParentID
		}
		clientState.NoteTurnResult(result.ConversationID, parentMessageID)
		if continue_info == nil {
			break
		}
		translated_request.Messages = nil
		translated_request.Action = "continue"
		translated_request.ConversationID = continue_info.ConversationID
		translated_request.ParentMessageID = continue_info.ParentID

		response, wsConn, status, err = h.postConversationGptClientOrder(&client, &secret, translated_request, proxyUrl, original_request.Stream, clientState)
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
				wsConn = nil
			}
			return
		}
	}
	if c.Writer.Status() != 200 {
		return
	}
	if !original_request.Stream {
		output_tokens := util.CountToken(full_response)
		c.JSON(200, officialtypes.NewChatCompletionWithMetadata(full_response, input_tokens, output_tokens, reqModel, conversationID, sentinel))
	} else {
		writeChatCompletionStreamDone(c, stopSent, reqModel, conversationID)
	}
}

func (h *Handler) responses(c *gin.Context) {
	var responsesRequest officialtypes.ResponsesAPIRequest
	err := c.BindJSON(&responsesRequest)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    err.Error(),
		}})
		return
	}

	original_request, err := responsesRequest.ToAPIRequest()
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "invalid_request_error",
			"param":   "input",
			"code":    "invalid_request_error",
		}})
		return
	}

	proxyUrl := h.proxy.GetProxyIP()
	input_tokens := 0
	for _, message := range original_request.Messages {
		input_tokens += util.CountToken(message.Text())
	}
	secret, status, err := h.secretFromAuthorization(c, original_requestHasFiles(original_request), false, proxyUrl)
	if err != nil {
		c.JSON(status, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "authorization_error",
			"param":   "Authorization",
			"code":    status,
		}})
		return
	}
	if secret == nil {
		c.JSON(400, gin.H{"error": "Not Account Found."})
		c.Abort()
		return
	}

	uid := uuid.NewString()
	client := bogdanfinn.NewStdClient()

	translated_request := chatgptrequestconverter.ConvertAPIRequest(original_request, secret, proxyUrl, client)
	clientState := chatgpt.NewChatClientState()
	clientState.ConversationID = translated_request.ConversationID
	clientState.ParentMessageID = translated_request.ParentMessageID
	reqModel := original_request.Model
	if reqModel == "" {
		reqModel = "auto"
	}

	response, wsConn, status, err := h.postConversationGptClientOrder(&client, &secret, translated_request, proxyUrl, false, clientState)
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
			wsConn = nil
		}
		return
	}

	var full_response string
	for i := 3; i > 0; i-- {
		var continue_info *chatgpt.ContinueInfo
		var response_part string
		result := chatgpt.HandlerDetailedWithOptions(c, response, client, secret, uid, translated_request, false, reqModel, chatgpt.HandlerDetailedOptions{
			Websocket:   wsConn,
			ClientState: clientState,
		})
		wsConn = nil
		response_part, continue_info = result.Text, result.Continue
		full_response += response_part
		parentMessageID := result.ParentMessageID
		if continue_info != nil {
			parentMessageID = continue_info.ParentID
		}
		clientState.NoteTurnResult(result.ConversationID, parentMessageID)
		if continue_info == nil {
			break
		}
		translated_request.Messages = nil
		translated_request.Action = "continue"
		translated_request.ConversationID = continue_info.ConversationID
		translated_request.ParentMessageID = continue_info.ParentID

		response, wsConn, status, err = h.postConversationGptClientOrder(&client, &secret, translated_request, proxyUrl, false, clientState)
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
				wsConn = nil
			}
			return
		}
	}
	if c.Writer.Status() != 200 {
		return
	}

	output_tokens := util.CountToken(full_response)
	responsesResponse := officialtypes.NewResponsesResponse(full_response, input_tokens, output_tokens, reqModel)
	if !responsesRequest.Stream || os.Getenv("STREAM_MODE") == "false" {
		c.JSON(200, responsesResponse)
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.String(200, "event: response.created\ndata: "+officialtypes.ResponsesCreated(responsesResponse)+"\n\n")
	c.String(200, "event: response.output_text.delta\ndata: "+officialtypes.ResponsesTextDelta(full_response)+"\n\n")
	c.String(200, "event: response.completed\ndata: "+officialtypes.ResponsesCompleted(responsesResponse)+"\n\n")
	c.String(200, "data: [DONE]\n\n")
}

func (h *Handler) imageGenerations(c *gin.Context) {
	var imageRequest officialtypes.ImageGenerationRequest
	err := c.BindJSON(&imageRequest)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Request must be proper JSON",
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    err.Error(),
		}})
		return
	}
	if imageRequest.Prompt == "" {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Missing required parameter: prompt",
			"type":    "invalid_request_error",
			"param":   "prompt",
			"code":    "missing_required_parameter",
		}})
		return
	}
	if imageRequest.N <= 0 {
		imageRequest.N = 1
	}
	if imageRequest.N > 10 {
		imageRequest.N = 10
	}
	if imageRequest.ResponseFormat == "" {
		imageRequest.ResponseFormat = "b64_json"
	}

	proxyUrl := h.proxy.GetProxyIP()
	secret, status, err := h.secretFromAuthorization(c, true, true, proxyUrl)
	if err != nil {
		c.JSON(status, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "authorization_error",
			"param":   "Authorization",
			"code":    status,
		}})
		return
	}
	if secret == nil || secret.Token == "" {
		c.JSON(400, gin.H{"error": "Images API requires a logged-in ChatGPT access token."})
		c.Abort()
		return
	}
	if secret.IsFree {
		c.JSON(403, gin.H{"error": "Images API does not support free/noauth accounts. Use a ChatGPT access token."})
		return
	}

	client := bogdanfinn.NewStdClient()
	turnStile, status, err := h.initTurnStileWithRetry(&client, &secret, proxyUrl)
	if err != nil {
		c.JSON(status, gin.H{
			"message": err.Error(),
			"type":    "InitTurnStile_request_error",
			"param":   err,
			"code":    status,
		})
		return
	}

	var data []officialtypes.ImageGenerationData
	for i := 0; i < imageRequest.N; i++ {
		imageResults, upstreamText, err := chatgpt.GeneratePictureConversationImages(client, secret, turnStile, imageRequest.Prompt, imageRequest.Model, proxyUrl)
		if err != nil {
			c.JSON(500, gin.H{"error": gin.H{
				"message": err.Error(),
				"type":    "image_generation_error",
				"param":   nil,
				"code":    "image_generation_error",
			}})
			return
		}
		for _, imageResult := range imageResults {
			item := officialtypes.ImageGenerationData{
				RevisedPrompt: imageRequest.Prompt,
			}
			if imageRequest.ResponseFormat == "b64_json" {
				if imageResult.B64JSON != "" {
					item.B64JSON = imageResult.B64JSON
				} else if imageResult.URL != "" {
					imageBytes, err := chatgpt.DownloadImageBytes(client, imageResult.URL, secret)
					if err != nil {
						c.JSON(500, gin.H{"error": gin.H{
							"message": err.Error(),
							"type":    "image_download_error",
							"param":   nil,
							"code":    "image_download_error",
						}})
						return
					}
					item.B64JSON = base64.StdEncoding.EncodeToString(imageBytes)
				}
			} else {
				item.URL = imageResult.URL
				if item.URL == "" && imageResult.B64JSON != "" {
					item.B64JSON = imageResult.B64JSON
				}
			}
			data = append(data, item)
			if len(data) >= imageRequest.N {
				break
			}
		}
		if len(imageResults) == 0 && upstreamText != "" {
			c.JSON(500, gin.H{"error": gin.H{
				"message": "No image result found in response: " + upstreamText,
				"type":    "image_generation_error",
				"param":   nil,
				"code":    "image_generation_error",
			}})
			return
		}
		if len(data) >= imageRequest.N {
			break
		}
	}
	if len(data) == 0 {
		c.JSON(500, gin.H{"error": gin.H{
			"message": "No image result found in response",
			"type":    "image_generation_error",
			"param":   nil,
			"code":    "image_generation_error",
		}})
		return
	}
	c.JSON(200, officialtypes.NewImageGenerationResponse(data))
}

func (h *Handler) files(c *gin.Context) {
	secret, status, err := h.secretFromAuthorization(c, true, true, h.proxy.GetProxyIP())
	if err != nil {
		c.JSON(status, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "authorization_error",
			"param":   "Authorization",
			"code":    status,
		}})
		return
	}
	if secret == nil || secret.Token == "" || secret.IsFree {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Files API requires a logged-in ChatGPT access token.",
			"type":    "invalid_request_error",
			"param":   nil,
			"code":    "missing_access_token",
		}})
		return
	}

	formFile, err := c.FormFile("file")
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Missing required multipart field: file",
			"type":    "invalid_request_error",
			"param":   "file",
			"code":    "missing_required_parameter",
		}})
		return
	}
	file, err := formFile.Open()
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "invalid_request_error",
			"param":   "file",
			"code":    "file_open_error",
		}})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		c.JSON(400, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "invalid_request_error",
			"param":   "file",
			"code":    "file_read_error",
		}})
		return
	}
	if len(data) == 0 {
		c.JSON(400, gin.H{"error": gin.H{
			"message": "Uploaded file is empty",
			"type":    "invalid_request_error",
			"param":   "file",
			"code":    "empty_file",
		}})
		return
	}

	contentType := formFile.Header.Get("Content-Type")
	client := bogdanfinn.NewStdClient()
	client.SetCookies("https://chatgpt.com", chatgpt.BasicCookies)
	uploaded, status, err := chatgpt.UploadFile(client, secret, h.proxy.GetProxyIP(), formFile.Filename, contentType, data)
	if err != nil {
		c.JSON(status, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "file_upload_error",
			"param":   "file",
			"code":    "file_upload_error",
		}})
		return
	}
	uploaded.CreatedAt = time.Now().Unix()
	chatgpt.RegisterUploadedFile(uploaded)
	c.JSON(200, uploaded)
}

func (h *Handler) engines(c *gin.Context) {
	type ResData struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int    `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	type JSONData struct {
		Object string    `json:"object"`
		Data   []ResData `json:"data"`
	}

	models := []string{
		"auto",
		"gpt-5-5-instant",
		"gpt-5-5-thinking",
		"gpt-5-5-pro",
		"gpt-5",
		"gpt-4o",
		"gpt-4o-mini",
		"o3",
		"o4-mini",
		"o4-mini-high",
	}
	var resModelList []ResData
	for _, model := range models {
		resModelList = append(resModelList, ResData{
			ID:      model,
			Object:  "model",
			Created: 1685474247,
			OwnedBy: "openai",
		})
	}

	c.JSON(200, JSONData{
		Object: "list",
		Data:   resModelList,
	})
}

func (h *Handler) secretFromAuthorization(c *gin.Context, needsPaid bool, allowFallbackPaid bool, proxy string) (*tokens.Secret, int, error) {
	secret := h.token.GetSecret()
	if needsPaid || allowFallbackPaid {
		secret = h.token.GetPaidSecret()
	}

	authToken, teamAccountID, hasAuthorizationTeamID := authorizationTokenAndTeam(c)
	if authToken != "" && os.Getenv("Authorization") != "" && authToken == os.Getenv("Authorization") {
		authToken = ""
	}
	if authToken != "" {
		if strings.HasPrefix(authToken, "eyJhbGciOiJSUzI1NiI") {
			secret = h.token.GenerateTempToken(authToken)
		} else if isUUID(authToken) {
			secret = h.token.GenerateDeviceId(authToken)
		} else if hasAuthorizationTeamID || teamAccountID != "" {
			accessToken, status, err := h.accessTokenFromRefreshToken(authToken, proxy)
			if err != nil {
				return nil, status, err
			}
			secret = h.token.GenerateTempToken(accessToken)
		}
	}
	if needsPaid && (secret == nil || secret.Token == "" || secret.IsFree) && !allowFallbackPaid {
		return nil, 0, nil
	}
	return secret.WithTeamUserID(teamAccountID), 0, nil
}

func (h *Handler) accessTokenFromRefreshToken(refreshToken string, proxy string) (string, int, error) {
	client := bogdanfinn.NewStdClient()
	result, status, err := chatgpt.GETTokenForRefreshToken(client, refreshToken, proxy)
	if status == 0 {
		status = http.StatusBadRequest
	}
	if err != nil {
		return "", status, err
	}
	if data, ok := result.(map[string]interface{}); ok {
		if accessToken, ok := data["access_token"].(string); ok && accessToken != "" {
			return accessToken, status, nil
		}
	}
	return "", status, errors.New("refresh token response did not include access_token")
}

func isUUID(str string) bool {
	_, err := uuid.Parse(str)
	return err == nil
}

func teamAccountIDFromRequest(c *gin.Context) string {
	for _, header := range []string{"ChatGPT-Account-ID", "Chatgpt-Account-Id", "Team-Account-ID", "X-ChatGPT-Account-ID"} {
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			return value
		}
	}
	_, teamAccountID := splitAuthorizationTokenAndTeam(c.GetHeader("Authorization"))
	return teamAccountID
}

func authorizationTokenAndTeam(c *gin.Context) (string, string, bool) {
	token, authorizationTeamID := splitAuthorizationTokenAndTeam(c.GetHeader("Authorization"))
	if teamID := teamAccountIDFromRequest(c); teamID != "" {
		return token, teamID, authorizationTeamID != ""
	}
	return token, authorizationTeamID, authorizationTeamID != ""
}

func splitAuthorizationTokenAndTeam(authHeader string) (string, string) {
	payload := strings.TrimSpace(authHeader)
	if len(payload) >= len("Bearer ") && strings.EqualFold(payload[:len("Bearer ")], "Bearer ") {
		payload = strings.TrimSpace(payload[len("Bearer "):])
	}
	parts := strings.SplitN(payload, ",", 2)
	token := strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		return token, ""
	}
	return token, strings.TrimSpace(parts[1])
}

func countMessagesTokens(messages []officialtypes.APIMessage) int {
	total := 0
	for _, message := range messages {
		total += util.CountToken(message.Text())
	}
	return total
}

func original_requestHasFiles(request officialtypes.APIRequest) bool {
	for _, message := range request.Messages {
		if len(message.Files()) > 0 {
			return true
		}
	}
	return false
}

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

func (h *Handler) tts(c *gin.Context) {
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

	proxyUrl := h.proxy.GetProxyIP()
	secret, status, err := h.secretFromAuthorization(c, true, true, proxyUrl)
	if err != nil {
		c.JSON(status, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "authorization_error",
			"param":   "Authorization",
			"code":    status,
		}})
		return
	}
	if secret == nil || secret.Token == "" {
		c.JSON(400, gin.H{"error": "TTS requires a logged-in ChatGPT access token."})
		return
	}
	if secret.IsFree {
		c.JSON(403, gin.H{"error": "TTS does not support free/noauth accounts. Use a ChatGPT access token."})
		return
	}

	client := bogdanfinn.NewStdClient()

	// Convert the chat request to a ChatGPT request
	translated_request := chatgptrequestconverter.ConvertTTSAPIRequest(original_request.Input)
	clientState := chatgpt.NewChatClientState()
	clientState.ConversationID = translated_request.ConversationID
	clientState.ParentMessageID = translated_request.ParentMessageID

	response, wsConn, status, err := h.postConversationGptClientOrder(&client, &secret, translated_request, proxyUrl, false, clientState)
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
			wsConn = nil
		}
		return
	}
	msgId, convId := chatgpt.HandlerTTS(response, original_request.Input)
	if msgId == "" || convId == "" {
		c.JSON(500, gin.H{"error": "failed to get TTS message id"})
		return
	}
	defer chatgpt.RemoveConversation(client, secret, convId, proxyUrl)

	format := ttsFmtMap[original_request.Format]
	if format == "" {
		format = "aac"
	}
	voice := ttsVoiceMap[original_request.Voice]
	if voice == "" {
		voice = "cove"
	}
	data, status, err := chatgpt.GetTTS(client, secret, msgId, convId, voice, format, proxyUrl)
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

func (h *Handler) chatgptConversation(c *gin.Context) {
	var original_request chatgpt_types.ChatGPTRequest
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
	if original_request.Messages[0].Author.Role == "" {
		original_request.Messages[0].Author.Role = "user"
	}

	proxyUrl := h.proxy.GetProxyIP()

	secret, status, err := h.secretFromAuthorization(c, false, false, proxyUrl)
	if err != nil {
		c.JSON(status, gin.H{"error": gin.H{
			"message": err.Error(),
			"type":    "authorization_error",
			"param":   "Authorization",
			"code":    status,
		}})
		return
	}

	client := bogdanfinn.NewStdClient()
	turnStile, status, err := h.initTurnStileWithRetry(&client, &secret, proxyUrl)
	if err != nil {
		c.JSON(status, gin.H{
			"message": err.Error(),
			"type":    "InitTurnStile_request_error",
			"param":   err,
			"code":    status,
		})
		return
	}

	response, err := chatgpt.POSTconversation(client, original_request, secret, turnStile, proxyUrl)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "error sending request",
		})
		return
	}
	defer response.Body.Close()

	if chatgpt.Handle_request_error(c, response) {
		return
	}

	c.Header("Content-Type", response.Header.Get("Content-Type"))
	if cacheControl := response.Header.Get("Cache-Control"); cacheControl != "" {
		c.Header("Cache-Control", cacheControl)
	}

	_, err = io.Copy(c.Writer, response.Body)
	if err != nil {
		c.JSON(500, gin.H{"error": "Error sending response"})
	}
}
