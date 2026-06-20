package chatgpt

import (
	"io"
	"net/http"
	"sync"
	"time"

	"aurora/httpclient"
	"aurora/internal/tokens"
)

// CookieBootstrap 负责把 Cloudflare 在 chatgpt.com 下发的
// cf_clearance / __cf_bm / session-token / oai-chat-web-route / oai-sc
// / __Secure-oai-is / _dd_s / _uasid / _umsid 等 cookie 引导进 CookieJar。
//
// 真实登录用户(浏览器)首次访问 chatgpt.com 时,CF 会通过 Set-Cookie 注入
// 这些关键 cookie,后续请求会自动带上。Go HTTP client 在没有这些 cookie 的
// 情况下,直接发 /f/conversation/prepare 会被 CF 当作机器人拦截。
//
// CookieBootstrap 不会"破解" Turnstile —— 那是 JS challenge,纯 HTTP 过不去。
// 它只是**接收** CF 在简单 GET 请求(非 challenge 路径)下发的 cookie,并维持
// CookieJar 的活跃度。当 cf_clearance 过期(15-30 分钟)时,需要重新走一次
// 浏览器登录刷新。
type CookieBootstrap struct {
	mu           sync.Mutex
	bootstrapped bool
	lastBootAt   time.Time
}

// 全局引导器,所有 secret 共享同一个 CookieJar(每个 client 各自有自己
// 的 jar,这里只负责"告诉 caller 何时需要重引导")。
var cookieBootstrap CookieBootstrap

// bootstrapCookieJar 主动访问 chatgpt.com 几个非 challenge 端点,
// 让 CF 把 cf_clearance / __cf_bm 等 cookie 写入 jar。
//
// 返回 true 表示 jar 已经有"基本可用"的 cookie,可以继续发请求。
// 返回 false 表示拿不到(被 CF 拦了)。
func bootstrapCookieJar(client httpclient.AuroraHttpClient, secret *tokens.Secret) bool {
	cookieBootstrap.mu.Lock()
	defer cookieBootstrap.mu.Unlock()

	// 1) 快速路径:如果 jar 里已经有 cf_clearance 且 10 分钟内没刷新,跳过
	if cookies := client.GetCookies("https://chatgpt.com"); hasUsableCookies(cookies) {
		if time.Since(cookieBootstrap.lastBootAt) < 10*time.Minute {
			cookieBootstrap.bootstrapped = true
			return true
		}
	}

	// 2) 拉首页 + 几个 warm-up 端点
	header := createBaseHeader()
	urls := []string{
		"https://chatgpt.com/",                       // 首页 — CF 注入核心 cookie
		"https://chatgpt.com/api/auth/session",       // 拉 session 状态,触发更多 Set-Cookie
		"https://chatgpt.com/cdn-cgi/trace",          // CF trace,简单 200
	}
	for _, u := range urls {
		resp, err := client.Request(http.MethodGet, u, header, nil, nil)
		if err != nil || resp == nil {
			continue
		}
		// 把响应里 Set-Cookie 的内容消费掉(让 jar 自动收集)
		// tls-client 的 jar 在 Do() 时就会解析 Set-Cookie,这里只需 Drain body
		if resp.Body != nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		_ = header
	}

	cookieBootstrap.lastBootAt = time.Now()
	cookieBootstrap.bootstrapped = true

	// 3) 校验:必须至少有 cf_clearance 或 session-token,否则视为引导失败
	cookies := client.GetCookies("https://chatgpt.com")
	if !hasUsableCookies(cookies) {
		return false
	}
	return true
}

// hasUsableCookies 检查 jar 里是否有"能过 CF"的关键 cookie。
// 至少要有 cf_clearance 或 __Secure-next-auth.session-token.0 之一。
func hasUsableCookies(cookies []*http.Cookie) bool {
	if len(cookies) == 0 {
		return false
	}
	names := make(map[string]bool, len(cookies))
	for _, c := range cookies {
		names[c.Name] = true
	}
	return names["cf_clearance"] || names["__Secure-next-auth.session-token.0"]
}

// ensureBootstrapped 在请求前调用,jar 没初始化就主动引导一次。
// 失败也不 fatal —— 调用方自己判断后续请求是否被 CF 拦截。
func ensureBootstrapped(client httpclient.AuroraHttpClient, secret *tokens.Secret) {
	cookies := client.GetCookies("https://chatgpt.com")
	if hasUsableCookies(cookies) {
		return
	}
	bootstrapCookieJar(client, secret)
}
