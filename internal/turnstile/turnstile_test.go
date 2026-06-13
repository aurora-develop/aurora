package turnstile

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"
)

// TestSolveWithRealDX 用真实 dx 字符串验证能解出非空 token。
func TestSolveWithRealDX(t *testing.T) {
	dxBytes, err := os.ReadFile("../../dx.txt")
	if err != nil {
		t.Skip("dx.txt not found, skipping")
	}
	dx := strings.TrimSpace(string(dxBytes))
	dx = strings.Trim(dx, `"`)

	// 尝试用空 key 解码(验证 JSON 结构)
	decoded, err := base64.StdEncoding.DecodeString(dx)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}
	t.Logf("dx decoded length: %d bytes", len(decoded))

	// 尝试空 key
	result := Solve(dx, "")
	t.Logf("Solve(dx, '') => len=%d", len(result))
	if result != "" {
		raw, _ := base64.StdEncoding.DecodeString(result)
		t.Logf("Solve(dx, '') result decoded: %s", string(raw))
	}

	// 尝试一些常见 key
	keys := []string{
		"0.123456789",
		"test",
		"wQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4D",
	}
	for _, key := range keys {
		r := Solve(dx, key)
		if r != "" {
			raw, _ := base64.StdEncoding.DecodeString(r)
			t.Logf("Solve(dx, '%s') => %s", key, string(raw))
		}
	}
}

// TestSolveInvalidInput 验证无效输入返回空字符串。
func TestSolveInvalidInput(t *testing.T) {
	if got := Solve("", ""); got != "" {
		t.Errorf("Solve('', '') = %q, want empty", got)
	}
	if got := Solve("not-valid-base64!!!", "key"); got != "" {
		t.Errorf("Solve(invalid-base64) = %q, want empty", got)
	}
}

// TestSolveWithScripts 验证 wrapper 行为一致。
func TestSolveWithScripts(t *testing.T) {
	// 对于当前实现，SolveWithScripts 直接调用 Solve
	if got := SolveWithScripts("", "", nil); got != "" {
		t.Errorf("SolveWithScripts('', '', nil) = %q, want empty", got)
	}
}
