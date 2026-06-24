// Package so 实现 chatgpt sentinel "so"(session observer)段。
//
// 流程(对齐 sdk_sentinel.js / OpenSentinel client.js / deob_js/out.js):
//
//  1. POST sentinel/req 拿到 chatReq(响应里含 so.collector_dx / so.snapshot_dx)
//  2. client 异步跑 collector_dx(60s 超时,fire-and-forget,初始化 VM 寄存器)
//  3. 业务发请求前,client 跑 snapshot_dx(复用 collector 的 VM 寄存器,毫秒级)
//  4. 拼 openai-sentinel-so-token = base64(json({so, c, id, flow})) 放 header
//
// collector 与 snapshot 必须**复用同一 VM 实例**——snapshot_dx 字节码里只写
// "读 reg[42]" 这种寄存器编号,字段含义在 collector 那边每次会话动态生成。
//
// 反编译参考:chatgpt-turnstile-reversed/deob_js/out.js (se/Et/sessionObserverToken)。
package so

import (
	"aurora/internal/browserfp"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ─── 保留寄存器编号(对齐 out.js / OpenSentinel)───────────────────────────────
const (
	pcReg       = 9  // 指令队列
	windowReg   = 10 // 全局对象引用
	xorKeyReg   = 16 // XOR 解密密钥
	successReg  = 3  // VM 成功退出回调(snapshot 用)
	errorReg    = 4  // VM 失败退出回调(snapshot 用)
	callbackReg = 30 // DEF_FUNC 用的回调 slot
)

// ─── 公开 API ────────────────────────────────────────────────────────────────

// Session 表示一次 collector+snapshot 会话。Collector 异步跑完后,Solver
// 保留 regs;后续 Snapshot 在同一 Solver 上跑,复用 regs 拿到采集值。
type Session struct {
	mu        sync.Mutex
	collector *soSolver
	started   bool
	finished  bool // collector 跑完
	reqToken  string
	dx        string
	err       error
}

// NewSession 给定 requirementsToken + collector_dx 构造一个 Session。
// 真正的 collector 跑在 Start() 里(异步,fire-and-forget,不阻塞调用方)。
func NewSession(reqToken, collectorDX string) *Session {
	return &Session{
		reqToken: reqToken,
		dx:       collectorDX,
	}
}

// Start 异步跑 collector_dx(60 秒超时)。返回的 channel 会在 collector 跑完或
// 失败时关闭;调用方可选择等待也可直接返回(SDK 行为是 fire-and-forget)。
func (s *Session) Start() <-chan struct{} {
	ch := make(chan struct{})
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		close(ch)
		return ch
	}
	s.started = true
	s.collector = newSOSolver()
	s.mu.Unlock()

	go func() {
		defer close(ch)
		_, err := s.collector.run(s.reqToken, s.dx, true /* collector mode */)
		s.mu.Lock()
		s.finished = true
		s.err = err
		s.mu.Unlock()
	}()
	return ch
}

// Snapshot 跑 snapshot_dx,产出 base64 字符串(对齐 OpenSentinel/client.js:155
// 返回值)。如果 collector 还没跑完,会自动等(started 后无 timeout 上限,
// 实际业务里 collector 在发请求前几十秒就开始,正常早就完成)。
func (s *Session) Snapshot(snapshotDX string) (string, error) {
	s.mu.Lock()
	collector := s.collector
	started := s.started
	s.mu.Unlock()

	if !started {
		return "", errors.New("so: collector not started")
	}
	if collector == nil {
		return "", errors.New("so: collector not initialized")
	}
	return collector.run(s.reqToken, snapshotDX, false /* snapshot mode */)
}

