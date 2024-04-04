package util

import (
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
