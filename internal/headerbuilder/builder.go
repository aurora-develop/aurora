package headerbuilder

import (
	"aurora/httpclient"
	"aurora/internal/browserfp"
	"aurora/internal/tokens"
	"aurora/util"
	"strings"
)

// Builder HTTP header 构造器，封装鉴权、cookie、team account 等公共逻辑。
type Builder struct {
	header httpclient.AuroraHeaders
}

// New 创建一个新的 header builder。
func New() *Builder {
	return &Builder{
		header: make(httpclient.AuroraHeaders),
	}
}

// Build 返回构造好的 header。
func (b *Builder) Build() httpclient.AuroraHeaders {
	return b.header
}

// ── 基础 header ──

// WithBaseHeaders 设置浏览器基础 header（Accept, Origin, sec-ch-ua 等）。
func (b *Builder) WithBaseHeaders(conversationID string) *Builder {
	b.header.Set("Accept", "*/*")
	b.header.Set("Accept-Language", "en-US,en;q=0.9")
	b.header.Set("Oai-Language", "en-US")
	b.header.Set("Origin", "https://chatgpt.com")
	if conversationID != "" {
		b.header.Set("Referer", "https://chatgpt.com/c/"+conversationID)
	} else {
		b.header.Set("Referer", "https://chatgpt.com/")
	}
	b.header.Set("Sec-Ch-Ua", `"Chromium";v="148", "Google Chrome";v="148", "Not/A)Brand";v="99"`)
	b.header.Set("Sec-Ch-Ua-Mobile", "?0")
	b.header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	b.header.Set("Priority", "u=1, i")
	b.header.Set("Sec-Fetch-Dest", "empty")
	b.header.Set("Sec-Fetch-Mode", "cors")
	b.header.Set("Sec-Fetch-Site", "same-origin")
	b.header.Set("User-Agent", util.FixedUserAgent)
	b.header.Set("Oai-Client-Build-Number", "7823760")
	if fp := browserfp.Get(); fp != nil {
		b.header.Set("Oai-Client-Version", fp.BuildID)
	} else {
		b.header.Set("Oai-Client-Version", browserfp.DefaultBuildID)
	}
	return b
}

// WithUserAgent 设置 User-Agent。
func (b *Builder) WithUserAgent(ua string) *Builder {
	if ua != "" {
		b.header.Set("User-Agent", ua)
	}
	return b
}

// WithDeviceID 设置设备 ID。
func (b *Builder) WithDeviceID(deviceID string) *Builder {
	if deviceID != "" {
		b.header.Set("Oai-Device-Id", deviceID)
	}
	return b
}

// WithSessionID 设置 session ID。
func (b *Builder) WithSessionID(sessionID string) *Builder {
	if sessionID != "" {
		b.header.Set("Oai-Session-Id", sessionID)
	}
	return b
}

// ── 鉴权 header ──

// WithAuth 根据 secret 设置鉴权 header（Bearer token 或 Oai-Device-Id）。
func (b *Builder) WithAuth(secret *tokens.Secret) *Builder {
	if secret == nil {
		return b
	}
	if secret.IsFree && secret.Token != "" {
		b.header.Set("Oai-Device-Id", secret.Token)
	}
	if !secret.IsFree && secret.Token != "" {
		b.header.Set("Authorization", "Bearer "+secret.Token)
	}
	return b
}

// WithCookies 根据 secret 设置 cookie header。
func (b *Builder) WithCookies(secret *tokens.Secret) *Builder {
	if secret == nil {
		return b
	}
	cookieStr := ""
	if secret.PUID != "" {
		cookieStr = "_puid=" + secret.PUID
	}
	if secret.IsFree && secret.Token != "" {
		if cookieStr != "" {
			cookieStr += "; "
		}
		cookieStr += "oai-did=" + secret.Token
	}
	if cookieStr != "" {
		b.header["Cookie"] = cookieStr
	}
	return b
}

