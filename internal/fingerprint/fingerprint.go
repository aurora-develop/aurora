// Package fingerprint — Build25 浏览器指纹。
//
// 对齐 2026-06 chatgpt.com sentinel SDK 抓包: 25 元素 fingerprint config,
// 用于 /chat-requirements/prepare、/sentinel/req 和 PoW proof token。
// 老 Build23 已删除(不再使用)。
package fingerprint

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

var CommonUserAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36",
}

// CommonLanguages 真实浏览器 navigator.languages 组合(全英文环境,避免风控)。
var CommonLanguages = [][]string{
	{"en-US", "en"},
	{"en-GB", "en", "en-US"},
	{"en", "en-US"},
}

// CommonPlatforms 真 platform 值。
var CommonPlatforms = []string{"Win32", "MacIntel", "Linux x86_64"}

// CommonDocumentKeys 真实 document 上的 own-enumerable 属性(React 18 mount 后会
// 出现 __reactContainer$xxx / __reactListening$xxx 等等)。
var CommonDocumentKeys = []string{
	"_reactListening8in7sfyhjvp",
	"_reactListeningo743lnnpvdg",
	"_reactContainer$5pyziap1brc",
	"__reactContainer$b63yiita51i",
	"location",
	"cookie",
	"referrer",
	"currentScript",
	"body",
	"head",
	"documentElement",
}

