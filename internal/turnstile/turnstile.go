// Package turnstile implements the ChatGPT Sentinel SDK (20260423af3c) VM.
//
// The dx challenge is decoded as:
//
//	base64_decode(dx) → XOR(decoded, key) → JSON → [[opcode, ...args], ...]
//
// The engine executes the opcode queue, resolving with a base64 token.
// 35 symbolic opcodes aligned to sdk.pretty.js:1230-1250.
package turnstile

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"
)

// ── opcode 常量(对齐 sdk.pretty.js:1230-1250) ──────────────────────────────
const (
	opLt = 0  // 启动 Pn 异步流程
	opJt = 1  // slot[n] = XOR(toStr(slot[n]), toStr(slot[m]))
	opGt = 2  // slot[n] = v (直接赋值)
	opWt = 3  // resolve(btoa(toStr(slot[t])))
	opZt = 4  // reject(btoa(Sn + ": " + t))
	opBt = 5  // array push 或字符串拼接
	opHt = 6  // slot[n] = toStr(slot[m])[toStr(slot[r])] (字符串索引/属性访问)
	opZt7 = 7 // 通用调用 slot[n](...args.map(toStr))
	opKt = 8  // slot[n] = toStr(slot[m])
	opQt = 9  // token 队列(特殊槽)
	opYt = 10 // window 对象
	opXt = 11 // 扫 document.scripts 匹配 c/[^/]*/_, 返回 src
	optn = 12 // slot[t] = Cn(self reference)
	opnn = 13 // (reserved)
	open = 14 // slot[n] = JSON.parse(slot[m])
	oprn = 15 // slot[n] = JSON.stringify(slot[m])
	opon = 16 // XOR key 常量
	opcn = 17 // 异步安全调用,异常转字符串
	opsn = 18 // slot[n] = atob(slot[n])
	opun = 19 // slot[n] = btoa(slot[n])
	opfn = 20 // m === n ? r(...o) : null
	opln = 21 // abs(a-b) > c ? r(...args) : null (NOP)
	opdn = 22 // 闭包作用域: save Qt → push args → run → restore Qt
	opan = 23 // undefined !== n ? m(...args) : null
	opVt = 24 // slot[n] = slot[m][slot[r]].bind(slot[m])
	ophn = 25 // NOP
	opin = 26 // NOP
	opmn = 27 // array splice(0,1) 或数值减
	opwn = 28 // NOP
	opyn = 29 // slot[n] = toStr(m) < toStr(r)
	opgn = 30 // 可重入 push(在闭包外延展)
	op31 = 31 // NOP
	op32 = 32 // NOP
	opvn = 33 // slot[n] = Number(m) * Number(r)
	opbn = 34 // Promise.resolve(slot[m]).then(slot[n] = result)
	opkn = 35 // slot[n] = Number(m) / Number(r) (除零保护)
)

// ── types ───────────────────────────────────────────────────────────────────

// tokenItem 是队列中的一个指令: [opcode, ...args]
type tokenItem struct {
	opcode int
	args   []any
}

// nativeBoundMethod 表示 opcode 24 bind 的方法引用
type nativeBoundMethod struct {
	method string
	bound  string // bound this.toString()
}

// engine 是 turnstile VM 的运行状态
type engine struct {
	slots      map[int]any
	queue      []tokenItem
	sn         int
	result     string
	resolved   bool
	rejected   bool
	errMsg     string
	scriptSrcs []string
}

// ── public API ──────────────────────────────────────────────────────────────

// Solve 解算 dx 挑战字符串,返回 base64 token。
// key 是 requirementsToken(存在 slot 16,用作 XOR key)。
func Solve(dx string, key string) string {
	decoded, err := base64.StdEncoding.DecodeString(dx)
	if err != nil {
		return ""
	}
	decrypted := xorBytes(decoded, key)
	var queue []tokenItem
	if err := json.Unmarshal([]byte(decrypted), &queue); err != nil {
		return ""
	}
	if len(queue) == 0 {
		return ""
	}

	result, ok := runWithTimeout(queue, key, nil, 500*time.Millisecond)
	if !ok {
		return ""
	}
	return result
}

