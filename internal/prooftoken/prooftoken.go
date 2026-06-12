package prooftoken

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/sha3"
)

const (
	powPrefixRequirements = "gAAAAAC"
	powPrefixProof        = "gAAAAAB"

	requirementsDifficulty = "0fffff"

	maxRequirementsIter = 500_000
	maxProofIter        = 100_000

	powFallback = "gAAAAABwQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4D"
)

var (
	powCores   = []int{16, 24, 32}
	powScreens = []int{3000, 4000, 6000}

	powNavKeys = []string{
		"webdriver-false", "vendor-Google Inc.", "cookieEnabled-true",
		"pdfViewerEnabled-true", "hardwareConcurrency-32",
		"language-zh-CN", "mimeTypes-[object MimeTypeArray]",
		"userAgentData-[object NavigatorUAData]",
	}
	powWinKeys = []string{
		"innerWidth", "innerHeight", "devicePixelRatio", "screen",
		"chrome", "location", "history", "navigator",
	}

	powReactListeners = []string{"_reactListeningcfilawjnerp", "_reactListening9ne2dfo1i47"}
	powProofEvents    = []string{"alert", "ontransitionend", "onprogress"}

	// perfCounter 模拟浏览器 performance.counter() 的单调递增(亚秒级)。
	perfCounter uint64
)

// POWConfig 是 18 元素的客户端指纹数组(requirements_token 用)。
type POWConfig struct {
	userAgent string
	arr       [18]interface{}
}

// NewPOWConfig 构造一个随机化的客户端指纹,用于 requirements + proof 两种场景。
func NewPOWConfig(userAgent string) *POWConfig {
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36 Edg/147.0.0.0"
	}
	//nolint:gosec // 非加密用途
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	now := time.Now().UTC()
	timeStr := now.Format("Mon Jan 02 2006 15:04:05") + " GMT+0000 (UTC)"
	perf := float64(atomic.AddUint64(&perfCounter, 1)) + rng.Float64()

	c := &POWConfig{userAgent: userAgent}
	c.arr = [18]interface{}{
		powCores[rng.Intn(len(powCores))] + powScreens[rng.Intn(len(powScreens))], // 0
		timeStr,       // 1
		nil,           // 2
		rng.Float64(), // 3 - 迭代会覆盖
		userAgent,     // 4
		nil,           // 5
		"dpl=1440a687921de39ff5ee56b92807faaadce73f13", // 6
		"en-US",                               // 7
		"en-US,zh-CN",                         // 8
		0,                                     // 9 - 迭代会覆盖
		powNavKeys[rng.Intn(len(powNavKeys))], // 10
		"location",                            // 11
		powWinKeys[rng.Intn(len(powWinKeys))], // 12
		perf,                                  // 13
		randomUUID(rng),                       // 14
		"",                                    // 15
		8,                                     // 16
		now.Unix(),                            // 17
	}
	return c
}

// RequirementsToken 生成 /sentinel/chat-requirements 的 "p" 字段值。
// 对齐 gen_image.py.get_requirements_token:固定难度 0fffff,前缀 gAAAAAC。
func (c *POWConfig) RequirementsToken() string {
	//nolint:gosec
	seed := strconv.FormatFloat(rand.Float64(), 'f', -1, 64)
	b64, ok := c.solveRequirements(seed, requirementsDifficulty)
	if !ok {
		return powPrefixRequirements + powFallback +
			base64.StdEncoding.EncodeToString([]byte(`"`+seed+`"`))
	}
	return powPrefixRequirements + b64
}

