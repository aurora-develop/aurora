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

// fingerprintSize 23 元素 config, 对齐新版 SDK 算法。
const fingerprintSize = 23

// windowKeys 候选 [13] (Object.getOwnPropertyNames(window) 随机键)
var windowKeys = []string{
	"requestIdleCallback", "webkitRequestAnimationFrame", "onfocus", "onblur",
}

// reactLetters 是 [12] 随机 suffix 字符表
var reactLetters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

// Config 持有 p 字段生成所需的全部上下文 (对齐新版 BrowserSession 字段)。
type Config struct {
	DeviceID            string
	UserAgent           string
	Language            string
	Languages           string
	// Timezone IANA 时区名(如 "America/Los_Angeles" / "Asia/Shanghai"),传给
	// fingerprint.Options.Timezone。如果为空,fingerprint 用 Go 本地时区。
	Timezone            string
	ScreenWidth         int
	ScreenHeight        int
	HardwareConcurrency int
	SentinelSV          string // SDK 版本, e.g. "20260219f9f6"
	BuildID             string // 来自 chatgpt.com 页面的 data-build
	// 可选:固定 Math.random (用于测试)
	FixedRandom *float64
}

// NewConfig 用默认 Windows / zh-CN / Edge UA 配置构造。
// userAgent 为空时使用 Chrome 147 (与新版 SDK 抓包一致)。
func NewConfig(userAgent string) *Config {
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36 Edg/147.0.0.0"
	}
	return &Config{
		DeviceID:            randomUUID(),
		UserAgent:           userAgent,
		Language:            "zh-CN",
		Languages:           "zh-CN,en,en-GB,en-US",
		Timezone:            "Asia/Shanghai",
		ScreenWidth:         1920,
		ScreenHeight:        1080,
		HardwareConcurrency: 8,
		SentinelSV:          "20260219f9f6",
		BuildID:             "",
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

// buildConfig 构造 23 元素 fingerprint config (但 [3]/[9] 是占位, 由调用方设置)。
//
// 走 internal/fingerprint 拿到真实浏览器形态(对齐 deob_js/out.js 真值样本),
// 然后只覆盖 PoW 的 [3] nonce 和 [9] elapsed 字段。
func (c *Config) buildConfig(rng *mathRand, attempt *int, elapsedMs *int64) []any {
	// 把 c 的字段映射成 fingerprint.Options;rng 注入保持 deterministic
	opts := fingerprint.Options{
		UserAgent:       c.UserAgent,
		Platform:        "Win32", // Config 暂时没存 platform,固定 Win32 跟 [17] 一致
		ScreenWidth:     c.ScreenWidth,
		ScreenHeight:    c.ScreenHeight,
		JSHeapSizeLimit: 4294967296,
		BuildID:         c.BuildID,
		// SessionID: Config 里的 DeviceID 不写到 [15] (浏览器真实为空);
		//          真要 device id,应该走 [15] 之外的字段(不影响 PoW 算)。
		Timezone: c.Timezone, // IANA 名,空则 fingerprint fallback 用 local
		Rand:     rng,
	}
	// 如果 Languages 是 "zh-CN,en,en-GB,en-US" 字符串形式,拆成 []string
	if c.Languages != "" {
		opts.Languages = splitLangList(c.Languages)
	} else {
		opts.Languages = []string{c.Language, "en"}
	}

	config := fingerprint.Build23(opts)

	// [3] / [9] 覆盖:PoW 阶段用 nonce(int) 和 elapsedMs(int64)
	if attempt != nil {
		config[3] = *attempt
	} else {
		// requirements 阶段:保留 fingerprint 生成的 Math.random() (float)
		// (新版 SDK requirements 也跑 mini-PoW,所以 [3] 是 nonce)
		config[3] = c.rngFloat(rng)
	}
	if elapsedMs != nil {
		config[9] = *elapsedMs
	} else {
		config[9] = c.rngFloat(rng)
	}
	return config
}

// splitLangList 把 "zh-CN,en,en-GB,en-US" 拆成 []string。
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

// GenerateRequirementsToken 生成首次 sentinel/req 的 p 字段值。
// 跑 mini-PoW (difficulty="0" 期望 16 次命中,1/16 概率);若 500_000 次未命中返回 fallback。
func (c *Config) GenerateRequirementsToken() string {
	rng := mathRandNew(time.Now().UnixNano())
	const minDifficulty = "0"
	for i := 0; i < 500_000; i++ {
		nonce := i
		config := c.buildConfig(rng, &nonce, nil)
		encoded := EncodeConfig(config)
		// mini-PoW 用 Math.random() 作 seed(SDK 行为,见 deob/out.js)
		seed := String(c.rngFloat(rng))
		if FNV1aHash(seed + encoded)[:len(minDifficulty)] <= minDifficulty {
			return PrefixRequirements + encoded + Suffix
		}
	}
	// fallback:跑一次 buildConfig 后直接返回
	return PrefixRequirements + EncodeConfig(c.buildConfig(rng, nil, nil)) + Suffix
}

// SolveProofOfWork 按服务端挑战求解 proof token (gAAAAAB 前缀 + FNV-1a 哈希)。
// [14]/[18] 时间自洽由 fingerprint.Build23 内部保证,这里不用再覆盖。
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
	// 达到最大次数仍未找到,返回 fallback
	return PrefixProof + "wQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4D" + Suffix
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
