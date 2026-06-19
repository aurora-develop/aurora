// Package fingerprint — Build25 扩展。
//
// 新版 chatgpt sentinel SDK (2026-04+ / 2026-06 抓包样本) 把 fingerprint
// config 从 23 元素扩到 25 元素:多了一个 navigator 探测字段 + 多 3 个
// "X in window" 探测(尾部 7 个 0)。Build25 生成 25 元素格式;老 Build23
// 保持不变(向后兼容)。
package fingerprint

import (
	"fmt"
	"math/rand"
	"time"
)

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

// pickNavigatorProbe 从 CommonNavigatorProbes 随机挑一个。
func pickNavigatorProbe(r *rand.Rand) string {
	if len(CommonNavigatorProbes) == 0 {
		return "geolocation−[object Geolocation]"
	}
	return CommonNavigatorProbes[r.Intn(len(CommonNavigatorProbes))]
}

// Build25 生成 25 元素 fingerprint 数组(顺序/类型严格按浏览器真实)。
// 对齐新版 SDK 抓包 (conversation.txt 2026-06 样本):
//
//	[0]  String(screen.w + screen.h)         string
//	[1]  Date.toString()                     string (PDT/EST 等带时区名)
//	[2]  String(jsHeapSizeLimit)             string
//	[3]  Math.random() / nonce (int)         number
//	[4]  Math.random()                       number
//	[5]  navigator.userAgent                 string
//	[6]  currentScript.src                   string
//	[7]  documentElement[data-build]         string
//	[8]  navigator.language                  string
//	[9]  Math.random() / elapsedMs (int)     number
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
// nonce/elapsed 由调用方注入 [3]/[9](requirements 阶段也是 nonce)。
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

	// [0] String(screen.w + screen.h) — 注意:字符串!
	screenSum := fmt.Sprintf("%d", w+h)

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
		} else if CommonBuildIDs != "" {
			buildID = CommonBuildIDs
		}
	}

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

	// [14] deviceID — 留空(浏览器真实情况,准备阶段由调用方覆盖)
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

	// [18-24] 7 个 "X in window" 探测(对齐 conversation.txt 2026-06 抓包):
	//   Number("X" in window) 返回 0/1;真值 Chrome 上 cache/data 至少一项为 1。
	// 默认全 0 偏保守,改用 envFlags() 给的 4 项 + 后 3 项沿用 0;首项给 cache=1 增加真实性。
	env := envFlags(Options{
		HasAI:             boolPtr(true),
		HasInstallTrigger: boolPtr(false),
		HasSolana:         boolPtr(false),
		HasTextEncoder:    boolPtr(true),
	})
	windowProbes := [7]int{env[0], env[1], env[2], env[3], 0, 0, 0}

	config := []any{
		screenSum,        // [0]  string: screen.w+h
		dateStr,          // [1]  string: Date.toString()
		jsHeapStr,        // [2]  string: jsHeapSizeLimit
		0,                // [3]  nonce (caller 覆盖)
		r4,               // [4]  Math.random()
		ua,               // [5]  navigator.userAgent
		scriptSrc,        // [6]  <script src> 随机
		buildID,          // [7]  c/<build>/_/ build id
		primaryLang,      // [8]  navigator.language
		0,                // [9]  elapsed ms (caller 覆盖)
		navigatorProbe,   // [10] "X in navigator" 探测
		docKey,           // [11] Object.keys(document)
		winKey,           // [12] window 随机键
		perfNow,          // [13] performance.now()
		deviceID,         // [14] device_id (caller 覆盖)
		searchJoined,     // [15] URLSearchParams keys
		hwConc,           // [16] hardwareConcurrency
		timeOrigin,       // [17] performance.timeOrigin
		windowProbes[0],  // [18] "X in window" 探测 (1/7)
		windowProbes[1],  // [19] "X in window" 探测 (2/7)
		windowProbes[2],  // [20] "X in window" 探测 (3/7)
		windowProbes[3],  // [21] "X in window" 探测 (4/7)
		windowProbes[4],  // [22] "X in window" 探测 (5/7)
		windowProbes[5],  // [23] "X in window" 探测 (6/7)
		windowProbes[6],  // [24] "X in window" 探测 (7/7)
	}
	_ = platform // 新版 SDK 不再用 platform 字段
	return config
}

// boolPtr 是 *bool 简写(给 envFlags() 用)。
func boolPtr(b bool) *bool { return &b }
