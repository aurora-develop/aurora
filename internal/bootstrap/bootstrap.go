package bootstrap

import (
	"os"
	"strings"
	"time"

	"aurora/internal/accounts"
	"aurora/internal/browserfp"
	"aurora/internal/chatgpt"
	"aurora/internal/config"
	"aurora/internal/handler"
	"aurora/internal/proxy"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

// App 封装应用启动所需的所有依赖
type App struct {
	Router      *gin.Engine
	Config      *config.Config
	AccountPool *accounts.Pool
	Cleanup     func() // 服务退出时调用，停止后台协程
}

// Init 完成所有初始化逻辑，返回 App 实例
func Init() (*App, error) {
	gin.SetMode(gin.ReleaseMode)
	_ = godotenv.Load(".env")
	browserfp.Init()

	cfg := config.Load()

	proxies := loadProxyList()
	proxyPool := proxy.NewPool(proxies)

	// ─── 加载账号 ────────────────────────────────────────────────

	profiles := accounts.DefaultProfiles
	var accs []*accounts.Account

	// 去重 key: chatgpt_account_id (for free/puid) 或 token (for noauth/refresh)
	seen := make(map[string]bool)

	// 1. access_tokens.txt — 纯 access token，不可续期
	for _, t := range accounts.LoadTokensFromFile("access_tokens.txt") {
		// 从 access_token JWT 解析 chatgpt_account_id 用于去重
		chatGPTID := accounts.ExtractChatGPTAccountID(t.Token)
		key := "access:" + chatGPTID
		if seen[key] {
			continue
		}
		seen[key] = true

		acct := accounts.CreateAccount(t.Token, accounts.TypeFree, profiles)
		acct.TeamUserID = t.TeamID
		acct.ChatGPTAccountID = chatGPTID
		acct.Status = accounts.StatusActive
		acct.Proxy = proxyPool.Allocate()
		accs = append(accs, acct)
	}

	// 2. refresh_tokens.txt — 带 refresh_token，可续期
	for _, t := range accounts.LoadTokensFromFile("refresh_tokens.txt") {
		// 立即交换一次获取 access_token 才能解析 chatgpt_account_id
		// 先用 refresh_token 本身做去重 key,避免重复交换
		key := "refresh:" + t.Token
		if seen[key] {
			continue
		}
		seen[key] = true

		acct := accounts.CreateAccount("", accounts.TypeFree, profiles)
		acct.RefreshToken = t.Token
		acct.TeamUserID = t.TeamID
		acct.Proxy = proxyPool.Allocate()
		// 立即交换一次获取 access_token
		if exchangeRefreshToken(acct) {
			// exchange 成功后从 access_token 解析 chatgpt_account_id
			acct.ChatGPTAccountID = accounts.ExtractChatGPTAccountID(acct.Token)
			acct.Status = accounts.StatusActive
		} else {
			acct.Status = accounts.StatusExpired
		}
		accs = append(accs, acct)
	}

	// 3. session_tokens.txt — ChatGPT session token，用于免费账号续期
	for _, t := range accounts.LoadTokensFromFile("session_tokens.txt") {
		key := "session:" + t.Token
		if seen[key] {
			continue
		}
		seen[key] = true

		acct := accounts.CreateAccount("", accounts.TypeFree, profiles)
		acct.SessionToken = t.Token
		acct.Proxy = proxyPool.Allocate()
		// 立即交换一次获取 access_token
		if exchangeSessionToken(acct) {
			// exchange 成功后从 access_token 解析 chatgpt_account_id
			acct.ChatGPTAccountID = accounts.ExtractChatGPTAccountID(acct.Token)
			acct.Status = accounts.StatusActive
		} else {
			acct.Status = accounts.StatusExpired
		}
		accs = append(accs, acct)
	}

	// 4. free_tokens.txt — 设备 UUID
	for _, t := range accounts.LoadTokensFromFile("free_tokens.txt") {
		key := "free:" + t.Token
		if seen[key] {
			continue
		}
		seen[key] = true

		acct := accounts.CreateAccount(t.Token, accounts.TypeNoAuth, profiles)
		acct.Proxy = proxyPool.Allocate()
		acct.Status = accounts.StatusActive
		accs = append(accs, acct)
	}

	// 5. FREE_ACCOUNTS — 自动生成 UUID 账号
	if cfg.FreeAccounts {
		for i := 0; i < cfg.FreeAccountsNum; i++ {
			uid := uuid.NewString()
			// UUID 设备号天然唯一,不需要去重
			acct := accounts.CreateAccount(uid, accounts.TypeNoAuth, profiles)
			acct.Proxy = proxyPool.Allocate()
			acct.Status = accounts.StatusActive
			accs = append(accs, acct)
		}
	}

	// 初始化 TLS Client
	for _, acct := range accs {
		_ = acct.InitClient()
	}

	accountPool := accounts.NewPool(accs)

	// 启动健康检查（每 10 分钟续期过期 token）
	renewFn := func(acct *accounts.Account) bool {
		if acct.RefreshToken != "" {
			return exchangeRefreshToken(acct)
		}
		if acct.SessionToken != "" {
			return exchangeSessionToken(acct)
		}
		return false
	}
	stopHealthCheck := accountPool.StartHealthCheck(10*time.Minute, renewFn)

	// 注册路由
	router := handler.RegisterRouter(accountPool, &cfg)

	return &App{
		Router:      router,
		Config:      &cfg,
		AccountPool: accountPool,
		Cleanup:     func() { stopHealthCheck() },
	}, nil
}

// exchangeRefreshToken 用 refresh_token 换 access_token，使用账号自身的 Client（已绑定代理）
func exchangeRefreshToken(acct *accounts.Account) bool {
	if acct.RefreshToken == "" {
		return false
	}
	if acct.Client == nil {
		_ = acct.InitClient()
	}
	result, _, err := chatgpt.GETTokenForRefreshToken(acct.Client, acct.RefreshToken, "")
	if err != nil {
		return false
	}
	// 优先尝试 struct 形式 (*OpenAIAccessTokenWithSession 等)
	if r, ok := result.(interface{ GetAccessToken() string }); ok {
		if at := r.GetAccessToken(); at != "" {
			acct.Token = at
			return true
		}
	}
	// fallback: map[string]interface{} (旧 API 直接返回 map)
	if data, ok := result.(map[string]interface{}); ok {
		if accessToken, ok := data["access_token"].(string); ok && accessToken != "" {
			acct.Token = accessToken
			return true
		}
	}
	return false
}

// exchangeSessionToken 用 session_token 换 access_token，使用账号自身的 Client（已绑定代理）
func exchangeSessionToken(acct *accounts.Account) bool {
	if acct.SessionToken == "" {
		return false
	}
	if acct.Client == nil {
		_ = acct.InitClient()
	}
	result, _, err := chatgpt.GETTokenForSessionToken(acct.Client, acct.SessionToken, "")
	if err != nil {
		return false
	}
	// 优先尝试 struct 形式 (*OpenAIAccessTokenWithSession)
	if r, ok := result.(interface{ GetAccessToken() string }); ok {
		if at := r.GetAccessToken(); at != "" {
			acct.Token = at
			return true
		}
	}
	// fallback: map[string]interface{}
	if data, ok := result.(map[string]interface{}); ok {
		if accessToken, ok := data["accessToken"].(string); ok && accessToken != "" {
			acct.Token = accessToken
			return true
		}
	}
	return false
}

// loadProxyList 从 proxies.txt / PROXY_URL / http_proxy 加载代理列表
func loadProxyList() []string {
	var proxies []string
	proxyUrl := os.Getenv("PROXY_URL")
	if proxyUrl != "" {
		proxies = append(proxies, proxyUrl)
	}

	if _, err := os.Stat("proxies.txt"); err == nil {
		data, err := os.ReadFile("proxies.txt")
		if err == nil {
			lines := string(data)
			for _, line := range strings.Split(lines, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					proxies = append(proxies, line)
				}
			}
		}
	}

	if len(proxies) == 0 {
		proxy := os.Getenv("http_proxy")
		if proxy != "" {
			proxies = append(proxies, proxy)
		}
	}

	return proxies
}
