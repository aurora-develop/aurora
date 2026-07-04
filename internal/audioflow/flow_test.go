package audioflow

import (
	"testing"
)

func TestNormalizeFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"mp3", "mp3"},
		{"opus", "opus"},
		{"aac", "aac"},
		{"flac", "aac"},
		{"wav", "aac"},
		{"pcm", "aac"},
		{"", "aac"},
		{"unknown", "aac"},
	}
	for _, tt := range tests {
		result := NormalizeFormat(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeFormat(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeVoice(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"alloy", "cove"},
		{"ash", "fathom"},
		{"coral", "vale"},
		{"echo", "ember"},
		{"fable", "breeze"},
		{"onyx", "orbit"},
		{"nova", "maple"},
		{"sage", "glimmer"},
		{"shimmer", "juniper"},
		{"", "cove"},
		{"unknown", "cove"},
	}
	for _, tt := range tests {
		result := NormalizeVoice(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeVoice(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestContentTypeForFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"mp3", "audio/mpeg"},
		{"opus", "audio/ogg"},
		{"aac", "audio/aac"},
		{"unknown", "audio/aac"},
	}
	for _, tt := range tests {
		result := ContentTypeForFormat(tt.input)
		if result != tt.expected {
			t.Errorf("ContentTypeForFormat(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestValidateTranscriptionFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"json", true},
		{"text", true},
		{"verbose_json", true},
		{"unknown", false},
		{"", false},
	}
	for _, tt := range tests {
		_, ok := ValidateTranscriptionFormat(tt.input)
		if ok != tt.expected {
			t.Errorf("ValidateTranscriptionFormat(%q) = %v, want %v", tt.input, ok, tt.expected)
		}
	}
}
