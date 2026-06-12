package prooftoken

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	"aurora/internal/turnstile"

	"golang.org/x/crypto/sha3"
)

const (
	powPrefixRequirements = "gAAAAAC"
	powPrefixProof        = "gAAAAAB"

	requirementsDifficulty = "0fffff"

	maxRequirementsIter = 500_000
	maxProofIter        = 100_000

	powFallback = "wQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4D"
)

var (
	powCores   = []int{8, 16, 24, 32}
	powScreens = []int{3000, 4000, 5000}

	powNavKeys = []string{
		"registerProtocolHandler−function registerProtocolHandler() { [native code] }",
		"storage−[object StorageManager]",
		"locks−[object LockManager]",
		"appCodeName−Mozilla",
		"permissions−[object Permissions]",
		"share−function share() { [native code] }",
		"webdriver−false",
		"managed−[object NavigatorManagedData]",
		"canShare−function canShare() { [native code] }",
		"vendor−Google Inc.",
		"mediaDevices−[object MediaDevices]",
		"vibrate−function vibrate() { [native code] }",
		"storageBuckets−[object StorageBucketManager]",
		"mediaCapabilities−[object MediaCapabilities]",
		"cookieEnabled−true",
		"virtualKeyboard−[object VirtualKeyboard]",
		"product−Gecko",
		"presentation−[object Presentation]",
		"onLine−true",
		"mimeTypes−[object MimeTypeArray]",
		"credentials−[object CredentialsContainer]",
		"serviceWorker−[object ServiceWorkerContainer]",
		"keyboard−[object Keyboard]",
		"gpu−[object GPU]",
		"doNotTrack",
		"serial−[object Serial]",
		"pdfViewerEnabled−true",
		"language−zh-CN",
		"geolocation−[object Geolocation]",
		"userAgentData−[object NavigatorUAData]",
		"getUserMedia−function getUserMedia() { [native code] }",
		"sendBeacon−function sendBeacon() { [native code] }",
		"hardwareConcurrency−32",
		"windowControlsOverlay−[object WindowControlsOverlay]",
	}
	powWinKeys = []string{
		"0", "window", "self", "document", "name", "location",
		"customElements", "history", "navigation",
		"innerWidth", "innerHeight", "scrollX", "scrollY",
		"visualViewport", "screenX", "screenY",
		"outerWidth", "outerHeight", "devicePixelRatio",
		"screen", "chrome", "navigator",
		"onresize", "performance", "crypto",
		"indexedDB", "sessionStorage", "localStorage", "scheduler",
		"alert", "atob", "btoa", "fetch", "matchMedia",
		"postMessage", "queueMicrotask", "requestAnimationFrame",
		"setInterval", "setTimeout", "caches",
		"__NEXT_DATA__", "__BUILD_MANIFEST", "__NEXT_PRELOADREADY",
	}

	powDocKeys = []string{"_reactListeningo743lnnpvdg", "location"}

	defaultScriptSources = []string{"https://chatgpt.com/backend-api/sentinel/sdk.js"}
)

// POWConfig 是 18 元素的客户端指纹数组(requirements_token 用)。
type POWConfig struct {
	userAgent string
	arr       [18]interface{}
}

func NewPOWConfig(userAgent string, scriptSources []string, dataBuild string) *POWConfig {
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36 Edg/147.0.0.0"
	}
	if len(scriptSources) == 0 {
		scriptSources = defaultScriptSources
	}
	//nolint:gosec // 非加密用途
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	perfNow := float64(time.Now().UnixNano()) / 1e6 // performance.now() 等价
	timeStr := _legacyParseTime()
	scriptSrc := scriptSources[rng.Intn(len(scriptSources))]

	c := &POWConfig{userAgent: userAgent}
	c.arr = [18]interface{}{
		powScreens[rng.Intn(len(powScreens))], // 0 — screen value
		timeStr,                               // 1
		4294705152,                            // 2 — 硬编码常量
		0,                                     // 3 — 迭代会覆盖
		userAgent,                             // 4
		scriptSrc,                             // 5 — <script src> 或默认 sdk.js
		dataBuild,                             // 6 — data-build 属性或 c/.../_ 匹配
		"en-US",                               // 7
		"en-US,es-US,en,es",                   // 8
		0,                                     // 9 — 迭代会覆盖
		powNavKeys[rng.Intn(len(powNavKeys))], // 10
		powDocKeys[rng.Intn(len(powDocKeys))], // 11
		powWinKeys[rng.Intn(len(powWinKeys))], // 12
		perfNow,                               // 13 — perf_counter()*1000
		randomUUID(rng),                       // 14
		"",                                    // 15
		powCores[rng.Intn(len(powCores))],     // 16
		float64(time.Now().UnixMilli()) - perfNow, // 17 — timeOrigin
	}
	return c
}

