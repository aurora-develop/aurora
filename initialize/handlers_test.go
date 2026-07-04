package initialize

import (
	"aurora/internal/httpstream"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestWriteChatCompletionStreamDoneAddsStopBeforeDone(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)

	httpstream.WriteChatCompletionDone(c, false, "auto", "conv-xxx")

	lines := sseDataLines(writer.Body.String())
	if len(lines) != 2 {
		t.Fatalf("data line count = %d, want 2; output: %s", len(lines), writer.Body.String())
	}
	var stopChunk map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &stopChunk); err != nil {
		t.Fatalf("invalid stop chunk: %v", err)
	}
	if stopChunk["conversation_id"] != "conv-xxx" {
		t.Fatalf("conversation_id = %#v, want conv-xxx", stopChunk["conversation_id"])
	}
	choices := stopChunk["choices"].([]interface{})
	if choices[0].(map[string]interface{})["finish_reason"] != "stop" {
		t.Fatalf("finish_reason = %#v, want stop", choices[0].(map[string]interface{})["finish_reason"])
	}
	if lines[1] != "[DONE]" {
		t.Fatalf("last data line = %q, want [DONE]", lines[1])
	}
}

func TestWriteChatCompletionStreamDoneSkipsDuplicateStop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	writer := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(writer)

	httpstream.WriteChatCompletionDone(c, true, "auto", "conv-xxx")

	lines := sseDataLines(writer.Body.String())
	if len(lines) != 1 || lines[0] != "[DONE]" {
		t.Fatalf("data lines = %#v, want only [DONE]", lines)
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

func sseDataLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		lines = append(lines, strings.TrimPrefix(line, "data: "))
	}
	return lines
}
