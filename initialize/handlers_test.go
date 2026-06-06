package initialize

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSplitAuthorizationTokenAndTeam(t *testing.T) {
	token, teamID := splitAuthorizationTokenAndTeam("Bearer access-token,team-account-id")
	if token != "access-token" {
		t.Fatalf("token = %q, want %q", token, "access-token")
	}
	if teamID != "team-account-id" {
		t.Fatalf("teamID = %q, want %q", teamID, "team-account-id")
	}

	token, teamID = splitAuthorizationTokenAndTeam("Bearer access-token")
	if token != "access-token" {
		t.Fatalf("token = %q, want %q", token, "access-token")
	}
	if teamID != "" {
		t.Fatalf("teamID = %q, want empty", teamID)
	}
}

func TestTeamAccountIDFromRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	headerContext := testContextWithHeaders(map[string]string{
		"Authorization":      "Bearer access-token,team-from-auth",
		"ChatGPT-Account-ID": "team-from-header",
	})
	if got := teamAccountIDFromRequest(headerContext); got != "team-from-header" {
		t.Fatalf("teamAccountIDFromRequest = %q, want %q", got, "team-from-header")
	}

	authContext := testContextWithHeaders(map[string]string{
		"Authorization": "Bearer access-token,team-from-auth",
	})
	if got := teamAccountIDFromRequest(authContext); got != "team-from-auth" {
		t.Fatalf("teamAccountIDFromRequest = %q, want %q", got, "team-from-auth")
	}

	emptyContext := testContextWithHeaders(map[string]string{
		"Authorization": "Bearer access-token",
	})
	if got := teamAccountIDFromRequest(emptyContext); got != "" {
		t.Fatalf("teamAccountIDFromRequest = %q, want empty", got)
	}
}

func TestAuthorizationTokenAndTeam(t *testing.T) {
	gin.SetMode(gin.TestMode)

	headerContext := testContextWithHeaders(map[string]string{
		"Authorization":      "Bearer refresh-token",
		"ChatGPT-Account-ID": "team-from-header",
	})
	token, teamID, hasAuthorizationTeamID := authorizationTokenAndTeam(headerContext)
	if token != "refresh-token" {
		t.Fatalf("token = %q, want %q", token, "refresh-token")
	}
	if teamID != "team-from-header" {
		t.Fatalf("teamID = %q, want %q", teamID, "team-from-header")
	}
	if hasAuthorizationTeamID {
		t.Fatalf("hasAuthorizationTeamID = true, want false")
	}

	authContext := testContextWithHeaders(map[string]string{
		"Authorization": "Bearer refresh-token,team-from-auth",
	})
	token, teamID, hasAuthorizationTeamID = authorizationTokenAndTeam(authContext)
	if token != "refresh-token" {
		t.Fatalf("token = %q, want %q", token, "refresh-token")
	}
	if teamID != "team-from-auth" {
		t.Fatalf("teamID = %q, want %q", teamID, "team-from-auth")
	}
	if !hasAuthorizationTeamID {
		t.Fatalf("hasAuthorizationTeamID = false, want true")
	}
}

func testContextWithHeaders(headers map[string]string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	c.Request = req
	return c
}