// solveRequirements 高性能迭代:预拼 JSON 的三段字节前缀,只在内循环拼 d1/d2。
// 严格对齐 gen_image.py._generate_answer。
func (c *POWConfig) solveRequirements(seed, difficulty string) (string, bool) {
	target, err := hex.DecodeString(difficulty)
	if err != nil {
		return "", false
	}
	diffLen := len(difficulty) // 字符数(与 Python 对齐)

	// 预拼 p1/p2/p3。config[3] 和 config[9] 位置留给迭代器。
	arr := c.arr
	// p1 = json(arr[:3])[:-1] + ","
	head, _ := json.Marshal([]interface{}{arr[0], arr[1], arr[2]})
	p1 := append(head[:len(head)-1:len(head)-1], ',')

	mid, _ := json.Marshal([]interface{}{arr[4], arr[5], arr[6], arr[7], arr[8]})
	// p2 = "," + json(arr[4:9])[1:-1] + ","
	p2 := make([]byte, 0, len(mid)+2)
	p2 = append(p2, ',')
	p2 = append(p2, mid[1:len(mid)-1]...)
	p2 = append(p2, ',')

	tail, _ := json.Marshal([]interface{}{
		arr[10], arr[11], arr[12], arr[13], arr[14], arr[15], arr[16], arr[17],
	})
	// p3 = "," + json(arr[10:])[1:]  => "," + "element1,...,elementN]"
	p3 := make([]byte, 0, len(tail)+1)
	p3 = append(p3, ',')
	p3 = append(p3, tail[1:]...)

	hasher := sha3.New512()
	seedB := []byte(seed)
	buf := make([]byte, 0, len(p1)+32+len(p2)+16+len(p3))
	b64buf := make([]byte, base64.StdEncoding.EncodedLen(cap(buf)))

	for i := 0; i < maxRequirementsIter; i++ {
		d1 := strconv.Itoa(i)
		d2 := strconv.Itoa(i >> 1)

		buf = buf[:0]
		buf = append(buf, p1...)
		buf = append(buf, d1...)
		buf = append(buf, p2...)
		buf = append(buf, d2...)
		buf = append(buf, p3...)

		n := base64.StdEncoding.EncodedLen(len(buf))
		if cap(b64buf) < n {
			b64buf = make([]byte, n)
		}
		b64buf = b64buf[:n]
		base64.StdEncoding.Encode(b64buf, buf)

		hasher.Reset()
		hasher.Write(seedB)
		hasher.Write(b64buf)
		sum := hasher.Sum(nil)

		// Python: h[:diff_len] <= target
		// diff_len 是字符数(6),target 是字节(3)。Python bytes cmp 按短的逐字节比较。
		// 这里保持等价:取 min(len(target), len(sum)) 字节比较。
		n2 := diffLen
		if n2 > len(sum) {
			n2 = len(sum)
		}
		cmpLen := n2
		if cmpLen > len(target) {
			cmpLen = len(target)
		}
		if bytes.Compare(sum[:cmpLen], target[:cmpLen]) <= 0 {
			return string(b64buf), true
		}
	}
	return "", false
}

// SolveProofToken 按服务端挑战求解 proof token(header 用,前缀 gAAAAAB)。
// 迁移自 gen_image.py.generate_proof_token 的轻量 13 元素 config。
func SolveProofToken(seed, difficulty, userAgent string) string {
	if seed == "" || difficulty == "" {
		return ""
	}
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36 Edg/147.0.0.0"
	}
	//nolint:gosec
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	screen := powScreens[rng.Intn(len(powScreens))] * (1 << rng.Intn(3)) // *1/2/4

	timeStr := time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 GMT")

	proofConfig := []interface{}{
		screen, // 0
		timeStr,
		nil,
		0, // 3 - 迭代
		userAgent,
		"https://tcr9i.chat.openai.com/v2/35536E1E-65B4-4D96-9D97-6ADB7EFF8147/api.js",
		"dpl=1440a687921de39ff5ee56b92807faaadce73f13",
		"en",
		"en-US",
		nil,
		"plugins-[object PluginArray]",
		powReactListeners[rng.Intn(len(powReactListeners))],
		powProofEvents[rng.Intn(len(powProofEvents))],
	}

	diffLen := len(difficulty)
	hasher := sha3.New512()
	for i := 0; i < maxProofIter; i++ {
		proofConfig[3] = i
		raw, err := json.Marshal(proofConfig)
		if err != nil {
			continue
		}
		b64 := base64.StdEncoding.EncodeToString(raw)
		hasher.Reset()
		hasher.Write([]byte(seed + b64))
		sum := hasher.Sum(nil)
		hexStr := hex.EncodeToString(sum)
		if strings.Compare(hexStr[:diffLen], difficulty) <= 0 {
			return powPrefixProof + b64
		}
	}
	return powPrefixProof + powFallback +
		base64.StdEncoding.EncodeToString([]byte(`"`+seed+`"`))
}

