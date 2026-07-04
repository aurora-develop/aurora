package authresolver

import (
	"aurora/httpclient/bogdanfinn"
	"aurora/internal/chatgpt"
	"aurora/internal/tokens"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Resolver 从请求上下文和账号池中解析最终使用的 secret。
type Resolver struct {
	token *tokens.AccessToken
}

func NewResolver(token *tokens.AccessToken) *Resolver {
	return &Resolver{token: token}
}

// ResolveRequest 鉴权解析参数。
type ResolveRequest struct {
	NeedsPaid         bool
	AllowFallbackPaid bool
	ProxyURL          string
}

// ResolveResult 鉴权解析结果。
type ResolveResult struct {
	Secret  *tokens.Secret
	Status  int
	Error   error
	TeamID  string
}

// Resolve 从 gin.Context 和账号池中解析最终使用的 secret。
func (r *Resolver) Resolve(c *gin.Context, req ResolveRequest) ResolveResult {
	secret := r.token.GetSecret()
	if req.NeedsPaid || req.AllowFallbackPaid {
		secret = r.token.GetPaidSecret()
	}

	authToken, teamAccountID, _ := authorizationTokenAndTeam(c)
	if authToken != "" && os.Getenv("Authorization") != "" && authToken == os.Getenv("Authorization") {
		authToken = ""
	}
	if authToken != "" {
		if strings.HasPrefix(authToken, "eyJhbGciOiJSUzI1NiI") {
			secret = r.token.GenerateTempToken(authToken)
		} else if isUUID(authToken) {
			secret = r.token.GenerateDeviceId(authToken)
		} else if teamAccountID != "" {
			accessToken, status, err := accessTokenFromRefreshToken(authToken, req.ProxyURL)
			if err != nil {
				return ResolveResult{Status: status, Error: err}
			}
			secret = r.token.GenerateTempToken(accessToken)
		}
	}
	if req.NeedsPaid && (secret == nil || secret.Token == "" || secret.IsFree) && !req.AllowFallbackPaid {
		return ResolveResult{}
	}
	return ResolveResult{
		Secret: secret.WithTeamUserID(teamAccountID),
		Status: 0,
	}
}

// ResolveWithPaidCheck 解析 secret 并校验 paid token 可用性。
// 用于 images、audio 等必须 paid token 的接口。
// 返回 (secret, 需要中止的 http status, error)。
func (r *Resolver) ResolveWithPaidCheck(c *gin.Context, proxyURL string) (*tokens.Secret, int, error) {
	result := r.Resolve(c, ResolveRequest{
		NeedsPaid:         true,
		AllowFallbackPaid: true,
		ProxyURL:          proxyURL,
	})
	if result.Error != nil {
		return nil, result.Status, result.Error
	}
	if result.Secret == nil || result.Secret.Token == "" {
		return nil, 0, nil
	}
	if result.Secret.IsFree {
		return nil, 0, nil
	}
	return result.Secret, 0, nil
}

func accessTokenFromRefreshToken(refreshToken string, proxy string) (string, int, error) {
	client := bogdanfinn.NewStdClient()
	result, status, err := chatgpt.GETTokenForRefreshToken(client, refreshToken, proxy)
	if status == 0 {
		status = http.StatusBadRequest
	}
	if err != nil {
		return "", status, err
	}
	if data, ok := result.(map[string]interface{}); ok {
		if accessToken, ok := data["access_token"].(string); ok && accessToken != "" {
			return accessToken, status, nil
		}
	}
	return "", status, errors.New("refresh token response did not include access_token")
}

func authorizationTokenAndTeam(c *gin.Context) (string, string, bool) {
	token, authorizationTeamID := splitAuthorizationTokenAndTeam(c.GetHeader("Authorization"))
	if teamID := teamAccountIDFromRequest(c); teamID != "" {
		return token, teamID, authorizationTeamID != ""
	}
	return token, authorizationTeamID, authorizationTeamID != ""
}

func teamAccountIDFromRequest(c *gin.Context) string {
	for _, header := range []string{"ChatGPT-Account-ID", "Chatgpt-Account-Id", "Team-Account-ID", "X-ChatGPT-Account-ID"} {
		if value := strings.TrimSpace(c.GetHeader(header)); value != "" {
			return value
		}
	}
	_, teamAccountID := splitAuthorizationTokenAndTeam(c.GetHeader("Authorization"))
	return teamAccountID
}

func splitAuthorizationTokenAndTeam(authHeader string) (string, string) {
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

func isUUID(str string) bool {
	_, err := uuid.Parse(str)
	return err == nil
}
