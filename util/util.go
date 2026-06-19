package util

import (
	"log/slog"
	"math/rand"
	"time"

	"github.com/pkoukk/tiktoken-go"
)

// RandomLanguage 返回一个随机但偏好英文的 navigator.language(全英文环境,避免风控)。
// 已剔除 zh-Hans/zh-Hant 等中文 locale,只保留 en-US/en-GB/en 等英文及常见欧洲语种作为多样性来源。
func RandomLanguage() string {
	rand.Seed(time.Now().UnixNano())
	languages := []string{"en-US", "en-GB", "en", "en-AU", "en-CA", "en-NZ", "en-IE", "en-ZA"}
	return languages[rand.Intn(len(languages))]
}

func RandomHexadecimalString() string {
	rand.Seed(time.Now().UnixNano())
	const charset = "0123456789abcdef"
	const length = 16 // The length of the string you want to generate
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
func CountToken(input string) int {
	encoding := "gpt-3.5-turbo"
	tkm, err := tiktoken.EncodingForModel(encoding)
	if err != nil {
		slog.Warn("tiktoken.EncodingForModel error", "error", err)
		return 0
	}
	token := tkm.Encode(input, nil, nil)
	return len(token)
}
