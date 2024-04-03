package main

import (
	chatgpt_request_converter "aurora/conversion/requests/chatgpt"
	chatgpt "aurora/internal/chatgpt"
	official_types "aurora/typings/official"
	tls_client "github.com/bogdanfinn/tls-client"
	"github.com/bogdanfinn/tls-client/profiles"
	"sync"

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

func nightmare(c *gin.Context) {

	client, _ := tls_client.NewHttpClient(tls_client.NewNoopLogger(), []tls_client.HttpClientOption{
		tls_client.WithCookieJar(tls_client.NewCookieJar()),
		tls_client.WithTimeoutSeconds(600),
		tls_client.WithClientProfile(profiles.Okhttp4Android13),
	}...)

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

	token, puid := "", ""

	var proxy_url = PROXY_URL
	uid := uuid.NewString()
	oidDid := uuid.NewString()
	var chat_require *chatgpt.ChatRequire
	var wg sync.WaitGroup
	wg.Add(1)
	//go func() {
	//	defer wg.Done()
	//	err = chatgpt.InitWSConn(token, uid, proxy_url)
	//}()
	go func() {
		defer wg.Done()
		chat_require = chatgpt.CheckRequire(client, token, puid, proxy_url, oidDid)
	}()
	wg.Wait()
	//if err != nil {
	//	c.JSON(500, gin.H{"error": "unable to create ws tunnel"})
	//	return
	//}
	if chat_require == nil {
		c.JSON(500, gin.H{"error": "unable to check chat requirement"})
		return
	}
	// Convert the chat request to a ChatGPT request
	translated_request := chatgpt_request_converter.ConvertAPIRequest(original_request, puid, chat_require.Arkose.Required, proxy_url)

	response, err := chatgpt.POSTconversation(client, translated_request, token, puid, chat_require.Token, proxy_url, oidDid)
	if err != nil || response == nil {
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
		response_part, continue_info = chatgpt.Handler(c, client, response, token, puid, uid, translated_request, original_request.Stream)
		full_response += response_part
		if continue_info == nil {
			break
		}
		translated_request.Messages = nil
		translated_request.Action = "continue"
		translated_request.ConversationID = continue_info.ConversationID
		translated_request.ParentMessageID = continue_info.ParentID
		response, err = chatgpt.POSTconversation(client, translated_request, token, puid, chat_require.Token, proxy_url, oidDid)
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
	chatgpt.UnlockSpecConn(token, uid)
}
