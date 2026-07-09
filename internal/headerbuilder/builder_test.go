package headerbuilder

import (
	"testing"

	"aurora/internal/accounts"
)

func TestNew(t *testing.T) {
	b := New()
	if b == nil {
		t.Fatal("expected non-nil builder")
	}
}

func TestBuild(t *testing.T) {
	h := New().Build()
	if h == nil {
		t.Fatal("expected non-nil header")
	}
}

func TestWithContentType(t *testing.T) {
	h := New().WithContentType("application/json").Build()
	if h["Content-Type"] != "application/json" {
		t.Fatalf("expected application/json, got %s", h["Content-Type"])
	}
}

func TestWithAccept(t *testing.T) {
	h := New().WithAccept("text/event-stream").Build()
	if h["Accept"] != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", h["Accept"])
	}
}

func TestWithTargetPath(t *testing.T) {
	h := New().WithTargetPath("/backend-api/test").Build()
	if h["X-Openai-Target-Path"] != "/backend-api/test" {
		t.Fatalf("expected /backend-api/test, got %s", h["X-Openai-Target-Path"])
	}
	if h["X-Openai-Target-Route"] != "/backend-api/test" {
		t.Fatalf("expected /backend-api/test, got %s", h["X-Openai-Target-Route"])
	}
}

func TestWithAuth_PaidUser(t *testing.T) {
	acct := &accounts.Account{Type: accounts.TypeFree, Token: "test-token"}
	h := New().WithAuth(acct).Build()
	if h["Authorization"] != "Bearer test-token" {
		t.Fatalf("expected Bearer test-token, got %s", h["Authorization"])
	}
}

func TestWithAuth_FreeUser(t *testing.T) {
	acct := &accounts.Account{Type: accounts.TypeNoAuth, Token: "free-uuid"}
	h := New().WithAuth(acct).Build()
	if h["Oai-Device-Id"] != "free-uuid" {
		t.Fatalf("expected free-uuid, got %s", h["Oai-Device-Id"])
	}
}

func TestWithAuth_NilAccount(t *testing.T) {
	h := New().WithAuth(nil).Build()
	if h["Authorization"] != "" {
		t.Fatalf("expected empty, got %s", h["Authorization"])
	}
}

func TestWithTeamAccount(t *testing.T) {
	acct := &accounts.Account{Type: accounts.TypeFree, Token: "token", TeamUserID: "team-123"}
	h := New().WithTeamAccount(acct).Build()
	if h["Chatgpt-Account-Id"] != "team-123" {
		t.Fatalf("expected team-123, got %s", h["Chatgpt-Account-Id"])
	}
}

func TestWithTeamAccount_Empty(t *testing.T) {
	acct := &accounts.Account{Type: accounts.TypeFree, Token: "token"}
	h := New().WithTeamAccount(acct).Build()
	if h["Chatgpt-Account-Id"] != "" {
		t.Fatalf("expected empty, got %s", h["Chatgpt-Account-Id"])
	}
}

func TestWithConduitToken(t *testing.T) {
	h := New().WithConduitToken("conduit-123").Build()
	if h["X-Conduit-Token"] != "conduit-123" {
		t.Fatalf("expected conduit-123, got %s", h["X-Conduit-Token"])
	}
}

func TestWithConduitToken_Empty(t *testing.T) {
	h := New().WithConduitToken("").Build()
	if h["X-Conduit-Token"] != "" {
		t.Fatalf("expected empty, got %s", h["X-Conduit-Token"])
	}
}

func TestWithTurnTraceID(t *testing.T) {
	h := New().WithTurnTraceID("trace-123").Build()
	if h["X-Oai-Turn-Trace-Id"] != "trace-123" {
		t.Fatalf("expected trace-123, got %s", h["X-Oai-Turn-Trace-Id"])
	}
}

func TestWithSentinelTokens(t *testing.T) {
	sentinelTokens := SentinelTokens{
		TurnStileToken:  "turnstile-123",
		ProofOfWorkToken: "proof-123",
		TurnstileToken:  "turnstile-token-123",
	}
	h := New().WithSentinelTokens(sentinelTokens).Build()
	if h["Openai-Sentinel-Chat-Requirements-Token"] != "turnstile-123" {
		t.Fatalf("expected turnstile-123, got %s", h["Openai-Sentinel-Chat-Requirements-Token"])
	}
	if h["Openai-Sentinel-Proof-Token"] != "proof-123" {
		t.Fatalf("expected proof-123, got %s", h["Openai-Sentinel-Proof-Token"])
	}
	if h["Openai-Sentinel-Turnstile-Token"] != "turnstile-token-123" {
		t.Fatalf("expected turnstile-token-123, got %s", h["Openai-Sentinel-Turnstile-Token"])
	}
}

func TestWithConversationHeaders_MainRequest(t *testing.T) {
	h := New().WithConversationHeaders("/f/conversation").Build()
	if h["Oai-Echo-Logs"] == "" {
		t.Fatal("expected Oai-Echo-Logs for main conversation request")
	}
}

func TestWithConversationHeaders_PrepareRequest(t *testing.T) {
	h := New().WithConversationHeaders("/f/conversation/prepare").Build()
	if h["Oai-Echo-Logs"] != "" {
		t.Fatal("expected no Oai-Echo-Logs for prepare request")
	}
}

func TestNewBaseHeader(t *testing.T) {
	h := NewBaseHeader()
	if h["Origin"] != "https://chatgpt.com" {
		t.Fatalf("expected https://chatgpt.com, got %s", h["Origin"])
	}
	if h["User-Agent"] == "" {
		t.Fatal("expected non-empty User-Agent")
	}
}

func TestBuilderChaining(t *testing.T) {
	acct := &accounts.Account{Type: accounts.TypeFree, Token: "test-token", TeamUserID: "team-456"}
	h := New().
		WithBaseHeaders("conv-123").
		WithContentType("application/json").
		WithAccept("*/*").
		WithTargetPath("/backend-api/test").
		WithAuth(acct).
		WithTeamAccount(acct).
		WithConduitToken("conduit-123").
		WithTurnTraceID("trace-123").
		Build()

	if h["Content-Type"] != "application/json" {
		t.Fatalf("expected application/json, got %s", h["Content-Type"])
	}
	if h["Authorization"] != "Bearer test-token" {
		t.Fatalf("expected Bearer test-token, got %s", h["Authorization"])
	}
	if h["X-Conduit-Token"] != "conduit-123" {
		t.Fatalf("expected conduit-123, got %s", h["X-Conduit-Token"])
	}
	if h["Chatgpt-Account-Id"] != "team-456" {
		t.Fatalf("expected team-456, got %s", h["Chatgpt-Account-Id"])
	}
}
