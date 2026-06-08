package official

import (
	"encoding/json"
	"testing"
)

func TestNewChatCompletionWithMetadata(t *testing.T) {
	response := NewChatCompletionWithMetadata(
		"hello",
		1,
		2,
		"gpt-4o",
		"conv-xxx",
		[]map[string]interface{}{
			{
				"event":      "artifact",
				"kind":       "generated_image",
				"slot_index": 1,
				"url":        "http://example.test/image.png",
			},
			{
				"event":      "artifact_slot_final",
				"kind":       "generated_image",
				"slot_index": 1,
			},
		},
	)

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if raw["conversation_id"] != "conv-xxx" {
		t.Fatalf("conversation_id = %#v, want conv-xxx", raw["conversation_id"])
	}
	choices := raw["choices"].([]interface{})
	message := choices[0].(map[string]interface{})["message"].(map[string]interface{})
	if message["content"] != "hello" {
		t.Fatalf("message content = %#v, want hello", message["content"])
	}
	sentinel := raw["sentinel"].([]interface{})
	if len(sentinel) != 2 {
		t.Fatalf("sentinel count = %d, want 2", len(sentinel))
	}
	if sentinel[0].(map[string]interface{})["event"] != "artifact" {
		t.Fatalf("first sentinel = %#v, want artifact", sentinel[0])
	}
}
