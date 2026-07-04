package httpstream

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func setupTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/test", nil)
	return c, w
}

func TestWriteSSEHeader(t *testing.T) {
	c, w := setupTestContext()
	WriteSSEHeader(c)

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("expected Cache-Control no-cache, got %s", w.Header().Get("Cache-Control"))
	}
	if w.Header().Get("Connection") != "keep-alive" {
		t.Errorf("expected Connection keep-alive, got %s", w.Header().Get("Connection"))
	}
}

func TestWriteSSEEvent(t *testing.T) {
	c, w := setupTestContext()
	payload := map[string]string{"key": "value"}
	ok := WriteSSEEvent(c, "test.event", payload)

	if !ok {
		t.Error("WriteSSEEvent should return true")
	}
	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}
}

func TestWriteDone(t *testing.T) {
	c, w := setupTestContext()
	ok := WriteDone(c)

	if !ok {
		t.Error("WriteDone should return true")
	}
	body := w.Body.String()
	if body != "data: [DONE]\n\n" {
		t.Errorf("expected 'data: [DONE]\\n\\n', got %q", body)
	}
}

func TestWriteImageStreamHeader(t *testing.T) {
	c, w := setupTestContext()
	WriteImageStreamHeader(c)

	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", w.Header().Get("Content-Type"))
	}
}

func TestWriteImageStreamDone(t *testing.T) {
	c, w := setupTestContext()
	ok := WriteImageStreamDone(c)

	if !ok {
		t.Error("WriteImageStreamDone should return true")
	}
	body := w.Body.String()
	if body != "data: [DONE]\n\n" {
		t.Errorf("expected 'data: [DONE]\\n\\n', got %q", body)
	}
}

func TestWriteImageStreamError(t *testing.T) {
	c, w := setupTestContext()
	WriteImageStreamError(c, 0, 1, "test error")

	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}
}
