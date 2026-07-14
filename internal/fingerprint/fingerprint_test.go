package fingerprint

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestBuild25_MatchesSample(t *testing.T) {
	stableRand := rand.New(rand.NewSource(42))
	opts := Options{
		UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
		Languages:           []string{"en-US", "en"},
		Platform:            "Win32",
		ScreenWidth:         1920,
		ScreenHeight:        1080,
		Timezone:            "America/Los_Angeles",
		BuildID:             "prod-ab8a6348980a3e1d771c463b9f4f3e4e584f2769",
		RandomDocKey:        "_reactListening8in7sfyhjvp",
		RandomWindowKey:     "onchange",
		PageOpenedSeconds:   6574.8986000000015,
		JSHeapSizeLimit:     4294967296,
		HardwareConcurrency: 8,
		Rand:                stableRand,
	}
	got := Build25(opts)

	// 对齐 2026-06 浏览器 25 元素真值样本类型
	wantType := []string{
		"int",    // [0]  screen sum (number)
		"string", // [1]  Date.toString()
		"int64",  // [2]  jsHeapSizeLimit (number)
		"int",    // [3]  nonce (0, caller overrides)
		"string", // [4]  UA
		"string", // [5]  SDK script URL
		"string", // [6]  buildID
		"string", // [7]  navigator.language
		"string", // [8]  navigator.languages (comma-separated)
		"float",  // [9]  Math.random()
		"string", // [10] navigator probe
		"string", // [11] document key
		"string", // [12] window key
		"float",  // [13] performance.now()
		"string", // [14] device_id (empty, caller overrides)
		"string", // [15] location.search (empty)
		"int",    // [16] hardwareConcurrency
		"float",  // [17] timeOrigin
		"int",    // [18]
		"int",    // [19]
		"int",    // [20]
		"int",    // [21]
		"int",    // [22]
		"int",    // [23]
		"int",    // [24]
	}
	if len(got) != 25 {
		t.Fatalf("len != 25: got %d", len(got))
	}
	for i, v := range got {
		t.Run(fmt.Sprintf("[%d] type", i), func(t *testing.T) {
			gotType := typeOf(v)
			if gotType != wantType[i] {
				t.Errorf("[%d] 类型不对: got %s want %s (v=%v)", i, gotType, wantType[i], v)
			}
		})
	}

	// [0]: screen sum (1920+1080=3000, NUMBER not string)
	if got[0] != 3000 {
		t.Errorf("[0]: got %v want 3000 (number)", got[0])
	}
	// [2]: jsHeapSizeLimit number
	if got[2] != int64(4294967296) {
		t.Errorf("[2]: got %v (type=%T) want 4294967296 (int64)", got[2], got[2])
	}
	// [4]: UA
	if got[4] != opts.UserAgent {
		t.Errorf("[4]: got %v", got[4])
	}
	// [5]: SDK script URL (NOT null)
	if got[5] != "https://chatgpt.com/backend-api/sentinel/sdk.js" {
		t.Errorf("[5]: got %v want SDK URL", got[5])
	}
	// [6]: buildID
	if got[6] != opts.BuildID {
		t.Errorf("[6]: got %v want %v", got[6], opts.BuildID)
	}
	// [7]: navigator.language
	if got[7] != "en-US" {
		t.Errorf("[7]: got %v want \"en-US\"", got[7])
	}
	// [8]: navigator.languages comma-separated
	if got[8] != "en-US,en" {
		t.Errorf("[8]: got %v want \"en-US,en\"", got[8])
	}
	// [11] / [12]: docKey / winKey
	if got[11] != opts.RandomDocKey {
		t.Errorf("[11]: got %v", got[11])
	}
	if got[12] != opts.RandomWindowKey {
		t.Errorf("[12]: got %v", got[12])
	}
	// [13]: performance.now() ≈ pageOpenedSeconds*1000
	if v, ok := got[13].(float64); !ok || v < 6574000 || v > 6575000 {
		t.Errorf("[13]: got %v want ~6574898.6", got[13])
	}
	// [15]: location.search (empty)
	if got[15] != "" {
		t.Errorf("[15]: got %v want \"\"", got[15])
	}
	// [16]: hardwareConcurrency
	if got[16] != 8 {
		t.Errorf("[16]: got %v want 8", got[16])
	}
	// [18-24]: window probes all 0
	for i, want := range []int{0, 0, 0, 0, 0, 0, 0} {
		if v, _ := got[18+i].(int); v != want {
			t.Errorf("[%d]: got %v want %d", 18+i, got[18+i], want)
		}
	}
}

func typeOf(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case int:
		return "int"
	case int64:
		return "int64"
	case float64:
		return "float"
	case []any:
		return "[]any"
	}
	return fmt.Sprintf("%T", v)
}