// SolveWithScripts 同 Solve,但注入 <script src> 列表供 opcode 11 使用。
func SolveWithScripts(dx string, key string, scriptSrcs []string) string {
	decoded, err := base64.StdEncoding.DecodeString(dx)
	if err != nil {
		return ""
	}
	decrypted := xorBytes(decoded, key)
	var queue []tokenItem
	if err := json.Unmarshal([]byte(decrypted), &queue); err != nil {
		return ""
	}
	if len(queue) == 0 {
		return ""
	}

	result, ok := runWithTimeout(queue, key, scriptSrcs, 500*time.Millisecond)
	if !ok {
		return ""
	}
	return result
}

// ── JSON unmarshalling for tokenItem ────────────────────────────────────────

func (t *tokenItem) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if len(raw) == 0 {
		return fmt.Errorf("empty token")
	}
	var opcode int
	if err := json.Unmarshal(raw[0], &opcode); err != nil {
		return err
	}
	t.opcode = opcode
	t.args = make([]any, len(raw)-1)
	for i, r := range raw[1:] {
		var v any
		if err := json.Unmarshal(r, &v); err != nil {
			t.args[i] = nil
		} else {
			t.args[i] = v
		}
	}
	return nil
}

// ── engine core ─────────────────────────────────────────────────────────────

func newEngine(queue []tokenItem, key string, scriptSrcs []string) *engine {
	e := &engine{
		slots:      make(map[int]any),
		queue:      queue,
		scriptSrcs: scriptSrcs,
	}
	e.slots[opQt] = queue   // 9 — token 队列
	e.slots[opYt] = "window" // 10
	e.slots[opon] = key      // 16 — XOR key
	return e
}

func (e *engine) resolve(v any) {
	if e.resolved || e.rejected {
		return
	}
	e.result = base64.StdEncoding.EncodeToString([]byte(toString(v)))
	e.resolved = true
}

func (e *engine) reject(errMsg string) {
	if e.resolved || e.rejected {
		return
	}
	e.errMsg = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d: %s", e.sn, errMsg)))
	e.rejected = true
}

// runQueue 对齐 _n 函数:sdk.pretty.js:1279-1288
func (e *engine) runQueue() {
	for len(e.queue) > 0 && !e.resolved && !e.rejected {
		item := e.queue[0]
		e.queue = e.queue[1:]
		e.sn++
		e.dispatch(item)
	}
}

