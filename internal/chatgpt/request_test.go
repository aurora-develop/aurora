package chatgpt

import (
	"aurora/httpclient"
	"aurora/internal/tokens"
	"aurora/typings/chatgpt"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type fakeAuroraClient struct {
	response *http.Response
	headers  httpclient.AuroraHeaders
	body     string
}

func (f *fakeAuroraClient) Request(method httpclient.HttpMethod, url string, headers httpclient.AuroraHeaders, cookies []*http.Cookie, body io.Reader) (*http.Response, error) {
	f.headers = headers
	if body != nil {
		data, _ := io.ReadAll(body)
		f.body = string(data)
	}
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
	request := chatGPTRequestForTest()

	full, continueInfo := Handler(c, response, nil, nil, "request-id", request, true, request.Model)

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
	request := chatGPTRequestForTest()

	result := HandlerDetailed(c, response, nil, nil, "request-id", request, false, request.Model)

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

func TestPrepareConversationConduitDoesNotUseSentinelHeaders(t *testing.T) {
	client := &fakeAuroraClient{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"status":"ok","conduit_token":"abc"}`)),
		},
	}

	token, err := PrepareConversationConduit(client, chatGPTRequestForTest(), &tokens.Secret{}, "", "trace-id")

	if err != nil {
		t.Fatalf("PrepareConversationConduit returned error: %v", err)
	}
	if token != "abc" {
		t.Fatalf("token = %q, want abc", token)
	}
	for _, key := range []string{"openai-sentinel-chat-requirements-token", "openai-sentinel-proof-token", "openai-sentinel-turnstile-token"} {
		if _, ok := client.headers[key]; ok {
			t.Fatalf("prepare conduit unexpectedly includes %s", key)
		}
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
	// UA must be the Edge variant to stay consistent with the hardcoded
	// sec-ch-ua = "Microsoft Edge";v="146". Version is randomized.
	ua := first["user-agent"]
	if !strings.Contains(ua, "Edg/") {
		t.Fatalf("user-agent = %q, want Edge variant to match sec-ch-ua=Microsoft Edge 146", ua)
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

func TestPrepareConversationConduitUsesClientState(t *testing.T) {
	client := &fakeAuroraClient{
		response: &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"status":"ok","conduit_token":"abc"}`)),
		},
	}
	state := NewChatClientState()
	state.DeviceID = "device-state"
	state.SessionID = "session-state"
	state.ParentMessageID = "parent-state"
	state.StartTime = time.Now().Add(-2500 * time.Millisecond)

	token, err := PrepareConversationConduitWithState(client, chatGPTRequestForTest(), &tokens.Secret{}, "", "trace-id", state)

	if err != nil {
		t.Fatalf("PrepareConversationConduitWithState returned error: %v", err)
	}
	if token != "abc" {
		t.Fatalf("token = %q, want abc", token)
	}
	if client.headers["oai-device-id"] != "device-state" {
		t.Fatalf("oai-device-id = %q, want state device", client.headers["oai-device-id"])
	}
	if client.headers["oai-session-id"] != "session-state" {
		t.Fatalf("oai-session-id = %q, want state session", client.headers["oai-session-id"])
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(client.body), &payload); err != nil {
		t.Fatalf("prepare body is invalid json: %v", err)
	}
	if payload["parent_message_id"] != "parent-state" {
		t.Fatalf("parent_message_id = %#v, want parent-state", payload["parent_message_id"])
	}
	contextInfo, ok := payload["client_contextual_info"].(map[string]interface{})
	if !ok {
		t.Fatalf("client_contextual_info missing: %#v", payload)
	}
	loaded, ok := contextInfo["time_since_loaded"].(float64)
	if !ok || loaded < 2 || loaded > 5 {
		t.Fatalf("time_since_loaded = %#v, want dynamic seconds around 3", contextInfo["time_since_loaded"])
	}
}

func TestChatWebsocketConversationUpdateItem(t *testing.T) {
	frame := map[string]interface{}{
		"type":     "message",
		"topic_id": "conversations",
		"payload": map[string]interface{}{
			"type":            "conversation-update",
			"conversation_id": "conv-img",
			"payload": map[string]interface{}{
				"asset_pointer": "sediment://file-img123",
			},
		},
	}

	items := chatWebsocketSSEItems(frame, "conversation-turn-abc")

	if len(items) != 1 {
		t.Fatalf("items = %#v, want one conversation-update SSE item", items)
	}
	payloads := sseDataPayloads(items[0])
	if len(payloads) != 1 || !strings.Contains(payloads[0], "conversation-update") {
		t.Fatalf("payloads = %#v, want conversation-update data", payloads)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(payloads[0]), &raw); err != nil {
		t.Fatalf("conversation-update item is invalid json: %v", err)
	}
	signals := ExtractSignalsFromJSON(raw)
	if len(ImageFileIDsFromSignals(signals)) != 1 || ImageFileIDsFromSignals(signals)[0] != "file-img123" {
		t.Fatalf("signals = %#v, want image file id", signals)
	}
}

