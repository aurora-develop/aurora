package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"aurora/internal/accounts"
)

// chdirTemp 把工作目录切到临时目录，persistRotatedSessionToken 用的是相对路径。
func chdirTemp(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	return dir
}

func readTokenFile(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "session_tokens.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestPersistRotatedSessionTokenReplacesOnlyMatchingLine(t *testing.T) {
	dir := chdirTemp(t)
	if err := os.WriteFile("session_tokens.txt", []byte("aaa\nbbb\nccc\n"), 0644); err != nil {
		t.Fatal(err)
	}

	persistRotatedSessionToken("bbb", "bbb-new", "")

	if got, want := readTokenFile(t, dir), "aaa\nbbb-new\nccc\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPersistRotatedSessionTokenPreservesTeamID(t *testing.T) {
	dir := chdirTemp(t)
	if err := os.WriteFile("session_tokens.txt", []byte("aaa:team-a\nbbb\n"), 0644); err != nil {
		t.Fatal(err)
	}

	persistRotatedSessionToken("aaa", "aaa-new", "team-a")

	if got, want := readTokenFile(t, dir), "aaa-new:team-a\nbbb\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// 旧实现用 HasPrefix，短 token 会错误命中以它为前缀的另一行。
func TestPersistRotatedSessionTokenDoesNotMatchOnPrefix(t *testing.T) {
	dir := chdirTemp(t)
	if err := os.WriteFile("session_tokens.txt", []byte("abcdef\nabc\n"), 0644); err != nil {
		t.Fatal(err)
	}

	persistRotatedSessionToken("abc", "abc-new", "")

	if got, want := readTokenFile(t, dir), "abcdef\nabc-new\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPersistRotatedSessionTokenAppendsWhenAbsent(t *testing.T) {
	dir := chdirTemp(t)
	if err := os.WriteFile("session_tokens.txt", []byte("aaa\n"), 0644); err != nil {
		t.Fatal(err)
	}

	persistRotatedSessionToken("zzz", "zzz-new", "")

	if got, want := readTokenFile(t, dir), "aaa\nzzz-new\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestPersistRotatedSessionTokenCreatesFileWhenMissing(t *testing.T) {
	dir := chdirTemp(t)

	persistRotatedSessionToken("old", "new", "team-x")

	if got, want := readTokenFile(t, dir), "new:team-x\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// 轮换写回后重新加载，应能读到新令牌（重启持久性）。
func TestRotatedTokenSurvivesReload(t *testing.T) {
	chdirTemp(t)
	if err := os.WriteFile("session_tokens.txt", []byte("old:team-a\n"), 0644); err != nil {
		t.Fatal(err)
	}

	persistRotatedSessionToken("old", "rotated", "team-a")

	loaded := accounts.LoadTokensFromFile("session_tokens.txt")
	if len(loaded) != 1 {
		t.Fatalf("expected 1 token, got %d", len(loaded))
	}
	if loaded[0].Token != "rotated" {
		t.Errorf("token = %q, want %q", loaded[0].Token, "rotated")
	}
	if loaded[0].TeamID != "team-a" {
		t.Errorf("teamID = %q, want %q", loaded[0].TeamID, "team-a")
	}
}

func TestExchangeSessionTokenNoopWithoutToken(t *testing.T) {
	acct := &accounts.Account{}
	if exchangeSessionToken(acct) {
		t.Error("expected false for empty session token")
	}
}
