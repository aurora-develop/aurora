package turnstile

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// TestSolveWithRealDX 用真实 dx 字符串验证 VM 能解出非空 token。
func TestSolveWithRealDX(t *testing.T) {
	dxBytes, err := os.ReadFile("../../dx.txt")
	if err != nil {
		t.Skip("dx.txt not found, skipping")
	}
	dx := strings.TrimSpace(string(dxBytes))
	dx = strings.Trim(dx, `"`)

	// 尝试用空 key 解码(验证 JSON 结构)
	decoded, err := base64.StdEncoding.DecodeString(dx)
	if err != nil {
		t.Fatalf("base64 decode failed: %v", err)
	}
	t.Logf("dx decoded length: %d bytes", len(decoded))

	// 尝试空 key
	result := Solve(dx, "")
	t.Logf("Solve(dx, '') => len=%d", len(result))
	if result != "" {
		raw, _ := base64.StdEncoding.DecodeString(result)
		t.Logf("Solve(dx, '') result decoded: %s", string(raw))
	}

	// 尝试一些常见 key
	keys := []string{
		"0.123456789",
		"test",
		"wQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4D",
	}
	for _, key := range keys {
		r := Solve(dx, key)
		if r != "" {
			raw, _ := base64.StdEncoding.DecodeString(r)
			t.Logf("Solve(dx, '%s') => %s", key, string(raw))
		}
	}
}

// TestXORDecode 验证 XOR 解码后能否得到合法 JSON。
func TestXORDecode(t *testing.T) {
	dxBytes, err := os.ReadFile("../../dx.txt")
	if err != nil {
		t.Skip("dx.txt not found")
	}
	dx := strings.TrimSpace(string(dxBytes))
	dx = strings.Trim(dx, `"`)
	decoded, err := base64.StdEncoding.DecodeString(dx)
	if err != nil {
		t.Fatal(err)
	}

	// 用空 key 看看是否是合法 JSON
	noXor := string(decoded)
	if json.Valid([]byte(noXor)) {
		t.Log("dx decoded is valid JSON without XOR!")
		var queue []tokenItem
		if err := json.Unmarshal([]byte(noXor), &queue); err == nil {
			t.Logf("Parsed %d tokens without XOR", len(queue))
			for i, tok := range queue {
				if i >= 5 {
					t.Log("...")
					break
				}
				t.Logf("  token[%d]: opcode=%d args=%v", i, tok.opcode, tok.args)
			}
		}
		return
	}

	// 尝试逐字节统计
	var printable, nonPrintable int
	for _, b := range decoded {
		if b >= 32 && b < 127 {
			printable++
		} else {
			nonPrintable++
		}
	}
	t.Logf("Without XOR: printable=%d non-printable=%d total=%d", printable, nonPrintable, len(decoded))

	// 尝试各种 key 做 XOR
	keyCandidates := []string{
		"",
		"0.123456789",
		"test",
		"wQ8Lk5FbGpA2NcR9dShT6gYjU7VxZ4D",
	}
	for _, key := range keyCandidates {
		xored := xorBytes(decoded, key)
		p, np := 0, 0
		for _, b := range xored {
			if b >= 32 && b < 127 {
				p++
			} else {
				np++
			}
		}
		valid := json.Valid(xored)
		t.Logf("key='%s' printable=%d non-printable=%d valid_json=%v", key, p, np, valid)
		if valid {
			var queue []tokenItem
			if err := json.Unmarshal(xored, &queue); err == nil {
				t.Logf("  => %d tokens parsed!", len(queue))
				for i, tok := range queue {
					if i >= 3 {
						t.Log("  ...")
						break
					}
					t.Logf("    token[%d]: opcode=%d args_len=%d", i, tok.opcode, len(tok.args))
				}
			}
		}
	}
}

