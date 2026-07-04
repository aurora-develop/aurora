package apierrors

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

func TestInvalidRequest(t *testing.T) {
	c, w := setupTestContext()
	InvalidRequest(c, "bad input", "test_error")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
	body := w.Body.String()
	if body == "" {
		t.Error("expected non-empty response body")
	}
}

func TestMissingParam(t *testing.T) {
	c, w := setupTestContext()
	MissingParam(c, "prompt", "missing_required_parameter")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestAuthError(t *testing.T) {
	c, w := setupTestContext()
	AuthError(c, http.StatusUnauthorized, "invalid token")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestInternalError(t *testing.T) {
	c, w := setupTestContext()
	InternalError(c, "server_error", "something broke", 500)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}
}

func TestBadRequest(t *testing.T) {
	c, w := setupTestContext()
	BadRequest(c, "invalid_type", "bad request", "test_code")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestNotFoundAccount(t *testing.T) {
	c, w := setupTestContext()
	NotFoundAccount(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}
