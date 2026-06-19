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

// CommonLanguages 真实浏览器 navigator.languages 组合。
var CommonLanguages = [][]string{
	{"en-US", "en"},
	{"zh-CN", "en", "en-GB", "en-US"},
	{"en-GB", "en", "en-US"},
	{"de-DE", "de", "en-US", "en"},
	{"ja-JP", "en-US", "en"},
	{"fr-FR", "fr", "en-US", "en"},
	{"es-ES", "es", "en-US", "en"},
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

// CommonScripts 真实网页上可能存在的 <script src> 列表(从 chatgpt.com 抓包)。
// [6] 从这里随机挑。
var CommonScripts = []string{
	"https://connect.facebook.net/en_US/fbevents.js",
	"https://www.googletagmanager.com/gtm.js",
	"https://www.google-analytics.com/analytics.js",
	"https://www.google.com/recaptcha/api.js",
	"https://cdn.oaistatic.com/_next/static/chunks/main-app.js",
	"https://chatgpt.com/sentinel/20260423af3c/sdk.js",
	"https://cdn.oaistatic.com/assets/relay.js",
	"https://chatgpt.com/cdn-cgi/scripts/.../cloudflare-static/email-decode.min.js",
}

// CommonBuildIDs 真实 build id 样本(c/<build>/_/ 模式里那个 build)。
// [7] 从这里随机挑或根据 [6] 推断。
var CommonBuildIDs = "prod-ab8a6348980a3e1d771c463b9f4f3e4e584f2769"

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
	// PageOpenedSeconds 页面打开多久(影响 [14] performance.now())
	PageOpenedSeconds float64
	// BuildID 强制 [7] 取这个;空则从 CommonBuildIDs 随机
	BuildID string
	// SessionID [15] 强制值;空则空(浏览器真实情况)
	SessionID string
	// RandomScript [6] 强制值;空则从 CommonScripts 随机
	RandomScript string
	// RandomDocKey [12] 强制值;空则从 CommonDocumentKeys 随机
	RandomDocKey string
	// RandomWindowKey [13] 强制值;空则从 CommonWindowKeys 随机
	RandomWindowKey string
	// EnvFlags [19-22];空则用默认(ai=0, InstallTrigger=0, solana=0, TextEncoder=1)
	EnvFlags [4]int
	// SetEnvFlags 为 true 时使用 EnvFlags;false 时用上面默认值
	SetEnvFlags bool
	// JSHeapSizeLimit 字节;默认 4294967296
	JSHeapSizeLimit int64
	// HasTextEncoder / HasAI / HasInstallTrigger / HasSolana 单独控制(优先生效)
	HasTextEncoder    *bool
	HasAI             *bool
	HasInstallTrigger *bool
	HasSolana         *bool
	// Rand 随机源(测试用;nil 用包级 rand)
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

// Build23 生成 23 元素 fingerprint 数组(顺序/类型严格按浏览器真实)。
// nonce/elapsed 是 PoW 阶段由调用方注入,这里传 0 占位。
func Build23(opts Options) []any {
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

	// [0] String(screen.w + screen.h) — 注意:字符串!
	screenSum := fmt.Sprintf("%d", w+h)

	// [1] new Date().toString() — 浏览器格式:
	//   "Fri Jun 19 2026 04:38:41 GMT-0700 (Pacific Daylight Time)"
	// Go time.Time.String() 给的是 "2026-06-19 04:38:41 -0700 PDT" — 不一样
	var dateStr string
	if opts.Timezone != "" {
		if loc, err := time.LoadLocation(opts.Timezone); err == nil {
			dateStr = jsDateToString(time.Now().In(loc))
		}
	}
	if dateStr == "" {
		dateStr = jsDateToString(time.Now())
	}

	// [2] String(jsHeapSizeLimit) — 字符串!
	jsHeap := opts.JSHeapSizeLimit
	if jsHeap == 0 {
		jsHeap = 4294967296
	}
	jsHeapStr := fmt.Sprintf("%d", jsHeap)

	// [4] Math.random()
	r4 := r.Float64()

	// [6] <script src> 随机挑
	scriptSrc := opts.RandomScript
	if scriptSrc == "" && len(CommonScripts) > 0 {
		scriptSrc = CommonScripts[r.Intn(len(CommonScripts))]
	}

	// [7] c/<build>/_/ 匹配
	buildID := opts.BuildID
	if buildID == "" {
		// 优先从 scriptSrc 里匹配 c/<id>/_/ 模式
		matched := extractBuildIDFromScript(scriptSrc)
		if matched != "" {
			buildID = matched
		} else if len(CommonBuildIDs) > 0 {
			buildID = CommonBuildIDs
		}
	}

	// [10] navigator.languages — 数组
	langsArr := make([]any, len(langs))
	for i, l := range langs {
		langsArr[i] = l
	}

	// [11] Math.random()
	r11 := r.Float64()

	// [12] Object.keys(document) 随机
	docKey := opts.RandomDocKey
	if docKey == "" && len(CommonDocumentKeys) > 0 {
		docKey = CommonDocumentKeys[r.Intn(len(CommonDocumentKeys))]
	}

	// [13] Object.getOwnPropertyNames(window) 随机
	winKey := opts.RandomWindowKey
	if winKey == "" && len(CommonWindowKeys) > 0 {
		winKey = CommonWindowKeys[r.Intn(len(CommonWindowKeys))]
	}

	// [14] performance.now() — 页面打开 X 秒后
	perfNow := opts.PageOpenedSeconds * 1000
	if perfNow < 0 {
		perfNow = 0
	}

	// [15] sessionStorage.sid — 浏览器真实为空
	sid := opts.SessionID

	// [16] URLSearchParams(location.search) join — 通常空
	searchJoined := ""

	// [18] performance.timeOrigin — Unix ms 时间戳
	timeOrigin := float64(time.Now().UnixMilli()) - perfNow

	// [19-22] env flags
	env := envFlags(opts)

	config := []any{
		screenSum,    // [0]  string: screen.w+h
		dateStr,      // [1]  string: Date.toString()
		jsHeapStr,    // [2]  string: jsHeapSizeLimit
		0,            // [3]  nonce (caller 覆盖)
		r4,           // [4]  Math.random()
		ua,           // [5]  navigator.userAgent
		scriptSrc,    // [6]  <script src> 随机
		buildID,      // [7]  c/<build>/_/ build id
		primaryLang,  // [8]  navigator.language
		0,            // [9]  elapsed ms (caller 覆盖)
		langsArr,     // [10] navigator.languages (array)
		r11,          // [11] Math.random()
		docKey,       // [12] Object.keys(document)
		winKey,       // [13] window 随机键
		perfNow,      // [14] performance.now()
		sid,          // [15] sessionStorage.sid
		searchJoined, // [16] URLSearchParams keys
		platform,     // [17] navigator.platform
		timeOrigin,   // [18] performance.timeOrigin
		env[0],       // [19] "ai" in window
		env[1],       // [20] "InstallTrigger" in window
		env[2],       // [21] "solana" in window
		env[3],       // [22] "TextEncoder" in window
	}
	return config
}

// envFlags 决定 [19-22] 的真值。HasXxx 指针优先;否则用 opts.EnvFlags(若 SetEnvFlags)。
func envFlags(opts Options) [4]int {
	if opts.HasAI != nil {
		b := *opts.HasAI
		if b {
			opts.EnvFlags[0] = 1
		} else {
			opts.EnvFlags[0] = 0
		}
	}
	if opts.HasInstallTrigger != nil {
		opts.EnvFlags[1] = boolToInt(*opts.HasInstallTrigger)
	}
	if opts.HasSolana != nil {
		opts.EnvFlags[2] = boolToInt(*opts.HasSolana)
	}
	if opts.HasTextEncoder != nil {
		opts.EnvFlags[3] = boolToInt(*opts.HasTextEncoder)
	}
	if !opts.SetEnvFlags {
		// 默认:ai=0, InstallTrigger=0, solana=0, TextEncoder=1
		return [4]int{0, 0, 0, 1}
	}
	return opts.EnvFlags
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// extractBuildIDFromScript 从 <script src> 里按 c/<id>/_/ 模式抽 build。
// 例如: https://chatgpt.com/_next/static/c/prod-ab8a.../_/...  → "prod-ab8a..."
func extractBuildIDFromScript(src string) string {
	if src == "" {
		return ""
	}
	idx := strings.Index(src, "/c/")
	if idx < 0 {
		return ""
	}
	rest := src[idx+3:] // 跳过 "/c/"
	endIdx := strings.Index(rest, "/_/")
	if endIdx < 0 {
		return ""
	}
	return rest[:endIdx]
}

// jsDateToString 模拟浏览器 Date.prototype.toString() 格式:
//
//	"Fri Jun 19 2026 04:38:41 GMT-0700 (Pacific Daylight Time)"
//
// 对齐 V8/Blink/SpiderMonkey 的输出(包括 DST 名字)。
func jsDateToString(t time.Time) string {
	// t.Format 用 Go 参考时间: Mon Jan 2 2006 15:04:05 MST
	// 浏览器输出:                       Fri Jun 19 2026 04:38:41 GMT-0700 (Pacific Daylight Time)
	// 第一段 Mon Jan 2 2006 15:04:05 → "Fri Jun 19 2026 04:38:41"
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
	// Go 的 time.Zone() 返回 (短名, offset秒) — 不直接说是否 DST。
	// 检测 DST:跟 UTC 同时间如果 offset 不同,说明当前是 DST。
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
// (无论是否 DST 都给"夏令时全名"或"标准时全名",浏览器也是这样)。
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
