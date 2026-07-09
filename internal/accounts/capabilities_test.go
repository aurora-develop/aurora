package accounts

import (
	"testing"
)

func TestAccountTypeSatisfies(t *testing.T) {
	tests := []struct {
		acctType AccountType
		cap      Capability
		want     bool
	}{
		{TypePUID, CapChat, true},
		{TypePUID, CapImageGenerate, true},
		{TypePUID, CapTTS, true},
		{TypeFree, CapChat, true},
		{TypeFree, CapImageGenerate, true},
		{TypeFree, CapTTS, true},
		{TypeNoAuth, CapChat, true},
		{TypeNoAuth, CapImageGenerate, false},
		{TypeNoAuth, CapTTS, false},
	}

	for _, tt := range tests {
		got := tt.acctType.Satisfies(tt.cap)
		if got != tt.want {
			t.Errorf("%s.Satisfies(%s) = %v, want %v", tt.acctType, tt.cap.Name, got, tt.want)
		}
	}
}