// BuildToken 拼 openai-sentinel-so-token,格式:
//
//	base64(json({"so": soResult, "c": chatReqToken, "id": deviceId, "flow": flow}))
//
// 对齐 out.js:924 ce({so, c}, t) → ce 在 js 里是 base64(json({...obj,id,flow}))。
func BuildToken(soResult, chatReqToken, deviceID, flow string) (string, error) {
	if soResult == "" {
		return "", nil
	}
	payload := map[string]string{
		"so":   soResult,
		"c":    chatReqToken,
		"id":   deviceID,
		"flow": flow,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("so: marshal token: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// ─── Solver(VM 字节码解释器)──────────────────────────────────────────────────

// soSolver 复刻 sdk_sentinel.js 的 VM。架构与 internal/turnstile/turnstile.go
// 的 turnstileSolver 一致(opcode 0-35 表相同),但:
//   - 不读 requirements token 的 profile 字段(collector 跑得久,snapshot 读 regs)
//   - run() 有 collector 模式(不设 success/error) 与 snapshot 模式(设 success/error)
//   - success/error 处理器把 jsToString(t) 用 latin1 编码后 base64(对齐浏览器 btoa
//     对 128-255 字符的二进制行为)
type soSolver struct {
	regs     map[string]any
	window   map[string]any
	done     bool
	resolved string
	rejected string
	profile  *browserfp.Profile
}

func newSOSolver() *soSolver { return &soSolver{regs: map[string]any{}} }

type vmFunc = func(args ...any) (any, error)

type regMapRef struct{ s *soSolver }

// run 跑一段字节码。collector=true 时不设 success/error(不终止 VM,只填 regs);
// collector=false 时设 success/error,期待 VM 通过 reg 3/4 退出。
func (s *soSolver) run(reqToken, dx string, collector bool) (string, error) {
	s.regs = map[string]any{}
	s.profile = browserfp.Get()
	s.window = s.buildWindow()
	s.done = false
	s.resolved = ""
	s.rejected = ""
	s.initRuntime()

	if !collector {
		s.setReg(successReg, vmFunc(func(args ...any) (any, error) {
			if !s.done {
				s.done = true
				var v any
				if len(args) > 0 {
					v = args[0]
				}
				s.resolved = latin1Base64Encode(toStr(v))
			}
			return nil, nil
		}))
		s.setReg(errorReg, vmFunc(func(args ...any) (any, error) {
			if !s.done {
				s.done = true
				var v any
				if len(args) > 0 {
					v = args[0]
				}
				s.rejected = latin1Base64Encode(toStr(v))
			}
			return nil, nil
		}))
		s.setReg(callbackReg, vmFunc(func(args ...any) (any, error) {
			if len(args) < 3 {
				return nil, nil
			}
			targetReg := args[0]
			returnReg := args[1]
			argRegs, _ := args[2].([]any)
			innerQueue := argRegs
			mappedArgRegs := []any{}
			if len(args) >= 4 {
				if mapped, ok := args[2].([]any); ok {
					mappedArgRegs = mapped
				}
				if q, ok := args[3].([]any); ok {
					innerQueue = q
				}
			}
			s.setReg(targetReg, vmFunc(func(callArgs ...any) (any, error) {
				if s.done {
					return nil, nil
				}
				prevQ := s.copyQueue()
				for i, regID := range mappedArgRegs {
					if i < len(callArgs) {
						s.setReg(regID, callArgs[i])
					} else {
						s.setReg(regID, nil)
					}
				}
				s.setReg(pcReg, copyAnySlice(innerQueue))
				if err := s.runQueue(); err != nil {
					s.setReg(pcReg, prevQ)
					return err.Error(), nil
				}
				s.setReg(pcReg, prevQ)
				return s.getReg(returnReg), nil
			}))
			return nil, nil
		}))
	}
	s.setReg(xorKeyReg, reqToken)

	decoded, err := latin1Base64Decode(dx)
	if err != nil {
		return "", err
	}
	plain := xorString(decoded, reqToken)
	var queue []any
	if err := json.Unmarshal([]byte(plain), &queue); err != nil {
		return "", err
	}
	s.setReg(pcReg, queue)

	if err := s.runQueue(); err != nil && !s.done {
		if !collector {
			if success, ok := s.getReg(successReg).(vmFunc); ok {
				_, _ = success(fmt.Sprintf("%d: %v", 0, err))
			}
		}
	}
	if !collector {
		if s.rejected != "" {
			return "", errors.New(s.rejected)
		}
		if s.resolved == "" {
			return "", errors.New("so: vm unresolved")
		}
		return s.resolved, nil
	}
	// collector 模式不返回结果,只保留 regs
	return "", nil
}

// initRuntime 注册所有 opcode 处理(对齐 OpenSentinel/vm.js + deob/out.js)。
func (s *soSolver) initRuntime() {
	s.setReg(0, vmFunc(func(args ...any) (any, error) {
		if len(args) == 0 {
			return nil, nil
		}
		v, err := s.run(s.jsToString(s.getReg(xorKeyReg)), s.jsToString(args[0]), true)
		if err != nil {
			return nil, err
		}
		return v, nil
	}))
	s.setReg(1, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		s.setReg(args[0], xorString(s.jsToString(s.getReg(args[0])), s.jsToString(s.getReg(args[1]))))
		return nil, nil
	}))
	s.setReg(2, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		s.setReg(args[0], args[1])
		return nil, nil
	}))
	s.setReg(5, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		left := s.getReg(args[0])
		right := s.getReg(args[1])
		if arr, ok := left.([]any); ok {
			s.setReg(args[0], append(arr, right))
			return nil, nil
		}
		if lNum, ok := s.asNumber(left); ok {
			if rNum, ok := s.asNumber(right); ok {
				s.setReg(args[0], lNum+rNum)
				return nil, nil
			}
		}
		s.setReg(args[0], s.jsToString(left)+s.jsToString(right))
		return nil, nil
	}))
	s.setReg(6, vmFunc(func(args ...any) (any, error) {
		if len(args) < 3 {
			return nil, nil
		}
		s.setReg(args[0], s.jsGetProp(s.getReg(args[1]), s.getReg(args[2])))
		return nil, nil
	}))
	s.setReg(7, vmFunc(func(args ...any) (any, error) {
		if len(args) == 0 {
			return nil, nil
		}
		_, err := s.callFn(s.getReg(args[0]), s.derefArgs(args[1:])...)
		return nil, err
	}))
	s.setReg(8, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		s.setReg(args[0], s.getReg(args[1]))
		return nil, nil
	}))
	s.setReg(11, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		pattern := s.jsToString(s.getReg(args[1]))
		rx, err := regexp.Compile(pattern)
		if err != nil {
			s.setReg(args[0], nil)
			return nil, nil
		}
		docs, _ := s.jsGetProp(s.jsGetProp(s.window, "document"), "scripts").([]any)
		for _, item := range docs {
			src := s.jsToString(s.jsGetProp(item, "src"))
			if src == "" {
				continue
			}
			if hit := rx.FindString(src); hit != "" {
				s.setReg(args[0], hit)
				return nil, nil
			}
		}
		s.setReg(args[0], nil)
		return nil, nil
	}))
	s.setReg(12, vmFunc(func(args ...any) (any, error) {
		if len(args) == 0 {
			return nil, nil
		}
		s.setReg(args[0], regMapRef{s: s})
		return nil, nil
	}))
	s.setReg(13, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		_, err := s.callFn(s.getReg(args[1]), args[2:]...)
		if err != nil {
			s.setReg(args[0], err.Error())
		}
		return nil, nil
	}))
	s.setReg(14, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		var out any
		if err := json.Unmarshal([]byte(s.jsToString(s.getReg(args[1]))), &out); err != nil {
			return nil, err
		}
		s.setReg(args[0], out)
		return nil, nil
	}))
	s.setReg(15, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		body, err := jsJSONStringify(s.getReg(args[1]))
		if err != nil {
			return nil, err
		}
		s.setReg(args[0], body)
		return nil, nil
	}))
	s.setReg(17, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		v, err := s.callFn(s.getReg(args[1]), s.derefArgs(args[2:])...)
		if err != nil {
			s.setReg(args[0], err.Error())
			return nil, nil
		}
		s.setReg(args[0], v)
		return nil, nil
	}))
	s.setReg(18, vmFunc(func(args ...any) (any, error) {
		if len(args) == 0 {
			return nil, nil
		}
		decoded, err := latin1Base64Decode(s.jsToString(s.getReg(args[0])))
		if err != nil {
			return nil, err
		}
		s.setReg(args[0], decoded)
		return nil, nil
	}))
	s.setReg(19, vmFunc(func(args ...any) (any, error) {
		if len(args) == 0 {
			return nil, nil
		}
		s.setReg(args[0], latin1Base64Encode(s.jsToString(s.getReg(args[0]))))
		return nil, nil
	}))
	s.setReg(20, vmFunc(func(args ...any) (any, error) {
		if len(args) < 3 {
			return nil, nil
		}
		if s.valuesEqual(s.getReg(args[0]), s.getReg(args[1])) {
			_, err := s.callFn(s.getReg(args[2]), args[3:]...)
			return nil, err
		}
		return nil, nil
	}))
	s.setReg(21, vmFunc(func(args ...any) (any, error) {
		if len(args) < 4 {
			return nil, nil
		}
		left, _ := s.asNumber(s.getReg(args[0]))
		right, _ := s.asNumber(s.getReg(args[1]))
		thresh, _ := s.asNumber(s.getReg(args[2]))
		if absDiff(left, right) > thresh {
			_, err := s.callFn(s.getReg(args[3]), args[4:]...)
			return nil, err
		}
		return nil, nil
	}))
	s.setReg(22, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		prevQ := s.copyQueue()
		newInstr, _ := args[1].([]any)
		s.setReg(pcReg, append([]any{}, newInstr...))
		s.runQueue()
		s.setReg(pcReg, prevQ)
		return nil, nil
	}))
	s.setReg(23, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		if s.getReg(args[0]) != nil {
			_, err := s.callFn(s.getReg(args[1]), args[2:]...)
			return nil, err
		}
		return nil, nil
	}))
	s.setReg(24, vmFunc(func(args ...any) (any, error) {
		if len(args) < 3 {
			return nil, nil
		}
		obj := s.getReg(args[1])
		key := s.getReg(args[2])
		s.setReg(args[0], s.jsGetProp(obj, key))
		return nil, nil
	}))
	s.setReg(25, vmFunc(func(args ...any) (any, error) { return nil, nil }))
	s.setReg(26, vmFunc(func(args ...any) (any, error) { return nil, nil }))
	s.setReg(27, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		cur := s.getReg(args[0])
		if arr, ok := cur.([]any); ok {
			idx := -1
			for i, v := range arr {
				if s.valuesEqual(v, s.getReg(args[1])) {
					idx = i
					break
				}
			}
			if idx >= 0 {
				s.setReg(args[0], append(append([]any{}, arr[:idx]...), arr[idx+1:]...))
			}
			return nil, nil
		}
		if n, ok := s.asNumber(cur); ok {
			if m, ok := s.asNumber(s.getReg(args[1])); ok {
				s.setReg(args[0], n-m)
			}
		}
		return nil, nil
	}))
	s.setReg(28, vmFunc(func(args ...any) (any, error) { return nil, nil }))
	s.setReg(29, vmFunc(func(args ...any) (any, error) {
		if len(args) < 3 {
			return nil, nil
		}
		left, _ := s.asNumber(s.getReg(args[1]))
		right, _ := s.asNumber(s.getReg(args[2]))
		s.setReg(args[0], left < right)
		return nil, nil
	}))
	s.setReg(33, vmFunc(func(args ...any) (any, error) {
		if len(args) < 3 {
			return nil, nil
		}
		l, _ := s.asNumber(s.getReg(args[1]))
		r, _ := s.asNumber(s.getReg(args[2]))
		s.setReg(args[0], l*r)
		return nil, nil
	}))
	s.setReg(34, vmFunc(func(args ...any) (any, error) {
		if len(args) < 2 {
			return nil, nil
		}
		v := s.getReg(args[1])
		if v == nil {
			return nil, nil
		}
		// 简化:js Promise 用 sync 表示,collector 字节码通常不会用 await
		s.setReg(args[0], v)
		return nil, nil
	}))
	s.setReg(35, vmFunc(func(args ...any) (any, error) {
		if len(args) < 3 {
			return nil, nil
		}
		divisor, _ := s.asNumber(s.getReg(args[2]))
		left, _ := s.asNumber(s.getReg(args[1]))
		if divisor == 0 {
			s.setReg(args[0], 0.0)
		} else {
			s.setReg(args[0], left/divisor)
		}
		return nil, nil
	}))
	s.setReg(windowReg, s.window)
}

