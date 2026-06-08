package chatgpt

import (
	"aurora/httpclient"
	"aurora/internal/tokens"
	"aurora/typings/chatgpt"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type fakeAuroraClient struct {
	response *http.Response
	headers  httpclient.AuroraHeaders
}

func (f *fakeAuroraClient) Request(method httpclient.HttpMethod, url string, headers httpclient.AuroraHeaders, cookies []*http.Cookie, body io.Reader) (*http.Response, error) {
	f.headers = headers
	return f.response, nil
}

func (f *fakeAuroraClient) SetProxy(url string) error {
	return nil
}

func (f *fakeAuroraClient) SetCookies(rawUrl string, cookies []*http.Cookie) {
}

func (f *fakeAuroraClient) GetCookies(rawUrl string) []*http.Cookie {
	return nil
}

func TestHandlerStreamsPatchEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := strings.Join([]string{
		`data: {"p":"/conversation_id","o":"replace","v":"conv-test"}`,
		`data: {"p":"/message/id","o":"replace","v":"msg-test"}`,
		`data: {"p":"/message/content/parts/0","o":"append","v":"hello"}`,
		`data: {"p":"/message/content/parts/0","o":"append","v":" world"}`,
		`data: {"p":"/message/metadata/finish_details","o":"replace","v":{"type":"stop"}}`,
		`data: {"p":"/message/end_turn","o":"replace","v":true}`,
		`data: [DONE]`,
		``,
	}, "\n")
	response := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)

	full, continueInfo := Handler(c, response, nil, nil, "request-id", chatGPTRequestForTest(), true, "auto")

	if continueInfo != nil {
		t.Fatalf("continueInfo = %#v, want nil", continueInfo)
	}
	if full != "hello world" {
		t.Fatalf("full response = %q, want %q", full, "hello world")
	}
	output := writer.Body.String()
	if !strings.Contains(output, `"content":"hello"`) {
		t.Fatalf("stream output does not contain first delta: %s", output)
	}
	if !strings.Contains(output, `"content":" world"`) {
		t.Fatalf("stream output does not contain second delta: %s", output)
	}
	if !strings.Contains(output, `"finish_reason":"stop"`) {
		t.Fatalf("stream output does not contain stop chunk: %s", output)
	}
}