func randomUUID(rng *rand.Rand) string {
	var b [16]byte
	_, _ = rng.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Turnstile solver types
type orderedMap struct {
	keys   []string
	values map[string]any
}

func newOrderedMap() *orderedMap {
	return &orderedMap{values: make(map[string]any)}
}

func (m *orderedMap) add(key string, value any) {
	if _, ok := m.values[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

func (m *orderedMap) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, key := range m.keys {
		if i > 0 {
			buf.WriteString(", ")
		}
		keyBytes, _ := json.Marshal(key)
		valueBytes, _ := json.Marshal(m.values[key])
		buf.Write(keyBytes)
		buf.WriteString(": ")
		buf.Write(valueBytes)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

type vmFunc func(args ...any)

// Solve generates the turnstile token using the dx challenge and source p
func Solve(dx string, p string) string {
	decoded, err := base64.StdEncoding.DecodeString(dx)
	if err != nil {
		return ""
	}
	var tokenList []any
	if err := json.Unmarshal([]byte(xorString(string(decoded), p)), &tokenList); err != nil {
		return ""
	}
	processMap := map[float64]any{}
	startTime := time.Now()
	result := ""
	processMap[1] = vmFunc(func(args ...any) {
		e, t := num(args, 0), num(args, 1)
		processMap[e] = xorString(turnstileString(processMap[e]), turnstileString(processMap[t]))
	})
	processMap[2] = vmFunc(func(args ...any) {
		processMap[num(args, 0)] = value(args, 1)
	})
	processMap[3] = vmFunc(func(args ...any) {
		result = base64.StdEncoding.EncodeToString([]byte(turnstileString(value(args, 0))))
	})
	processMap[5] = vmFunc(func(args ...any) {
		e, t := num(args, 0), num(args, 1)
		current := processMap[e]
		incoming := processMap[t]
		if list, ok := asList(current); ok {
			processMap[e] = append(list, incoming)
			return
		}
		if isStringOrNumber(current) || isStringOrNumber(incoming) {
			processMap[e] = turnstileString(current) + turnstileString(incoming)
			return
		}
		processMap[e] = "NaN"
	})
	processMap[6] = vmFunc(func(args ...any) {
		e, t, n := num(args, 0), num(args, 1), num(args, 2)
		tv, tok := processMap[t].(string)
		nv, nok := processMap[n].(string)
		if !tok || !nok {
			return
		}
		joined := tv + "." + nv
		if joined == "window.document.location" {
			processMap[e] = "https://chatgpt.com/"
			return
		}
		processMap[e] = joined
	})
	processMap[7] = vmFunc(func(args ...any) {
		target := processMap[num(args, 0)]
		values := lookup(processMap, args[1:]...)
		if target == "window.Reflect.set" && len(values) >= 3 {
			if obj, ok := values[0].(*orderedMap); ok {
				obj.add(turnstileString(values[1]), values[2])
			}
			return
		}
		if fn, ok := target.(vmFunc); ok {
			fn(values...)
		}
	})
	processMap[8] = vmFunc(func(args ...any) {
		processMap[num(args, 0)] = processMap[num(args, 1)]
	})
	processMap[9] = tokenList
	processMap[10] = "window"
	processMap[14] = vmFunc(func(args ...any) {
		var v any
		if err := json.Unmarshal([]byte(turnstileString(processMap[num(args, 1)])), &v); err == nil {
			processMap[num(args, 0)] = v
		}
	})
	processMap[15] = vmFunc(func(args ...any) {
		processMap[num(args, 0)] = jsonDumps(processMap[num(args, 1)])
	})
	processMap[16] = p
	processMap[17] = vmFunc(func(args ...any) {
		e := num(args, 0)
		target := processMap[num(args, 1)]
		callArgs := lookup(processMap, args[2:]...)
		switch target {
		case "window.performance.now":
			processMap[e] = float64(time.Since(startTime).Nanoseconds())/1e6 + rand.Float64()/1e6
		case "window.Object.create":
			processMap[e] = newOrderedMap()
		case "window.Object.keys":
			if len(callArgs) > 0 && callArgs[0] == "window.localStorage" {
				processMap[e] = []any{
					"STATSIG_LOCAL_STORAGE_INTERNAL_STORE_V4",
					"STATSIG_LOCAL_STORAGE_STABLE_ID",
					"client-correlated-secret",
					"oai/apps/capExpiresAt",
					"oai-did",
					"STATSIG_LOCAL_STORAGE_LOGGING_REQUEST",
					"UiState.isNavigationCollapsed.1",
				}
			}
		case "window.Math.random":
			processMap[e] = rand.Float64()
		default:
			if fn, ok := target.(vmFunc); ok {
				fn(callArgs...)
			}
		}
	})
	processMap[18] = vmFunc(func(args ...any) {
		if decoded, err := base64.StdEncoding.DecodeString(turnstileString(processMap[num(args, 0)])); err == nil {
			processMap[num(args, 0)] = string(decoded)
		}
	})
	processMap[19] = vmFunc(func(args ...any) {
		processMap[num(args, 0)] = base64.StdEncoding.EncodeToString([]byte(turnstileString(processMap[num(args, 0)])))
	})
	processMap[20] = vmFunc(func(args ...any) {
		e, t, n := num(args, 0), num(args, 1), num(args, 2)
		if !equal(processMap[e], processMap[t]) {
			return
		}
		if fn, ok := processMap[n].(vmFunc); ok {
			fn(lookup(processMap, args[3:]...)...)
		}
	})
	processMap[21] = vmFunc(func(args ...any) {})
	processMap[23] = vmFunc(func(args ...any) {
		if processMap[num(args, 0)] == nil {
			return
		}
		if fn, ok := processMap[num(args, 1)].(vmFunc); ok {
			fn(args[2:]...)
		}
	})
	processMap[24] = vmFunc(func(args ...any) {
		tv, tok := processMap[num(args, 1)].(string)
		nv, nok := processMap[num(args, 2)].(string)
		if tok && nok {
			processMap[num(args, 0)] = tv + "." + nv
		}
	})
	for _, token := range tokenList {
		items, ok := asList(token)
		if !ok || len(items) == 0 {
			continue
		}
		fn, ok := processMap[toFloat(items[0])].(vmFunc)
		if !ok {
			continue
		}
		func() {
			defer func() { _ = recover() }()
			fn(items[1:]...)
		}()
	}
	return result
}

func turnstileString(v any) string {
	if v == nil {
		return "undefined"
	}
	if s, ok := v.(string); ok {
		switch s {
		case "window.Math":
			return "[object Math]"
		case "window.Reflect":
			return "[object Reflect]"
		case "window.performance":
			return "[object Performance]"
		case "window.localStorage":
			return "[object Storage]"
		case "window.Object":
			return "function Object() { [native code] }"
		case "window.Reflect.set":
			return "function set() { [native code] }"
		case "window.performance.now":
			return "function () { [native code] }"
		case "window.Object.create":
			return "function create() { [native code] }"
		case "window.Object.keys":
			return "function keys() { [native code] }"
		case "window.Math.random":
			return "function random() { [native code] }"
		default:
			return s
		}
	}
	if list, ok := asList(v); ok && allStrings(list) {
		parts := make([]string, 0, len(list))
		for _, item := range list {
			parts = append(parts, turnstileString(item))
		}
		return strings.Join(parts, ",")
	}
	if n, ok := v.(float64); ok {
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	if b, ok := v.(bool); ok {
		if b {
			return "True"
		}
		return "False"
	}
	return fmt.Sprint(v)
}

func xorString(text string, key string) string {
	if key == "" {
		return text
	}
	out := []rune(text)
	keyRunes := []rune(key)
	for i, ch := range out {
		out[i] = ch ^ keyRunes[i%len(keyRunes)]
	}
	return string(out)
}

func jsonDumps(v any) string {
	return pyJSONDumps(v)
}

func pyJSONDumps(v any) string {
	switch typed := v.(type) {
	case *orderedMap:
		b, _ := typed.MarshalJSON()
		return string(b)
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			parts = append(parts, pyJSONDumps(item))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		parts := make([]string, 0, len(typed))
		for key, item := range typed {
			keyBytes, _ := json.Marshal(key)
			parts = append(parts, string(keyBytes)+": "+pyJSONDumps(item))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func value(args []any, index int) any {
	if index >= len(args) {
		return nil
	}
	return args[index]
}

func num(args []any, index int) float64 {
	return toFloat(value(args, index))
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	default:
		return 0
	}
}

func lookup(processMap map[float64]any, args ...any) []any {
	values := make([]any, 0, len(args))
	for _, arg := range args {
		values = append(values, processMap[toFloat(arg)])
	}
	return values
}

func asList(v any) ([]any, bool) {
	switch list := v.(type) {
	case []any:
		return list, true
	default:
		return nil, false
	}
}

func allStrings(list []any) bool {
	for _, item := range list {
		if _, ok := item.(string); !ok {
			return false
		}
	}
	return true
}

func isStringOrNumber(v any) bool {
	switch v.(type) {
	case string, float64:
		return true
	default:
		return false
	}
}

func equal(a any, b any) bool {
	if af, ok := a.(float64); ok {
		return af == toFloat(b)
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}
