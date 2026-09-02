package chatgpt

import (
	"aurora/internal/accounts"
	"testing"
)

func TestEnsureSOTokenNilReceiver(t *testing.T) {
	var turnstile *TurnStile
	if got := turnstile.ensureSOToken("device"); got != "" {
		t.Fatalf("nil receiver token = %q, want empty", got)
	}
}

func TestSentinelDeviceIDUsesStablePriority(t *testing.T) {
	account := &accounts.Account{
		Type:  accounts.TypePUID,
		Token: "access-token",
		Fingerprint: accounts.BrowserFingerprint{
			OaiDeviceID: "account-device",
		},
	}
	state := &ChatClientState{DeviceID: "state-device"}

	if got := sentinelDeviceID(account, state); got != "state-device" {
		t.Fatalf("state device ID = %q, want state-device", got)
	}
	if got := sentinelDeviceID(account, nil); got != "account-device" {
		t.Fatalf("account device ID = %q, want account-device", got)
	}

	account.Fingerprint.OaiDeviceID = ""
	account.Type = accounts.TypeNoAuth
	account.Token = "free-device"
	if got := sentinelDeviceID(account, nil); got != "free-device" {
		t.Fatalf("free account device ID = %q, want free-device", got)
	}
}