func TestHandlerStreamsOpenAIChunksAndSentinel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"role":"assistant"}}]}`,
		`data: {"choices":[{"delta":{"content":"你"}}]}`,
		`data: {"choices":[{"delta":{"content":"好"}}]}`,
		`data: {"choices":[{"delta":{}}],"sentinel":{"event":"artifact_pending","kind":"generated_image","title":"正在生成图片..."}}`,
		`data: {"choices":[{"delta":{}}],"sentinel":{"event":"artifact","kind":"generated_image","slot_index":1,"revision":2,"file_id":"file-yyy","url":"http://example.test/image.png"}}`,
		`data: {"choices":[{"delta":{},"finish_reason":"stop"}],"conversation_id":"conv-xxx"}`,
		`data: [DONE]`,
		``,
	}, "\n")
	response := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)

	full, continueInfo := Handler(c, response, nil, nil, "request-id", chatGPTRequestForTest(), true, "gpt-4o")

	if continueInfo != nil {
		t.Fatalf("continueInfo = %#v, want nil", continueInfo)
	}
	if full != "你好" {
		t.Fatalf("full response = %q, want %q", full, "你好")
	}

	chunks := parseSSEChunks(t, writer.Body.String())
	if len(chunks) != 6 {
		t.Fatalf("chunk count = %d, want 6; output: %s", len(chunks), writer.Body.String())
	}
	if chunks[0]["choices"].([]interface{})[0].(map[string]interface{})["delta"].(map[string]interface{})["role"] != "assistant" {
		t.Fatalf("first chunk should include assistant role: %#v", chunks[0])
	}
	if chunks[1]["choices"].([]interface{})[0].(map[string]interface{})["delta"].(map[string]interface{})["content"] != "你" {
		t.Fatalf("second chunk should include first content delta: %#v", chunks[1])
	}
	if chunks[3]["sentinel"].(map[string]interface{})["event"] != "artifact_pending" {
		t.Fatalf("sentinel pending chunk missing: %#v", chunks[3])
	}
	if chunks[4]["sentinel"].(map[string]interface{})["file_id"] != "file-yyy" {
		t.Fatalf("sentinel artifact chunk missing file_id: %#v", chunks[4])
	}
	if chunks[5]["conversation_id"] != "conv-xxx" {
		t.Fatalf("stop chunk conversation_id = %#v, want conv-xxx", chunks[5]["conversation_id"])
	}
	if chunks[5]["choices"].([]interface{})[0].(map[string]interface{})["finish_reason"] != "stop" {
		t.Fatalf("stop chunk finish_reason missing: %#v", chunks[5])
	}
}

func TestHandlerStreamsConcatenatedOpenAIChunks(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := strings.Join([]string{
		`data: {"id":"chunk-1","object":"chat.completion.chunk","created":0,"model":"auto","choices":[{"delta":{"content":"is","role":"assistant"},"index":0,"finish_reason":null}]}`,
		`data: {"id":"chunk-2","object":"chat.completion.chunk","created":0,"model":"auto","choices":[{"delta":{"content":"This is a test!"},"index":0,"finish_reason":null}]}`,
		`data: {"id":"chunk-3","object":"chat.completion.chunk","created":0,"model":"auto","choices":[{"delta":{},"index":0,"finish_reason":"stop"}],"conversation_id":"conv-stream"}`,
		`data: [DONE]`,
	}, "")
	response := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)

	result := HandlerDetailed(c, response, nil, nil, "request-id", chatGPTRequestForTest(), true, "auto")

	if result.Text != "isThis is a test!" {
		t.Fatalf("text = %q, want concatenated deltas", result.Text)
	}
	if result.ConversationID != "conv-stream" {
		t.Fatalf("conversationID = %q, want conv-stream", result.ConversationID)
	}
	if !result.StopSent {
		t.Fatalf("StopSent = false, want true")
	}

	chunks := parseSSEChunks(t, writer.Body.String())
	if len(chunks) != 3 {
		t.Fatalf("chunk count = %d, want 3; output: %s", len(chunks), writer.Body.String())
	}
	firstDelta := chunks[0]["choices"].([]interface{})[0].(map[string]interface{})["delta"].(map[string]interface{})
	if firstDelta["role"] != "assistant" || firstDelta["content"] != "is" {
		t.Fatalf("first chunk delta = %#v, want role+content in one chunk", firstDelta)
	}
	if chunks[1]["choices"].([]interface{})[0].(map[string]interface{})["delta"].(map[string]interface{})["content"] != "This is a test!" {
		t.Fatalf("second chunk should include second content delta: %#v", chunks[1])
	}
	if chunks[2]["conversation_id"] != "conv-stream" {
		t.Fatalf("stop chunk conversation_id = %#v, want conv-stream", chunks[2]["conversation_id"])
	}
}

func TestStreamHandoffTopicFromPayload(t *testing.T) {
	payload := `{"type":"stream_handoff","options":[{"type":"subscribe_ws_topic","topic_id":"conversation-turn-abc"}]}`
	topicID, skip := streamHandoffTopicFromPayload(payload, "")

	if !skip {
		t.Fatalf("skip = false, want true")
	}
	if topicID != "conversation-turn-abc" {
		t.Fatalf("topicID = %q, want conversation-turn-abc", topicID)
	}

	topicID, skip = streamHandoffTopicFromPayload(`{"metadata":{"turn_exchange_id":"xyz"}}`, "server_ste_metadata")
	if !skip || topicID != "conversation-turn-xyz" {
		t.Fatalf("server metadata topic = %q skip=%v, want conversation-turn-xyz true", topicID, skip)
	}
}

func TestChatWebsocketEncodedItem(t *testing.T) {
	frame := map[string]interface{}{
		"type":     "message",
		"topic_id": "conversation-turn-abc",
		"payload": map[string]interface{}{
			"payload": map[string]interface{}{
				"encoded_item": "data: {\"v\":\"hello\"}\n",
			},
		},
	}

	encoded := chatWebsocketEncodedItem(frame, "conversation-turn-abc")
	if encoded != "data: {\"v\":\"hello\"}\n" {
		t.Fatalf("encoded = %q, want websocket encoded item", encoded)
	}
	if chatWebsocketEncodedItem(frame, "conversation-turn-other") != "" {
		t.Fatalf("encoded item should be ignored for other topic")
	}
}

func TestHandlerStreamsShortDeltaEncoding(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := strings.Join([]string{
		`event: delta_encoding`,
		`data: "v1"`,
		`event: delta`,
		`data: {"v":"hello"}`,
		`event: delta`,
		`data: {"v":" world"}`,
		`data: [DONE]`,
		``,
	}, "\n")
	response := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)

	result := HandlerDetailed(c, response, nil, nil, "request-id", chatGPTRequestForTest(), true, "auto")

	if result.Text != "hello world" {
		t.Fatalf("text = %q, want hello world", result.Text)
	}
	if !strings.Contains(writer.Body.String(), `"content":"hello"`) {
		t.Fatalf("stream output missing short delta content: %s", writer.Body.String())
	}
}

func TestHandlerCollectsPatchEventsWithoutStreaming(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := strings.Join([]string{
		`data: {"p":"/message/content/parts/0","o":"append","v":"hello"}`,
		`data: {"p":"/message/content/parts/0","o":"append","v":" world"}`,
		`data: {"p":"/message/end_turn","o":"replace","v":true}`,
		`data: [DONE]`,
		``,
	}, "\n")
	response := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)

	full, continueInfo := Handler(c, response, nil, nil, "request-id", chatGPTRequestForTest(), false, "auto")

	if continueInfo != nil {
		t.Fatalf("continueInfo = %#v, want nil", continueInfo)
	}
	if full != "hello world" {
		t.Fatalf("full response = %q, want %q", full, "hello world")
	}
	if writer.Body.Len() != 0 {
		t.Fatalf("non-streaming handler wrote body %q, want no direct write", writer.Body.String())
	}
}

func TestHandlerDetailedCollectsOpenAIChunkMetadataWithoutStreaming(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"role":"assistant"}}]}`,
		`data: {"choices":[{"delta":{"content":"你"}}]}`,
		`data: {"choices":[{"delta":{"content":"好"}}]}`,
		`data: {"choices":[{"delta":{}}],"sentinel":{"event":"artifact","kind":"generated_image","slot_index":1,"url":"http://example.test/image.png"}}`,
		`data: {"choices":[{"delta":{},"finish_reason":"stop"}],"conversation_id":"conv-xxx"}`,
		`data: [DONE]`,
		``,
	}, "\n")
	response := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)

	result := HandlerDetailed(c, response, nil, nil, "request-id", chatGPTRequestForTest(), false, "gpt-4o")

	if result.Text != "你好" {
		t.Fatalf("text = %q, want %q", result.Text, "你好")
	}
	if result.ConversationID != "conv-xxx" {
		t.Fatalf("conversationID = %q, want conv-xxx", result.ConversationID)
	}
	if !result.StopSent {
		t.Fatalf("StopSent = false, want true")
	}
	if len(result.Sentinel) != 1 {
		t.Fatalf("sentinel count = %d, want 1", len(result.Sentinel))
	}
	if result.Sentinel[0]["event"] != "artifact" || result.Sentinel[0]["url"] != "http://example.test/image.png" {
		t.Fatalf("sentinel = %#v, want artifact with url", result.Sentinel[0])
	}
	if writer.Body.Len() != 0 {
		t.Fatalf("non-streaming handler wrote body %q, want no direct write", writer.Body.String())
	}
}

