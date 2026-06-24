// Package fingerprint — Build25 浏览器指纹。
//
// 对齐 2026-06 chatgpt.com sentinel SDK 抓包: 25 元素 fingerprint config,
// 用于 /chat-requirements/prepare、/sentinel/req 和 PoW proof token。
//
// 所有指纹数据池统一来自 internal/browserfp,本包只做 Build25 算法。
package fingerprint

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"aurora/internal/browserfp"
)

// Options 浏览器指纹模拟选项。所有字段由调用方从 browserfp 传入。
type Options struct {
	UserAgent           string
	Languages           []string
	Platform            string
	ScreenWidth         int
	ScreenHeight        int
	HardwareConcurrency int
	Timezone            string
	PageOpenedSeconds   float64
	BuildID             string
	RandomDocKey        string
	RandomWindowKey     string
	JSHeapSizeLimit     int64
	Rand                *rand.Rand
}

// DefaultOptions 返回测试用默认配置。
func DefaultOptions() Options {
	return Options{
		UserAgent:           browserfp.UserAgents[0],
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

// Build25 生成 25 元素 fingerprint 数组(顺序/类型严格按浏览器真实)。
// 对齐 2026-06-24 chatgpt.com 浏览器 sentinel/chat-requirements/prepare 抓包:
//
//	[0]  screen.w + screen.h                 number
//	[1]  Date.toString()                     string (带时区名)
//	[2]  jsHeapSizeLimit                     number
//	[3]  nonce (caller 覆盖)
//	[4]  navigator.userAgent                 string
//	[5]  script URL (hPt 随机选)             string
//	[6]  buildID (data-build 属性)           string
//	[7]  navigator.language                  string
//	[8]  navigator.languages (逗号分隔)       string
//	[9]  Math.random() / elapsedMs (caller 覆盖)
//	[10] navigator probe (hPt 随机选)        string
//	[11] document key (hPt 随机选)           string
//	[12] window key (hPt 随机选)             string
//	[13] performance.now()                   number
//	[14] device_id (caller 覆盖)             string
//	[15] location.search joined              string
//	[16] hardwareConcurrency                 number
//	[17] performance.timeOrigin              number
//	[18-24] "X in window" 检查               number (7 个, 默认 0)
func Build25(opts Options) []any {
	r := opts.Rand
	if r == nil {
		r = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	ua := opts.UserAgent
	if ua == "" {
		ua = browserfp.UserAgents[r.Intn(len(browserfp.UserAgents))]
	}

	langs := opts.Languages
	if len(langs) == 0 {
		slices := browserfp.LanguageSlices()
		langs = slices[r.Intn(len(slices))]
	}
	primaryLang := langs[0]

	platform := opts.Platform
	if platform == "" {
		platform = browserfp.Platforms[r.Intn(len(browserfp.Platforms))]
	}

	w, h := opts.ScreenWidth, opts.ScreenHeight
	if w == 0 {
		w = 1920
	}
	if h == 0 {
		h = 1080
	}

	screenSum := w + h

	var dateStr string
	if opts.Timezone != "" {
		if loc, err := time.LoadLocation(opts.Timezone); err == nil {
			dateStr = jsDateToString(time.Now().In(loc))
		}
	}
	if dateStr == "" {
		dateStr = jsDateToString(time.Now())
	}

	jsHeap := opts.JSHeapSizeLimit
	if jsHeap == 0 {
		jsHeap = 4294967296
	}

	r4 := r.Float64()
	buildID := opts.BuildID
	langsStr := strings.Join(langs, ",")
	navigatorProbe := browserfp.NavigatorProbes[r.Intn(len(browserfp.NavigatorProbes))]

	docKey := opts.RandomDocKey
	if docKey == "" {
		docKey = browserfp.DocumentKeys[r.Intn(len(browserfp.DocumentKeys))]
	}

	winKey := opts.RandomWindowKey
	if winKey == "" {
		winKey = browserfp.WindowKeys[r.Intn(len(browserfp.WindowKeys))]
	}

	perfNow := opts.PageOpenedSeconds * 1000
	if perfNow < 0 {
		perfNow = 0
	}

	deviceID := ""
	searchJoined := ""

	hwConc := opts.HardwareConcurrency
	if hwConc == 0 {
		hwConc = 8
	}

	timeOrigin := float64(time.Now().UnixMilli()) - perfNow
	// [5] script URL: hPt 随机选一个 (对齐新版 SDK 行为)
	scriptURL := browserfp.ScriptURLs[r.Intn(len(browserfp.ScriptURLs))]
	// [18-24] window 属性探测 (对齐新版 SDK: ai, createPRNG, cache, data, solana, dump, InstallTrigger)
	windowProbes := [7]int{0, 0, 0, 0, 0, 0, 0}

	config := []any{
		screenSum,       // [0]
		dateStr,         // [1]
		jsHeap,          // [2]
		0,               // [3] nonce (caller 覆盖)
		ua,              // [4]
		scriptURL,       // [5] hPt 随机选 script URL
		buildID,         // [6]
		primaryLang,     // [7]
		langsStr,        // [8]
		r4,              // [9] (caller 覆盖)
		navigatorProbe,  // [10] hPt 随机选 navigator 属性
		docKey,          // [11] hPt 随机选 document key
		winKey,          // [12] hPt 随机选 window key
		perfNow,         // [13]
		deviceID,        // [14] (caller 覆盖)
		searchJoined,    // [15]
		hwConc,          // [16]
		timeOrigin,      // [17]
		windowProbes[0], // [18] ai in window
		windowProbes[1], // [19] createPRNG in window
		windowProbes[2], // [20] cache in window
		windowProbes[3], // [21] data in window
		windowProbes[4], // [22] solana in window
		windowProbes[5], // [23] dump in window
		windowProbes[6], // [24] InstallTrigger in window
	}
	_ = platform
	return config
}

// ─── 时区辅助 ──────────────────────────────────────────────────────────────

func jsDateToString(t time.Time) string {
	head := t.Format("Mon Jan 2 2006 15:04:05")
	_, offset := t.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	hours := offset / 3600
	minutes := (offset % 3600) / 60
	gmt := fmt.Sprintf("GMT%s%02d%02d", sign, hours, minutes)
	tzShort := shortName(t)
	tzFull := fullTzName(tzShort)
	if tzFull == "" {
		tzFull = tzShort
	}
	return head + " " + gmt + " (" + tzFull + ")"
}

func shortName(t time.Time) string {
	name, _ := t.Zone()
	return name
}

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
