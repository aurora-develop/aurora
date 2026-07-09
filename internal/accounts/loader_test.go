package accounts

import (
	"os"
	"testing"
)

func TestLoadTokensFromFile(t *testing.T) {
	// 写临时文件
	content := "token1\n# comment\ntoken2:team123\n\ntoken3\n"
	tmpfile := "_test_tokens.txt"
	os.WriteFile(tmpfile, []byte(content), 0644)
	defer os.Remove(tmpfile)

	secrets := LoadTokensFromFile(tmpfile)
	if len(secrets) != 3 {
		t.Fatalf("got %d tokens, want 3", len(secrets))
	}
	if secrets[0].Token != "token1" {
		t.Errorf("secrets[0].Token = %q, want %q", secrets[0].Token, "token1")
	}
	if secrets[1].Token != "token2" {
		t.Errorf("secrets[1].Token = %q, want %q", secrets[1].Token, "token2")
	}
	if secrets[1].TeamID != "team123" {
		t.Errorf("secrets[1].TeamID = %q, want %q", secrets[1].TeamID, "team123")
	}
	if secrets[2].Token != "token3" {
		t.Errorf("secrets[2].Token = %q, want %q", secrets[2].Token, "token3")
	}
}

func TestLoadTokensFromFileMissing(t *testing.T) {
	secrets := LoadTokensFromFile("_nonexistent_file_")
	if secrets == nil || len(secrets) != 0 {
		t.Errorf("missing file should return empty list, got %v", secrets)
	}
}
