package chatgpt

import (
	"encoding/json"
	"testing"
)

func TestNewChatGPTRequestMatchesWebConversationShape(t *testing.T) {
	request := NewChatGPTRequest()
	request.Model = "gpt-5-5-pro"
	request.AddMessage("user", "hello")

	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	if raw["parent_message_id"] != "client-created-root" {
		t.Fatalf("parent_message_id = %#v, want client-created-root", raw["parent_message_id"])
	}
	if raw["model"] != "gpt-5-5-pro" {
		t.Fatalf("model = %#v, want gpt-5-5-pro", raw["model"])
	}
	for _, key := range []string{"client_prepare_state", "force_use_sse", "force_rate_limit", "reset_rate_limits", "suggestions", "enable_message_followups", "supports_buffering", "supported_encodings", "client_contextual_info"} {
		if _, ok := raw[key]; ok {
			t.Fatalf("request unexpectedly includes %s: %s", key, string(data))
		}
	}
}

func TestAddMessageCopiesRequestSystemHintsToUserMetadata(t *testing.T) {
	request := NewChatGPTRequest()
	request.SystemHints = []string{"reason"}
	request.AddMessage("user", "hello")

	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	messages := raw["messages"].([]interface{})
	metadata := messages[0].(map[string]interface{})["metadata"].(map[string]interface{})
	hints := metadata["system_hints"].([]interface{})
	if len(hints) != 1 || hints[0] != "reason" {
		t.Fatalf("message system_hints = %#v, want [reason]", metadata["system_hints"])
	}
}
