package browserfp

import (
	"math/rand"
	"strings"
	"time"
)

// ─── Language ──────────────────────────────────────────────────────────────

// Language 浏览器语言条目。
type Language struct {
	Code     string // "en-US"
	JoinList string // "en-US,en"
}

// Slice 返回 [][]string 的 []string 形式。
func (l Language) Slice() []string { return strings.Split(l.JoinList, ",") }

// LanguageJoin 返回 code 对应的 languagesJoin 字符串。
func LanguageJoin(code string) string {
	for _, l := range Languages {
		if l.Code == code {
			return l.JoinList
		}
	}
	return code + ",en"
}

// LanguageSlices 所有 Language 的 [][]string 形式。
func LanguageSlices() [][]string {
	out := make([][]string, len(Languages))
	for i, l := range Languages {
		out[i] = l.Slice()
	}
	return out
}

// ─── 通用数据池 ──────────────────────────────────────────────────────────

// UserAgents 真实 Chrome UA 池。
var UserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
}

// Languages 浏览器 navigator.languages 组合。
var Languages = []Language{
	{"en-US", "en-US,en"},
	{"zh-CN", "zh-CN,zh"},
	{"en-GB", "en-GB,en"},
	{"ja", "ja,en"},
	{"de", "de,en"},
	{"fr", "fr,en"},
	{"es", "es,en"},
	{"ko", "ko,en"},
}

// Platforms 浏览器 navigator.platform 值。
var Platforms = []string{"Win32", "MacIntel", "Linux x86_64"}

// DocumentKeys document 上 own-enumerable 属性名池。
var DocumentKeys = []string{
	"_reactListening8in7sfyhjvp", "_reactListeningo743lnnpvdg",
	"_reactContainer$5pyziap1brc", "__reactContainer$b63yiita51i",
	"location", "cookie", "referrer", "currentScript", "body", "head", "documentElement",
}

// WindowKeys window 全局属性名池。
var WindowKeys = []string{
	"onchange", "onclick", "onload", "onerror", "onresize",
	"onmouseover", "onmouseout", "onfocus", "onblur", "onscroll",
	"onkeydown", "onkeyup", "onkeypress",
	"requestIdleCallback", "requestAnimationFrame", "setTimeout",
	"fetch", "console", "Promise", "Map", "Set", "WeakMap", "WeakSet",
	"crypto", "performance", "navigator", "document", "location", "history",
	"localStorage", "sessionStorage", "indexedDB",
	"Image", "XMLHttpRequest", "FormData", "Headers", "Request", "Response",
	"alert", "confirm", "prompt", "close", "focus", "blur",
	"addEventListener", "removeEventListener", "dispatchEvent",
	"scrollTo", "scrollBy", "scroll", "matchMedia", "getComputedStyle",
	"getSelection", "find", "stop", "open", "print", "captureEvents",
	"releaseEvents", "queueMicrotask", "reportError", "structuredClone",
	"isSecureContext", "crossOriginIsolated", "originAgentCluster",
	"speechSynthesis", "MediaSource", "Blob", "File", "FileReader",
	"Atomics", "SharedArrayBuffer", "WebAssembly", "BigInt", "Symbol", "Proxy",
}

// NavigatorProbes "X in navigator" 探测格式。
var NavigatorProbes = []string{
	"windowControlsOverlay−[object WindowControlsOverlay]",
	"geolocation−[object Geolocation]",
	"clipboard−[object Clipboard]",
	"mediaDevices−[object MediaDevices]",
	"permissions−[object Permissions]",
	"bluetooth−[object Bluetooth]",
	"usb−[object USB]",
	"serial−[object Serial]",
	"hid−[object HID]",
	"presentation−[object Presentation]",
	"credentials−[object CredentialsContainer]",
}

// DefaultBuildID 当前 chatgpt.com 的 data-build 属性。
const DefaultBuildID = "prod-2e2e6a5279d822603df0be74f1018da3099d7573"

// VendorForPlatform 返回给定平台的 navigator.vendor 值。
func VendorForPlatform(platform string) string {
	if platform == "MacIntel" {
		return "Apple Computer, Inc."
	}
	return "Google Inc."
}

// ─── Profile ──────────────────────────────────────────────────────────────

// Profile 浏览器指纹配置。
type Profile struct {
	WebGLUnmaskedRenderer string
	WebGLUnmaskedVendor   string
	Language              string
	BuildID               string
	Platform              string
	ScreenWidth           int
	ScreenHeight          int
	ScreenAvailHeight     int
	ScreenColorDepth      int
	HardwareConcurrency   int
	DeviceMemory          int
	JSHeapSizeLimit       int64
	NetworkDownlink       float64
	NetworkRTT            int
	TimezoneOffset        int
}

// ─── 全局单例 ──────────────────────────────────────────────────────────────

var current *Profile

// Init 启动时调用一次，生成全局唯一的 Profile。
func Init() { current = Generate(nil) }

// Get 返回全局唯一的 Profile。
func Get() *Profile { return current }

// Generate 从真实数据池中随机生成 Profile。
func Generate(rng *rand.Rand) *Profile {
	if rng == nil {
		rng = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	lang := Languages[rng.Intn(len(Languages))]
	platform := Platforms[rng.Intn(len(Platforms))]
	sr := screenResolutions[rng.Intn(len(screenResolutions))]
	sw, sh := sr[0]+rng.Intn(101)-50, sr[1]+rng.Intn(101)-50
	if sw < 1024 {
		sw = 1024
	}
	if sh < 600 {
		sh = 600
	}

	return &Profile{
		WebGLUnmaskedRenderer: webglUnmaskedRenderers[rng.Intn(len(webglUnmaskedRenderers))],
		WebGLUnmaskedVendor:   webglUnmaskedVendors[rng.Intn(len(webglUnmaskedVendors))],

		Language: lang.Code,
		BuildID:  DefaultBuildID,
		Platform: platform,

		ScreenWidth:       sw,
		ScreenHeight:      sh,
		ScreenAvailHeight: sh - (20 + rng.Intn(40)),
		ScreenColorDepth:  24,

		HardwareConcurrency: pickInt(rng, []int{4, 8, 12, 16, 20, 24, 32}),
		DeviceMemory:        pickInt(rng, []int{4, 8, 16, 32}),
		JSHeapSizeLimit:     pickInt64(rng, []int64{2_147_483_648, 4_294_967_296, 8_589_934_592, 17_179_869_184}),

		NetworkDownlink: networkDownlinks[rng.Intn(len(networkDownlinks))],
		NetworkRTT:      networkRTTs[rng.Intn(len(networkRTTs))],

		TimezoneOffset: (rng.Intn(23) - 12) * 60,
	}
}

// ─── 内部辅助 ──────────────────────────────────────────────────────────────

func pickStr(rng *rand.Rand, opts []string) string { return opts[rng.Intn(len(opts))] }
func pickInt(rng *rand.Rand, opts []int) int        { return opts[rng.Intn(len(opts))] }
func pickInt64(rng *rand.Rand, opts []int64) int64  { return opts[rng.Intn(len(opts))] }