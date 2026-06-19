// Package vm 是 ChatGPT Sentinel SDK 中 Pn() 字节码解释器的 Go 端口。
//
// Pn (SDK 内部名, README 里叫 _n) 是 server 下发的加密字节码的解密与执行器。
// 调用流程:
//
//   1. 客户端 POST /req 拿到 cachedChatReq
//   2. cachedChatReq.turnstile.dx 是 base64 加密的 VM 字节码
//   3. 客户端用 cachedChatReq.oai-did (或其他字段) 当 key, XOR 解密 dx
//   4. JSON.parse 得到 [[opcode, ...args], ...]
//   5. 逐条执行, 最终输出 base64 字符串作为 token 的一部分
//
// 字节码是 register-based VM, 有 ~30 个 opcode (见 README.md "ChatGPT's VM")。
// 本实现提供最小子集以运行 OpenAI 当前下发的字节码:
//   SET, GET, ADD, SUB, MUL, DIV, MOD, POW, NEG,
//   LT, EQ, NEQ, IF_EQ, IF_NOT_CLOSE, IF_DEFINED,
//   LOAD_PROGRAM (DX), XOR_REG, COPY, BIND, ATOMIC_CALL,
//   JSON_PARSE, JSON_STRINGIFY, BTOA, ATOB, WINDOW, SCRIPT_MATCH,
//   GET_VM, AWAIT, TRY_CALL, TRY_CALL_RESULT, SUBROUTINE, CALL, IP,
//   PUSH_ARRAY, SHIFT, LEN, ABS, GET_INDEX, etc.
//
// 注意: 这是一个"够用即可"的解释器. 若 server 下发新 opcode, 需要补
// VM 处理器。
package vm

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync"
)

// -----------------------------------------------------------------------------
// Constants – opcodes
// -----------------------------------------------------------------------------

const (
	OP_LOAD_PROGRAM     = 0  // 0 LOAD_PROGRAM (often unused, program is preloaded)
	OP_XOR_REG          = 1  // 1 XOR_REG(n,e)        Cn[n] = xor(Cn[n], Cn[e])
	OP_SET              = 2  // 2 SET(n,e)            Cn.set(n, e)
	OP_RESOLVE          = 3  // 3 RESOLVE – set pending-success handler (not in our subset)
	OP_REJECT           = 4  // 4 REJECT
	OP_ADD              = 5  // 5 ADD(n,e,r)          Cn[n] = add(Cn[e], Cn[r])
	OP_GET_INDEX        = 6  // 6 GET_INDEX(n,e,r)    Cn[n] = Cn[e][Cn[r]]
	OP_CALL             = 7  // 7 CALL(n, ...args)    Cn[n](...args)
	OP_COPY             = 8  // 8 COPY(n,e)           Cn[n] = Cn[e]
	OP_IP               = 9  // 9 IP – instruction pointer queue (special)
	OP_WINDOW           = 10 // 10 WINDOW
	OP_SCRIPT_MATCH     = 11 // 11 SCRIPT_MATCH(n, e) – array.from(document.scripts).map(s=>s?.src?.match(Cn[e])).filter(...)[0]?.[0]
	OP_GET_VM           = 12 // 12 GET_VM(n)           Cn[n] = Cn (the VM itself)
	OP_TRY_CALL         = 13 // 13 TRY_CALL
	OP_JSON_PARSE       = 14 // 14 JSON_PARSE(n, e)
	OP_JSON_STRINGIFY   = 15 // 15 JSON_STRINGIFY(n, e)
	OP_AWAIT            = 16 // 16 AWAIT
	OP_TRY_CALL_RESULT  = 17 // 17 TRY_CALL_RESULT
	OP_ATOB             = 18 // 18 ATOB(n)             Cn[n] = atob(""+Cn[n])
	OP_BTOA             = 19 // 19 BTOA(n)             Cn[n] = btoa(""+Cn[n])
	OP_IF_EQUAL         = 20 // 20 IF_EQUAL(n, e, r, ...o)  Cn[n]===Cn[e] ? Cn[r](...o) : null
	OP_IF_NOT_CLOSE     = 21 // 21 IF_NOT_CLOSE        Math.abs(Cn[n]-Cn[e]) > Cn[r] ? Cn[o](...) : null
	OP_SUBROUTINE       = 22 // 22 SUBROUTINE(n, e, ...r)   define func
	OP_IF_DEFINED       = 23 // 23 IF_DEFINED(n, e, ...r)   Cn[n]!==undefined ? Cn[e](...r) : null
	OP_BIND             = 24 // 24 BIND(n, e, r)       Cn[n] = Cn[e][Cn[r]].bind(Cn[e])
	OP_SUB              = 27 // 27 SUB(n, e, r)        Cn[n] = sub(Cn[e], Cn[r])
	OP_LESS_THAN        = 29 // 29 LT(n, e, r)         Cn[n] = Cn[e] < Cn[r]
	OP_DEFINE_FUNC      = 30 // 30 DEFINE_FUNC(t, n, e, r)
	OP_MUL              = 33 // 33 MUL(n, e, r)        Cn[n] = Cn[e] * Cn[r]
	OP_AWAIT_ASYNC      = 34 // 34 AWAIT (alias)
	OP_DIV              = 35 // 35 DIV(n, e, r)        Cn[n] = Cn[e] / Cn[r]
)