func _legacyParseTime() string {
	loc := time.FixedZone("EST", -5*60*60)
	return time.Now().In(loc).Format("Mon Jan 02 2006 15:04:05") + " GMT-0500 (Eastern Standard Time)"
}

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

func (c *POWConfig) solveRequirements(seed, difficulty string) (string, bool) {
	target, err := hex.DecodeString(difficulty)
	if err != nil {
		return "", false
	}
	diffLen := len(difficulty) / 2 // hex 字符数

	// 预拼 p1/p2/p3。config[3] 和 config[9] 位置留给迭代器。
	arr := c.arr
	// p1 = compact_json(arr[:3])[:-1] + ","
	head := marshalCompact([]interface{}{arr[0], arr[1], arr[2]})
	p1 := append(head[:len(head)-1:len(head)-1], ',')

	// p2 = "," + compact_json(arr[4:9])[1:-1] + ","
	mid := marshalCompact([]interface{}{arr[4], arr[5], arr[6], arr[7], arr[8]})
	p2 := make([]byte, 0, len(mid)+2)
	p2 = append(p2, ',')
	p2 = append(p2, mid[1:len(mid)-1]...)
	p2 = append(p2, ',')

	// p3 = "," + compact_json(arr[10:])[1:]
	tail := marshalCompact([]interface{}{
		arr[10], arr[11], arr[12], arr[13], arr[14], arr[15], arr[16], arr[17],
	})
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

		if bytes.Compare(sum[:diffLen], target) <= 0 {
			return string(b64buf), true
		}
	}
	return "", false
}

func SolveProofToken(seed, difficulty, userAgent string, scriptSources []string, dataBuild string) string {
	if seed == "" || difficulty == "" {
		return ""
	}
	if userAgent == "" {
		userAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36 Edg/147.0.0.0"
	}
	if len(scriptSources) == 0 {
		scriptSources = defaultScriptSources
	}
	//nolint:gosec
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	screen := powScreens[rng.Intn(len(powScreens))]

	timeStr := _legacyParseTime()
	scriptSrc := scriptSources[rng.Intn(len(scriptSources))]

	proofConfig := []interface{}{
		screen,                                // 0
		timeStr,                               // 1
		4294705152,                            // 2
		0,                                     // 3 — 迭代
		userAgent,                             // 4
		scriptSrc,                             // 5
		dataBuild,                             // 6
		"en-US",                               // 7
		"en-US,es-US,en,es",                   // 8
		0,                                     // 9
		powNavKeys[rng.Intn(len(powNavKeys))], // 10
		powDocKeys[rng.Intn(len(powDocKeys))], // 11
		powWinKeys[rng.Intn(len(powWinKeys))], // 12
		float64(time.Now().UnixNano()) / 1e6,  // 13 — perf_counter()*1000
		randomUUID(rng),                       // 14
		"",                                    // 15
		powCores[rng.Intn(len(powCores))],     // 16
		float64(time.Now().UnixMilli()) - float64(time.Now().UnixNano())/1e6, // 17
	}

	diffLen := len(difficulty) / 2
	//nolint:gosec
	target, _ := hex.DecodeString(difficulty)
	seedB := []byte(seed)
	hasher := sha3.New512()
	for i := 0; i < maxProofIter; i++ {
		proofConfig[3] = i
		proofConfig[9] = i >> 1
		raw := marshalCompact(proofConfig)
		b64 := base64.StdEncoding.EncodeToString(raw)
		hasher.Reset()
		hasher.Write(seedB)
		hasher.Write([]byte(b64))
		sum := hasher.Sum(nil)
		if bytes.Compare(sum[:diffLen], target) <= 0 {
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

func marshalCompact(v interface{}) []byte {
	b, _ := json.Marshal(v)
	buf := new(bytes.Buffer)
	_ = json.Compact(buf, b)
	return buf.Bytes()
}

// Solve delegates to the canonical turnstile VM implementation.
func Solve(dx string, p string) string {
	return turnstile.Solve(dx, p)
}