func (e *engine) dispatch(item tokenItem) {
	defer func() {
		if r := recover(); r != nil {
			e.reject(fmt.Sprintf("panic: %v", r))
		}
	}()

	switch item.opcode {
	case opLt: // 0 — 启动流程(初始化)
		// no-op: queue already running

	case opJt: // 1 — slot[n] = XOR(toStr(slot[n]), toStr(slot[m]))
		if len(item.args) < 2 {
			return
		}
		n := toInt(item.args[0])
		m := toInt(item.args[1])
		e.slots[n] = xorString(toString(e.slots[n]), toString(e.slots[m]))

	case opGt: // 2 — slot[n] = v
		if len(item.args) < 2 {
			return
		}
		e.slots[toInt(item.args[0])] = item.args[1]

	case opWt: // 3 — resolve(btoa(toStr(slot[t])))
		if len(item.args) < 1 {
			return
		}
		e.resolve(toString(e.slots[toInt(item.args[0])]))

	case opZt: // 4 — reject(btoa(Sn + ": " + t))
		if len(item.args) < 1 {
			return
		}
		e.reject(toString(e.slots[toInt(item.args[0])]))

	case opBt: // 5 — push 或拼接
		if len(item.args) < 2 {
			return
		}
		n := toInt(item.args[0])
		m := toInt(item.args[1])
		cur := e.slots[n]
		inc := e.slots[m]
		if arr, ok := cur.([]any); ok {
			e.slots[n] = append(arr, inc)
		} else {
			e.slots[n] = toString(cur) + toString(inc)
		}

	case opHt: // 6 — slot[n] = toStr(slot[m])[toStr(slot[r])]
		if len(item.args) < 3 {
			return
		}
		n := toInt(item.args[0])
		m := toInt(item.args[1])
		r := toInt(item.args[2])
		parent := toString(e.slots[m])
		child := toString(e.slots[r])
		prop := parent + "." + child
		if prop == "window.document.location" {
			e.slots[n] = "https://chatgpt.com/"
		} else {
			e.slots[n] = prop
		}

	case opZt7: // 7 — 通用调用 slot[n](...args.map(toString))
		if len(item.args) < 1 {
			return
		}
		target := e.slots[toInt(item.args[0])]
		args := make([]any, len(item.args)-1)
		for i, a := range item.args[1:] {
			args[i] = toString(e.slots[toInt(a)])
		}
		if s, ok := target.(string); ok {
			e.nativeCall(s, args)
		}

	case opKt: // 8 — slot[n] = toStr(slot[m])
		if len(item.args) < 2 {
			return
		}
		e.slots[toInt(item.args[0])] = toString(e.slots[toInt(item.args[1])])

	case opQt: // 9 — 队列引用(不执行)
	case opYt: // 10 — window 对象(不执行)

	case opXt: // 11 — 扫 document.scripts 匹配 c/[^/]*/_
		if len(item.args) < 1 {
			return
		}
		n := toInt(item.args[0])
		found := ""
		for _, src := range e.scriptSrcs {
			if matchScriptBuild(src) {
				found = src
				break
			}
		}
		if found == "" {
			found = "prod-81e0c5cdf6140e8c5db714d613337f4aeab94029"
		}
		e.slots[n] = found

	case optn: // 12 — slot[t] = Cn(self reference)
		if len(item.args) < 1 {
			return
		}
		e.slots[toInt(item.args[0])] = "self"

	case opnn: // 13 — (reserved)
	case open: // 14 — slot[n] = JSON.parse(slot[m])
		if len(item.args) < 2 {
			return
		}
		n := toInt(item.args[0])
		raw := toString(e.slots[toInt(item.args[1])])
		var v any
		if json.Unmarshal([]byte(raw), &v) == nil {
			e.slots[n] = v
		}

	case oprn: // 15 — slot[n] = JSON.stringify(slot[m])
		if len(item.args) < 2 {
			return
		}
		n := toInt(item.args[0])
		b, err := json.Marshal(e.slots[toInt(item.args[1])])
		if err == nil {
			e.slots[n] = string(b)
		}

	case opon: // 16 — XOR key 常量(已在 newEngine 设置)
	case opcn: // 17 — 异步安全调用
		if len(item.args) < 2 {
			return
		}
		n := toInt(item.args[0])
		target := e.slots[toInt(item.args[1])]
		args := make([]any, len(item.args)-2)
		for i, a := range item.args[2:] {
			args[i] = toString(e.slots[toInt(a)])
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					e.slots[n] = fmt.Sprintf("%v", r)
				}
			}()
			if s, ok := target.(string); ok {
				e.nativeCall(s, args)
			}
		}()

	case opsn: // 18 — slot[n] = atob(slot[n])
		if len(item.args) < 1 {
			return
		}
		n := toInt(item.args[0])
		raw := toString(e.slots[n])
		if dec, err := base64.StdEncoding.DecodeString(raw); err == nil {
			e.slots[n] = string(dec)
		}

	case opun: // 19 — slot[n] = btoa(slot[n])
		if len(item.args) < 1 {
			return
		}
		n := toInt(item.args[0])
		e.slots[n] = base64.StdEncoding.EncodeToString([]byte(toString(e.slots[n])))

	case opfn: // 20 — m === n ? r(...o) : null
		if len(item.args) < 3 {
			return
		}
		eq := toInt(item.args[0])
		val := toInt(item.args[1])
		if eq != val {
			return
		}
		fnSlot := toInt(item.args[2])
		if fn, ok := e.slots[fnSlot].(string); ok {
			args := make([]any, len(item.args)-3)
			for i, a := range item.args[3:] {
				args[i] = toString(e.slots[toInt(a)])
			}
			e.nativeCall(fn, args)
		}

	case opln: // 21 — abs(a-b) > c ? r(...args) : null (NOP in practice)
	case opdn: // 22 — 闭包作用域: save Qt → push args → run → restore Qt
		if len(item.args) < 2 {
			return
		}
		n := toInt(item.args[0])
		m := toInt(item.args[1])
		closure, ok := e.slots[m].([]tokenItem)
		if !ok {
			return
		}
		// _n 的闭包逻辑:保存队列 → 追加闭包内容 → 执行 → 恢复
		saved := e.queue
		e.queue = append([]tokenItem{}, closure...)
		e.runQueue()
		e.queue = saved
		_ = n // n 在某些用法中存储结果,但主要靠副作用

	case opan: // 23 — undefined !== n ? m(...args) : null
		if len(item.args) < 2 {
			return
		}
		n := toInt(item.args[0])
		if e.slots[n] == nil {
			return
		}
		m := toInt(item.args[1])
		if fn, ok := e.slots[m].(string); ok {
			args := make([]any, len(item.args)-2)
			for i, a := range item.args[2:] {
				args[i] = toString(e.slots[toInt(a)])
			}
			e.nativeCall(fn, args)
		}

	case opVt: // 24 — slot[n] = slot[m][slot[r]].bind(slot[m])
		if len(item.args) < 3 {
			return
		}
		n := toInt(item.args[0])
		m := toInt(item.args[1])
		r := toInt(item.args[2])
		e.slots[n] = &nativeBoundMethod{
			method: toString(e.slots[r]),
			bound:  toString(e.slots[m]),
		}

	case ophn, opin, opwn: // 25, 26, 28 — NOP

	case opmn: // 27 — splice(0,1) 或数值减
		if len(item.args) < 2 {
			return
		}
		n := toInt(item.args[0])
		m := toInt(item.args[1])
		if arr, ok := e.slots[n].([]any); ok && len(arr) > 0 {
			e.slots[n] = arr[1:]
			e.slots[m] = arr[0]
		} else {
			a := toFloat64(e.slots[n])
			b := toFloat64(e.slots[m])
			e.slots[n] = a - b
		}

	case opyn: // 29 — slot[n] = toStr(m) < toStr(r)
		if len(item.args) < 3 {
			return
		}
		n := toInt(item.args[0])
		m := toInt(item.args[1])
		r := toInt(item.args[2])
		e.slots[n] = toString(e.slots[m]) < toString(e.slots[r])

	case opgn: // 30 — 可重入 push
		if len(item.args) < 2 {
			return
		}
		n := toInt(item.args[0])
		m := toInt(item.args[1])
		cur := e.slots[n]
		if arr, ok := cur.([]any); ok {
			e.slots[n] = append(arr, e.slots[m])
		} else {
			e.slots[n] = []any{cur, e.slots[m]}
		}

	case op31, op32: // 31, 32 — NOP

	case opvn: // 33 — slot[n] = Number(m) * Number(r)
		if len(item.args) < 3 {
			return
		}
		n := toInt(item.args[0])
		e.slots[n] = toFloat64(e.slots[toInt(item.args[1])]) * toFloat64(e.slots[toInt(item.args[2])])

	case opbn: // 34 — Promise.resolve(slot[m]).then(slot[n] = result)
		if len(item.args) < 2 {
			return
		}
		n := toInt(item.args[0])
		m := toInt(item.args[1])
		e.slots[n] = e.slots[m]

	case opkn: // 35 — slot[n] = Number(m) / Number(r) (除零保护)
		if len(item.args) < 3 {
			return
		}
		n := toInt(item.args[0])
		divisor := toFloat64(e.slots[toInt(item.args[2])])
		if divisor == 0 {
			e.slots[n] = math.NaN()
		} else {
			e.slots[n] = toFloat64(e.slots[toInt(item.args[1])]) / divisor
		}
	}
}