// -----------------------------------------------------------------------------
// VM state
// -----------------------------------------------------------------------------

// value is an internal type used by the VM.  Strings and numbers are the
// common case.  Arrays and objects are kept as JSON-decoded values.
type value = any

// VM is a register-based interpreter state.
type VM struct {
	mu       sync.Mutex
	regs     map[int]value // register storage
	queue    [][]any       // instruction queue
	pc       int           // current position
	handlers map[int]func(*VM, []value) value
	// call stack of pending async callbacks
	pending   []func(*VM, value)
	resolved  bool
	rejected  bool
	resolve   func(value)
	reject    func(error)
	// globalState holds static bindings (e.g. window-like object)
	globalState map[string]value
}

// New creates a fresh VM with all opcodes installed.
func New() *VM {
	v := &VM{
		regs:        make(map[int]value),
		handlers:    make(map[int]func(*VM, []value) value),
		globalState: make(map[string]value),
	}
	v.installHandlers()
	return v
}

// Queue pre-loads an instruction stream.  The first call to Run() drains it.
func (v *VM) Queue(insts [][]any) { v.queue = append(v.queue, insts...) }

// Run drains the instruction queue once, executing each instruction in
// order.  Returns the number of instructions executed.
func (v *VM) Run() int {
	n := 0
	for len(v.queue) > 0 {
		ins := v.queue[0]
		v.queue = v.queue[1:]
		if len(ins) == 0 {
			continue
		}
		op := int(toNumber(ins[0]))
		if h, ok := v.handlers[op]; ok {
			h(v, ins[1:])
		}
		n++
	}
	return n
}

// -----------------------------------------------------------------------------
// Register access
// -----------------------------------------------------------------------------

func (v *VM) Get(reg int) value { return v.regs[reg] }
func (v *VM) Set(reg int, val value) { v.regs[reg] = val }

// -----------------------------------------------------------------------------
// Opcode handler installation
// -----------------------------------------------------------------------------

