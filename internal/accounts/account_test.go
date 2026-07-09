package accounts

import (
	"testing"
)

func TestAccountTypeStrings(t *testing.T) {
	if TypeNoAuth.String() != "noauth" {
		t.Errorf("TypeNoAuth = %q, want %q", TypeNoAuth.String(), "noauth")
	}
	if TypeFree.String() != "free" {
		t.Errorf("TypeFree = %q, want %q", TypeFree.String(), "free")
	}
	if TypePUID.String() != "puid" {
		t.Errorf("TypePUID = %q, want %q", TypePUID.String(), "puid")
	}
}

func TestAccountStatusStrings(t *testing.T) {
	if StatusActive.String() != "active" {
		t.Errorf("StatusActive = %q, want %q", StatusActive.String(), "active")
	}
}
