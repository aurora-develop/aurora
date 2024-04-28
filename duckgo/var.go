package duckgo

import (
	"sync"
	"time"
)

const (
	Claude = "claude-instant-1.2"
	GPT3   = "gpt-3.5-turbo-0125"
	UA     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

var (
	Token *XqdgToken
)

type XqdgToken struct {
	Token    string     `json:"token"`
	M        sync.Mutex `json:"-"`
	ExpireAt time.Time  `json:"expire"`
}
