package authresolver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"aurora/internal/tokens"

	"github.com/gin-gonic/gin"
)

func setupTestContext(token *tokens.AccessToken) (*gin.Context, *httptest.ResponseRecorder, *Resolver) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest("POST", "/test", nil)
	resolver := NewResolver(token)
	return c, w, resolver
}

func TestResolve_NoAuth(t *testing.T) {
	tokenPool := tokens.NewAccessToken([]*tokens.Secret{
		tokens.NewSecret("test-token"),
	})
	c, _, resolver := setupTestContext(&tokenPool)

	result := resolver.Resolve(c, ResolveRequest{
		NeedsPaid:         false,
		AllowFallbackPaid: false,
		ProxyURL:          "",
	})

	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}
	if result.Secret == nil {
		t.Error("expected non-nil secret")
	}
}

func TestResolve_WithBearerToken(t *testing.T) {
	tokenPool := tokens.NewAccessToken([]*tokens.Secret{
		tokens.NewSecret("pool-token"),
	})
	c, _, resolver := setupTestContext(&tokenPool)
	c.Request.Header.Set("Authorization", "Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.test")

	result := resolver.Resolve(c, ResolveRequest{
		NeedsPaid:         false,
		AllowFallbackPaid: false,
		ProxyURL:          "",
	})

	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}
	if result.Secret == nil {
		t.Error("expected non-nil secret")
	}
	if result.Secret.Token != "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.test" {
		t.Errorf("expected token from bearer, got %q", result.Secret.Token)
	}
}

func TestResolve_WithUUID(t *testing.T) {
	tokenPool := tokens.NewAccessToken([]*tokens.Secret{
		tokens.NewSecret("pool-token"),
	})
	c, _, resolver := setupTestContext(&tokenPool)
	c.Request.Header.Set("Authorization", "Bearer 550e8400-e29b-41d4-a716-446655440000")

	result := resolver.Resolve(c, ResolveRequest{
		NeedsPaid:         false,
		AllowFallbackPaid: false,
		ProxyURL:          "",
	})

	if result.Error != nil {
		t.Errorf("unexpected error: %v", result.Error)
	}
	if result.Secret == nil {
		t.Error("expected non-nil secret")
	}
	if !result.Secret.IsFree {
		t.Error("expected free secret for UUID token")
	}
}

func TestResolve_NeedsPaid_NoPaidAvailable(t *testing.T) {
	tokenPool := tokens.NewAccessToken([]*tokens.Secret{
		tokens.NewSecretWithFree("free-uuid"),
	})
	c, _, resolver := setupTestContext(&tokenPool)

	result := resolver.Resolve(c, ResolveRequest{
		NeedsPaid:         true,
		AllowFallbackPaid: false,
		ProxyURL:          "",
	})

	// When needsPaid=true and no paid secret available, secret should be nil
	if result.Secret != nil {
		t.Error("expected nil secret when no paid account available")
	}
}

func TestSplitAuthorizationTokenAndTeam(t *testing.T) {
	tests := []struct {
		input       string
		wantToken   string
		wantTeamID  string
	}{
		{"Bearer abc123", "abc123", ""},
		{"Bearer abc123,team-456", "abc123", "team-456"},
		{"abc123", "abc123", ""},
		{"", "", ""},
	}
	for _, tt := range tests {
		token, teamID := splitAuthorizationTokenAndTeam(tt.input)
		if token != tt.wantToken {
			t.Errorf("splitAuthorizationTokenAndTeam(%q) token = %q, want %q", tt.input, token, tt.wantToken)
		}
		if teamID != tt.wantTeamID {
			t.Errorf("splitAuthorizationTokenAndTeam(%q) teamID = %q, want %q", tt.input, teamID, tt.wantTeamID)
		}
	}
}

func TestIsUUID(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"550e8400-e29b-41d4-a716-446655440000", true},
		{"not-a-uuid", false},
		{"", false},
	}
	for _, tt := range tests {
		result := isUUID(tt.input)
		if result != tt.expected {
			t.Errorf("isUUID(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}
