package fingerprint

import (
	"fmt"
	"math/rand"
	"testing"
)

func TestBuild23_MatchesSample(t *testing.T) {
	// 固定 [4]/[11] 的 random 源让输出稳定;其他字段固定到样本里的值
	stableRand := rand.New(rand.NewSource(42))
	opts := Options{
		UserAgent:           "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
		Languages:           []string{"en-US", "en"},
		Platform:            "Win32",
		ScreenWidth:         1920,
		ScreenHeight:        1080,
		Timezone:            "America/Los_Angeles",
		RandomScript:        "https://connect.facebook.net/en_US/fbevents.js",
		BuildID:             "prod-ab8a6348980a3e1d771c463b9f4f3e4e584f2769",
		RandomDocKey:        "_reactListening8in7sfyhjvp",
		RandomWindowKey:     "onchange",
		PageOpenedSeconds:   6574.8986000000015,
		JSHeapSizeLimit:     4294967296,
		SetEnvFlags:         true,
		EnvFlags:            [4]int{0, 0, 0, 1},
		Rand:                stableRand,
	}
	got := Build23(opts)

	// 期望:跟浏览器真值样本的 23 元素逐字段对照。
	// [4] [11] 是 Math.random() (每次不同,只能验类型/范围);
	// [18] 是 now (每次不同);其他应该精确匹配。
	wantType := []string{
		"string", "string", "string", "int",
		"float", "string", "string", "string", "string",
		"int", "[]any", "float", "string", "string",
		"float", "string", "string", "string", "float",
		"int", "int", "int", "int",
	}
	for i, v := range got {
		t.Run(fmt.Sprintf("[%d] type", i), func(t *testing.T) {
			gotType := typeOf(v)
			if gotType != wantType[i] {
				t.Errorf("[%d] 类型不对: got %s want %s (v=%v)", i, gotType, wantType[i], v)
			}
		})
	}

	// [0]: String(3000) = "3000"
	if got[0] != "3000" {
		t.Errorf("[0]: got %v want \"3000\"", got[0])
	}
	// [2]: jsHeapSizeLimit 字符串
	if got[2] != "4294967296" {
		t.Errorf("[2]: got %v want \"4294967296\"", got[2])
	}
	// [5]: UA
	if got[5] != opts.UserAgent {
		t.Errorf("[5]: got %v", got[5])
	}
	// [6]: script src
	if got[6] != opts.RandomScript {
		t.Errorf("[6]: got %v want %v", got[6], opts.RandomScript)
	}
	// [7]: build id
	if got[7] != opts.BuildID {
		t.Errorf("[7]: got %v want %v", got[7], opts.BuildID)
	}
	// [8]: navigator.language
	if got[8] != "en-US" {
		t.Errorf("[8]: got %v want \"en-US\"", got[8])
	}
	// [10]: navigator.languages 数组
	arr, ok := got[10].([]any)
	if !ok {
		t.Fatalf("[10] 不是 array")
	}
	if len(arr) != 2 || arr[0] != "en-US" || arr[1] != "en" {
		t.Errorf("[10] 数组内容: %v", arr)
	}
	// [12] / [13]: docKey / winKey
	if got[12] != opts.RandomDocKey {
		t.Errorf("[12]: got %v", got[12])
	}
	if got[13] != opts.RandomWindowKey {
		t.Errorf("[13]: got %v", got[13])
	}
	// [14]: performance.now() ≈ 6574898.6
	if v, ok := got[14].(float64); !ok || v < 6574000 || v > 6575000 {
		t.Errorf("[14]: got %v want ~6574898.6", got[14])
	}
	// [15]: sid(空,真实浏览器没存)
	if got[15] != "" {
		t.Errorf("[15]: got %v want \"\"", got[15])
	}
	// [16]: URLSearchParams join(空,真实无 query)
	if got[16] != "" {
		t.Errorf("[16]: got %v want \"\"", got[16])
	}
	// [17]: navigator.platform
	if got[17] != "Win32" {
		t.Errorf("[17]: got %v", got[17])
	}
	// [19-22]: env flags 全 0/0/0/1
	for i, want := range []int{0, 0, 0, 1} {
		if v, _ := got[19+i].(int); v != want {
			t.Errorf("[%d] env: got %v want %d", 19+i, got[19+i], want)
		}
	}
}

func TestBuild23_Default(t *testing.T) {
	// 不指定任何字段,看默认输出是否合理
	got := Build23(DefaultOptions())
	if len(got) != 23 {
		t.Fatalf("len != 23: got %d", len(got))
	}
	for i, v := range got {
		t.Logf("[%d] %T %v", i, v, v)
	}
	// 抽查: [0] 必须是 string
	if _, ok := got[0].(string); !ok {
		t.Errorf("[0] 应该是 string,got %T", got[0])
	}
	// [10] 必须是 array
	if _, ok := got[10].([]any); !ok {
		t.Errorf("[10] 应该是 array,got %T", got[10])
	}
	// [2] 必须是 string
	if _, ok := got[2].(string); !ok {
		t.Errorf("[2] 应该是 string,got %T", got[2])
	}
	// [22] (TextEncoder) 默认 1
	if v, _ := got[22].(int); v != 1 {
		t.Errorf("[22] 默认应是 1,got %v", got[22])
	}
	fmt.Println("TestBuild23_Default 通过")
}

func typeOf(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case int:
		return "int"
	case float64:
		return "float"
	case []any:
		return "[]any"
	}
	return fmt.Sprintf("%T", v)
}
