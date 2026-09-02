package browserfp

import (
	cryptorand "crypto/rand"
	"encoding/binary"
	mathrand "math/rand"
	"strings"
	"sync"
	"sync/atomic"
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

// UserAgents 与 Chrome 150 TLS profile 对齐的桌面 UA 池。
var UserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36",
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

// Platforms 当前启用的 Chrome 150 平台。
var Platforms = []string{"Win32"}

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

// ScriptURLs sentinel SDK 脚本 URL 池。
// 浏览器中 hPt(Array.from(document.scripts).map(e=>e?.src).filter(e=>e))
// 会随机选一个 script URL。
var ScriptURLs = []string{
	"https://chatgpt.com/backend-api/sentinel/sdk.js",
	"https://chatgpt.com/sentinel/20260423af3c/sdk.js",
}

// DefaultBuildID 当前 chatgpt.com 的 data-build 属性。
const DefaultBuildID = "prod-46437587156517d920436051cb9ab60a95f0503a"

// VendorForPlatform 返回 Chrome 的 navigator.vendor 值。
func VendorForPlatform(string) string {
	return "Google Inc."
}

// ─── Profile ──────────────────────────────────────────────────────────────

// Profile 浏览器指纹配置。
type Profile struct {
	UserAgent             string
	WebGLUnmaskedRenderer string
	WebGLUnmaskedVendor   string
	Language              string
	BuildID               string
	Platform              string
	Timezone              string
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
	DevicePixelRatio      float64
}

type screenProfile struct {
	width      int
	height     int
	availInset int
	pixelRatio float64
}

type deviceProfile struct {
	userAgent           string
	platform            string
	language            Language
	timezone            string
	screens             []screenProfile
	hardwareConcurrency []int
	deviceMemory        []int
	jsHeapSizeLimit     []int64
}

var deviceProfiles = []deviceProfile{
	{
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Safari/537.36",
		platform:  "Win32",
		language:  Language{"en-US", "en-US,en"},
		timezone:  "America/Los_Angeles",
		screens: []screenProfile{
			{1920, 1080, 40, 1.0},
			{1920, 1080, 40, 1.25},
			{2560, 1440, 40, 1.0},
			{1536, 864, 40, 1.25},
			{1366, 768, 40, 1.0},
		},
		hardwareConcurrency: []int{4, 8, 12, 16},
		deviceMemory:        []int{4, 8},
		jsHeapSizeLimit:     []int64{2_147_483_648, 4_294_967_296},
	},
}

// ─── 全局单例 ──────────────────────────────────────────────────────────────

var (
	current  atomic.Pointer[Profile]
	initOnce sync.Once
)

// Init 启动时调用一次，生成全局唯一的 Profile。
func Init() {
	initOnce.Do(func() {
		current.Store(Generate(nil))
	})
}

// Get 返回全局唯一 Profile 的副本。未显式初始化时会惰性初始化。
func Get() *Profile {
	Init()
	profile := current.Load()
	if profile == nil {
		profile = Generate(nil)
		current.Store(profile)
	}
	clone := *profile
	return &clone
}

// Generate 从成套设备画像中随机生成 Profile。
func Generate(rng *mathrand.Rand) *Profile {
	return generateAt(rng, time.Now())
}

func generateAt(rng *mathrand.Rand, now time.Time) *Profile {
	if rng == nil {
		rng = newRand()
	}

	device := deviceProfiles[rng.Intn(len(deviceProfiles))]
	screen := device.screens[rng.Intn(len(device.screens))]
	webGLVendor, webGLRenderer := pickWebGL(rng, device.platform)

	return &Profile{
		UserAgent:             device.userAgent,
		WebGLUnmaskedVendor:   webGLVendor,
		WebGLUnmaskedRenderer: webGLRenderer,
		Language:              device.language.Code,
		BuildID:               DefaultBuildID,
		Platform:              device.platform,
		Timezone:              device.timezone,
		ScreenWidth:           screen.width,
		ScreenHeight:          screen.height,
		ScreenAvailHeight:     screen.height - screen.availInset,
		ScreenColorDepth:      24,
		HardwareConcurrency:   pickInt(rng, device.hardwareConcurrency),
		DeviceMemory:          pickInt(rng, device.deviceMemory),
		JSHeapSizeLimit:       pickInt64(rng, device.jsHeapSizeLimit),
		NetworkDownlink:       pickFloat(rng, networkDownlinks, 10),
		NetworkRTT:            pickRealisticRTT(rng),
		TimezoneOffset:        timezoneOffsetMinutes(device.timezone, now),
		DevicePixelRatio:      screen.pixelRatio,
	}
}

func newRand() *mathrand.Rand {
	var seedBytes [8]byte
	if _, err := cryptorand.Read(seedBytes[:]); err == nil {
		return mathrand.New(mathrand.NewSource(int64(binary.LittleEndian.Uint64(seedBytes[:]))))
	}
	return mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
}

func pickWebGL(rng *mathrand.Rand, platform string) (string, string) {
	type candidate struct {
		vendor   string
		renderer string
	}

	candidates := make([]candidate, 0, 32)
	for vendorIndex, vendor := range webglUnmaskedVendors {
		if vendorIndex >= len(webglUnmaskedRenderersMap) {
			break
		}
		for _, renderer := range webglUnmaskedRenderersMap[vendorIndex] {
			if webGLMatchesPlatform(platform, renderer) {
				candidates = append(candidates, candidate{vendor: vendor, renderer: renderer})
			}
		}
	}
	if len(candidates) == 0 {
		return "Google Inc. (Google)", "ANGLE (Google, Vulkan 1.3.0 (SwiftShader Device (Subzero) (0x0000C0DE)), SwiftShader driver)"
	}
	selected := candidates[rng.Intn(len(candidates))]
	return selected.vendor, selected.renderer
}

func webGLMatchesPlatform(platform, renderer string) bool {
	switch platform {
	case "Win32":
		return strings.Contains(renderer, "Direct3D11") || strings.Contains(renderer, "SwiftShader")
	case "MacIntel":
		return strings.Contains(renderer, "Metal Renderer")
	case "Linux x86_64":
		return strings.Contains(renderer, "Mesa ") ||
			strings.Contains(renderer, "radeonsi") ||
			strings.Contains(renderer, "NVIDIA Corporation")
	default:
		return false
	}
}

func timezoneOffsetMinutes(timezone string, now time.Time) int {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return 480
	}
	_, offsetSeconds := now.In(location).Zone()
	return -offsetSeconds / 60
}

func pickRealisticRTT(rng *mathrand.Rand) int {
	candidates := make([]int, 0, len(networkRTTs))
	for _, rtt := range networkRTTs {
		if rtt >= 50 && rtt <= 1000 {
			candidates = append(candidates, rtt)
		}
	}
	return pickInt(rng, candidates)
}

func pickFloat(rng *mathrand.Rand, opts []float64, fallback float64) float64 {
	if len(opts) == 0 {
		return fallback
	}
	return opts[rng.Intn(len(opts))]
}

func pickInt(rng *mathrand.Rand, opts []int) int {
	if len(opts) == 0 {
		return 0
	}
	return opts[rng.Intn(len(opts))]
}

func pickInt64(rng *mathrand.Rand, opts []int64) int64 {
	if len(opts) == 0 {
		return 0
	}
	return opts[rng.Intn(len(opts))]
}