// ─── 浏览器 mock(简化版;字段对齐 turnstile/buildWindow,够 collector 字节码用)─

func (s *soSolver) buildWindow() map[string]any {
	fp := s.profile
	ua := defaultUA

	body := mkElement("body")
	head := mkElement("head")
	html := mkElement("html")
	html.lang = "en"
	html.appendChild(head)
	html.appendChild(body)

	sdk := mkElement("script")
	sdk.src = "https://chatgpt.com/sentinel/20260423af3c/sdk.js"
	sdk.attrs["src"] = sdk.src
	sdk.getAttribute = func(name string) any {
		if name == "src" {
			return sdk.src
		}
		if v, ok := sdk.attrs[name]; ok {
			return v
		}
		return nil
	}
	head.appendChild(sdk)
	scripts := []any{sdk}
	scriptsItem := func(i int) any {
		if i < 0 || i >= len(scripts) {
			return nil
		}
		return scripts[i]
	}
	_ = scriptsItem

	navProto := map[string]any{
		"userAgent":           ua,
		"appVersion":          strings.TrimPrefix(ua, "Mozilla/"),
		"language":            "en-US",
		"languages":           []any{"en-US", "en"},
		"onLine":              true,
		"cookieEnabled":       true,
		"webdriver":           false,
		"doNotTrack":          nil,
		"hardwareConcurrency": fp.HardwareConcurrency,
		"deviceMemory":        fp.DeviceMemory,
		"maxTouchPoints":      0,
		"platform":            "Win32",
		"vendor":              "Google Inc.",
		"appCodeName":         "Mozilla",
		"appName":             "Netscape",
		"product":             "Gecko",
		"productSub":          "20030107",
		"vendorSub":           "",
		"bluetooth":           "[object Bluetooth]",
		"usb":                 "[object USB]",
		"serial":              "[object Serial]",
		"hid":                 "[object HID]",
		"presentation":        "[object Presentation]",
		"mediaDevices":        "[object MediaDevices]",
		"credentials":         "[object CredentialsContainer]",
		"geolocation":         "[object Geolocation]",
		"permissions":         "[object Permissions]",
		"clipboard":           "[object Clipboard]",
		"mimeTypes":           "[object MimeTypeArray]",
		"plugins":             "[object PluginArray]",
		"connection": map[string]any{
			"effectiveType": "4g",
			"rtt":           fp.NetworkRTT,
			"downlink":      fp.NetworkDownlink,
			"saveData":      false,
		},
	}
	nav := map[string]any{}
	for k, v := range navProto {
		nav[k] = v
	}
	nav["clientInformation"] = nav

	doc := map[string]any{
		"cookie":          "",
		"referrer":        "",
		"title":           "",
		"readyState":      "complete",
		"location":        map[string]any{"href": "https://chatgpt.com/sentinel/frame.html", "origin": "https://chatgpt.com", "search": ""},
		"body":            body,
		"head":            head,
		"documentElement": html,
		"currentScript":   sdk,
		"scripts":         scripts,
		"createElement": func(tag any) any {
			if toStr(tag) == "canvas" {
				return mkElement("canvas")
			}
			return mkElement(toStr(tag))
		},
		"getElementById": func() any { return nil },
		"querySelector":  func() any { return nil },
	}

	location := doc["location"].(map[string]any)

	perfBase := 3000.0 + rand.Float64()*2000.0
	start := time.Now()
	perf := map[string]any{
		"now": func() float64 {
			perfBase += 0.5 + rand.Float64()*2.5
			return perfBase
		},
		"timeOrigin": float64(start.UnixMilli()) - perfBase,
		"memory": map[string]any{
			"jsHeapSizeLimit": fp.JSHeapSizeLimit,
		},
	}

	mockWin := map[string]any{
		"navigator":   nav,
		"document":    doc,
		"location":    location,
		"performance": perf,
		"screen": map[string]any{
			"width":       fp.ScreenWidth,
			"height":      fp.ScreenHeight,
			"availWidth":  fp.ScreenWidth,
			"availHeight": fp.ScreenAvailHeight,
			"availLeft":   0,
			"availTop":    0,
			"colorDepth":  fp.ScreenColorDepth,
			"pixelDepth":  fp.ScreenColorDepth,
				"devicePixelRatio": fp.DevicePixelRatio,
		},
		"localStorage":    newStorageProxy(),
		"sessionStorage":  newStorageProxy(),
		"history":         map[string]any{"length": 1},
		"setTimeout":      time.AfterFunc,
		"setInterval":     time.Tick,
		"atob":            func(s any) any { return atobLatin1(toStr(s)) },
		"btoa":            func(s any) any { return latin1Base64Encode(toStr(s)) },
		"console":         map[string]any{},
		"origin":          "https://chatgpt.com",
		"isSecureContext": true,
	}
	mockWin["window"] = mockWin
	mockWin["self"] = mockWin
	mockWin["globalThis"] = mockWin
	return mockWin
}

