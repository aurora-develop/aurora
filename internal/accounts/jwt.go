package accounts

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
)

// ParseJWTClaims 解析 JWT payload claims (不验证签名,只解析第二段)。
// 返回整个 claims map。解析失败返回 error。
func ParseJWTClaims(jwt string) (map[string]interface{}, error) {
	parts := strings.Split(jwt, ".")
	if len(parts) < 2 {
		return nil, errors.New("invalid JWT: not 3 parts")
	}
	payload := parts[1]

	// 优先用 RawURLEncoding (JWT spec), 失败再退到 StdEncoding (补 padding)
	var decoded []byte
	var err error
	if decoded, err = base64.RawURLEncoding.DecodeString(payload); err != nil {
		if decoded, err = base64.StdEncoding.DecodeString(payload); err != nil {
			// base64 with padding
			if pad := len(payload) % 4; pad != 0 {
				payload += strings.Repeat("=", 4-pad)
			}
			if decoded, err = base64.StdEncoding.DecodeString(payload); err != nil {
				return nil, err
			}
		}
	}

	var claims map[string]interface{}
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return nil, err
	}
	return claims, nil
}

// ExtractChatGPTAccountID 从 JWT claims 中提取 chatgpt_account_id。
// 来源路径: https://api.openai.com/auth.chatgpt_account_id
// 返回空字符串表示提取失败。
func ExtractChatGPTAccountID(jwt string) string {
	claims, err := ParseJWTClaims(jwt)
	if err != nil {
		return ""
	}
	auth, ok := claims["https://api.openai.com/auth"].(map[string]interface{})
	if !ok {
		return ""
	}
	id, _ := auth["chatgpt_account_id"].(string)
	return id
}

// ExtractChatGPTUserID 从 JWT claims 中提取 chatgpt_user_id。
// 来源路径: https://api.openai.com/auth.chatgpt_user_id
func ExtractChatGPTUserID(jwt string) string {
	claims, err := ParseJWTClaims(jwt)
	if err != nil {
		return ""
	}
	auth, ok := claims["https://api.openai.com/auth"].(map[string]interface{})
	if !ok {
		return ""
	}
	uid, _ := auth["chatgpt_user_id"].(string)
	return uid
}

// ExtractEmail 从 JWT claims 中提取 email。
// 来源路径: https://api.openai.com/profile.email
func ExtractEmail(jwt string) string {
	claims, err := ParseJWTClaims(jwt)
	if err != nil {
		return ""
	}
	profile, ok := claims["https://api.openai.com/profile"].(map[string]interface{})
	if !ok {
		return ""
	}
	email, _ := profile["email"].(string)
	return email
}

// ExtractPlanType 从 JWT claims 中提取 chatgpt_plan_type。
// 顶层字段,不是嵌套。
func ExtractPlanType(jwt string) string {
	claims, err := ParseJWTClaims(jwt)
	if err != nil {
		return ""
	}
	plan, _ := claims["chatgpt_plan_type"].(string)
	return plan
}
