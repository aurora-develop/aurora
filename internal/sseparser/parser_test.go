package sseparser

import (
	"testing"

	chatgpt_types "aurora/typings/chatgpt"
)

func TestDataPayloads_SingleDataLine(t *testing.T) {
	line := "data: {\"key\":\"value\"}\n\n"
	payloads := DataPayloads(line)
	if len(payloads) != 1 {
		t.Fatalf("expected 1 payload, got %d", len(payloads))
	}
	if payloads[0] != `{"key":"value"}` {
		t.Errorf("unexpected payload: %s", payloads[0])
	}
}

func TestDataPayloads_MultipleDataLines(t *testing.T) {
	line := "data: {\"a\":1}\ndata: {\"b\":2}\n\n"
	payloads := DataPayloads(line)
	if len(payloads) != 2 {
		t.Fatalf("expected 2 payloads, got %d", len(payloads))
	}
}

func TestDataPayloads_DoneMarker(t *testing.T) {
	line := "data: [DONE]\n\n"
	payloads := DataPayloads(line)
	if len(payloads) != 1 || payloads[0] != "[DONE]" {
		t.Fatalf("expected [DONE], got %v", payloads)
	}
}

func TestDataPayloads_EmptyLine(t *testing.T) {
	payloads := DataPayloads("")
	if len(payloads) != 0 {
		t.Fatalf("expected 0 payloads, got %d", len(payloads))
	}
}

func TestSplitDataPayloads_JSON(t *testing.T) {
	payloads := SplitDataPayloads(`{"key":"value"}`)
	if len(payloads) != 1 || payloads[0] != `{"key":"value"}` {
		t.Fatalf("unexpected: %v", payloads)
	}
}

func TestSplitDataPayloads_Done(t *testing.T) {
	payloads := SplitDataPayloads("[DONE]")
	if len(payloads) != 1 || payloads[0] != "[DONE]" {
		t.Fatalf("unexpected: %v", payloads)
	}
}

func TestEventName_Present(t *testing.T) {
	line := "event: response.created\ndata: {}\n\n"
	name, ok := EventName(line)
	if !ok || name != "response.created" {
		t.Fatalf("expected response.created, got %q (ok=%v)", name, ok)
	}
}

func TestEventName_Absent(t *testing.T) {
	line := "data: {}\n\n"
	name, ok := EventName(line)
	if ok {
		t.Fatalf("expected no event, got %q", name)
	}
}

func TestNumberToInt64_Float64(t *testing.T) {
	v, ok := NumberToInt64(float64(42.5))
	if !ok || v != 42 {
		t.Fatalf("expected 42, got %d (ok=%v)", v, ok)
	}
}

func TestNumberToInt64_Int(t *testing.T) {
	v, ok := NumberToInt64(int(100))
	if !ok || v != 100 {
		t.Fatalf("expected 100, got %d (ok=%v)", v, ok)
	}
}

func TestNumberToInt64_String(t *testing.T) {
	_, ok := NumberToInt64("not a number")
	if ok {
		t.Fatal("expected false for string input")
	}
}

func TestChannelFromValue_Nested(t *testing.T) {
	value := map[string]interface{}{
		"delta": map[string]interface{}{
			"channel": "test-channel",
		},
	}
	ch := ChannelFromValue(value)
	if ch != "test-channel" {
		t.Fatalf("expected test-channel, got %q", ch)
	}
}

func TestChannelFromValue_Empty(t *testing.T) {
	ch := ChannelFromValue(map[string]interface{}{})
	if ch != "" {
		t.Fatalf("expected empty, got %q", ch)
	}
}

func TestFirstStringPart(t *testing.T) {
	parts := []interface{}{"hello", "world"}
	if s := FirstStringPart(parts); s != "hello" {
		t.Fatalf("expected hello, got %q", s)
	}
	if s := FirstStringPart(nil); s != "" {
		t.Fatalf("expected empty, got %q", s)
	}
}

func TestNormalizeContentDelta(t *testing.T) {
	if d := NormalizeContentDelta("", "hello"); d != "hello" {
		t.Fatalf("expected hello, got %q", d)
	}
	if d := NormalizeContentDelta("hel", "hello"); d != "lo" {
		t.Fatalf("expected lo, got %q", d)
	}
	if d := NormalizeContentDelta("hello", ""); d != "" {
		t.Fatalf("expected empty, got %q", d)
	}
}

func TestIsUsableConversationResponse(t *testing.T) {
	// Empty response is not usable
	if IsUsableConversationResponse(chatgpt_types.ChatGPTResponse{}) {
		t.Fatal("empty response should not be usable")
	}
}
