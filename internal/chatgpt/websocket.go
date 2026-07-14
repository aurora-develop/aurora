package chatgpt

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	fhttp "github.com/bogdanfinn/fhttp"
	"github.com/bogdanfinn/websocket"

	"aurora/httpclient"
	"aurora/internal/accounts"
)

type chatWebsocketURLResponse struct {
	WebsocketURL string `json:"websocket_url"`
}

var chatWebsocketIDCounter int64 = 4

func nextChatWebsocketID() int64 {
	return atomic.AddInt64(&chatWebsocketIDCounter, 1)
}

func getChatWebsocketURL(client httpclient.AuroraHttpClient, account *accounts.Account) (string, error) {
	return getChatWebsocketURLWithState(client, account, nil)
}

func getChatWebsocketURLWithState(client httpclient.AuroraHttpClient, account *accounts.Account, state *ChatClientState) (string, error) {
	apiURL, targetPath := conversationURL(account, "/celsius/ws/user")
	header := conversationHeadersWithState(account, nil, "*/*", targetPath, "", "", state)
	response, err := client.Request(http.MethodGet, apiURL, header, nil, nil)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("celsius ws user failed: %s", string(body))
	}
	var result chatWebsocketURLResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}
	if result.WebsocketURL == "" {
		return "", fmt.Errorf("celsius ws user missing websocket_url: %s", string(body))
	}
	return result.WebsocketURL, nil
}

// DialChatWebsocket 拨号到 ChatGPT WebSocket。
func DialChatWebsocket(client httpclient.AuroraHttpClient, account *accounts.Account) (*websocket.Conn, error) {
	return DialChatWebsocketWithState(client, account, nil)
}

// DialChatWebsocketWithState 拨号到 ChatGPT WebSocket（带 state）。
func DialChatWebsocketWithState(client httpclient.AuroraHttpClient, account *accounts.Account, state *ChatClientState) (*websocket.Conn, error) {
	return DialChatWebsocketWithStateAndProxy(client, account, state, "")
}

// DialChatWebsocketWithStateAndProxy 拨号到 ChatGPT WebSocket（带 state 和代理）。
func DialChatWebsocketWithStateAndProxy(client httpclient.AuroraHttpClient, account *accounts.Account, state *ChatClientState, proxy string) (*websocket.Conn, error) {
	wsURL, err := getChatWebsocketURLWithState(client, account, state)
	if err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{
		HandshakeTimeout: 15 * time.Second,
		Proxy:            fhttp.ProxyFromEnvironment,
	}
	if proxyFunc, err := websocketProxyFunc(proxy); err != nil {
		return nil, err
	} else if proxyFunc != nil {
		dialer.Proxy = proxyFunc
	}
	header := fhttp.Header{}
	ua := defaultUserAgent()
	if state != nil && state.UserAgent != "" {
		ua = state.UserAgent
	}
	header.Set("User-Agent", ua)
	header.Set("Origin", "https://chatgpt.com")
	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		return nil, err
	}
	initMsg := []map[string]interface{}{
		{"id": 1, "command": map[string]interface{}{
			"type":     "connect",
			"presence": map[string]string{"type": "presence", "state": "background"},
		}},
		{"id": 2, "command": map[string]interface{}{"type": "subscribe", "topic_id": "calpico-chatgpt"}},
		{"id": 3, "command": map[string]interface{}{"type": "subscribe", "topic_id": "conversations"}},
		{"id": 4, "command": map[string]interface{}{"type": "subscribe", "topic_id": "app_notifications"}},
	}
	if err := conn.WriteJSON(initMsg); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func websocketProxyFunc(proxy string) (func(*fhttp.Request) (*url.URL, error), error) {
	if proxy == "" {
		return fhttp.ProxyFromEnvironment, nil
	}
	proxyURL, err := url.Parse(proxy)
	if err != nil {
		return nil, err
	}
	return fhttp.ProxyURL(proxyURL), nil
}

func parseChatWebsocketFrames(raw []byte) []map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	if raw[0] == '[' {
		var frames []map[string]interface{}
		if err := json.Unmarshal(raw, &frames); err != nil {
			return nil
		}
		return frames
	}
	var frame map[string]interface{}
	if err := json.Unmarshal(raw, &frame); err != nil {
		return nil
	}
	return []map[string]interface{}{frame}
}

