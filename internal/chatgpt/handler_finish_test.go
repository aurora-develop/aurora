package chatgpt

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurora/internal/httpstream"

	"github.com/gin-gonic/gin"
)

func TestHandlerWaitsForTrueEndTurn(t *testing.T) {
	gin.SetMode(gin.TestMode)

	encodings := []struct {
		name string
		wrap func(string) string
	}{
		{name: "direct", wrap: func(frame string) string { return frame }},
		{name: "envelope", wrap: func(frame string) string { return `{"v":` + frame + `}` }},
	}
	modes := []struct {
		name     string
		stream   bool
		suppress bool
	}{
		{name: "stream", stream: true},
		{name: "nonstream"},
		{name: "suppressed_stream", stream: true, suppress: true},
	}
	for _, encoding := range encodings {
		for _, endTurn := range []string{"false", "null", `"false"`, `"true"`, "0"} {
			for _, mode := range modes {
				t.Run(encoding.name+"/"+endTurn+"/"+mode.name, func(t *testing.T) {
					initial := fmt.Sprintf(`{"conversation_id":"conv-test","message":{"id":"msg-test","author":{"role":"assistant"},"recipient":"all","content":{"content_type":"text","parts":["hello"]},"end_turn":%s,"metadata":{"message_type":"next"}}}`, endTurn)
					final := `{"conversation_id":"conv-test","message":{"id":"msg-test","author":{"role":"assistant"},"recipient":"all","content":{"content_type":"text","parts":["hello world"]},"end_turn":true,"metadata":{"message_type":"next","finish_details":{"type":"stop"}}}}`
					body := "data: " + encoding.wrap(initial) + "\n\ndata: " + encoding.wrap(final) + "\n\ndata: [DONE]\n\n"
					response := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
					writer := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(writer)

					result := HandlerDetailedWithOptions(c, response, nil, nil, "request-id", chatGPTRequestForTest(), mode.stream, "auto", HandlerDetailedOptions{SuppressOutput: mode.suppress})

					if result.Text != "hello world" {
						t.Fatalf("text = %q, want complete answer", result.Text)
					}
					if result.Continue != nil {
						t.Fatalf("Continue = %#v, want nil", result.Continue)
					}
					if mode.stream && !mode.suppress {
						httpstream.WriteChatCompletionDone(c, result.StopSent, "auto", result.ConversationID)
						assertCompletedChatStream(t, writer.Body.String(), "hello world", "stop")
					} else if writer.Body.Len() != 0 {
						t.Fatalf("non-output mode wrote response: %s", writer.Body.String())
					}
				})
			}
		}
	}
}

func TestHandlerPatchEndTurnFalseKeepsReading(t *testing.T) {
	gin.SetMode(gin.TestMode)
	body := strings.Join([]string{
		`data: {"p":"/conversation_id","o":"replace","v":"conv-test"}`,
		`data: {"p":"/message/id","o":"replace","v":"msg-test"}`,
		`data: {"p":"/message/end_turn","o":"replace","v":false}`,
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

	result := HandlerDetailed(c, response, nil, nil, "request-id", chatGPTRequestForTest(), true, "auto")
	if result.Text != "hello world" {
		t.Fatalf("text = %q, want complete patched answer", result.Text)
	}
	httpstream.WriteChatCompletionDone(c, result.StopSent, "auto", result.ConversationID)
	assertCompletedChatStream(t, writer.Body.String(), "hello world", "stop")
}

func TestHandlerCompletesWithValidFinishReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name, finishDetails, want string
	}{
		{name: "missing", want: "stop"},
		{name: "null", finishDetails: `,"finish_details":null`, want: "stop"},
		{name: "empty_object", finishDetails: `,"finish_details":{}`, want: "stop"},
		{name: "empty_type", finishDetails: `,"finish_details":{"type":""}`, want: "stop"},
		{name: "stop", finishDetails: `,"finish_details":{"type":"stop"}`, want: "stop"},
		{name: "max_tokens", finishDetails: `,"finish_details":{"type":"max_tokens"}`, want: "length"},
		{name: "length", finishDetails: `,"finish_details":{"type":"length"}`, want: "length"},
		{name: "content_filter", finishDetails: `,"finish_details":{"type":"content_filter"}`, want: "content_filter"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `data: {"conversation_id":"conv-test","message":{"id":"msg-test","author":{"role":"assistant"},"recipient":"all","content":{"content_type":"text","parts":["hello world"]},"end_turn":true,"metadata":{"message_type":"next"` + tc.finishDetails + `}}}` + "\n\ndata: [DONE]\n\n"
			response := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
			writer := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(writer)

			result := HandlerDetailed(c, response, nil, nil, "request-id", chatGPTRequestForTest(), true, "auto")
			if !result.StopSent {
				t.Fatal("StopSent = false after completed turn")
			}
			httpstream.WriteChatCompletionDone(c, result.StopSent, "auto", result.ConversationID)
			assertCompletedChatStream(t, writer.Body.String(), "hello world", tc.want)
		})
	}
}

func TestHandlerMaxTokensPreservesContinuation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endTurn := range []string{"false", "null"} {
		for _, ending := range []string{"done", "eof"} {
			t.Run(endTurn+"/"+ending, func(t *testing.T) {
				body := fmt.Sprintf(`data: {"conversation_id":"conv-test","message":{"id":"msg-test","author":{"role":"assistant"},"recipient":"all","content":{"content_type":"text","parts":["partial answer"]},"end_turn":%s,"metadata":{"message_type":"next","finish_details":{"type":"max_tokens"}}}}`, endTurn) + "\n\n"
				if ending == "done" {
					body += "data: [DONE]\n\n"
				}
				response := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
				writer := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(writer)

				result := HandlerDetailed(c, response, nil, nil, "request-id", chatGPTRequestForTest(), true, "auto")
				if result.Text != "partial answer" {
					t.Fatalf("text = %q, want partial answer", result.Text)
				}
				if result.Continue == nil || result.Continue.ConversationID != "conv-test" || result.Continue.ParentID != "msg-test" {
					t.Fatalf("Continue = %#v, want conv-test/msg-test", result.Continue)
				}
				if result.StopSent {
					t.Fatal("StopSent = true before continuation")
				}
				for _, chunk := range parseSSEChunks(t, writer.Body.String()) {
					choice := chunk["choices"].([]interface{})[0].(map[string]interface{})
					if choice["finish_reason"] != nil {
						t.Fatalf("terminal chunk before continuation: %#v", chunk)
					}
				}
			})
		}
	}
}

func assertCompletedChatStream(t *testing.T, output, wantText, wantReason string) {
	t.Helper()
	var text strings.Builder
	terminals := 0
	done := 0
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		if line == "data: [DONE]" {
			if terminals != 1 {
				t.Fatalf("terminal chunks before DONE = %d, want 1: %s", terminals, output)
			}
			done++
			continue
		}
		if done != 0 {
			t.Fatalf("data after DONE: %s", output)
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason *string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk); err != nil {
			t.Fatal(err)
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				if terminals != 0 {
					t.Fatalf("content after terminal chunk: %s", output)
				}
				text.WriteString(choice.Delta.Content)
			}
			if choice.FinishReason != nil {
				terminals++
				if *choice.FinishReason != wantReason {
					t.Errorf("finish_reason = %q, want %q", *choice.FinishReason, wantReason)
				}
			}
		}
	}
	if text.String() != wantText || terminals != 1 || done != 1 {
		t.Fatalf("text=%q terminals=%d DONE=%d; want %q, 1, 1", text.String(), terminals, done, wantText)
	}
}
