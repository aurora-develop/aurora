package initialize

import (
	"aurora/internal/httpstream"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestWriteChatCompletionStreamDoneAddsStopBeforeDone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)

	httpstream.WriteChatCompletionDone(c, false, "auto", "conv-xxx")

	lines := sseDataLines(writer.Body.String())
	if len(lines) != 2 {
		t.Fatalf("data line count = %d, want 2; output: %s", len(lines), writer.Body.String())
	}
	var stopChunk map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &stopChunk); err != nil {
		t.Fatalf("invalid stop chunk: %v", err)
	}
	if stopChunk["conversation_id"] != "conv-xxx" {
		t.Fatalf("conversation_id = %#v, want conv-xxx", stopChunk["conversation_id"])
	}
	choices := stopChunk["choices"].([]interface{})
	if choices[0].(map[string]interface{})["finish_reason"] != "stop" {
		t.Fatalf("finish_reason = %#v, want stop", choices[0].(map[string]interface{})["finish_reason"])
	}
	if lines[1] != "[DONE]" {
		t.Fatalf("last line = %q, want [DONE]", lines[1])
	}
}

func TestWriteChatCompletionStreamDoneSkipsDuplicateStop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)

	httpstream.WriteChatCompletionDone(c, true, "auto", "conv-xxx")

	lines := sseDataLines(writer.Body.String())
	if len(lines) != 1 || lines[0] != "[DONE]" {
		t.Fatalf("data lines = %#v, want only [DONE]", lines)
	}
}

func sseDataLines(body string) []string {
	var lines []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "data: ") {
			lines = append(lines, strings.TrimPrefix(line, "data: "))
		}
	}
	return lines
}