// ── native call dispatch ────────────────────────────────────────────────────

func (e *engine) nativeCall(target string, args []any) {
	switch target {
	case "window.performance.now":
		n := toInt(args[0])
		e.slots[n] = float64(time.Now().UnixNano())/1e6 + rand.Float64()/1e6

	case "window.Object.create":
		n := toInt(args[0])
		e.slots[n] = newOrderedMap()

	case "window.Object.keys":
		if len(args) < 2 {
			return
		}
		n := toInt(args[0])
		v := args[1]
		if s, ok := v.(string); ok && s == "window.localStorage" {
			e.slots[n] = []any{
				"STATSIG_LOCAL_STORAGE_INTERNAL_STORE_V4",
				"STATSIG_LOCAL_STORAGE_STABLE_ID",
				"client-correlated-secret",
				"oai/apps/capExpiresAt",
				"oai-did",
				"STATSIG_LOCAL_STORAGE_LOGGING_REQUEST",
				"UiState.isNavigationCollapsed.1",
			}
		} else if om, ok := v.(*orderedMap); ok {
			keys := make([]any, len(om.keys))
			for i, k := range om.keys {
				keys[i] = k
			}
			e.slots[n] = keys
		}

	case "window.Math.random":
		n := toInt(args[0])
		e.slots[n] = rand.Float64()

	case "window.Reflect.set":
		if len(args) >= 3 {
			if obj, ok := args[0].(*orderedMap); ok {
				obj.add(toString(args[1]), args[2])
			}
		}

	case "window.String.fromCharCode":
		if len(args) >= 2 {
			n := toInt(args[0])
			codes := make([]byte, len(args)-1)
			for i, a := range args[1:] {
				codes[i] = byte(toFloat64FromString(fmt.Sprint(a)))
			}
			e.slots[n] = string(codes)
		}

	case "window.Array.isArray":
		if len(args) >= 2 {
			n := toInt(args[0])
			_, ok := args[1].([]any)
			e.slots[n] = ok
		}

	default:
		// 未知 native:安全忽略
	}
}

