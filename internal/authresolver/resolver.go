// Package authresolver 解析请求的 Authorization header,返回对应的 *accounts.Account。
//
// 设计原则:
//   - 不持有 token 池,池由 accounts.Pool 管理
//   - 只负责"从请求头拿到 token 字符串,定位/创建一个 account"
//   - customer key 匹配: 视为无外部 token,返回 nil,让 handler 从池中取一个
//   - 外部 access_token: 创建临时账号 (TypeNoAuth 或 TypeFree)
//   - 外部 JWT (eyJhbGciOiJSUzI1NiI...): 创建临时账号 (TypeFree)
//   - UUID 设备号: 创建临时账号 (TypeNoAuth)
//
// 配合 handler 的使用模式:
//   authresolver.ResolveAccessToken(c) → 返回 access_token 字符串
//   handler 拿字符串去 pool.Lookup() / pool.CreateTemporary()
package authresolver

import (
	"aurora/internal/accounts"
	"errors"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// AccessTokenResolver 从 gin.Context 解析 access token。
type AccessTokenResolver struct {
	// CustomerKey 全局密钥,匹配时视为"无外部 token" (用 customer key 当默认鉴权)
	CustomerKey string
}

func New() *AccessTokenResolver {
	return &AccessTokenResolver{
		CustomerKey: os.Getenv("Authorization"),
	}
}

// ResolveAccessToken 从 gin.Context 解析 Authorization header 中的 token 字符串。
//
// 返回:
//   - token: 解析出的 token (可能是 customer key / JWT access token / UUID device id / "")
//   - isCustomerKey: true 表示 token 等于 CustomerKey,handler 应走"无外部 token"分支
//   - err: 解析失败 (极少发生)
func (r *AccessTokenResolver) ResolveAccessToken(c *gin.Context) (token string, isCustomerKey bool, err error) {
	authHeader := c.GetHeader("Authorization")
	token, _ = splitAuthHeader(authHeader)

	if token != "" && r.CustomerKey != "" && token == r.CustomerKey {
		// customer key 匹配,视为无外部 token
		return "", true, nil
	}
	return token, false, nil
}

// ResolveAccount 从 gin.Context 解析后直接返回对应的 *accounts.Account(临时账号)。
//
// 行为:
//   - customer key 或无 token → 返回 nil (handler 应从池中取)
//   - UUID → 创建 TypeNoAuth 临时账号
//   - JWT access_token → 创建 TypeFree 临时账号 (Token 字段存 JWT)
//   - 其它 → 兜底: 当成 TypeFree
//
// 不消耗 pool 中的账号,不修改 pool。
func (r *AccessTokenResolver) ResolveAccount(c *gin.Context) (*accounts.Account, error) {
	token, isCustomerKey, err := r.ResolveAccessToken(c)
	if err != nil {
		return nil, err
	}
	if isCustomerKey || token == "" {
		return nil, nil
	}

	// 区分 UUID 和 JWT
	if isValidUUID(token) {
		acct := accounts.NewAccount(uuid.NewString(), accounts.TypeNoAuth, token)
		return acct, nil
	}

	// JWT access_token (eyJhbGciOiJSUzI1NiI...) 或其它 → TypeFree
	acct := accounts.NewAccount(uuid.NewString(), accounts.TypeFree, token)
	return acct, nil
}

func splitAuthHeader(authHeader string) (string, string) {
	payload := strings.TrimSpace(authHeader)
	if len(payload) >= len("Bearer ") && strings.EqualFold(payload[:len("Bearer ")], "Bearer ") {
		payload = strings.TrimSpace(payload[len("Bearer "):])
	}
	parts := strings.SplitN(payload, ",", 2)
	token := strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		return token, ""
	}
	return token, strings.TrimSpace(parts[1])
}

func isValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// ErrNoToken 表示请求未携带有效 token
var ErrNoToken = errors.New("no token in request")
