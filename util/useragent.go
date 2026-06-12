package util

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// userAgentSpec 描述一个主流桌面浏览器的 User-Agent 模板
// 模板中可使用 %d 作为版本占位符
type userAgentSpec struct {
	Template   string
	MinVersion int
	MaxVersion int // 闭区间上界
	Family     string
}

// 模板覆盖 Chrome / Edge / Firefox / Safari 四大主流桌面浏览器，
// 版本号在 [MinVersion, MaxVersion] 闭区间内随机。
var userAgentSpecs = []userAgentSpec{
	{
		Template:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.0.0.0 Safari/537.36",
		MinVersion: 120,
		MaxVersion: 147,
		Family:     "Chrome-Win",
	},
	{
		Template:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.0.0.0 Safari/537.36",
		MinVersion: 120,
		MaxVersion: 147,
		Family:     "Chrome-Mac",
	},
	{
		Template:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.0.0.0 Safari/537.36 Edg/%d.0.0.0",
		MinVersion: 120,
		MaxVersion: 147,
		Family:     "Edge-Win",
	},
	{
		Template:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:%d.0) Gecko/20100101 Firefox/%d.0",
		MinVersion: 120,
		MaxVersion: 132,
		Family:     "Firefox-Win",
	},
	{
		Template:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:%d.0) Gecko/20100101 Firefox/%d.0",
		MinVersion: 120,
		MaxVersion: 132,
		Family:     "Firefox-Mac",
	},
	{
		Template:   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/%d.0 Safari/605.1.15",
		MinVersion: 17,
		MaxVersion: 18,
		Family:     "Safari-Mac",
	},
}

var (
	uaRand     *rand.Rand
	uaRandOnce sync.Once
)

func initUARand() {
	uaRandOnce.Do(func() {
		uaRand = rand.New(rand.NewSource(time.Now().UnixNano()))
	})
}

// RandomUserAgent 返回一个随机的主流桌面浏览器 User-Agent
func RandomUserAgent() string {
	initUARand()
	spec := userAgentSpecs[uaRand.Intn(len(userAgentSpecs))]

	version := spec.MinVersion
	if spec.MaxVersion > spec.MinVersion {
		version += uaRand.Intn(spec.MaxVersion - spec.MinVersion + 1)
	}

	// 部分模板里有两个 %d（如 Edge、Fx），用同一个 version 填充
	return fmt.Sprintf(spec.Template, version, version)
}
