package util

import (
	"github.com/pkoukk/tiktoken-go"
	"log/slog"
	"math/rand"
	"time"
)

func RandomLanguage() string {
	// 初始化随机数生成器
	rand.Seed(time.Now().UnixNano())
	// 语言列表
	languages := []string{"af", "am", "ar-sa", "as", "az-Latn", "be", "bg", "bn-BD", "bn-IN", "bs", "ca", "ca-ES-valencia", "cs", "cy", "da", "de", "de-de", "el", "en-GB", "en-US", "es", "es-ES", "es-US", "es-MX", "et", "eu", "fa", "fi", "fil-Latn", "fr", "fr-FR", "fr-CA", "ga", "gd-Latn", "gl", "gu", "ha-Latn", "he", "hi", "hr", "hu", "hy", "id", "ig-Latn", "is", "it", "it-it", "ja", "ka", "kk", "km", "kn", "ko", "kok", "ku-Arab", "ky-Cyrl", "lb", "lt", "lv", "mi-Latn", "mk", "ml", "mn-Cyrl", "mr", "ms", "mt", "nb", "ne", "nl", "nl-BE", "nn", "nso", "or", "pa", "pa-Arab", "pl", "prs-Arab", "pt-BR", "pt-PT", "qut-Latn", "quz", "ro", "ru", "rw", "sd-Arab", "si", "sk", "sl", "sq", "sr-Cyrl-BA", "sr-Cyrl-RS", "sr-Latn-RS", "sv", "sw", "ta", "te", "tg-Cyrl", "th", "ti", "tk-Latn", "tn", "tr", "tt-Cyrl", "ug-Arab", "uk", "ur", "uz-Latn", "vi", "wo", "xh", "yo-Latn", "zh-Hans", "zh-Hant", "zu"}
	// 随机选择一个语言
	randomIndex := rand.Intn(len(languages))
	return languages[randomIndex]
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
		slog.Warn("tiktoken.EncodingForModel error:", err)
		return 0
	}
	token := tkm.Encode(input, nil, nil)
	return len(token)
}