// ── timeout wrapper ─────────────────────────────────────────────────────────

func runWithTimeout(queue []tokenItem, key string, scriptSrcs []string, timeout time.Duration) (string, bool) {
	type result struct {
		value string
		ok    bool
	}
	ch := make(chan result, 1)
	go func() {
		e := newEngine(queue, key, scriptSrcs)
		e.runQueue()
		if e.resolved {
			ch <- result{e.result, true}
		} else if e.rejected {
			ch <- result{e.errMsg, false}
		} else {
			ch <- result{"", false}
		}
	}()

	select {
	case r := <-ch:
		return r.value, r.ok
	case <-time.After(timeout):
		return "", false
	}
}

// ── orderedMap (Reflect.set / Object.create 目标) ───────────────────────────

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
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range m.keys {
		if i > 0 {
			b.WriteByte(',')
		}
		kb, _ := json.Marshal(k)
		vb, _ := json.Marshal(m.values[k])
		b.Write(kb)
		b.WriteByte(':')
		b.Write(vb)
	}
	b.WriteByte('}')
	return []byte(b.String()), nil
}

// ── helpers ─────────────────────────────────────────────────────────────────

func xorBytes(data []byte, key string) []byte {
	if key == "" {
		return data
	}
	keyBytes := []byte(key)
	out := make([]byte, len(data))
	for i, b := range data {
		out[i] = b ^ keyBytes[i%len(keyBytes)]
	}
	return out
}

func xorString(text, key string) string {
	if key == "" {
		return text
	}
	return string(xorBytes([]byte(text), key))
}

func toString(v any) string {
	if v == nil {
		return "undefined"
	}
	switch s := v.(type) {
	case string:
		return toNativeString(s)
	case *nativeBoundMethod:
		return "function " + s.method + "() { [native code] }"
	case *orderedMap:
		return "[object Object]"
	case []any:
		parts := make([]string, len(s))
		for i, item := range s {
			parts[i] = toString(item)
		}
		return strings.Join(parts, ",")
	case float64:
		if s == math.Trunc(s) && !math.IsInf(s, 0) {
			return fmt.Sprintf("%d", int64(s))
		}
		return fmt.Sprintf("%g", s)
	case bool:
		if s {
			return "true"
		}
		return "false"
	case json.Number:
		return s.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func toNativeString(s string) string {
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

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		f, _ := n.Float64()
		return int(f)
	default:
		return 0
	}
}

func toFloat64(v any) float64 {
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

func toFloat64FromString(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

// matchScriptBuild 检查 script src 是否匹配 c/[^/]*/_ 模式(Xt opcode 用)
func matchScriptBuild(src string) bool {
	// 简化:查找 "c/" 后跟非 "/" 字符再跟 "/_" 的模式
	idx := strings.Index(src, "/c/")
	if idx < 0 {
		idx = strings.Index(src, "c/")
		if idx < 0 {
			return false
		}
	} else {
		idx++ // skip leading /
	}
	rest := src[idx+2:] // after "c/"
	slashIdx := strings.Index(rest, "/")
	if slashIdx <= 0 {
		return false
	}
	afterSlash := rest[slashIdx:]
	return strings.HasPrefix(afterSlash, "/_")
}

// ── SolveProofToken stub (Phase E: 将在集成时更新) ──────────────────────────

// SolveProofTokenForTurnstile 用 turnstile 引擎解算 proof token。
// 如果 dx 非空则走 VM;否则返回空(调用方回退到旧逻辑)。
func SolveProofTokenForTurnstile(dx, key string) string {
	if dx == "" {
		return ""
	}
	return Solve(dx, key)
}

// unused suppression for context import
var _ = context.Background