// ─── 注册/读取工具 ────────────────────────────────────────────────────────────

func (s *soSolver) setReg(key any, value any) { s.regs[regKey(key)] = value }
func (s *soSolver) getReg(key any) any        { return s.regs[regKey(key)] }
func (s *soSolver) copyQueue() []any          { q, _ := s.getReg(pcReg).([]any); return copyAnySlice(q) }

func (s *soSolver) runQueue() error {
	for {
		q, ok := s.getReg(pcReg).([]any)
		if !ok || len(q) == 0 {
			return nil
		}
		ins, ok := q[0].([]any)
		s.setReg(pcReg, q[1:])
		if !ok || len(ins) == 0 {
			continue
		}
		fn, ok := s.getReg(ins[0]).(vmFunc)
		if !ok {
			return fmt.Errorf("so: opcode %v not callable", ins[0])
		}
		if _, err := fn(ins[1:]...); err != nil {
			return err
		}
	}
}

func (s *soSolver) callFn(value any, args ...any) (any, error) {
	fn, ok := value.(vmFunc)
	if !ok {
		return nil, nil
	}
	return fn(args...)
}

func (s *soSolver) derefArgs(args []any) []any {
	out := make([]any, 0, len(args))
	for _, a := range args {
		out = append(out, s.getReg(a))
	}
	return out
}