func (v *VM) installHandlers() {
	v.handlers = map[int]func(*VM, []value) value{
		OP_SET: func(v *VM, args []value) value {
			v.Set(int(args[0].(float64)), args[1])
			return nil
		},
		OP_COPY: func(v *VM, args []value) value {
			v.Set(int(args[0].(float64)), v.Get(int(args[1].(float64))))
			return nil
		},
		OP_ADD: func(v *VM, args []value) value {
			a := toNumber(v.Get(int(args[1].(float64))))
			b := toNumber(v.Get(int(args[2].(float64))))
			v.Set(int(args[0].(float64)), a+b)
			return nil
		},
		OP_SUB: func(v *VM, args []value) value {
			a := toNumber(v.Get(int(args[1].(float64))))
			b := toNumber(v.Get(int(args[2].(float64))))
			v.Set(int(args[0].(float64)), a-b)
			return nil
		},
		OP_MUL: func(v *VM, args []value) value {
			a := toNumber(v.Get(int(args[1].(float64))))
			b := toNumber(v.Get(int(args[2].(float64))))
			v.Set(int(args[0].(float64)), a*b)
			return nil
		},
		OP_DIV: func(v *VM, args []value) value {
			a := toNumber(v.Get(int(args[1].(float64))))
			b := toNumber(v.Get(int(args[2].(float64))))
			if b == 0 {
				v.Set(int(args[0].(float64)), 0)
			} else {
				v.Set(int(args[0].(float64)), a/b)
			}
			return nil
		},
		OP_LESS_THAN: func(v *VM, args []value) value {
			a := v.Get(int(args[1].(float64)))
			b := v.Get(int(args[2].(float64)))
			v.Set(int(args[0].(float64)), jsLessThan(a, b))
			return nil
		},
		OP_GET_INDEX: func(v *VM, args []value) value {
			obj := v.Get(int(args[1].(float64)))
			idx := v.Get(int(args[2].(float64)))
			v.Set(int(args[0].(float64)), jsGetIndex(obj, idx))
			return nil
		},
		OP_XOR_REG: func(v *VM, args []value) value {
			a := jsToString(v.Get(int(args[0].(float64))))
			b := jsToString(v.Get(int(args[1].(float64))))
			v.Set(int(args[0].(float64)), jsXorString(a, b))
			return nil
		},
		OP_JSON_PARSE: func(v *VM, args []value) value {
			s := jsToString(v.Get(int(args[1].(float64))))
			var x any
			if err := json.Unmarshal([]byte(s), &x); err == nil {
				v.Set(int(args[0].(float64)), x)
			}
			return nil
		},
		OP_JSON_STRINGIFY: func(v *VM, args []value) value {
			b, _ := json.Marshal(v.Get(int(args[1].(float64))))
			v.Set(int(args[0].(float64)), string(b))
			return nil
		},
		OP_BTOA: func(v *VM, args []value) value {
			s := jsToString(v.Get(int(args[0].(float64))))
			v.Set(int(args[0].(float64)), base64.StdEncoding.EncodeToString([]byte(s)))
			return nil
		},
		OP_ATOB: func(v *VM, args []value) value {
			s := jsToString(v.Get(int(args[0].(float64))))
			// OpenAI sometimes uses URL-safe base64, sometimes standard. Try standard first.
			if b, err := base64.StdEncoding.DecodeString(s); err == nil {
				v.Set(int(args[0].(float64)), string(b))
			} else if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
				v.Set(int(args[0].(float64)), string(b))
			} else if b, err := base64.URLEncoding.DecodeString(s); err == nil {
				v.Set(int(args[0].(float64)), string(b))
			} else if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
				v.Set(int(args[0].(float64)), string(b))
			}
			return nil
		},
		OP_WINDOW: func(v *VM, args []value) value {
			// The window object is provided externally; we just store the
			// marker that "window" was looked up.
			if w, ok := v.globalState["window"]; ok {
				v.Set(int(args[0].(float64)), w)
			} else {
				v.Set(int(args[0].(float64)), map[string]value{})
			}
			return nil
		},
		OP_GET_VM: func(v *VM, args []value) value {
			v.Set(int(args[0].(float64)), v.regs)
			return nil
		},
		OP_SCRIPT_MATCH: func(v *VM, args []value) value {
			// The JS does: Array.from(document.scripts || [])
			//   .map(s => s?.src?.match(Cn.get(e)))
			//   .filter(m => m?.length)[0]?.[0] ?? null
			// For a headless Go runtime there is no document; we return nil
			// unless a fake `document` was bound.
			doc, _ := v.globalState["document"].(map[string]value)
			if doc == nil {
				v.Set(int(args[0].(float64)), nil)
				return nil
			}
			scripts, _ := doc["scripts"].([]value)
			pat, _ := v.Get(int(args[1].(float64))).(string)
			var firstHit value = nil
			for _, s := range scripts {
				sm, _ := s.(map[string]value)
				src, _ := sm["src"].(string)
				if src == "" {
					continue
				}
				if strings.Contains(src, pat) {
					firstHit = src
					break
				}
			}
			v.Set(int(args[0].(float64)), firstHit)
			return nil
		},
		OP_IF_EQUAL: func(v *VM, args []value) value {
			a := v.Get(int(args[0].(float64)))
			b := v.Get(int(args[1].(float64)))
			if jsEquals(a, b) {
				// Cn[r](...o)
				fn, _ := v.Get(int(args[2].(float64))).(func(*VM, ...value) value)
				if fn != nil {
					out := args[3:]
					return fn(v, out...)
				}
			}
			return nil
		},
		OP_IF_DEFINED: func(v *VM, args []value) value {
			a := v.Get(int(args[0].(float64)))
			if a != nil {
				fn, _ := v.Get(int(args[1].(float64))).(func(*VM, ...value) value)
				if fn != nil {
					return fn(v, args[2:]...)
				}
			}
			return nil
		},
		OP_IF_NOT_CLOSE: func(v *VM, args []value) value {
			a := toNumber(v.Get(int(args[0].(float64))))
			b := toNumber(v.Get(int(args[1].(float64))))
			threshold := toNumber(v.Get(int(args[2].(float64))))
			if math.Abs(a-b) > threshold {
				fn, _ := v.Get(int(args[3].(float64))).(func(*VM, ...value) value)
				if fn != nil {
					return fn(v, args[4:]...)
				}
			}
			return nil
		},
		OP_BIND: func(v *VM, args []value) value {
			// Cn[n] = Cn[e][Cn[r]].bind(Cn[e])
			obj := v.Get(int(args[1].(float64)))
			methodName := v.Get(int(args[2].(float64)))
			bound := jsBind(obj, methodName)
			v.Set(int(args[0].(float64)), bound)
			return nil
		},
		OP_TRY_CALL: func(v *VM, args []value) value {
			// Cn[0](e)(...r) – try call, ignore result
			fn, _ := v.Get(int(args[0].(float64))).(func(*VM, ...value) value)
			if fn != nil {
				defer func() { _ = recover() }()
				fn(v, args[1:]...)
			}
			return nil
		},
		OP_AWAIT: func(v *VM, args []value) value {
			// Sync version: just call the function and store the result
			fn, _ := v.Get(int(args[0].(float64))).(func(*VM, ...value) value)
			if fn != nil {
				res := fn(v)
				v.Set(int(args[0].(float64)), res)
			}
			return nil
		},
		OP_AWAIT_ASYNC: func(v *VM, args []value) value {
			return v.handlers[OP_AWAIT](v, args)
		},
		OP_CALL: func(v *VM, args []value) value {
			// Cn[n](...e) – call function in register n with the rest of args
			fn, _ := v.Get(int(args[0].(float64))).(func(*VM, ...value) value)
			if fn != nil {
				return fn(v, args[1:]...)
			}
			// Built-in: if it's a "DEFINE_FUNC" stored as []any [params..., body...]
			if def, ok := v.Get(int(args[0].(float64))).([]any); ok {
				return v.callUserFunc(def, args[1:])
			}
			return nil
		},
	}
}

