package httpstream

import (
	"encoding/json"
	"fmt"

	officialtypes "aurora/typings/official"

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

// ── Image Stream 类型 ──

// ImageStreamChunk 图片生成进度事件。
type ImageStreamChunk struct {
	Object            string `json:"object"`
	Index             int    `json:"index"`
	Total             int    `json:"total"`
	Created           int64  `json:"created"`
	ProgressText      string `json:"progress_text,omitempty"`
	UpstreamEventType string `json:"upstream_event_type,omitempty"`
	Model             string `json:"model,omitempty"`
}

// ImageStreamResult 图片生成结果事件。
type ImageStreamResult struct {
	Object  string                              `json:"object"`
	Index   int                                 `json:"index"`
	Total   int                                 `json:"total"`
	Created int64                               `json:"created"`
	Model   string                              `json:"model,omitempty"`
	Data    []officialtypes.ImageGenerationData `json:"data"`
}

// ImageStreamCompleted 图片生成完成事件。
type ImageStreamCompleted struct {
	Object  string                              `json:"object"`
	Created int64                               `json:"created"`
	Model   string                              `json:"model,omitempty"`
	Data    []officialtypes.ImageGenerationData `json:"data"`
}

// ── Image Stream 写出函数 ──

// WriteImageStreamHeader 设置图片流式输出的 SSE 响应头。
func WriteImageStreamHeader(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(200)
}

// WriteImageStreamEvent 写入一个图片流式事件。
func WriteImageStreamEvent(c *gin.Context, event string, payload interface{}) bool {
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

// WriteImageStreamDone 写入图片流式输出的终止标记。
func WriteImageStreamDone(c *gin.Context) bool {
	if _, err := c.Writer.WriteString("data: [DONE]\n\n"); err != nil {
		return false
	}
	c.Writer.Flush()
	return true
}

// WriteImageStreamChunk 写入图片生成进度事件。
func WriteImageStreamChunk(c *gin.Context, index, total int, model string) {
	WriteImageStreamEvent(c, "image.generation.chunk", ImageStreamChunk{
		Object:       "image.generation.chunk",
		Index:        index,
		Total:        total,
		Created:      0,
		Model:        model,
		ProgressText: fmt.Sprintf("Generating image %d/%d ...", index+1, total),
	})
}

// WriteImageStreamResult 写入单张图片结果事件。
func WriteImageStreamResult(c *gin.Context, index, total int, model string, data []officialtypes.ImageGenerationData) {
	WriteImageStreamEvent(c, "image.generation.result", ImageStreamResult{
		Object:  "image.generation.result",
		Index:   index,
		Total:   total,
		Created: 0,
		Model:   model,
		Data:    data,
	})
}

// WriteImageStreamCompleted 写入图片生成完成事件。
func WriteImageStreamCompleted(c *gin.Context, model string, data []officialtypes.ImageGenerationData) {
	WriteImageStreamEvent(c, "image.generation.completed", ImageStreamCompleted{
		Object:  "image.generation.completed",
		Created: 0,
		Model:   model,
		Data:    data,
	})
}

// WriteImageStreamError 写入图片生成错误事件。
func WriteImageStreamError(c *gin.Context, index, total int, message string) {
	WriteImageStreamEvent(c, "image.generation.error", map[string]interface{}{
		"object":  "image.generation.error",
		"index":   index,
		"total":   total,
		"message": message,
	})
}