// CommonWindowKeys 真实 window 全局属性名(浏览器标准事件/全局对象)。
var CommonWindowKeys = []string{
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

// CommonBuildIDs 真实 build id 样本(c/<build>/_/ 模式里那个 build)。
var CommonBuildIDs = "prod-ab8a6348980a3e1d771c463b9f4f3e4e584f2769"

// CommonNavigatorProbes 真实 navigator 探测样本(从 chatgpt.com 抓包)。
// [10] 字段(对齐新版 SDK "X in navigator" 真值样本):
//
//	"windowControlsOverlay−[object WindowControlsOverlay]"
//	"geolocation−[object Geolocation]"
//	"clipboard−[object Clipboard]"
//	"mediaDevices−[object MediaDevices]"
//	"permissions−[object Permissions]"
//	"bluetooth−[object Bluetooth]"
//	"usb−[object USB]"
//	"serial−[object Serial]"
//	"hid−[object HID]"
//	"presentation−[object Presentation]"
//	"credentials−[object CredentialsContainer]"
var CommonNavigatorProbes = []string{
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

// Options 浏览器指纹模拟选项。
type Options struct {
	UserAgent           string
	Languages           []string // 第一项是 navigator.language,其余是 languages 数组
	Platform            string
	ScreenWidth         int
	ScreenHeight        int
	HardwareConcurrency int
	// Timezone 浏览器时区(如 "America/Los_Angeles"),留空用 local
	Timezone string
	// PageOpenedSeconds 页面打开多久(影响 [13] performance.now())
	PageOpenedSeconds float64
	// BuildID 强制 [6] 取这个;空则从 CommonBuildIDs 取
	BuildID string
	// RandomDocKey [11] 强制值;空则从 CommonDocumentKeys 随机
	RandomDocKey string
	// RandomWindowKey [12] 强制值;空则从 CommonWindowKeys 随机
	RandomWindowKey string
	// JSHeapSizeLimit 字节;默认 4294967296
	JSHeapSizeLimit int64
	// Rand 随机源(测试用;nil 用 time-based rand)
	Rand *rand.Rand
}

// DefaultOptions 返回一个贴近真实 Chrome on Windows 的默认配置。
func DefaultOptions() Options {
	return Options{
		UserAgent:           CommonUserAgents[0],
		Languages:           []string{"en-US", "en"},
		Platform:            "Win32",
		ScreenWidth:         1920,
		ScreenHeight:        1080,
		HardwareConcurrency: 8,
		Timezone:            "America/Los_Angeles",
		PageOpenedSeconds:   10,
		JSHeapSizeLimit:     4294967296,
	}
}

// pickNavigatorProbe 从 CommonNavigatorProbes 随机挑一个。
func pickNavigatorProbe(r *rand.Rand) string {
	if len(CommonNavigatorProbes) == 0 {
		return "geolocation−[object Geolocation]"
	}
	return CommonNavigatorProbes[r.Intn(len(CommonNavigatorProbes))]
}

// Build25 生成 25 元素 fingerprint 数组(顺序/类型严格按浏览器真实)。
// 对齐 2026-06-24 chatgpt.com 浏览器 sentinel/chat-requirements/prepare 抓包:
//
//	[0]  screen.w + screen.h                 number
//	[1]  Date.toString()                     string (带时区名)
//	[2]  jsHeapSizeLimit                     number
//	[3]  nonce (requirements=1, PoW=iteration) number
//	[4]  navigator.userAgent                 string ← 浏览器把 UA 放这里!
//	[5]  SDK 脚本 URL                         string (NOT null!)
//	[6]  buildID (data-build 属性)           string
//	[7]  navigator.language                  string
//	[8]  navigator.languages (逗号分隔)       string
//	[9]  Math.random() / elapsedMs           number ← Math.random() 在这!
//	[10] "X in navigator" 探测              string ("name−[object Name]")
//	[11] document 随机 key (Object.keys)    string
//	[12] window 随机 key (getOwnPropertyNames) string
//	[13] performance.now()                   number
//	[14] device_id                           string
//	[15] location.search joined              string
//	[16] hardwareConcurrency                 number
//	[17] performance.timeOrigin              number
//	[18-24] "X in window" 检查               number (7 个, 默认 0)
//
// nonce/elapsed 由调用方注入 [3]/[9]。requirements 阶段 [9]=Math.random(),
// PoW 阶段 [3]=nonce(iteration), [9]=elapsedMs。
func Build25(opts Options) []any {
	r := opts.Rand
	if r == nil {
		r = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	ua := opts.UserAgent
	if ua == "" {
		ua = CommonUserAgents[r.Intn(len(CommonUserAgents))]
	}

	langs := opts.Languages
	if len(langs) == 0 {
		langs = CommonLanguages[r.Intn(len(CommonLanguages))]
	}
	primaryLang := langs[0]

	platform := opts.Platform
	if platform == "" {
		platform = CommonPlatforms[r.Intn(len(CommonPlatforms))]
	}

	w, h := opts.ScreenWidth, opts.ScreenHeight
	if w == 0 {
		w = 1920
	}
	if h == 0 {
		h = 1080
	}

	// [0] screen.w + screen.h — 对齐浏览器: NUMBER, 不是字符串!
	screenSum := w + h

	// [1] new Date().toString() — 浏览器格式:
	//   "Fri Jun 19 2026 04:38:41 GMT-0700 (Pacific Daylight Time)"
	var dateStr string
	if opts.Timezone != "" {
		if loc, err := time.LoadLocation(opts.Timezone); err == nil {
			dateStr = jsDateToString(time.Now().In(loc))
		}
	}
	if dateStr == "" {
		dateStr = jsDateToString(time.Now())
	}

	// [2] jsHeapSizeLimit — 对齐浏览器: NUMBER (int64), 不是字符串!
	jsHeap := opts.JSHeapSizeLimit
	if jsHeap == 0 {
		jsHeap = 4294967296
	}

	// [4] Math.random()
	r4 := r.Float64()

	buildID := opts.BuildID
	if buildID == "" {
		if CommonBuildIDs != "" {
			buildID = CommonBuildIDs
		}
	}

	// [8] navigator.languages — 浏览器逗号分隔格式, e.g. "zh-CN,zh"
	langsStr := strings.Join(langs, ",")

	// [10] "X in navigator" 探测:真值格式 "name−[object Name]"
	navigatorProbe := pickNavigatorProbe(r)

	// [11] Object.keys(document) 随机
	docKey := opts.RandomDocKey
	if docKey == "" && len(CommonDocumentKeys) > 0 {
		docKey = CommonDocumentKeys[r.Intn(len(CommonDocumentKeys))]
	}

	// [12] Object.getOwnPropertyNames(window) 随机
	winKey := opts.RandomWindowKey
	if winKey == "" && len(CommonWindowKeys) > 0 {
		winKey = CommonWindowKeys[r.Intn(len(CommonWindowKeys))]
	}

	// [13] performance.now() — 页面打开 X 秒后
	perfNow := opts.PageOpenedSeconds * 1000
	if perfNow < 0 {
		perfNow = 0
	}

	// [14] deviceID — 留空(浏览器真实情况,由调用方覆盖)
	deviceID := ""

	// [15] URLSearchParams(location.search) join — 通常空
	searchJoined := ""

	// [16] hardwareConcurrency
	hwConc := opts.HardwareConcurrency
	if hwConc == 0 {
		hwConc = 8
	}

	// [17] performance.timeOrigin — Unix ms 时间戳
	timeOrigin := float64(time.Now().UnixMilli()) - perfNow

	// [18-24] 7 个 "X in window" 探测(对齐 2026-06-24 chatgpt.com 浏览器抓包):
	//   浏览器全部返回 0 (Number("X" in window) → 0)。
	//   真浏览器 Chrome 上其他探测可能为 1,但 sentinel SDK 只检查这 7 个,
	//   且实测全部为 0。
	windowProbes := [7]int{0, 0, 0, 0, 0, 0, 0}

	// [5] currentScript.src — 浏览器始终传 SDK 脚本 URL，不是 null
	// 对齐 2026-06-24 chatgpt.com 抓包
	sdkScript := "https://chatgpt.com/backend-api/sentinel/sdk.js"

	config := []any{
		screenSum,       // [0]  number: screen.w+h
		dateStr,         // [1]  string: Date.toString()
		jsHeap,          // [2]  number: jsHeapSizeLimit
		0,               // [3]  nonce (caller 覆盖)
		ua,              // [4]  navigator.userAgent
		sdkScript,       // [5]  SDK script URL (NOT null!)
		buildID,         // [6]  build id
		primaryLang,     // [7]  navigator.language
		langsStr,        // [8]  navigator.languages (逗号分隔)
		r4,              // [9]  Math.random() / elapsed (caller 覆盖)
		navigatorProbe,  // [10] "X in navigator" 探测
		docKey,          // [11] Object.keys(document)
		winKey,          // [12] window 随机键
		perfNow,         // [13] performance.now()
		deviceID,        // [14] device_id (caller 覆盖)
		searchJoined,    // [15] URLSearchParams keys
		hwConc,          // [16] hardwareConcurrency
		timeOrigin,      // [17] performance.timeOrigin
		windowProbes[0], // [18]
		windowProbes[1], // [19]
		windowProbes[2], // [20]
		windowProbes[3], // [21]
		windowProbes[4], // [22]
		windowProbes[5], // [23]
		windowProbes[6], // [24]
	}
	_ = platform // 新版 SDK 不再用 platform 字段
	return config
}

// jsDateToString 模拟浏览器 Date.prototype.toString() 格式:
//
//	"Fri Jun 19 2026 04:38:41 GMT-0700 (Pacific Daylight Time)"
//
// 对齐 V8/Blink/SpiderMonkey 的输出(包括 DST 名字)。
func jsDateToString(t time.Time) string {
	// t.Format 用 Go 参考时间: Mon Jan 2 2006 15:04:05 MST
	// 浏览器输出:                       Fri Jun 19 2026 04:38:41 GMT-0700 (Pacific Daylight Time)
	head := t.Format("Mon Jan 2 2006 15:04:05")

	// GMT offset: GMT-0700 / GMT+0800 (没有冒号,跟浏览器一致)
	_, offset := t.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	hours := offset / 3600
	minutes := (offset % 3600) / 60
	gmt := fmt.Sprintf("GMT%s%02d%02d", sign, hours, minutes)

	// 括号内时区名:浏览器展开缩写为全名(PDT → Pacific Daylight Time)。
	tzShort := shortName(t)
	tzFull := fullTzName(tzShort)
	if tzFull == "" {
		tzFull = tzShort
	}

	return head + " " + gmt + " (" + tzFull + ")"
}

// shortName 提取 Go 当前 tz 的短名(避开 Zone() 的多返回值歧义)。
func shortName(t time.Time) string {
	name, _ := t.Zone()
	return name
}

// fullTzName 浏览器用全名;Go 用短名。映射常见 IANA/Windows 缩写 → 全名
// (无论是否 DST 都给全名,浏览器也是这样)。
func fullTzName(short string) string {
	tzMap := map[string]string{
		"PDT":    "Pacific Daylight Time",
		"PST":    "Pacific Standard Time",
		"EDT":    "Eastern Daylight Time",
		"EST":    "Eastern Standard Time",
		"CDT":    "Central Daylight Time",
		"CST":    "Central Standard Time",
		"MDT":    "Mountain Daylight Time",
		"MST":    "Mountain Standard Time",
		"BST":    "British Summer Time",
		"GMT":    "Greenwich Mean Time",
		"UTC":    "Coordinated Universal Time",
		"JST":    "Japan Standard Time",
		"KST":    "Korea Standard Time",
		"AEST":   "Australian Eastern Standard Time",
		"AEDT":   "Australian Eastern Daylight Time",
		"NZST":   "New Zealand Standard Time",
		"NZDT":   "New Zealand Daylight Time",
		"CST_CN": "China Standard Time",
	}
	return tzMap[short]
}
