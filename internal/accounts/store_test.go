package accounts

import (
	"os"
	"testing"
)

func TestJSONStoreSaveAndLoad(t *testing.T) {
	path := "_test_accounts.json"
	defer os.Remove(path)

	store := NewJSONStore(path)

	// 加载空文件
	accounts, err := store.Load()
	if err != nil {
		t.Fatalf("Load empty: %v", err)
	}
	if len(accounts) != 0 {
		t.Fatalf("expected empty list, got %d accounts", len(accounts))
	}

	// 保存
	acct := NewAccount("test-1", TypePUID, "test-token")
	acct.Status = StatusActive
	if err := store.Save([]*Account{acct}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// 重新加载
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("Load after save: %v", err)
	}
	if len(loaded) != 1 || loaded[0].ID != "test-1" || loaded[0].Token != "test-token" {
		t.Errorf("Load mismatch: got %+v", loaded[0])
	}
}
