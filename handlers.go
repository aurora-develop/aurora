package main

import (
	chatgpt_request_converter "freechatgpt/conversion/requests/chatgpt"
	chatgpt "freechatgpt/internal/chatgpt"
	"freechatgpt/internal/tokens"
	official_types "freechatgpt/typings/official"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func tokensHandler(c *gin.Context) {
	// Get the request_tokens from the request (json) and update the request_tokens
	var request_tokens map[string]tokens.Secret
	err := c.BindJSON(&request_tokens)
	if err != nil {
		c.String(400, "tokens not provided")
		return
	}
	ACCESS_TOKENS = tokens.NewAccessToken(request_tokens)
	ACCESS_TOKENS.Save()
	validAccounts = ACCESS_TOKENS.GetKeys()
	c.String(200, "tokens updated")
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

func simulateModel(c *gin.Context) {
	c.JSON(200, gin.H{
		"object": "list",
		"data": []gin.H{
			{
				"id":       "gpt-3.5-turbo",
				"object":   "model",
				"created":  1688888888,
				"owned_by": "chatgpt-to-api",
			},
			{
				"id":       "gpt-4",
				"object":   "model",
				"created":  1688888888,
				"owned_by": "chatgpt-to-api",
			},
		},
	})
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

	token, puid := "", ""

	var proxy_url string
	if len(proxies) == 0 {
		proxy_url = ""
	} else {
		proxy_url = proxies[0]
		// Push used proxy to the back of the list
		proxies = append(proxies[1:], proxies[0])
	}
	uid := uuid.NewString()
	var chat_require *chatgpt.ChatRequire
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		err = chatgpt.InitWSConn(token, uid, proxy_url)
	}()
	go func() {
		defer wg.Done()
		chat_require = chatgpt.CheckRequire(token, puid, proxy_url)
	}()
	wg.Wait()
	if err != nil {
		c.JSON(500, gin.H{"error": "unable to create ws tunnel"})
		return
	}
	if chat_require == nil {
		c.JSON(500, gin.H{"error": "unable to check chat requirement"})
		return
	}
	// Convert the chat request to a ChatGPT request
	translated_request := chatgpt_request_converter.ConvertAPIRequest(original_request, puid, chat_require.Arkose.Required, proxy_url)

	response, err := chatgpt.POSTconversation(translated_request, token, puid, chat_require.Token, proxy_url)
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
		response_part, continue_info = chatgpt.Handler(c, response, token, puid, uid, translated_request, original_request.Stream)
		full_response += response_part
		if continue_info == nil {
			break
		}
		println("Continuing conversation")
		translated_request.Messages = nil
		translated_request.Action = "continue"
		translated_request.ConversationID = continue_info.ConversationID
		translated_request.ParentMessageID = continue_info.ParentID
		if chat_require.Arkose.Required {
			chatgpt_request_converter.RenewTokenForRequest(&translated_request, puid, proxy_url)
		}
		response, err = chatgpt.POSTconversation(translated_request, token, puid, chat_require.Token, proxy_url)
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
