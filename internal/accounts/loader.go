package accounts

import (
	"bufio"
	"math/rand"
	"os"
	"strings"

	"github.com/google/uuid"
)

// RawToken 从文件加载的原始 token 信息
type RawToken struct {
	Token     string // access_token / refresh_token / session_token
	TeamID    string // 冒号后（仅 access_tokens.txt 支持）
}

// LoadTokensFromFile 从文件读取 token，空行和 # 开头被忽略
func LoadTokensFromFile(path string) []RawToken {
	f, err := os.Open(path)
	if err != nil {
		return []RawToken{}
	}
	defer f.Close()

	var tokens []RawToken
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		token := strings.TrimSpace(parts[0])
		if token == "" {
			continue
		}
		t := RawToken{Token: token}
		if len(parts) > 1 {
			t.TeamID = strings.TrimSpace(parts[1])
		}
		tokens = append(tokens, t)
	}
	return tokens
}

// CreateAccount 从原始 token 创建 Account，分配指纹但不初始化 Client
func CreateAccount(token string, acctType AccountType, profilePool []FingerprintProfile) *Account {
	acct := NewAccount(uuid.NewString(), acctType, token)
	acct.Fingerprint = randomProfile(profilePool)
	return acct
}

// randomProfile 从画像池随机选一个
func randomProfile(profiles []FingerprintProfile) BrowserFingerprint {
	if len(profiles) == 0 {
		return BrowserFingerprint{
			OaiDeviceID:  uuid.NewString(),
			OaiSessionID: uuid.NewString(),
		}
	}
	p := profiles[rand.Intn(len(profiles))]
	return BrowserFingerprint{
		OaiDeviceID:         uuid.NewString(),
		OaiSessionID:        uuid.NewString(),
		UserAgent:           p.UserAgent,
		ScreenWidth:         p.ScreenWidth,
		ScreenHeight:        p.ScreenHeight,
		HardwareConcurrency: p.HardwareConcurrency,
		Platform:            p.Platform,
		TLSProfileName:      p.TLSProfileName,
	}
}
