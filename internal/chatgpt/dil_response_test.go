package chatgpt

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandlerPrefersModelDILFallbackMarkdown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := strings.Join([]string{
		`data: {"p":"/conversation_id","o":"replace","v":"conv-dil"}`,
		`data: {"p":"/message/id","o":"replace","v":"msg-dil"}`,
		`data: {"p":"/message/content/parts/0","o":"append","v":"开头<box><AsyncImage query=\"x\"/>原始组件</box>"}`,
		`data: {"p":"/message/metadata/model_dil_v2/fallbackMarkdown","o":"append","v":"开头"}`,
		`data: {"p":"/message/metadata/model_dil_v2/fallbackMarkdown","o":"append","v":"普通文本"}`,
		`data: {"p":"/message/metadata/finish_details","o":"replace","v":{"type":"stop"}}`,
		`data: {"p":"/message/end_turn","o":"replace","v":true}`,
		`data: [DONE]`,
		``,
	}, "\n")

	result, output := runDILHandler(t, body, false, false)
	if result.Text != "开头普通文本" {
		t.Fatalf("text = %q, want server fallbackMarkdown", result.Text)
	}
	if output != "" {
		t.Fatalf("non-stream handler wrote output: %s", output)
	}
}

func TestHandlerSuppressedStreamPrefersModelDILFallbackMarkdown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := `data: {"conversation_id":"conv-dil","message":{"id":"msg-dil","author":{"role":"assistant"},"recipient":"all","content":{"content_type":"text","parts":["<box>原始组件</box>"]},"end_turn":true,"metadata":{"message_type":"next","finish_details":{"type":"stop"},"model_dil_v2":{"fallbackMarkdown":"Responses 普通文本"}}}}` + "\n\ndata: [DONE]\n\n"

	result, output := runDILHandler(t, body, true, true)
	if result.Text != "Responses 普通文本" {
		t.Fatalf("text = %q, want Responses fallbackMarkdown", result.Text)
	}
	if output != "" {
		t.Fatalf("suppressed stream wrote output: %s", output)
	}
}

func TestHandlerStreamDegradesUnknownDILComponents(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := strings.Join([]string{
		`data: {"p":"/conversation_id","o":"replace","v":"conv-dil"}`,
		`data: {"p":"/message/id","o":"replace","v":"msg-dil"}`,
		`data: {"p":"/message/content/parts/0","o":"append","v":"开头<Future"}`,
		`data: {"p":"/message/content/parts/0","o":"append","v":"Card mode={turn1.mode}>正文"}`,
		`data: {"p":"/message/content/parts/0","o":"append","v":"</FutureCard><AnotherNewComponent/>结尾"}`,
		`data: {"p":"/message/metadata/finish_details","o":"replace","v":{"type":"stop"}}`,
		`data: {"p":"/message/end_turn","o":"replace","v":true}`,
		`data: [DONE]`,
		``,
	}, "\n")

	result, output := runDILHandler(t, body, true, false)
	if result.Text != "开头正文结尾" {
		t.Fatalf("text = %q, want generic degraded text", result.Text)
	}
	chunks := parseSSEChunks(t, output)
	var streamed strings.Builder
	for _, chunk := range chunks {
		choices, _ := chunk["choices"].([]interface{})
		if len(choices) == 0 {
			continue
		}
		delta, _ := choices[0].(map[string]interface{})["delta"].(map[string]interface{})
		content, _ := delta["content"].(string)
		streamed.WriteString(content)
	}
	if streamed.String() != result.Text {
		t.Fatalf("streamed text = %q, result = %q; output=%s", streamed.String(), result.Text, output)
	}
	if strings.ContainsAny(streamed.String(), "<>{}") {
		t.Fatalf("stream leaked DIL syntax: %q", streamed.String())
	}
}

func runDILHandler(t *testing.T, body string, stream, suppress bool) (HandlerResult, string) {
	t.Helper()
	response := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)
	result := HandlerDetailedWithOptions(c, response, nil, nil, "request-id", chatGPTRequestForTest(), stream, "auto", HandlerDetailedOptions{SuppressOutput: suppress})
	return result, writer.Body.String()
}
