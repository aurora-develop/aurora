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
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"io"
	"os"
	"strings"
)

type Handler struct {
	proxy *proxys.IProxy
	token *tokens.AccessToken
}

func NewHandle(proxy *proxys.IProxy, token *tokens.AccessToken) *Handler {
	return &Handler{proxy: proxy, token: token}
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
	input_tokens := util.CountToken(original_request.Messages[0].Content)
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
	secret := h.token.GetSecret()
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		customAccessToken := strings.Replace(authHeader, "Bearer ", "", 1)
		if strings.HasPrefix(customAccessToken, "eyJhbGciOiJSUzI1NiI") {
			secret = h.token.GenerateTempToken(customAccessToken)
		}
	}
	if secret == nil {
		c.JSON(400, gin.H{"error": "Not Account Found."})
		c.Abort()
		return
	}

	uid := uuid.NewString()
	client := bogdanfinn.NewStdClient()
	client.SetCookies("https://chatgpt.com", chatgpt.BasicCookies)
	turnStile, status, err := chatgpt.InitTurnStile(client, secret, proxyUrl)
	if err != nil {
		c.JSON(status, gin.H{
			"message": err.Error(),
			"type":    "InitTurnStile_request_error",
			"param":   err,
			"code":    status,
		})
		return
	}
	if !secret.IsFree {
		err = chatgpt.InitWSConn(client, secret.Token, uid, proxyUrl)
		if err != nil {
			c.JSON(500, gin.H{"error": "unable to create ws tunnel"})
			return
		}
	}

	// Convert the chat request to a ChatGPT request
	translated_request := chatgptrequestconverter.ConvertAPIRequest(original_request, secret, turnStile.Arkose, proxyUrl)

	response, err := chatgpt.POSTconversation(client, translated_request, secret, turnStile, proxyUrl)

	if err != nil {
		c.JSON(500, gin.H{
			"error": "request conversion error",
		})
		return
	}
	defer response.Body.Close()
	if chatgpt.Handle_request_error(c, response) {
		return
	}
	var full_response string

	if os.Getenv("STREAM_MODE") == "false" {
		original_request.Stream = false
	}
	for i := 3; i > 0; i-- {
		var continue_info *chatgpt.ContinueInfo
		var response_part string
		response_part, continue_info = chatgpt.Handler(c, response, client, secret, uid, translated_request, original_request.Stream)
		full_response += response_part
		if continue_info == nil {
			break
		}
		translated_request.Messages = nil
		translated_request.Action = "continue"
		translated_request.ConversationID = continue_info.ConversationID
		translated_request.ParentMessageID = continue_info.ParentID

		if turnStile.Arkose {
			chatgptrequestconverter.RenewTokenForRequest(&translated_request, secret.PUID, proxyUrl)
		}
		response, err = chatgpt.POSTconversation(client, translated_request, secret, turnStile, proxyUrl)

		if err != nil {
			c.JSON(500, gin.H{
				"error": "request conversion error",
			})
			return
		}
		defer response.Body.Close()
		if chatgpt.Handle_request_error(c, response) {
			return
		}
	}
	if c.Writer.Status() != 200 {
		return
	}
	if !original_request.Stream {
		output_tokens := util.CountToken(full_response)
		c.JSON(200, officialtypes.NewChatCompletion(full_response, input_tokens, output_tokens))
	} else {
		c.String(200, "data: [DONE]\n\n")
	}
	chatgpt.UnlockSpecConn(secret.Token, uid)
}

func (h *Handler) engines(c *gin.Context) {
	proxyUrl := h.proxy.GetProxyIP()
	secret := h.token.GetSecret()
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		customAccessToken := strings.Replace(authHeader, "Bearer ", "", 1)
		// Check if customAccessToken starts with sk-
		if strings.HasPrefix(customAccessToken, "eyJhbGciOiJSUzI1NiI") {
			secret = h.token.GenerateTempToken(customAccessToken)
		}
	}
	if secret == nil || secret.Token == "" {
		c.JSON(400, gin.H{"error": "Not Account Found."})
		return
	}
	client := bogdanfinn.NewStdClient()
	resp, status, err := chatgpt.GETengines(client, secret, proxyUrl)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "error sending request",
		})
		return
	}

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

	modelS := JSONData{
		Object: "list",
	}
	var resModelList []ResData
	for _, model := range resp.Models {
		res_data := ResData{
			ID:      model.Slug,
			Object:  "model",
			Created: 1685474247,
			OwnedBy: "openai",
		}
		if model.Slug == "text-davinci-002-render-sha" {
			res_data.ID = "gpt-3.5-turbo"
		}
		resModelList = append(resModelList, res_data)
	}
	modelS.Data = resModelList
	c.JSON(status, modelS)
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

	var secret *tokens.Secret

	isUUID := func(str string) bool {
		_, err := uuid.Parse(str)
		return err == nil
	}

	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		customAccessToken := strings.Replace(authHeader, "Bearer ", "", 1)
		if strings.HasPrefix(customAccessToken, "eyJhbGciOiJSUzI1NiI") {
			secret = h.token.GenerateTempToken(customAccessToken)
		}
		if isUUID(customAccessToken) {
			secret = h.token.GenerateDeviceId(customAccessToken)
		}
	}

	if secret == nil {
		secret = h.token.GetSecret()
	}

	client := bogdanfinn.NewStdClient()
	turnStile, status, err := chatgpt.InitTurnStile(client, secret, proxyUrl)
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