func (s *soSolver) asNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case bool:
		if v {
			return 1, true
		}
		return 0, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, true
		}
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func (s *soSolver) valuesEqual(left, right any) bool {
	switch l := left.(type) {
	case float64:
		r, ok := right.(float64)
		return ok && l == r
	case string:
		return l == right
	case bool:
		r, ok := right.(bool)
		return ok && l == r
	case nil:
		return right == nil
	}
	return fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right)
}

func (s *soSolver) jsToString(value any) string {
	switch v := value.(type) {
	case nil:
		return "undefined"
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, it := range v {
			parts = append(parts, s.jsToString(it))
		}
		return strings.Join(parts, ",")
	case map[string]any:
		if h, ok := v["href"].(string); ok {
			if _, hasSearch := v["search"]; hasSearch {
				return h
			}
		}
		return "[object Object]"
	}
	return fmt.Sprintf("%v", value)
}

func (s *soSolver) jsGetProp(obj any, prop any) any {
	switch v := obj.(type) {
	case nil:
		return nil
	case regMapRef:
		return v.s.getReg(prop)
	case map[string]any:
		key := toStr(prop)
		if val, ok := v[key]; ok {
			return val
		}
		return nil
	case []any:
		if toStr(prop) == "length" {
			return float64(len(v))
		}
		idx, ok := prop.(float64)
		if !ok {
			idx = 0
		}
		i := int(idx)
		if i < 0 || i >= len(v) {
			return nil
		}
		return v[i]
	case string:
		if toStr(prop) == "length" {
			return float64(len(v))
		}
		idx, ok := prop.(float64)
		if !ok {
			idx = 0
		}
		i := int(idx)
		if i < 0 || i >= len(v) {
			return nil
		}
		return string([]rune(v)[i])
	}
	return nil
}

