// Package pow 实现 chatgpt.com sentinel 的 PoW 求解 (基于 Python 参考实现)。
//
// 三种 sentinel token:
//
//  1. RequirementsToken (gAAAAAC + base64(config) + ~S)
//     首次 sentinel/chat-requirements 的 p 字段。
//     config 是 18 元素 config,无 PoW (用 fixedRandom 填 config[3]/[9])。
//
//  2. ProofToken (gAAAAAB + base64(config) + ~S)
//     proofofwork.required=true 时的 p 字段。
//     config 18 元素,但 config[3]=nonce、config[9]=elapsedMs(整数);
//     哈希算法 FNV-1a 32-bit,fnv1a(seed + base64(config))[:len(difficulty)] <= difficulty。
//
//  3. SentinelToken (服务端返回的 token, 1500+ 字符 base64)
//     final / sentinel_token, 由 prepare/finalize 流程产生。
package prooftoken

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	mathrand "math/rand"
	"strconv"
	"time"
)

// mathRandNew 是 math/rand.New 的本地别名 (避免类型混淆)。
type mathRand = mathrand.Rand

func mathRandNew(seed int64) *mathRand {
	return mathrand.New(mathrand.NewSource(seed))
}

const (
	// PrefixRequirements RequirementsToken 前缀
	PrefixRequirements = "gAAAAAC"
	// PrefixProof ProofToken 前缀
	PrefixProof = "gAAAAAB"
	// Suffix 附加在 base64(config) 后面的分隔符
	Suffix = "~S"
)

// DefaultFlow 是 sentinel prepare/finalize 流程标识。
const DefaultFlow = "chatgpt"

// NavigatorProps 是 config[10] 候选 — 常见 navigator 原型方法 toString 表征
var navigatorProps = []string{
	"clearOriginJoinedAdInterestGroups−function clearOriginJoinedAdInterestGroups() { [native code] }",
	"canLoadAdAuctionFencedFrame−function canLoadAdAuctionFencedFrame() { [native code] }",
	"clipboard−[object Clipboard]",
	"getBattery−function getBattery() { [native code] }",
	"getGamepads−function getGamepads() { [native code] }",
	"javaEnabled−function javaEnabled() { [native code] }",
	"sendBeacon−function sendBeacon() { [native code] }",
	"vibrate−function vibrate() { [native code] }",
}

var windowKeys = []string{
	"requestIdleCallback", "webkitRequestAnimationFrame", "onfocus", "onblur",
}

// reactLetters 是 config[11] 随机 suffix 字符表
var reactLetters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

// Config 持有 p 字段生成所需的全部上下文 (对齐 Python BrowserSession 字段)。
type Config struct {
	DeviceID            string
	UserAgent           string
	Language            string
	Languages           string
	TimezoneString      string
	TimezoneLabel       string
	ScreenWidth         int
	ScreenHeight        int
	HardwareConcurrency int
	SentinelSV          string // SDK 版本, e.g. "20260423af3c"
	BuildID             string // 来自 chatgpt.com 页面的 data-build
	// 可选:固定 Math.random (用于测试)
	FixedRandom *float64
}

// NewConfig 用默认 Windows / zh-CN / Edge UA 配置构造。
// userAgent 为空时使用 Edge 147 (与 web2api buildProfileHeaders 一致)。
func NewConfig(userAgent string) *Config {
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36 Edg/147.0.0.0"
	}
	return &Config{
		DeviceID:            randomUUID(),
		UserAgent:           userAgent,
		Language:            "zh-CN",
		Languages:           "zh-CN,en,en-GB,en-US",
		TimezoneString:      "GMT+0800",
		TimezoneLabel:       "中国标准时间",
		ScreenWidth:         1920,
		ScreenHeight:        1080,
		HardwareConcurrency: 8,
		SentinelSV:          "20260423af3c",
		BuildID:             "prod-4987068829830ddc3ae6683bd4e633f61b79dec9",
	}
}

// randomUUID 生成 v4 UUID (与 crypto/rand 区分)。
func randomUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