// TestOpcodes 基础 opcode 单元测试。
func TestOpcodes(t *testing.T) {
	tests := []struct {
		name   string
		queue  []tokenItem
		key    string
		expect string // expected base64-decoded result
	}{
		{
			name: "opcode 2 assign + opcode 3 resolve",
			queue: []tokenItem{
				{opcode: opGt, args: []any{100.0, "hello"}},
				{opcode: opWt, args: []any{100.0}},
			},
			expect: "hello",
		},
		{
			name: "opcode 1 XOR + opcode 3 resolve",
			queue: []tokenItem{
				{opcode: opGt, args: []any{100.0, "abc"}},
				{opcode: opGt, args: []any{101.0, "key"}},
				{opcode: opJt, args: []any{100.0, 101.0}},
				{opcode: opWt, args: []any{100.0}},
			},
			expect: "\n\a\x1a", // XOR("abc","key") = 0x61^0x6b, 0x62^0x65, 0x63^0x79
		},
		{
			name: "opcode 4 reject",
			queue: []tokenItem{
				{opcode: opGt, args: []any{100.0, "error_msg"}},
				{opcode: opZt, args: []any{100.0}},
			},
			expect: "", // rejected, not resolved
		},
		{
			name: "opcode 8 toString",
			queue: []tokenItem{
				{opcode: opGt, args: []any{100.0, 42.0}},
				{opcode: opKt, args: []any{101.0, 100.0}},
				{opcode: opWt, args: []any{101.0}},
			},
			expect: "42",
		},
		{
			name: "opcode 5 string concat",
			queue: []tokenItem{
				{opcode: opGt, args: []any{100.0, "hello"}},
				{opcode: opGt, args: []any{101.0, " world"}},
				{opcode: opBt, args: []any{100.0, 101.0}},
				{opcode: opWt, args: []any{100.0}},
			},
			expect: "hello world",
		},
		{
			name: "opcode 19 btoa + opcode 3 resolve",
			queue: []tokenItem{
				{opcode: opGt, args: []any{100.0, "test"}},
				{opcode: opun, args: []any{100.0}},
				{opcode: opWt, args: []any{100.0}},
			},
			expect: "dGVzdA==",
		},
		{
			name: "opcode 33 multiply",
			queue: []tokenItem{
				{opcode: opGt, args: []any{100.0, 6.0}},
				{opcode: opGt, args: []any{101.0, 7.0}},
				{opcode: opvn, args: []any{102.0, 100.0, 101.0}},
				{opcode: opWt, args: []any{102.0}},
			},
			expect: "42",
		},
		{
			name: "opcode 35 divide",
			queue: []tokenItem{
				{opcode: opGt, args: []any{100.0, 84.0}},
				{opcode: opGt, args: []any{101.0, 2.0}},
				{opcode: opkn, args: []any{102.0, 100.0, 101.0}},
				{opcode: opWt, args: []any{102.0}},
			},
			expect: "42",
		},
		{
			name: "opcode 35 divide by zero",
			queue: []tokenItem{
				{opcode: opGt, args: []any{100.0, 1.0}},
				{opcode: opGt, args: []any{101.0, 0.0}},
				{opcode: opkn, args: []any{102.0, 100.0, 101.0}},
				{opcode: opWt, args: []any{102.0}},
			},
			expect: "0",
		},
		{
			name: "opcode 29 less-than",
			queue: []tokenItem{
				{opcode: opGt, args: []any{100.0, "a"}},
				{opcode: opGt, args: []any{101.0, "b"}},
				{opcode: opyn, args: []any{102.0, 100.0, 101.0}},
				{opcode: opWt, args: []any{102.0}},
			},
			expect: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := runWithTimeout(tt.queue, tt.key, nil, 500*time.Millisecond)
			if tt.expect == "" {
				if ok {
					t.Errorf("expected rejected, got resolved: %s", result)
				}
				return
			}
			if !ok {
				t.Fatal("expected resolved, got rejected/timeout")
			}
			decoded, err := base64.StdEncoding.DecodeString(result)
			if err != nil {
				t.Fatalf("result is not valid base64: %v", err)
			}
			if string(decoded) != tt.expect {
				t.Errorf("got %q, want %q", string(decoded), tt.expect)
			}
		})
	}
}
