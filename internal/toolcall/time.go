package toolcall

import (
	"crypto/rand"
	"encoding/hex"
)

// 24 字符的 hex 字符串,等价于 12 字节随机数;和  的
// "uuid.uuid4().hex[:24]" 行为一致 —— 24 个 hex 字符。
func newCallIDSuffix() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
