package main

import (
	chatgpt_request_converter "aurora/conversion/requests/chatgpt"
	"aurora/httpclient"
	"aurora/internal/chatgpt"

	official_types "aurora/typings/official"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func optionsHandler(c *gin.Context) {
	// Set headers for CORS
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "POST")
	c.Header("Access-Control-Allow-Headers", "*")
	c.JSON(200, gin.H{
		"message": "pong",
	})
}

func refresh_handler(c *gin.Context) {
	var refresh_token official_types.OpenAIRefreshToken
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
	proxy_url := ProxyIP.GetProxyIP()
	openaiRefreshToken, status, err := chatgpt.GETTokenForRefreshToken(refresh_token.RefreshToken, proxy_url)
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

func session_handler(c *gin.Context) {
	var session_token official_types.OpenAISessionToken
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
	proxy_url := ProxyIP.GetProxyIP()
	openaiSessionToken, status, err := chatgpt.GETTokenForSessionToken(session_token.SessionToken, proxy_url)
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

func nightmare(c *gin.Context) {
	var original_request official_types.APIRequest
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

	proxy_url := ProxyIP.GetProxyIP()
	secret := ACCESS_TOKENS.GetSecret()
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		customAccessToken := strings.Replace(authHeader, "Bearer ", "", 1)
		if strings.HasPrefix(customAccessToken, "eyJhbGciOiJSUzI1NiI") {
			secret = ACCESS_TOKENS.GenerateTempToken(customAccessToken)
		}
	}
	if secret == nil {
		c.JSON(400, gin.H{"error": "Not Account Found."})
		c.Abort()
		return
	}

	client := httpclient.NewStdClient()
	uid := uuid.NewString()
	turnStile, status, err := chatgpt.InitTurnStile(client, secret, proxy_url)
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
		err = chatgpt.InitWSConn(secret.Token, uid, proxy_url)
		if err != nil {
			c.JSON(500, gin.H{"error": "unable to create ws tunnel"})
			return
		}
	}

	// Convert the chat request to a ChatGPT request
	translated_request := chatgpt_request_converter.ConvertAPIRequest(original_request, secret, turnStile.Arkose, proxy_url)

	response, err := chatgpt.POSTconversation(client, translated_request, secret, turnStile, proxy_url)
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
	var full_response string
	for i := 3; i > 0; i-- {
		var continue_info *chatgpt.ContinueInfo
		var response_part string
		response_part, continue_info = chatgpt.Handler(c, response, secret, uid, translated_request, original_request.Stream)
		full_response += response_part
		if continue_info == nil {
			break
		}
		translated_request.Messages = nil
		translated_request.Action = "continue"
		translated_request.ConversationID = continue_info.ConversationID
		translated_request.ParentMessageID = continue_info.ParentID

		if turnStile.Arkose {
			chatgpt_request_converter.RenewTokenForRequest(&translated_request, secret.PUID, proxy_url)
		}
		response, err = chatgpt.POSTconversation(client, translated_request, secret, turnStile, proxy_url)
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
	}
	if c.Writer.Status() != 200 {
		return
	}
	if !original_request.Stream {
		c.JSON(200, official_types.NewChatCompletion(full_response))
	} else {
		c.String(200, "data: [DONE]\n\n")
	}
	chatgpt.UnlockSpecConn(secret.Token, uid)
}

func engines_handler(c *gin.Context) {
	proxy_url := ProxyIP.GetProxyIP()
	secret := ACCESS_TOKENS.GetSecret()
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		customAccessToken := strings.Replace(authHeader, "Bearer ", "", 1)
		// Check if customAccessToken starts with sk-
		if strings.HasPrefix(customAccessToken, "eyJhbGciOiJSUzI1NiI") {
			secret = ACCESS_TOKENS.GenerateTempToken(customAccessToken)
		}
	}
	if secret.Token == "" || secret == nil {
		c.JSON(400, gin.H{"error": "Not Account Found."})
		return
	}

	resp, status, err := chatgpt.GETengines(secret, proxy_url)
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
	if len(resp.Models) > 2 {
		res_data := ResData{
			ID:      "gpt-4-mobile",
			Object:  "model",
			Created: 1685474247,
			OwnedBy: "openai",
		}
		resModelList = append(resModelList, res_data)
	}
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