func TestHandlerDetailedDoesNotMarkStopWhenUpstreamOnlySendsDone(t *testing.T) {
	gin.SetMode(gin.TestMode)

	body := strings.Join([]string{
		`data: {"choices":[{"delta":{"role":"assistant"},"index":0,"finish_reason":null}]}`,
		`data: {"choices":[{"delta":{"content":"This"},"index":0,"finish_reason":null}]}`,
		`data: [DONE]`,
		``,
	}, "\n")
	response := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)

	result := HandlerDetailed(c, response, nil, nil, "request-id", chatGPTRequestForTest(), true, "auto")

	if result.Text != "This" {
		t.Fatalf("text = %q, want This", result.Text)
	}
	if result.StopSent {
		t.Fatalf("StopSent = true, want false so caller can emit stop before [DONE]")
	}
	output := writer.Body.String()
	if strings.Contains(output, `"finish_reason":"stop"`) {
		t.Fatalf("handler should not invent stop internally for upstream DONE-only stream: %s", output)
	}
	if !strings.Contains(output, `"content":"This"`) {
		t.Fatalf("stream output missing content chunk: %s", output)
	}
}

func TestGetConduitTokenAllowsNullToken(t *testing.T) {
	client := &fakeAuroraClient{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"status":"ok","conduit_token":null}`)),
		},
	}

	token, err := getConduitToken(client, chatGPTRequestForTest(), &tokens.Secret{}, nil, "trace-id")

	if err != nil {
		t.Fatalf("getConduitToken returned error for null token: %v", err)
	}
	if token != "" {
		t.Fatalf("token = %q, want empty token", token)
	}
	if client.headers["X-Conduit-Token"] != "no-token" {
		t.Fatalf("prepare X-Conduit-Token = %q, want no-token", client.headers["X-Conduit-Token"])
	}
}

func TestConversationHeadersKeepEmptyConduitHeaderForConversation(t *testing.T) {
	header := conversationHeaders(&tokens.Secret{}, nil, "text/event-stream", "/backend-api/f/conversation", "", "trace-id")

	if _, ok := header["X-Conduit-Token"]; !ok {
		t.Fatalf("X-Conduit-Token header missing for empty conversation conduit token")
	}
	if header["X-Conduit-Token"] != "" {
		t.Fatalf("X-Conduit-Token = %q, want empty string", header["X-Conduit-Token"])
	}
}

func TestShouldUseWebsocketHandoffSkipsCatchupAfterHTTPBody(t *testing.T) {
	if shouldUseWebsocketHandoff(false, "conversation-turn-abc", &websocket.Conn{}, "hello", nil) {
		t.Fatalf("websocket handoff should be skipped when HTTP SSE already emitted text")
	}
	if shouldUseWebsocketHandoff(false, "conversation-turn-abc", &websocket.Conn{}, "", []string{"![image](url)"}) {
		t.Fatalf("websocket handoff should be skipped when HTTP SSE already emitted image content")
	}
	if !shouldUseWebsocketHandoff(false, "conversation-turn-abc", &websocket.Conn{}, "", nil) {
		t.Fatalf("websocket handoff should be used when HTTP SSE has no body yet")
	}
	if shouldUseWebsocketHandoff(true, "conversation-turn-abc", &websocket.Conn{}, "", nil) {
		t.Fatalf("websocket handoff should not be used while already reading websocket")
	}
}

func TestCreateBaseHeaderMatchesWebClientShape(t *testing.T) {
	first := createBaseHeader()
	second := createBaseHeader()

	if first["oai-language"] != "zh-CN" {
		t.Fatalf("oai-language = %q, want zh-CN", first["oai-language"])
	}
	if !strings.Contains(first["user-agent"], "Edg/147.0.0.0") {
		t.Fatalf("user-agent = %q, want Edge 147 shape", first["user-agent"])
	}
	if first["oai-device-id"] == "" || first["oai-device-id"] != second["oai-device-id"] {
		t.Fatalf("oai-device-id should be stable across headers: first=%q second=%q", first["oai-device-id"], second["oai-device-id"])
	}
	if first["oai-session-id"] == "" || first["oai-session-id"] != second["oai-session-id"] {
		t.Fatalf("oai-session-id should be stable across headers: first=%q second=%q", first["oai-session-id"], second["oai-session-id"])
	}
	if first["oai-client-version"] != "prod-81e0c5cdf6140e8c5db714d613337f4aeab94029" {
		t.Fatalf("oai-client-version = %q", first["oai-client-version"])
	}
}

func chatGPTRequestForTest() chatgpt.ChatGPTRequest {
	return chatgpt.NewChatGPTRequest()
}

func parseSSEChunks(t *testing.T, output string) []map[string]interface{} {
	t.Helper()
	var chunks []map[string]interface{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}
		var chunk map[string]interface{}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk); err != nil {
			t.Fatalf("invalid chunk %q: %v", line, err)
		}
		chunks = append(chunks, chunk)
	}
	return chunks
}