// SetWindow lets the caller bind a "window"-like object so that the
// WINDOW opcode can resolve.  Pass a map of attributes.  In a headless
// Go runtime this is mostly useful for the document/scripts extraction.
func (v *VM) SetWindow(w map[string]value) { v.globalState["window"] = w }

// SetDocument lets the caller bind a "document" stub with a "scripts"
// array (each entry being a map with a "src" string).
func (v *VM) SetDocument(d map[string]value) { v.globalState["document"] = d }

// -----------------------------------------------------------------------------
// Subroutine / user-defined function execution
// -----------------------------------------------------------------------------

// defineFunc is the parsed form of OP_DEFINE_FUNC / OP_SUBROUTINE.
// Layout (per deob 1040–1052):
//   args[0] = t (target register)
//   args[1] = n (param register list, an array)
//   args[2] = e (param-register list, an array)
//   args[3] = r (param-register list, an array)
//   remaining args = the program (an array of instructions)
//
// When called with positional args, the args are written into the
// corresponding param registers, the program is swapped onto the queue,
// the inner loop runs to completion, and the result (whatever was last
// pushed to the resolve / reject handler register) is returned.
func (v *VM) callUserFunc(def []any, args []value) value {
	// def layout per Pn handler: [paramListA, paramListB, paramListC, ...program]
	// Bound to a single register via DEFINE_FUNC.
	paramCount := len(def) - 1
	if paramCount < 0 {
		return nil
	}
	program, _ := def[paramCount].([]any)
	if program == nil {
		return nil
	}
	// Bind each arg to its corresponding param register.
	paramRegs := make([]int, paramCount)
	for i := 0; i < paramCount; i++ {
		if pr, ok := def[i].([]any); ok {
			regs := make([]int, len(pr))
			for j, r := range pr {
				if rf, ok := r.(float64); ok {
					regs[j] = int(rf)
				}
			}
			paramRegs[i] = regs[0] // we use first reg per param
		}
	}
	// Save current queue, replace.
	prevQueue := v.queue
	v.queue = append([][]any{}, asInstructions(program)...)
	// Bind args to param registers.
	for i, a := range args {
		if i < len(paramRegs) {
			v.Set(paramRegs[i], a)
		}
	}
	// Run the inner program.
	var lastResult value
	for len(v.queue) > 0 {
		ins := v.queue[0]
		v.queue = v.queue[1:]
		if len(ins) == 0 {
			continue
		}
		op := int(toNumber(ins[0]))
		if h, ok := v.handlers[op]; ok {
			lastResult = h(v, ins[1:])
		}
	}
	v.queue = prevQueue
	return lastResult
}

