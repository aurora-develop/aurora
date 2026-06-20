package prooftoken

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	mathrand "math/rand"
	"strconv"
	"time"

	"aurora/internal/fingerprint"
	"aurora/util"
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
	// ErrorPrefix PoW 失败时的占位指纹前缀 (对齐 sdk.deob.pretty.js:292 /
	// 4813494d-lryin3horwb01cb5.js class constructor)。
	// 最终 token 格式: PrefixProof + ErrorPrefix + base64(JSON.stringify("e" or error)) + Suffix
	ErrorPrefix = "wQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4D"
	// DefaultErrorPayload 失败 fallback 默认 base64 内容 (= base64(JSON.stringify("e")) = "ImUi")。
	// 对齐 buildGenerateFailMessage(e) 里 String(e ?? "e") → NM("e")。
	DefaultErrorPayload = "ImUi"
)

// DefaultFlow 是 sentinel prepare/finalize 流程标识。
const DefaultFlow = "chatgpt"

// fingerprintSize 25 元素 config, 对齐新版 SDK 算法 (conversation.txt 2026-06 样本)。
const fingerprintSize = 25

// windowKeys 候选 [13] (Object.getOwnPropertyNames(window) 随机键)
var windowKeys = []string{
	"requestIdleCallback", "webkitRequestAnimationFrame", "onfocus", "onblur",
}

// reactLetters 是 [12] 随机 suffix 字符表
var reactLetters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

// Config 持有 p 字段生成所需的全部上下文 (对齐新版 BrowserSession 字段)。
type Config struct {
	DeviceID  string
	UserAgent string
	Language  string
	Languages string
	// Timezone IANA 时区名(如 "America/Los_Angeles"),传给
	// fingerprint.Options.Timezone。如果为空,fingerprint 用 Go 本地时区。
	Timezone            string
	ScreenWidth         int
	ScreenHeight        int
	HardwareConcurrency int
	SentinelSV          string // SDK 版本, e.g. "20260423af3c"
	BuildID             string // 来自 chatgpt.com 页面的 data-build
	// 可选:固定 Math.random (用于测试)
	FixedRandom *float64
}