// WithTeamAccount 根据 secret 设置 team account header。
func (b *Builder) WithTeamAccount(secret *tokens.Secret) *Builder {
	if secret != nil && strings.TrimSpace(secret.TeamUserID) != "" {
		b.header.Set("Chatgpt-Account-Id", strings.TrimSpace(secret.TeamUserID))
	}
	return b
}

// ── Content-Type / Accept ──

// WithContentType 设置 Content-Type。
func (b *Builder) WithContentType(ct string) *Builder {
	b.header.Set("Content-Type", ct)
	return b
}

// WithAccept 设置 Accept。
func (b *Builder) WithAccept(accept string) *Builder {
	b.header.Set("Accept", accept)
	return b
}

// ── Target Path ──

// WithTargetPath 设置 X-Openai-Target-Path 和 X-Openai-Target-Route。
func (b *Builder) WithTargetPath(path string) *Builder {
	b.header.Set("X-Openai-Target-Path", path)
	b.header.Set("X-Openai-Target-Route", path)
	return b
}

// ── Sentinel Token ──

// WithSentinelTokens 设置 sentinel 相关的 token header。
type SentinelTokens struct {
	TurnStileToken              string
	ChatRequirementsPrepareToken string
	ProofOfWorkToken            string
	TurnstileToken              string
	SOToken                     string
}

// WithSentinelTokens 设置 sentinel token header。
func (b *Builder) WithSentinelTokens(tokens SentinelTokens) *Builder {
	if tokens.TurnStileToken != "" {
		b.header.Set("Openai-Sentinel-Chat-Requirements-Token", tokens.TurnStileToken)
	}
	if tokens.ChatRequirementsPrepareToken != "" {
		b.header.Set("Openai-Sentinel-Chat-Requirements-Prepare-Token", tokens.ChatRequirementsPrepareToken)
	}
	if tokens.ProofOfWorkToken != "" {
		b.header.Set("Openai-Sentinel-Proof-Token", tokens.ProofOfWorkToken)
	}
	if tokens.TurnstileToken != "" {
		b.header.Set("Openai-Sentinel-Turnstile-Token", tokens.TurnstileToken)
	}
	if tokens.SOToken != "" {
		b.header.Set("Openai-Sentinel-So-Token", tokens.SOToken)
	}
	return b
}

// ── Conduit Token ──

// WithConduitToken 设置 X-Conduit-Token（即使为空也设置）。
func (b *Builder) WithConduitToken(token string) *Builder {
	b.header.Set("X-Conduit-Token", token)
	return b
}

// ── Turn Trace ID ──

// WithTurnTraceID 设置 X-Oai-Turn-Trace-Id。
func (b *Builder) WithTurnTraceID(id string) *Builder {
	if id != "" {
		b.header.Set("X-Oai-Turn-Trace-Id", id)
	}
	return b
}

// ── Conversation 特有 header ──

// WithConversationHeaders 设置 conversation 请求特有的 header。
func (b *Builder) WithConversationHeaders(targetPath string) *Builder {
	if strings.HasSuffix(targetPath, "/f/conversation") && !strings.HasSuffix(targetPath, "/prepare") {
		b.header.Set("Oai-Echo-Logs", "0,943,1,65876,0,68124,1,68930")
		b.header.Set("Oai-Telemetry", "[1,null]")
	}
	return b
}

// ── 便捷构造函数 ──

// NewBaseHeader 创建基础 header（无 state）。
func NewBaseHeader() httpclient.AuroraHeaders {
	return New().WithBaseHeaders("").Build()
}

// NewBaseHeaderWithState 创建带 state 的基础 header。
func NewBaseHeaderWithState(conversationID, deviceID, sessionID, userAgent string) httpclient.AuroraHeaders {
	b := New().WithBaseHeaders(conversationID)
	if deviceID != "" {
		b.WithDeviceID(deviceID)
	}
	if sessionID != "" {
		b.WithSessionID(sessionID)
	}
	if userAgent != "" {
		b.header.Set("User-Agent", userAgent)
	}
	return b.Build()
}
