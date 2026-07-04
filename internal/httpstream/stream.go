package httpstream

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
)

// WriteSSEHeader 设置标准 SSE 响应头。
func WriteSSEHeader(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(200)
}

// WriteSSEEvent 写入一个 SSE 事件。
func WriteSSEEvent(c *gin.Context, event string, payload interface{}) bool {
	data, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	if event != "" {
		if _, err := c.Writer.WriteString("event: " + event + "\n"); err != nil {
			return false
		}
	}
	if _, err := c.Writer.WriteString("data: "); err != nil {
		return false
	}
	if _, err := c.Writer.Write(data); err != nil {
		return false
	}
	if _, err := c.Writer.WriteString("\n\n"); err != nil {
		return false
	}
	c.Writer.Flush()
	return true
}

// WriteDone 写入 SSE 终止标记。
func WriteDone(c *gin.Context) bool {
	if _, err := c.Writer.WriteString("data: [DONE]\n\n"); err != nil {
		return false
	}
	c.Writer.Flush()
	return true
}

// WriteChatCompletionDone 写入 chat completion 的 stop chunk 和 [DONE]。
func WriteChatCompletionDone(c *gin.Context, stopSent bool, model string, conversationID string) {
	if !stopSent {
		chunk := map[string]interface{}{
			"id":      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
			"object":  "chat.completion.chunk",
			"choices": []interface{}{},
		}
		data, _ := json.Marshal(chunk)
		c.Writer.WriteString("data: " + string(data) + "\n\n")
		c.Writer.Flush()
	}
	c.Writer.WriteString("data: [DONE]\n\n")
	c.Writer.Flush()
}

// WriteUsageChunk 写入 token usage 的 SSE chunk。
func WriteUsageChunk(c *gin.Context, model string, inputTokens, outputTokens int) {
	chunk := map[string]interface{}{
		"id":      "chatcmpl-QXlha2FBbmROaXhpZUFyZUF3ZXNvbWUK",
		"object":  "chat.completion.chunk",
		"created": 0,
		"model":   model,
		"choices": []interface{}{},
		"usage": map[string]interface{}{
			"prompt_tokens":     inputTokens,
			"completion_tokens": outputTokens,
			"total_tokens":      inputTokens + outputTokens,
		},
	}
	data, _ := json.Marshal(chunk)
	c.Writer.WriteString("data: " + string(data) + "\n\n")
	c.Writer.Flush()
}
