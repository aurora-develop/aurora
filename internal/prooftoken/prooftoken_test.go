package prooftoken

import (
	"strings"
	"testing"
)

// TestSolveProofOfWork_FallbackFormat 验证 fallback 格式对齐 SDK:
//   "gAAAAAB" + ErrorPrefix + base64(JSON.stringify("e")) + "~S"
//
// 期望 token 形如: gAAAAABwQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4DImUi~S
//   - gAAAAAB                            PrefixProof
//   - wQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4D    ErrorPrefix (32 chars)
//   - ImUi                               base64(JSON.stringify("e")) = base64("\"e\"")
//   - ~S                                 Suffix
func TestSolveProofOfWork_FallbackFormat(t *testing.T) {
	// 用空 seed/empty difficulty 走短路,不应进 fallback
	tok := (&Config{}).SolveProofOfWork("", "")
	if tok != PrefixProof+Suffix {
		t.Fatalf("empty inputs should short-circuit to %q, got %q", PrefixProof+Suffix, tok)
	}

	// 真正触发 500k 循环 + fallback — 用极难 difficulty 强制 fallback
	// (5e5 次循环在不命中条件下大约几秒;这里测 BuildFailToken 直接验证格式)
	tok = BuildFailToken("")
	if !strings.HasPrefix(tok, PrefixProof+ErrorPrefix) {
		t.Fatalf("BuildFailToken prefix wrong: %q", tok)
	}
	if !strings.HasSuffix(tok, Suffix) {
		t.Fatalf("BuildFailToken suffix wrong: %q", tok)
	}
	// 中间段应该是 DefaultErrorPayload
	mid := tok[len(PrefixProof+ErrorPrefix) : len(tok)-len(Suffix)]
	if mid != DefaultErrorPayload {
		t.Fatalf("BuildFailToken middle = %q, want %q (= base64(JSON.stringify(\"e\")))", mid, DefaultErrorPayload)
	}

	// 显式错误信息:应该 base64(JSON.stringify(message))
	tok = BuildFailToken("hash error")
	if !strings.HasPrefix(tok, PrefixProof+ErrorPrefix) {
		t.Fatalf("BuildFailToken(err) prefix wrong: %q", tok)
	}
	if !strings.HasSuffix(tok, Suffix) {
		t.Fatalf("BuildFailToken(err) suffix wrong: %q", tok)
	}
	mid = tok[len(PrefixProof+ErrorPrefix) : len(tok)-len(Suffix)]
	wantMid := EncodeString("hash error")
	if mid != wantMid {
		t.Fatalf("BuildFailToken(err) middle = %q, want %q", mid, wantMid)
	}
}

// TestEncodeString 验证 EncodeString 行为 = base64(JSON.stringify(s))。
// JSON.stringify("e") = `"e"` (含引号) → base64 = "ImUi"
// JSON.stringify("hello") = `"hello"` (含引号) → base64 = "ImhlbGxvIg=="
func TestEncodeString(t *testing.T) {
	if got := EncodeString("e"); got != "ImUi" {
		t.Fatalf("EncodeString(\"e\") = %q, want %q", got, "ImUi")
	}
	if got := EncodeString("hello"); got != "ImhlbGxvIg==" {
		t.Fatalf("EncodeString(\"hello\") = %q, want %q", got, "ImhlbGxvIg==")
	}
}
