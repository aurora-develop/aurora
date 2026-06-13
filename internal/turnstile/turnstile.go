package turnstile

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

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

// Compatibility wrappers for local API

func SolveWithScripts(dx string, p string, scriptSrcs []string) string {
	return Solve(dx, p)
}
