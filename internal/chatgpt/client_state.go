package chatgpt

import (
	"math"
	"time"

	browser "github.com/EDDYCJY/fake-useragent"
	"github.com/google/uuid"

	chatgpt_types "aurora/typings/chatgpt"
)

type ChatClientState struct {
	DeviceID        string
	SessionID       string
	StartTime       time.Time
	ConversationID  string
	ParentMessageID string
	TurnCount       int
	UserAgent       string
}

func NewChatClientState() *ChatClientState {
	return &ChatClientState{
		DeviceID:        uuid.NewString(),
		SessionID:       uuid.NewString(),
		StartTime:       time.Now(),
		ParentMessageID: "client-created-root",
		UserAgent:       browser.Random(),
	}
}

func (s *ChatClientState) TimeSinceLoadedSeconds() int {
	if s == nil || s.StartTime.IsZero() {
		return 0
	}
	seconds := math.Round(float64(time.Since(s.StartTime).Milliseconds()) / 1000.0)
	if seconds < 0 {
		return 0
	}
	return int(seconds)
}

func (s *ChatClientState) ApplyToRequest(request *chatgpt_types.ChatGPTRequest) {
	if s == nil || request == nil {
		return
	}
	if request.ParentMessageID == "" || request.ParentMessageID == "client-created-root" {
		request.ParentMessageID = s.ParentMessageID
	}
	if request.ConversationID == "" && s.ConversationID != "" {
		request.ConversationID = s.ConversationID
	}
	ensureClientContextualInfo(request)
	request.ClientContextualInfo["time_since_loaded"] = s.TimeSinceLoadedSeconds()
}

func (s *ChatClientState) ClientContextualInfo() map[string]interface{} {
	request := chatgpt_types.ChatGPTRequest{}
	ensureClientContextualInfo(&request)
	if s != nil {
		request.ClientContextualInfo["time_since_loaded"] = s.TimeSinceLoadedSeconds()
	} else {
		request.ClientContextualInfo["time_since_loaded"] = 0
	}
	return request.ClientContextualInfo
}

func (s *ChatClientState) NoteTurnResult(conversationID, parentMessageID string) {
	if s == nil {
		return
	}
	if conversationID != "" {
		s.ConversationID = conversationID
	}
	if parentMessageID != "" {
		s.ParentMessageID = parentMessageID
	}
	s.TurnCount++
}

func ensureClientContextualInfo(request *chatgpt_types.ChatGPTRequest) {
	if request.ClientContextualInfo == nil {
		request.ClientContextualInfo = map[string]interface{}{}
	}
	if _, ok := request.ClientContextualInfo["is_dark_mode"]; !ok {
		request.ClientContextualInfo["is_dark_mode"] = false
	}
	if _, ok := request.ClientContextualInfo["page_height"]; !ok {
		request.ClientContextualInfo["page_height"] = 1014
	}
	if _, ok := request.ClientContextualInfo["page_width"]; !ok {
		request.ClientContextualInfo["page_width"] = 1055
	}
	if _, ok := request.ClientContextualInfo["pixel_ratio"]; !ok {
		request.ClientContextualInfo["pixel_ratio"] = 1
	}
	if _, ok := request.ClientContextualInfo["screen_height"]; !ok {
		request.ClientContextualInfo["screen_height"] = 1080
	}
	if _, ok := request.ClientContextualInfo["screen_width"]; !ok {
		request.ClientContextualInfo["screen_width"] = 1920
	}
	request.ClientContextualInfo["app_name"] = "chatgpt.com"
}

func requestWithClientState(request chatgpt_types.ChatGPTRequest, state *ChatClientState) chatgpt_types.ChatGPTRequest {
	if state != nil {
		state.ApplyToRequest(&request)
	}
	return request
}