// NewConfig 用默认 Windows / en-US / Chrome UA 配置构造(对齐 conversation.txt 2026-06 抓包)。
// userAgent 为空时使用 util.FixedUserAgent (与新版 SDK 抓包一致,与 chatgpt 包 UA 严格同步)。
func NewConfig(userAgent string) *Config {
	if userAgent == "" {
		userAgent = util.FixedUserAgent
	}
	return &Config{
		DeviceID:            randomUUID(),
		UserAgent:           userAgent,
		Language:            "en-US",
		Languages:           "en-US,en",
		Timezone:            "America/Los_Angeles",
		ScreenWidth:         1920,
		ScreenHeight:        1080,
		HardwareConcurrency: 16,
		SentinelSV:          "20260423af3c",
		BuildID:             "prod-497f333866796e100096ad083b51ca949d22e751",
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

// EncodeConfig 把 config 数组编码为 base64 字符串 (对齐 base64.b64encode(json_str.encode('utf-8')))。
func EncodeConfig(config []any) string {
	b, err := json.Marshal(config)
	if err != nil {
		return ""
	}
	return base64.StdEncoding.EncodeToString(b)
}

// EncodeString 把任意字符串 JSON 序列化后 base64 编码(对齐 SDK 的
// NM(String(e)) 行为:先把字符串当 JSON 值序列化,再 btoa + UTF-8)。
// 用在 PoW 失败 fallback 拼 base64(error)。
func EncodeString(s string) string {
	b, err := json.Marshal(s)
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

// envFlags 返回 [19-22] 的环境探测值 (Number("X" in window))。
// 当前 aurora 不是真浏览器,默认全部 0;后续如需模拟,可在 Config 里加字段。
func (c *Config) envFlags() [4]int {
	return [4]int{0, 0, 0, 0}
}

// buildConfig 构造 25 元素 fingerprint config (但 [3]/[9] 是占位, 由调用方设置)。
//
// 走 internal/fingerprint.Build25 拿到真实浏览器形态(对齐 deob_js/out.js 真值样本
// + conversation.txt 2026-06 抓包),然后只覆盖 PoW 的 [3] nonce 和 [9] elapsed 字段。
func (c *Config) buildConfig(rng *mathRand, attempt *int, elapsedMs *int64) []any {
	// 把 c 的字段映射成 fingerprint.Options;rng 注入保持 deterministic
	opts := fingerprint.Options{
		UserAgent:           c.UserAgent,
		Platform:            "Win32", // Config 暂时没存 platform,固定 Win32 跟 [17] 一致
		ScreenWidth:         c.ScreenWidth,
		ScreenHeight:        c.ScreenHeight,
		HardwareConcurrency: c.HardwareConcurrency,
		JSHeapSizeLimit:     4294967296,
		BuildID:             c.BuildID,
		Timezone:            c.Timezone, // IANA 名,空则 fingerprint fallback 用 local
		Rand:                rng,
	}
	// 如果 Languages 是 "en-US,en" 字符串形式,拆成 []string
	if c.Languages != "" {
		opts.Languages = splitLangList(c.Languages)
	} else {
		opts.Languages = []string{c.Language, "en"}
	}

	config := fingerprint.Build25(opts)

	// [3] / [9] 覆盖:PoW 阶段用 nonce(int) 和 elapsedMs(int64)
	if attempt != nil {
		config[3] = *attempt
	} else {
		// requirements 阶段:对齐 sdk.deob.pretty.js:413 `n[3] = 1`(固定 int 1,
		// 不是 Math.random)。requirements token 不跑 PoW,只是固定一个
		// 标记让服务端验设备形态。
		config[3] = 1
	}
	if elapsedMs != nil {
		config[9] = *elapsedMs
	} else {
		// requirements 阶段:[9] 是 performance.now() - t0 的一次性测时;
		// 这里不传 elapsedMs 时沿用 fingerprint 的 Math.random()(float),
		// 跟 SDK 行为一致(SDK 也不传)。
		config[9] = c.rngFloat(rng)
	}
	return config
}

// splitLangList 把 "en-US,en" 拆成 []string。
func splitLangList(s string) []string {
	out := []string{}
	cur := ""
	for _, r := range s {
		if r == ',' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// GenerateRequirementsToken 生成首次 sentinel/req 的 p 字段值 (gAAAAAC 前缀)。
//
// 对齐 sdk.deob.pretty.js:407-418 _generateRequirementsTokenAnswerBlocking:
//  1. 拿 fingerprint config(本包 buildConfig)
//  2. [3] = 1 (固定)
//  3. [9] = performance.now() - t0 (一次性测时,不循环)
//  4. base64(JSON.stringify(config)) → 返回
//  5. 失败 → errorPrefix + base64(error)  (errorPrefix = "wQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4D")
//
// 注意:requirements token **不跑 PoW**(proof token 才跑);这里只是
// 一次性拼一份带时间戳的 config 让服务端验设备形态。
func (c *Config) GenerateRequirementsToken() string {
	rng := mathRandNew(time.Now().UnixNano())
	startTime := time.Now()
	elapsed := time.Since(startTime).Milliseconds()
	config := c.buildConfig(rng, nil, &elapsed)
	config[3] = 1
	encoded := EncodeConfig(config)
	return PrefixRequirements + encoded + Suffix
}

// SolveProofOfWork 按服务端挑战求解 proof token (gAAAAAB 前缀 + FNV-1a 哈希)。
// [13]/[17] 时间自洽由 fingerprint.Build25 内部保证,这里不用再覆盖。
//
// 失败 fallback 格式(对齐 sdk.deob.pretty.js:329 + buildGenerateFailMessage:364-366):
//
//	"gAAAAAB" + ErrorPrefix + base64(JSON.stringify("e" or error)) + "~S"
//
// 500k 次未命中时 err=nil → 用 DefaultErrorPayload ("ImUi" = base64('"e"'))。
func (c *Config) SolveProofOfWork(seed, difficulty string) string {
	if seed == "" || difficulty == "" {
		return PrefixProof + Suffix
	}
	startTime := time.Now()
	rng := mathRandNew(time.Now().UnixNano())
	diffLen := len(difficulty)
	const maxIter = 500_000

	for i := 0; i < maxIter; i++ {
		nonce := i
		elapsed := time.Since(startTime).Milliseconds()
		config := c.buildConfig(rng, &nonce, &elapsed)
		encoded := EncodeConfig(config)
		hashInput := seed + encoded
		hashResult := FNV1aHash(hashInput)
		if hashResult[:diffLen] <= difficulty {
			return PrefixProof + encoded + Suffix
		}
	}
	// 达到最大次数仍未找到,返回 fallback (error=nil → 默认 "e")
	return PrefixProof + ErrorPrefix + DefaultErrorPayload + Suffix
}

// BuildFailToken 构造 PoW 失败 fallback token。
// errMessage 为空时使用 SDK 默认 "e"。
func BuildFailToken(errMessage string) string {
	if errMessage == "" {
		errMessage = "e"
	}
	return PrefixProof + ErrorPrefix + EncodeString(errMessage) + Suffix
}

// String 辅助:float64 → string (避免重复写 strconv.FormatFloat)。
func String(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
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