func TestWebsocketProxyFuncUsesExplicitProxy(t *testing.T) {
	proxyFunc, err := websocketProxyFunc("http://127.0.0.1:10808")
	if err != nil {
		t.Fatalf("websocketProxyFunc returned error: %v", err)
	}
	reqURL, _ := url.Parse("wss://example.test/ws")
	req := &http.Request{URL: reqURL}
	proxyURL, err := proxyFunc(req)
	if err != nil {
		t.Fatalf("proxy func returned error: %v", err)
	}
	if proxyURL.String() != "http://127.0.0.1:10808" {
		t.Fatalf("proxy URL = %q, want explicit proxy", proxyURL.String())
	}
}

func TestArtifactAccumulatorGeneratedImageRevision(t *testing.T) {
	acc := newArtifactAccumulator()
	raw := map[string]interface{}{
		"type":            "conversation-update",
		"conversation_id": "conv-img",
		"message": map[string]interface{}{
			"id":     "msg-img",
			"author": map[string]interface{}{"role": "assistant"},
			"metadata": map[string]interface{}{
				"image_gen_task_id": "task-1",
			},
			"content": map[string]interface{}{
				"content_type": "image_asset_pointer",
				"parts": []interface{}{
					map[string]interface{}{
						"asset_pointer": "sediment://file-img123",
						"metadata": map[string]interface{}{
							"dalle": map[string]interface{}{
								"gen_id":     "gen-1",
								"slot_index": float64(1),
							},
						},
					},
				},
			},
		},
	}

	events := acc.ObserveRaw(raw, "conv-img")
	finalEvents := acc.Finalize()

	if len(events) < 2 {
		t.Fatalf("events = %#v, want pending and artifact", events)
	}
	if events[0].Event != StreamEventArtifactPending {
		t.Fatalf("first event = %#v, want artifact_pending", events[0])
	}
	last := events[len(events)-1]
	if last.Event != StreamEventArtifact || last.Kind != "generated_image" || last.FileID != "file-img123" || last.GenID != "gen-1" || last.SlotIndex != 1 || last.Revision != 1 {
		t.Fatalf("image artifact event = %#v", last)
	}
	if len(finalEvents) != 1 || finalEvents[0].Event != StreamEventArtifactSlotFinal {
		t.Fatalf("finalEvents = %#v, want slot final", finalEvents)
	}
}

func TestHandlerSeparatesAnalysisAndFinalChannels(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := strings.Join([]string{
		`data: {"v":{"conversation_id":"conv-thinking","message":{"id":"msg-thinking","author":{"role":"assistant"},"channel":"analysis","content":{"content_type":"text","parts":["think"]},"metadata":{"message_type":"next"},"recipient":"all"}}}`,
		`data: {"v":{"conversation_id":"conv-thinking","message":{"id":"msg-final","author":{"role":"assistant"},"channel":"final","content":{"content_type":"text","parts":["answer"]},"metadata":{"message_type":"next"},"recipient":"all","end_turn":true}}}`,
		`data: [DONE]`,
		``,
	}, "\n")
	response := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)

	result := HandlerDetailedWithOptions(c, response, nil, nil, "request-id", chatGPTRequestForTest(), false, "auto", HandlerDetailedOptions{})

	if result.Text != "answer" {
		t.Fatalf("text = %q, want final answer only", result.Text)
	}
	if result.ThinkingText != "think" {
		t.Fatalf("thinking = %q, want analysis text", result.ThinkingText)
	}
	if len(result.Sentinel) == 0 || result.Sentinel[0]["event"] != "thinking" {
		t.Fatalf("sentinel = %#v, want thinking event", result.Sentinel)
	}
	if result.ParentMessageID != "msg-final" {
		t.Fatalf("parent message id = %q, want msg-final", result.ParentMessageID)
	}
}

func TestHandlerTTSParsesPatchStream(t *testing.T) {
	body := strings.Join([]string{
		`data: {"p":"/conversation_id","o":"replace","v":"conv-tts"}`,
		`data: {"p":"/message/id","o":"replace","v":"msg-tts"}`,
		`data: {"p":"/message/author/role","o":"replace","v":"assistant"}`,
		`data: {"p":"/message/content/parts/0","o":"append","v":"hello tts"}`,
		`data: [DONE]`,
		``,
	}, "\n")
	response := &http.Response{Body: io.NopCloser(strings.NewReader(body))}

	msgID, convID := HandlerTTS(response, "hello tts")

	if msgID != "msg-tts" || convID != "conv-tts" {
		t.Fatalf("msgID=%q convID=%q, want msg-tts conv-tts", msgID, convID)
	}
}

func TestHandlerTTSFallsBackToAssistantMessageID(t *testing.T) {
	body := strings.Join([]string{
		`data: {"conversation_id":"conv-tts","message":{"id":"msg-tts","author":{"role":"assistant"},"content":{"content_type":"text","parts":["different text"]},"metadata":{"message_type":"next"},"recipient":"all"}}`,
		`data: [DONE]`,
		``,
	}, "\n")
	response := &http.Response{Body: io.NopCloser(strings.NewReader(body))}

	msgID, convID := HandlerTTS(response, "requested text")

	if msgID != "msg-tts" || convID != "conv-tts" {
		t.Fatalf("msgID=%q convID=%q, want fallback assistant message", msgID, convID)
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