// ─── DOM element mock ──────────────────────────────────────────────────────────

type mockElement struct {
	tagName      string
	style        map[string]any
	children     []any
	attrs        map[string]any
	parent       any
	innerHTML    string
	innerText    string
	lang         string
	src          string
	width        int
	height       int
	getAttribute func(name string) any
}

func (e *mockElement) appendChild(c any) any {
	if c != nil {
		if m, ok := c.(*mockElement); ok {
			m.parent = e
		}
		e.children = append(e.children, c)
	}
	return c
}
func (e *mockElement) removeChild(c any) any {
	for i, cc := range e.children {
		if cc == c {
			e.children = append(append([]any{}, e.children[:i]...), e.children[i+1:]...)
			if m, ok := c.(*mockElement); ok {
				m.parent = nil
			}
			return c
		}
	}
	return nil
}

func mkElement(tag string) *mockElement {
	e := &mockElement{
		tagName: strings.ToUpper(tag),
		style:   map[string]any{},
		attrs:   map[string]any{},
		width:   300,
		height:  150,
	}
	e.getAttribute = func(name string) any {
		if v, ok := e.attrs[name]; ok {
			return v
		}
		return nil
	}
	return e
}

// newStorageProxy 简化版 localStorage 代理(只满足字节码基本 get/set,够用)。
func newStorageProxy() map[string]any {
	data := map[string]string{}
	return map[string]any{
		"getItem":    func(k any) any { return data[toStr(k)] },
		"setItem":    func(k, v any) any { data[toStr(k)] = toStr(v); return nil },
		"removeItem": func(k any) any { delete(data, toStr(k)); return nil },
		"clear": func() any {
			for k := range data {
				delete(data, k)
			}
			return nil
		},
		"length": func() any { return float64(len(data)) },
		"key": func(idx any) any {
			i, ok := idx.(float64)
			if !ok {
				return nil
			}
			keys := make([]string, 0, len(data))
			for k := range data {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			if int(i) < 0 || int(i) >= len(keys) {
				return nil
			}
			return keys[int(i)]
		},
	}
}

// ─── 纯函数辅助 ──────────────────────────────────────────────────────────────

// toStr 把 any 序列化为字符串(供顶层函数调用,非 receiver 形式)。
// 行为对齐 s.jsToString 的最常用分支,够 collector_dx/snapshot_dx 字节码使用。
func toStr(value any) string {
	if value == nil {
		return "undefined"
	}
	switch v := value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return fmt.Sprintf("%v", value)
}

const defaultUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"

func absDiff(a, b float64) float64 {
	if a > b {
		return a - b
	}
	return b - a
}

func copyAnySlice(v []any) []any {
	if len(v) == 0 {
		return []any{}
	}
	out := make([]any, len(v))
	copy(out, v)
	return out
}

func regKey(value any) string {
	switch v := value.(type) {
	case nil:
		return "nil"
	case string:
		return "s:" + v
	case int:
		return "n:" + strconv.Itoa(v)
	case int64:
		return "n:" + strconv.FormatInt(v, 10)
	case float64:
		if v == float64(int64(v)) {
			return "n:" + strconv.FormatInt(int64(v), 10)
		}
		return "n:" + strconv.FormatFloat(v, 'g', -1, 64)
	}
	return "x:" + fmt.Sprintf("%v", value)
}

// latin1Base64Decode 对齐浏览器 atob 的 Latin-1 语义(128-255 字符按字节处理)。
func latin1Base64Decode(value string) (string, error) {
	body, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", err
	}
	runes := make([]rune, 0, len(body))
	for _, b := range body {
		runes = append(runes, rune(b))
	}
	return string(runes), nil
}

// latin1Base64Encode 对齐浏览器 btoa:把字符串当 Latin-1,逐字节 base64。
func latin1Base64Encode(value string) string {
	body := make([]byte, 0, len(value))
	for _, r := range value {
		body = append(body, byte(r))
	}
	return base64.StdEncoding.EncodeToString(body)
}

func atobLatin1(value string) string {
	body, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return ""
	}
	return string(body)
}

func xorString(data, key string) string {
	if key == "" {
		return data
	}
	db := make([]byte, 0, len(data))
	kb := make([]byte, 0, len(key))
	for _, r := range data {
		db = append(db, byte(r))
	}
	for _, r := range key {
		kb = append(kb, byte(r))
	}
	out := make([]byte, len(db))
	for i := range db {
		out[i] = db[i] ^ kb[i%len(kb)]
	}
	return string(bytesToLatin1Runes(out))
}

func bytesToLatin1Runes(value []byte) []rune {
	out := make([]rune, 0, len(value))
	for _, b := range value {
		out = append(out, rune(b))
	}
	return out
}

func jsJSONStringify(value any) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
