package chatgpt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"aurora/httpclient"
	"aurora/internal/accounts"
	chatgpt_types "aurora/typings/chatgpt"
	official_types "aurora/typings/official"
)

// PrepareState 表示 /f/conversation/prepare 的客户端状态机:
// none -> sent -> success -> conversation
type PrepareState string

const (
	PrepareStateNone    PrepareState = "none"
	PrepareStateSent    PrepareState = "sent"
	PrepareStateSuccess PrepareState = "success"
)

// conversationInitResponse 是 POST /conversation/init 的响应。
type conversationInitResponse struct {
	Type              string `json:"type"`
	BannerInfo        any    `json:"banner_info"`
	DefaultModelSlug  string `json:"default_model_slug"`
	AtlasModeEnabled  any    `json:"atlas_mode_enabled"`
}

// POSTConversationInit 调用 /conversation/init 建立会话上下文。
func POSTConversationInit(client httpclient.AuroraHttpClient, account *accounts.Account, state *ChatClientState) (*conversationInitResponse, error) {
	var apiUrl string
	if account != nil && account.Type == accounts.TypeNoAuth {
		apiUrl = strings.Replace(BaseURL, "backend-api", "backend-anon", 1) + "/conversation/init"
	} else {
		apiUrl = BaseURL + "/conversation/init"
	}
	targetPath := "/backend-api/conversation/init"
	header := createBaseHeaderForState(state)
	header.Set("Accept", "*/*")
	header.Set("Content-Type", "application/json")
	header.Set("X-Openai-Target-Path", targetPath)
	header.Set("X-Openai-Target-Route", targetPath)
	if account != nil && account.Type == accounts.TypeNoAuth && account.Token != "" {
		header.Set("Oai-Device-Id", account.Token)
	}
	if account != nil && !(account.Type == accounts.TypeNoAuth) && account.Token != "" {
		header.Set("Authorization", "Bearer "+account.Token)
	}
	setTeamAccountHeader(header, account)
	payload := map[string]any{
		"requested_default_model": nil,
		"conversation_id":         nil,
		"timezone_offset_min":     -480,
		"conversation_origin":     nil,
	}
	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	response, err := client.Request(http.MethodPost, apiUrl, header, nil, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("conversation init failed: %s", readResponseSnippet(response.Body, 500))
	}
	var result conversationInitResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func getConduitToken(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, account *accounts.Account, chatToken *TurnStile, turnTraceID string) (string, error) {
	return getConduitTokenWithState(client, message, account, chatToken, turnTraceID, nil, PrepareStateNone, "")
}

func getConduitTokenWithState(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, account *accounts.Account, chatToken *TurnStile, turnTraceID string, state *ChatClientState, prepareState PrepareState, previousConduitToken string) (string, error) {
	message = requestWithClientState(message, state)
	apiUrl, targetPath := conversationURL(account, "/f/conversation/prepare")
	parentMessageID := message.ParentMessageID
	if parentMessageID == "" {
		parentMessageID = "client-created-root"
	}
	payload := map[string]interface{}{
		"action":                 "next",
		"parent_message_id":      parentMessageID,
		"model":                  conversationPrepareModel(message.Model),
		"client_prepare_state":   string(prepareState),
		"timezone_offset_min":    message.TimezoneOffsetMin,
		"timezone":               "America/Los_Angeles",
		"conversation_mode":      map[string]string{"kind": "primary_assistant"},
		"system_hints":           []string{},
		"supports_buffering":     true,
		"supported_encodings":    []string{"v1"},
		"client_contextual_info": conversationPrepareClientContext(message),
	}
	if prepareState == PrepareStateSent || prepareState == PrepareStateSuccess {
		payload["partial_query"] = map[string]interface{}{
			"id":      uuid.NewString(),
			"author":  map[string]string{"role": "user"},
			"content": map[string]interface{}{"content_type": "text", "parts": []string{conversationPartialText(message)}},
		}
	}
	if message.ConversationID != "" {
		payload["conversation_id"] = message.ConversationID
	}
	bodyJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	header := conversationHeadersWithState(account, chatToken, "*/*", targetPath, previousConduitToken, turnTraceID, state)
	response, err := client.Request(http.MethodPost, apiUrl, header, nil, bytes.NewReader(bodyJSON))
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("conversation prepare failed: %s", string(body))
	}
	var result struct {
		ConduitToken string `json:"conduit_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	return result.ConduitToken, nil
}

// PrepareConversationConduit 执行单步 prepare（无 state）。
func PrepareConversationConduit(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, account *accounts.Account, proxy string, turnTraceID string) (string, error) {
	return PrepareConversationConduitWithState(client, message, account, proxy, turnTraceID, nil)
}

// PrepareConversationConduitWithState 执行单步 prepare（带 state）。
func PrepareConversationConduitWithState(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, account *accounts.Account, proxy string, turnTraceID string, state *ChatClientState) (string, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	return getConduitTokenWithState(client, message, account, nil, turnTraceID, state, PrepareStateNone, "")
}

// PrepareConversationConduitFull 走完整的 none -> sent -> success 三态。
func PrepareConversationConduitFull(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, account *accounts.Account, proxy string, turnTraceID string, state *ChatClientState) (string, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	ensureBootstrapped(client, account)
	token1, err := getConduitTokenWithState(client, message, account, nil, turnTraceID, state, PrepareStateNone, "")
	if err != nil {
		return "", fmt.Errorf("prepare(none) failed: %w", err)
	}
	token2, err := getConduitTokenWithState(client, message, account, nil, turnTraceID, state, PrepareStateSent, token1)
	if err != nil {
		return "", fmt.Errorf("prepare(sent) failed: %w", err)
	}
	token3, err := getConduitTokenWithState(client, message, account, nil, turnTraceID, state, PrepareStateSuccess, token2)
	if err != nil {
		return "", fmt.Errorf("prepare(success) failed: %w", err)
	}
	return token3, nil
}

// PrepareConversationConduitFullWithSentinel 走完整三态 + sentinel headers。
func PrepareConversationConduitFullWithSentinel(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, account *accounts.Account, proxy string, turnTraceID string, state *ChatClientState, turnStile *TurnStile) (string, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	ensureBootstrapped(client, account)
	token1, err := getConduitTokenWithState(client, message, account, turnStile, turnTraceID, state, PrepareStateNone, "")
	if err != nil {
		return "", fmt.Errorf("prepare(none) failed: %w", err)
	}
	token2, err := getConduitTokenWithState(client, message, account, turnStile, turnTraceID, state, PrepareStateSent, token1)
	if err != nil {
		return "", fmt.Errorf("prepare(sent) failed: %w", err)
	}
	token3, err := getConduitTokenWithState(client, message, account, turnStile, turnTraceID, state, PrepareStateSuccess, token2)
	if err != nil {
		return "", fmt.Errorf("prepare(success) failed: %w", err)
	}
	return token3, nil
}

func conversationPrepareModel(model string) string {
	if model == "" {
		return "auto"
	}
	return model
}

func conversationPartialText(message chatgpt_types.ChatGPTRequest) string {
	for i := len(message.Messages) - 1; i >= 0; i-- {
		msg := message.Messages[i]
		if msg.Author.Role != "user" {
			continue
		}
		for _, part := range msg.Content.Parts {
			if text, ok := part.(string); ok && strings.TrimSpace(text) != "" {
				return runeSlice(text, 5)
			}
		}
	}
	return "h"
}

func runeSlice(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) > maxRunes {
		r = r[:maxRunes]
	}
	return string(r)
}

func conversationPrepareClientContext(message chatgpt_types.ChatGPTRequest) map[string]interface{} {
	info := map[string]interface{}{"app_name": "chatgpt.com"}
	for key, value := range message.ClientContextualInfo {
		info[key] = value
	}
	info["app_name"] = "chatgpt.com"
	return info
}

// POSTconversation 发送 /f/conversation（自动 prepare）。
func POSTconversation(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, account *accounts.Account, chat_token *TurnStile, proxy string) (*http.Response, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	turnTraceID := uuid.NewString()
	conduitToken, err := getConduitToken(client, message, account, nil, turnTraceID)
	if err != nil {
		return nil, err
	}
	return POSTconversationPrepared(client, message, account, chat_token, proxy, conduitToken, turnTraceID)
}

// POSTconversationPrepared 发送准备好的 /f/conversation（无 state）。
func POSTconversationPrepared(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, account *accounts.Account, chat_token *TurnStile, proxy string, conduitToken string, turnTraceID string) (*http.Response, error) {
	return POSTconversationPreparedWithState(client, message, account, chat_token, proxy, conduitToken, turnTraceID, nil)
}

// POSTconversationPreparedWithState 发送准备好的 /f/conversation（带 state）。
func POSTconversationPreparedWithState(client httpclient.AuroraHttpClient, message chatgpt_types.ChatGPTRequest, account *accounts.Account, chat_token *TurnStile, proxy string, conduitToken string, turnTraceID string, state *ChatClientState) (*http.Response, error) {
	if proxy != "" {
		client.SetProxy(proxy)
	}
	message = requestWithClientState(message, state)
	apiUrl, targetPath := conversationURL(account, "/f/conversation")
	if API_REVERSE_PROXY != "" {
		apiUrl = API_REVERSE_PROXY
	}
	body_json, err := json.Marshal(message)
	if err != nil {
		return &http.Response{}, err
	}
	header := conversationHeadersWithState(account, chat_token, "text/event-stream", targetPath, conduitToken, turnTraceID, state)
	if account.Type == accounts.TypeNoAuth {
		client.SetCookies("https://chatgpt.com", []*http.Cookie{
			{Name: "oai-device-id", Value: account.Token, Path: "/", Domain: "chatgpt.com"},
		})
	}

	response, err := client.Request(http.MethodPost, apiUrl, header, nil, bytes.NewBuffer(body_json))
	if err != nil {
		return nil, err
	}
	return response, nil
}

// Handle_request_error 检查响应状态码并在出错时写入 gin.Context。
func Handle_request_error(c *gin.Context, response *http.Response) bool {
	if response.StatusCode != 200 {
		var error_response map[string]interface{}
		err := json.NewDecoder(response.Body).Decode(&error_response)
		if err != nil {
			body, _ := io.ReadAll(response.Body)
			c.JSON(response.StatusCode, gin.H{"error": gin.H{
				"message": "Unknown error",
				"type":    "internal_server_error",
				"param":   nil,
				"code":    "500",
				"details": string(body),
			}})
			return true
		}
		c.JSON(response.StatusCode, gin.H{"error": gin.H{
			"message": error_response["detail"],
			"type":    response.Status,
			"param":   nil,
			"code":    "error",
		}})
		return true
	}
	return false
}

// ContinueInfo 表示需要继续对话的信息。
type ContinueInfo struct {
	ConversationID string `json:"conversation_id"`
	ParentID       string `json:"parent_id"`
}

// HandlerResult 是 HandlerDetailedWithOptions 的返回值。
type HandlerResult struct {
	Text              string
	ThinkingText      string
	ConversationID    string
	ParentMessageID   string
	Sentinel          []map[string]interface{}
	ArtifactSignals   []ArtifactSignal
	SandboxArtifacts  []SandboxArtifact
	PDFArtifacts      []PDFArtifact
	GeneratedImageIDs []string
	StopSent          bool
	Continue          *ContinueInfo
	ToolCalls         []official_types.ToolCall
}
