package chatgpt

import (
	"strings"

	"github.com/google/uuid"

	"aurora/httpclient"
	"aurora/internal/accounts"
	"aurora/internal/headerbuilder"
)

// conversationURL 根据账号类型返回后端 API URL 和 target path。
func conversationURL(account *accounts.Account, path string) (string, string) {
	if account != nil && account.Type == accounts.TypeNoAuth {
		return strings.Replace(BaseURL, "backend-api", "backend-anon", 1) + path, "/backend-anon" + path
	}
	return BaseURL + path, "/backend-api" + path
}

// sentinelURL 根据账号类型返回 sentinel API URL 和 target path。
func sentinelURL(account *accounts.Account, path string) (string, string) {
	if account != nil && account.Type == accounts.TypeNoAuth {
		return strings.Replace(BaseURL, "backend-api", "backend-anon", 1) + path, "/backend-anon" + path
	}
	return BaseURL + path, "/backend-api" + path
}

// conversationHeaders 创建对话请求的 header（无 state）。
func conversationHeaders(account *accounts.Account, chatToken *TurnStile, accept, targetPath, conduitToken, turnTraceID string) httpclient.AuroraHeaders {
	return conversationHeadersWithState(account, chatToken, accept, targetPath, conduitToken, turnTraceID, nil)
}

// conversationHeadersWithState 创建对话请求的 header（带 state）。
func conversationHeadersWithState(account *accounts.Account, chatToken *TurnStile, accept, targetPath, conduitToken, turnTraceID string, state *ChatClientState) httpclient.AuroraHeaders {
	conversationID := ""
	deviceID := account.Fingerprint.OaiDeviceID
	if deviceID == "" {
		deviceID = oaiDeviceID
	}
	sessionID := account.Fingerprint.OaiSessionID
	if sessionID == "" {
		sessionID = oaiSessionID
	}
	ua := account.Fingerprint.UserAgent
	if state != nil {
		if state.ConversationID != "" {
			conversationID = state.ConversationID
		}
		if state.DeviceID != "" {
			deviceID = state.DeviceID
		}
		if state.SessionID != "" {
			sessionID = state.SessionID
		}
		if state.UserAgent != "" {
			ua = state.UserAgent
		}
	}

	b := headerbuilder.New().
		WithBaseHeaders(conversationID).
		WithDeviceID(deviceID).
		WithSessionID(sessionID).
		WithUserAgent(ua).
		WithAccept(accept).
		WithContentType("application/json").
		WithTargetPath(targetPath).
		WithTurnTraceID(turnTraceID).
		WithConduitToken(conduitToken).
		WithConversationHeaders(targetPath).
		WithAuth(account).
		WithCookies(account).
		WithTeamAccount(account)

	if chatToken != nil {
		soToken := chatToken.ensureSOToken(soDeviceIDFor(account))
		b.WithSentinelTokens(headerbuilder.SentinelTokens{
			TurnStileToken:               chatToken.TurnStileToken,
			ChatRequirementsPrepareToken: chatToken.ChatRequirementsPrepareToken,
			ProofOfWorkToken:             chatToken.ProofOfWorkToken,
			TurnstileToken:               chatToken.TurnstileToken,
			SOToken:                      soToken,
		})
	}

	return b.Build()
}

// sentinelHeader 创建 sentinel 请求的 header（无 state）。
func sentinelHeader(account *accounts.Account, targetPath string) httpclient.AuroraHeaders {
	return sentinelHeaderWithState(account, targetPath, nil)
}

// sentinelHeaderWithState 创建 sentinel 请求的 header（带 state）。
func sentinelHeaderWithState(account *accounts.Account, targetPath string, state *ChatClientState) httpclient.AuroraHeaders {
	conversationID := ""
	deviceID := account.Fingerprint.OaiDeviceID
	if deviceID == "" {
		deviceID = oaiDeviceID
	}
	sessionID := account.Fingerprint.OaiSessionID
	if sessionID == "" {
		sessionID = oaiSessionID
	}
	ua := account.Fingerprint.UserAgent
	if state != nil {
		if state.ConversationID != "" {
			conversationID = state.ConversationID
		}
		if state.DeviceID != "" {
			deviceID = state.DeviceID
		}
		if state.SessionID != "" {
			sessionID = state.SessionID
		}
		if state.UserAgent != "" {
			ua = state.UserAgent
		}
	}
	b := headerbuilder.New().
		WithBaseHeaders(conversationID).
		WithDeviceID(deviceID).
		WithSessionID(sessionID).
		WithUserAgent(ua).
		WithContentType("application/json").
		WithTargetPath(targetPath).
		WithAuth(account).
		WithTeamAccount(account)
	return b.Build()
}

// imageConversationHeaders 创建图片对话请求的 header（无 state）。
func imageConversationHeaders(account *accounts.Account, turnStile *TurnStile, conduitToken, accept string) httpclient.AuroraHeaders {
	return imageConversationHeadersWithState(account, turnStile, conduitToken, accept, nil)
}

// imageConversationHeadersWithState 创建图片对话请求的 header（带 state）。
func imageConversationHeadersWithState(account *accounts.Account, turnStile *TurnStile, conduitToken, accept string, state *ChatClientState) httpclient.AuroraHeaders {
	conversationID := ""
	deviceID := account.Fingerprint.OaiDeviceID
	if deviceID == "" {
		deviceID = oaiDeviceID
	}
	sessionID := account.Fingerprint.OaiSessionID
	if sessionID == "" {
		sessionID = oaiSessionID
	}
	ua := account.Fingerprint.UserAgent
	if state != nil {
		if state.ConversationID != "" {
			conversationID = state.ConversationID
		}
		if state.DeviceID != "" {
			deviceID = state.DeviceID
		}
		if state.SessionID != "" {
			sessionID = state.SessionID
		}
		if state.UserAgent != "" {
			ua = state.UserAgent
		}
	}

	b := headerbuilder.New().
		WithBaseHeaders(conversationID).
		WithDeviceID(deviceID).
		WithSessionID(sessionID).
		WithUserAgent(ua).
		WithContentType("application/json").
		WithAccept(accept).
		WithConduitToken(conduitToken).
		WithAuth(account).
		WithCookies(account).
		WithTeamAccount(account).
		WithSentinelTokens(headerbuilder.SentinelTokens{
			TurnStileToken:   turnStile.TurnStileToken,
			ProofOfWorkToken: turnStile.ProofOfWorkToken,
			TurnstileToken:   turnStile.TurnstileToken,
		})

	if accept == "text/event-stream" {
		b.WithTurnTraceID(uuid.NewString())
	}

	return b.Build()
}

// conversationFetchHeaders 创建查询对话的 header。
func conversationFetchHeaders(account *accounts.Account) httpclient.AuroraHeaders {
	header := baseHeaderFromAccount(account)
	header.Set("Accept", "application/json")
	header.Set("Content-Type", "application/json")
	if account.Token != "" {
		header.Set("Authorization", "Bearer "+account.Token)
	}
	if account.PUID != "" {
		header.Set("Cookie", "_puid="+account.PUID+";")
	}
	setTeamAccountHeader(header, account)
	return header
}
