package util

import (
	"strings"
	"testing"
)

func TestRandomUserAgent(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 200; i++ {
		ua := RandomUserAgent()
		if strings.TrimSpace(ua) == "" {
			t.Fatalf("empty UA at iter %d", i)
		}
		if !strings.HasPrefix(ua, "Mozilla/5.0") {
			t.Errorf("UA does not start with Mozilla/5.0: %s", ua)
		}
		seen[ua] = true
	}
	// 跑 200 次应至少出现 ≥3 种不同的 UA —— 防止退化到固定值
	if len(seen) < 3 {
		t.Errorf("expected at least 3 distinct UAs in 200 draws, got %d", len(seen))
	}
}
