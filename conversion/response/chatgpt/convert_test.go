package chatgpt

import (
	"encoding/json"
	"testing"

	"aurora/typings"
	chatgpt_types "aurora/typings/chatgpt"
)

func TestConvertToStringUsesRequestModel(t *testing.T) {
	previous := &typings.StringStruct{}
	response := chatgpt_types.ChatGPTResponse{
		Message: chatgpt_types.Message{
			Author: chatgpt_types.Author{Role: "assistant"},
			Content: chatgpt_types.Content{
				Parts: []interface{}{"hello"},
			},
		},
	}

	data := ConvertToString(&response, previous, true, "gpt-5-5-pro")

	var event struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal([]byte(data[len("data: "):len(data)-2]), &event); err != nil {
		t.Fatalf("invalid SSE JSON: %v", err)
	}
	if event.Model != "gpt-5-5-pro" {
		t.Fatalf("model = %q, want request model", event.Model)
	}
}

func TestConvertToStringWaitsSourceMarkerForAnyModel(t *testing.T) {
	previous := &typings.StringStruct{Text: "answer"}
	response := chatgpt_types.ChatGPTResponse{
		Message: chatgpt_types.Message{
			Author: chatgpt_types.Author{Role: "assistant"},
			Content: chatgpt_types.Content{
				Parts: []interface{}{"answer【"},
			},
			Metadata: chatgpt_types.Metadata{ModelSlug: "gpt-5-5-pro"},
		},
	}

	data := ConvertToString(&response, previous, false, "gpt-5-5-pro")

	if data != "【" {
		t.Fatalf("data = %q, want source marker only", data)
	}
}
