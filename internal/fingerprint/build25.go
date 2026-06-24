// Package fingerprint — Build25 扩展。
//
// 新版 chatgpt sentinel SDK (2026-04+ / 2026-06 抓包样本) 把 fingerprint
// config 从 23 元素扩到 25 元素:多了一个 navigator 探测字段 + 多 3 个
// "X in window" 探测(尾部 7 个 0)。Build25 生成 25 元素格式;老 Build23
// 保持不变(向后兼容)。
package fingerprint

import (
	"math/rand"
	"strings"
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

	// [18-24] 7 个 "X in window" 探测(对齐 2026-06-24 chatgpt.com 浏览器抓包):
	//   浏览器全部返回 0 (Number("X" in window) → 0)。
	//   真浏览器 Chrome 上其他探测可能为 1,但 sentinel SDK 只检查这 7 个,
	//   且实测全部为 0。
	windowProbes := [7]int{0, 0, 0, 0, 0, 0, 0}

	// [5] currentScript.src — 浏览器始终传 SDK 脚本 URL，不是 null
	// 对齐 2026-06-24 chatgpt.com 抓包
	sdkScript := "https://chatgpt.com/backend-api/sentinel/sdk.js"

	config := []any{
		screenSum,        // [0]  number: screen.w+h
		dateStr,          // [1]  string: Date.toString()
		jsHeap,           // [2]  number: jsHeapSizeLimit
		0,                // [3]  nonce (caller 覆盖)
		ua,               // [4]  navigator.userAgent
		sdkScript,        // [5]  SDK script URL (NOT null!)
		buildID,          // [6]  build id
		primaryLang,      // [7]  navigator.language
		langsStr,         // [8]  navigator.languages (逗号分隔)
		r4,               // [9]  Math.random() / elapsed (caller 覆盖)
		navigatorProbe,   // [10] "X in navigator" 探测
		docKey,           // [11] Object.keys(document)
		winKey,           // [12] window 随机键
		perfNow,          // [13] performance.now()
		deviceID,         // [14] device_id (caller 覆盖)
		searchJoined,     // [15] URLSearchParams keys
		hwConc,           // [16] hardwareConcurrency
		timeOrigin,       // [17] performance.timeOrigin
		windowProbes[0],  // [18]
		windowProbes[1],  // [19]
		windowProbes[2],  // [20]
		windowProbes[3],  // [21]
		windowProbes[4],  // [22]
		windowProbes[5],  // [23]
		windowProbes[6],  // [24]
	}
	_ = platform // 新版 SDK 不再用 platform 字段
	return config
}