func chatWebsocketEncodedItem(frame map[string]interface{}, topicID string) string {
	if frame == nil {
		return ""
	}
	if frameTopicID, _ := frame["topic_id"].(string); frameTopicID != "" && frameTopicID != topicID {
		return ""
	}
	payload, ok := frame["payload"].(map[string]interface{})
	if !ok {
		return ""
	}
	nested, ok := payload["payload"].(map[string]interface{})
	if !ok {
		return ""
	}
	encoded, _ := nested["encoded_item"].(string)
	return encoded
}

func chatWebsocketSSEItems(frame map[string]interface{}, topicID string) []string {
	if encoded := chatWebsocketEncodedItem(frame, topicID); encoded != "" {
		return []string{encoded}
	}
	if update := chatWebsocketConversationUpdateItem(frame, topicID); update != "" {
		return []string{update}
	}
	return nil
}

func chatWebsocketConversationUpdateItem(frame map[string]interface{}, topicID string) string {
	if frame == nil {
		return ""
	}
	frameTopicID, _ := frame["topic_id"].(string)
	if frameTopicID != "" && frameTopicID != topicID && frameTopicID != "conversations" {
		return ""
	}
	payload, ok := frame["payload"].(map[string]interface{})
	if !ok {
		return ""
	}
	payloadType, _ := payload["type"].(string)
	if payloadType != "conversation-update" {
		if nested, ok := payload["payload"].(map[string]interface{}); ok {
			if nestedType, _ := nested["type"].(string); nestedType == "conversation-update" {
				payload = nested
				payloadType = nestedType
			}
		}
	}
	if payloadType != "conversation-update" {
		return ""
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return "data: " + string(body) + "\n"
}

func chatWebsocketWriteEncodedItem(writer *io.PipeWriter, encoded string) bool {
	if encoded == "" {
		return false
	}
	if !strings.HasSuffix(encoded, "\n") {
		encoded += "\n"
	}
	_, _ = writer.Write([]byte(encoded))
	return strings.Contains(encoded, "data: [DONE]") || strings.Contains(encoded, "data:[DONE]")
}

// chatWebsocketStreamReader 从 WebSocket 读取流式 SSE 数据，通过 Pipe 返回 io.ReadCloser。
func chatWebsocketStreamReader(conn *websocket.Conn, topicID string) (io.ReadCloser, error) {
	reader, writer := io.Pipe()
	subMsg := []map[string]interface{}{
		{"id": nextChatWebsocketID(), "command": map[string]interface{}{
			"type":     "subscribe",
			"topic_id": topicID,
			"offset":   "0",
		}},
	}
	if err := conn.WriteJSON(subMsg); err != nil {
		_ = reader.Close()
		return nil, err
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	})
	go func() {
		defer writer.Close()
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()
		done := make(chan error, 1)
		go func() {
			for {
				conn.SetReadDeadline(time.Now().Add(120 * time.Second))
				_, raw, err := conn.ReadMessage()
				if err != nil {
					done <- err
					return
				}
				for _, frame := range parseChatWebsocketFrames(raw) {
					frameType, _ := frame["type"].(string)
					if frameType == "reply" {
						reply, _ := frame["reply"].(map[string]interface{})
						replyTopicID, _ := reply["topic_id"].(string)
						if replyTopicID != topicID {
							continue
						}
						catchups, _ := reply["catchups"].([]interface{})
						for _, catchup := range catchups {
							catchupFrame, _ := catchup.(map[string]interface{})
							for _, item := range chatWebsocketSSEItems(catchupFrame, topicID) {
								if chatWebsocketWriteEncodedItem(writer, item) {
									done <- nil
									return
								}
							}
						}
						continue
					}
					if frameType != "message" {
						continue
					}
					for _, item := range chatWebsocketSSEItems(frame, topicID) {
						if chatWebsocketWriteEncodedItem(writer, item) {
							done <- nil
							return
						}
					}
				}
			}
		}()
		for {
			select {
			case <-ticker.C:
				_ = conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second))
			case err := <-done:
				if err != nil {
					_ = writer.CloseWithError(err)
				}
				return
			}
		}
	}()
	return reader, nil
}

// shouldUseWebsocketHandoff 判断是否应从 HTTP 切换到 WebSocket 读取。
func shouldUseWebsocketHandoff(readingWebsocket bool, handoffTopicID string, wsConn *websocket.Conn, text string, imgSource []string) bool {
	if readingWebsocket || handoffTopicID == "" || wsConn == nil {
		return false
	}
	return text == "" && strings.Join(imgSource, "") == ""
}
