package apierrors

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// JSONError 统一的 JSON 错误响应构造器。
// 所有 handler 使用此函数代替手动拼装 gin.H{"error": ...}。
func JSONError(c *gin.Context, status int, errType, message string, param *string, code interface{}) {
	errObj := gin.H{
		"message": message,
		"type":    errType,
		"param":   param,
		"code":    code,
	}
	c.JSON(status, gin.H{"error": errObj})
}

// BadRequest 返回 400 错误。
func BadRequest(c *gin.Context, errType, message string, code interface{}) {
	JSONError(c, http.StatusBadRequest, errType, message, nil, code)
}

// InvalidRequest 返回标准 "invalid_request_error" 400 错误。
func InvalidRequest(c *gin.Context, message string, code interface{}) {
	BadRequest(c, "invalid_request_error", message, code)
}

// MissingParam 返回缺少必填参数的 400 错误。
func MissingParam(c *gin.Context, param, code string) {
	InvalidRequest(c, "Missing required parameter: "+param, code)
	p := param
	JSONError(c, http.StatusBadRequest, "invalid_request_error",
		"Missing required parameter: "+param, &p, code)
}

// AuthError 返回鉴权错误。
func AuthError(c *gin.Context, status int, message string) {
	JSONError(c, status, "authorization_error", message, strPtr("Authorization"), status)
}

// InternalError 返回 500 级别错误。
func InternalError(c *gin.Context, errType, message string, code interface{}) {
	JSONError(c, http.StatusInternalServerError, errType, message, nil, code)
}

// NotFoundAccount 返回"未找到账号"错误。
func NotFoundAccount(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{"error": "Not Account Found."})
	c.Abort()
}

func strPtr(s string) *string {
	return &s
}