var _ = strconv.Itoa // 防 unused import 警告

// dateToString 模拟 Chrome Date.prototype.toString() 输出。
func (c *Config) dateToString() string {
	// 简化: 假设 TimezoneString 已经是 "GMT+0800" / TimezoneLabel 是 "中国标准时间"
	now := time.Now().UTC()
	return now.Format("Mon Jan 02 2006 15:04:05 ") + c.TimezoneString + " (" + c.TimezoneLabel + ")"
}

// EncodeConfig 把 config 数组编码为 base64 字符串 (对齐 Python base64.b64encode(json_str.encode('utf-8'))).
func EncodeConfig(config []any) string {
	b, err := json.Marshal(config)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

// FNV1aHash FNV-1a 32 位哈希, 返回 8 位 hex (小写)。
// 对齐 Python fnv1a_hash + _imul。
func FNV1aHash(text string) string {
	const (
		fnvOffset = 2166136261
		fnvPrime  = 16777619
	)
	h := uint32(fnvOffset)
	for _, ch := range text {
		h ^= uint32(ch)
		h = imul32(h, fnvPrime)
	}
	h ^= h >> 16
	h = imul32(h, 2246822507)
	h ^= h >> 13
	h = imul32(h, 3266489909)
	h ^= h >> 16
	return fmt.Sprintf("%08x", h)
}

// imul32 模拟 JavaScript Math.imul (32 位整数乘法)。
func imul32(a, b uint32) uint32 {
	return (a * b) & 0xFFFFFFFF
}

// rngFloat 返回 Math.random() (若 FixedRandom 非空则返回 fixed 值)。
func (c *Config) rngFloat(rng *mathRand) float64 {
	if c.FixedRandom != nil {
		return *c.FixedRandom
	}
	return rng.Float64()
}

// buildBaseConfig 返回 18 元素 config (但 [3] / [9] / [13] 是占位, 由调用方设置)。
func (c *Config) buildBaseConfig(rng *mathRand, attempt *int, elapsedMs *float64) []any {
	dateStr := c.dateToString()

	nowMs := float64(time.Now().UnixMilli())
	var perfNow float64
	if c.FixedRandom != nil {
		// 固定场景: perfNow 取一个固定值
		perfNow = 1000.0
	} else {
		// 真实场景: perfNow 与 timeOrigin 自洽
		perfNow = 1000.0 + rng.Float64()*49999
	}
	timeOrigin := nowMs - perfNow

	// config[3] / config[9] 填充规则
	var c3, c9 any
	if attempt != nil {
		c3 = *attempt
	} else {
		c3 = c.rngFloat(rng)
	}
	if elapsedMs != nil {
		c9 = int(*elapsedMs)
	} else {
		c9 = c.rngFloat(rng)
	}

	// config[11] _reactListening 随机 suffix
	reactSuffix := make([]rune, 11)
	for i := range reactSuffix {
		reactSuffix[i] = reactLetters[rng.Intn(len(reactLetters))]
	}

	// config[12] window 随机 key
	windowKey := windowKeys[rng.Intn(len(windowKeys))]

	// config[5] currentScript.src
	scriptSrc := "https://sentinel.openai.com/sentinel/" + c.SentinelSV + "/sdk.js"

	config := []any{
		c.ScreenWidth + c.ScreenHeight, // [0]
		dateStr,                        // [1] Date.toString()
		int64(4294967296),              // [2] jsHeapSizeLimit
		c3,                             // [3] Math.random() / PoW nonce
		c.UserAgent,                    // [4]
		scriptSrc,                      // [5]
		nil,                            // [6] documentElement[data-build] (auth.openai.com 为 null)
		c.Language,                     // [7]
		c.Languages,                    // [8]
		c9,                             // [9] Math.random() / PoW elapsed
		navigatorProps[rng.Intn(len(navigatorProps))], // [10]
		"_reactListening" + string(reactSuffix),       // [11]
		windowKey,                                     // [12]
		perfNow,                                       // [13] performance.now()
		c.DeviceID,                                    // [14] sid
		"",                                            // [15] location.search
		c.HardwareConcurrency,                         // [16]
		timeOrigin,                                    // [17] performance.timeOrigin
		0, 0, 0, 0, 0, 0, 0,                           // [18]-[24] 全 0 (其他 navigator 探测)
	}
	return config
}

// GenerateRequirementsToken 生成首次 sentinel/req 的 p 字段值。
// 不需要 PoW,18 元素 config 用 Math.random() 填 [3]/[9]。
func (c *Config) GenerateRequirementsToken() string {
	rng := mathRandNew(time.Now().UnixNano())
	config := c.buildBaseConfig(rng, nil, nil)
	return PrefixRequirements + EncodeConfig(config) + Suffix
}

// SolveProofOfWork 按服务端挑战求解 proof token (gAAAAAB 前缀 + FNV-1a 哈希)。
func (c *Config) SolveProofOfWork(seed, difficulty string) string {
	if seed == "" || difficulty == "" {
		return PrefixProof + Suffix
	}
	startTime := time.Now().UnixMilli()
	rng := mathRandNew(time.Now().UnixNano())
	diffLen := len(difficulty)
	const maxIter = 500_000

	for i := 0; i < maxIter; i++ {
		elapsed := time.Now().UnixMilli() - startTime
		elapsedF := float64(elapsed)
		attempt := i
		config := c.buildBaseConfig(rng, &attempt, &elapsedF)
		// 同步 perfNow 让 timeOrigin 保持自洽
		config[13] = config[17].(float64) - float64(startTime) + elapsedF
		encoded := EncodeConfig(config)
		hashInput := seed + encoded
		hashResult := FNV1aHash(hashInput)
		if hashResult[:diffLen] <= difficulty {
			return PrefixProof + encoded + Suffix
		}
	}
	// 达到最大次数仍未找到,返回 fallback
	return PrefixProof + "wQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4D" + Suffix
}

// BackwardCompat 旧 API 兼容 (client-go 旧版代码依赖这些)。

// (c *Config).RequirementsToken 兼容 client-go 旧名 (返回 gAAAAAC + base64 + ~S)。
func (c *Config) RequirementsToken() string {
	return c.GenerateRequirementsToken()
}

// SolveProofToken 兼容 client-go 旧名 (接受 userAgent 参数)。
func SolveProofToken(seed, difficulty, userAgent string) string {
	c := NewConfig(userAgent)
	return c.SolveProofOfWork(seed, difficulty)
}

// BuildSentinelRequestBody 构造 sentinel/req 请求体 (JSON 字符串)。
func BuildSentinelRequestBody(p, deviceID, flow string) string {
	if flow == "" {
		flow = DefaultFlow
	}
	body := map[string]string{"p": p, "id": deviceID, "flow": flow}
	b, _ := json.Marshal(body)
	return string(b)
}

// SentinelTokenHeader 是 f/conversation 头里 openai-sentinel-token 的值 (JSON)。
// 字段含义:
//
//	p: prepare 阶段用的 p (config 编码)
//	t: turnstile token (无则空)
//	c: sentinel token (服务端返回)
//	id: deviceID
//	flow: 流程标识
type SentinelTokenHeader struct {
	P    string `json:"p"`
	T    string `json:"t"`
	C    string `json:"c"`
	ID   string `json:"id"`
	Flow string `json:"flow"`
}

// BuildSentinelTokenHeader 构造 openai-sentinel-token 请求头值 (JSON 字符串)。
func BuildSentinelTokenHeader(p, turnstileToken, sentinelToken, deviceID, flow string) string {
	if flow == "" {
		flow = DefaultFlow
	}
	h := SentinelTokenHeader{P: p, T: turnstileToken, C: sentinelToken, ID: deviceID, Flow: flow}
	b, _ := json.Marshal(h)
	return string(b)
}

var _ = strconv.Itoa // 防止 import 警告 (strconv 用于 ext)