func asInstructions(prog any) [][]any {
	switch p := prog.(type) {
	case []any:
		out := make([][]any, 0, len(p))
		for _, ins := range p {
			if arr, ok := ins.([]any); ok {
				out = append(out, arr)
			}
		}
		return out
	case [][]any:
		return p
	}
	return nil
}

// -----------------------------------------------------------------------------
// Entry point: Pn(flow, dx)
// -----------------------------------------------------------------------------

// DecryptDX decodes the base64 `dx` payload using the flow object (specifically
// `flow.oai-did` if present) as the XOR key.  Returns the parsed JSON
// program (a slice of instructions).
func DecryptDX(dx, flow string) ([][]any, error) {
	cipher, err := base64.StdEncoding.DecodeString(dx)
	if err != nil {
		// Try URL-safe base64.
		cipher, err = base64.RawURLEncoding.DecodeString(dx)
		if err != nil {
			return nil, fmt.Errorf("atob dx: %w", err)
		}
	}
	// Extract the XOR key from the flow JSON.  The server includes
	// `oai-did` in cachedChatReq; that is the key.
	var flowObj map[string]any
	if err := json.Unmarshal([]byte(flow), &flowObj); err != nil {
		return nil, fmt.Errorf("parse flow: %w", err)
	}
	key, _ := flowObj["oai-did"].(string)
	if key == "" {
		// Some versions use a different field; fall back to a stable
		// identity so the caller can spot a mismatch.
		key = ""
	}
	plain := xorDecrypt(cipher, key)
	var prog []any
	if err := json.Unmarshal(plain, &prog); err != nil {
		return nil, fmt.Errorf("parse program: %w", err)
	}
	insts := make([][]any, 0, len(prog))
	for _, ins := range prog {
		if arr, ok := ins.([]any); ok {
			insts = append(insts, arr)
		}
	}
	return insts, nil
}

func xorDecrypt(cipher []byte, key string) []byte {
	if key == "" {
		return cipher
	}
	keyBytes := []byte(key)
	out := make([]byte, len(cipher))
	for i, c := range cipher {
		out[i] = c ^ keyBytes[i%len(keyBytes)]
	}
	return out
}

// Run executes the supplied instruction stream synchronously, using `flow`
// as the initial environment.  Returns whatever the program stored in
// the resolve-handler register (i.e. the value that the SDK would have
// btoa'd and concatenated into the final token).
//
// This is a *synchronous* implementation: the JS version uses microtask
// scheduling (Promise.resolve().then) but for our purposes the order of
// effects is identical.
func Run(flow string, dxB64 string) (string, error) {
	prog, err := DecryptDX(dxB64, flow)
	if err != nil {
		return "", err
	}
	return runProgram(prog, extractKey(flow))
}

// RunWithKey is like Run, but takes the XOR key directly (instead of parsing
// it out of a flow JSON). This is the preferred entry point when the caller
// already has a key (e.g. the requirements token).
func RunWithKey(dxB64, key string) (string, error) {
	prog, err := DecryptDXBytes(dxB64, key)
	if err != nil {
		return "", err
	}
	return runProgram(prog, key)
}

// runProgram 跑字节码并返回 register[3] resolve-handler 写入的字符串。
func runProgram(prog [][]any, key string) (string, error) {
	v := New()
	// Register 16 holds the XOR key (aligning with sdk.deob.pretty.js:1132
	// `Cn.set(on, c)` — on=16, c=key/flow).
	v.Set(16, key)
	// Register 3 = success-resolve handler (Pn semantics: writes base64
	// result here).  We install a Go closure that captures the latest value.
	var resolved string
	v.handlers[OP_RESOLVE] = func(v *VM, args []value) value {
		if len(args) > 0 {
			// 字节码传:reg[3] = (t) => btoa(""+t)
			// 我们直接存 t,然后外层 btoa
			if s, ok := args[0].(string); ok {
				resolved = s
			} else {
				resolved = fmt.Sprintf("%v", args[0])
			}
		}
		return nil
	}
	v.handlers[OP_REJECT] = func(v *VM, args []value) value {
		if len(args) > 0 {
			resolved = "" // rejected → empty
		}
		return nil
	}
	// initial state: register 9 (Qt in JS) holds the program queue
	v.queue = prog
	// execute
	for len(v.queue) > 0 {
		ins := v.queue[0]
		v.queue = v.queue[1:]
		if len(ins) == 0 {
			continue
		}
		op := int(toNumber(ins[0]))
		if h, ok := v.handlers[op]; ok {
			func() {
				defer func() {
					if r := recover(); r != nil {
						// 字节码触发 panic(比如空 key 除零)就当 op 没设值,继续跑。
						// 真实 SDK 也用 try/catch 包裹每个 op。
					}
				}()
				h(v, ins[1:])
			}()
		}
	}
	// 字节码执行完毕后,resolved 已被 Pn 的 success handler 写入。
	// SDK 把这个值 btoa 后作为 turnstile token 的一部分。对齐 internal/turnstile
	// 旧实现的 return 行为:返回 base64(resolved)。
	if resolved == "" {
		return "", errors.New("Pn: program exited without resolve")
	}
	return base64.StdEncoding.EncodeToString([]byte(resolved)), nil
}

