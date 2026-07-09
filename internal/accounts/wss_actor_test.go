package accounts

import (
	"testing"
	"time"
)

func TestWSSActorStartStop(t *testing.T) {
	acct := NewAccount("test", TypePUID, "token")
	actor := NewWSSActor(acct)

	actor.Start()
	time.Sleep(50 * time.Millisecond)
	actor.Stop()
	// 确认不 panic 即可
}

func TestWSSActorRestart(t *testing.T) {
	acct := NewAccount("test", TypePUID, "token")
	actor := NewWSSActor(acct)

	actor.Start()
	actor.Stop()

	// 重新启动应该正常工作
	actor.Start()
	actor.Stop()
}