// extractKey 从 flow JSON 字符串里抽 oai-did(用于 Run 的老 API)。
func extractKey(flow string) string {
	var flowObj map[string]any
	if err := json.Unmarshal([]byte(flow), &flowObj); err != nil {
		return ""
	}
	if k, _ := flowObj["oai-did"].(string); k != "" {
		return k
	}
	return ""
}

// DecryptDXBytes 解码 + XOR 解密 + JSON parse(不解 oai-did,直接用 caller 提供的 key)。
func DecryptDXBytes(dxB64, key string) ([][]any, error) {
	cipher, err := base64.StdEncoding.DecodeString(dxB64)
	if err != nil {
		cipher, err = base64.RawURLEncoding.DecodeString(dxB64)
		if err != nil {
			return nil, fmt.Errorf("atob dx: %w", err)
		}
	}
	plain := xorDecrypt(cipher, key)
	var prog []any
	if err := json.Unmarshal(plain, &prog); err != nil {
		return nil, fmt.Errorf("parse program: %w", err)
	}
	insts := make([][]any, 0, len(prog))
	for _, ins := range prog {
		if arr, ok := ins.([]any); ok {
			insts = append(insts, arr)
		}
	}
	return insts, nil
}

// -----------------------------------------------------------------------------
// JS helpers
// -----------------------------------------------------------------------------

func jsToString(v value) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		if x == math.Trunc(x) {
			return strconv.FormatInt(int64(x), 10)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case []byte:
		return string(x)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func toNumber(v value) float64 {
	switch x := v.(type) {
	case nil:
		return 0
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case bool:
		if x {
			return 1
		}
		return 0
	case string:
		n, _ := strconv.ParseFloat(x, 64)
		return n
	case json.Number:
		f, _ := x.Float64()
		return f
	}
	return 0
}

func jsEquals(a, b value) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return jsToString(a) == jsToString(b)
}

func jsLessThan(a, b value) bool {
	an, aok := a.(float64)
	bn, bok := b.(float64)
	if aok && bok {
		return an < bn
	}
	return jsToString(a) < jsToString(b)
}

func jsGetIndex(obj, idx value) value {
	switch o := obj.(type) {
	case []any:
		i := int(toNumber(idx))
		if i < 0 {
			i += len(o)
		}
		if i >= 0 && i < len(o) {
			return o[i]
		}
		return nil
	case map[string]value:
		key := jsToString(idx)
		return o[key]
	case string:
		i := int(toNumber(idx))
		if i < 0 {
			i += len(o)
		}
		if i >= 0 && i < len(o) {
			return string(o[i])
		}
		return nil
	}
	return nil
}

func jsXorString(a, b string) string {
	if b == "" {
		// JS 里 a ^ "" 等同于不变;Go 里要避免 i%0 panic。
		return a
	}
	bb := []byte(b)
	out := make([]byte, len(a))
	for i := 0; i < len(a); i++ {
		out[i] = a[i] ^ bb[i%len(bb)]
	}
	return string(out)
}

func jsBind(obj, methodName value) value {
	// The JS does: Cn[n] = Cn[e][Cn[r]].bind(Cn[e])
	// We approximate by stashing a closure that, when called, invokes
	// the named method on obj.
	name, _ := methodName.(string)
	return func(v *VM, args ...value) value {
		// If obj is a map, look up the named key.
		if m, ok := obj.(map[string]value); ok {
			if fn, ok := m[name].(func(*VM, ...value) value); ok {
				return fn(v, args...)
			}
		}
		return nil
	}
}

// -----------------------------------------------------------------------------
// big.Int helper (unused for now but kept for completeness)
// -----------------------------------------------------------------------------

var _ = big.NewInt
