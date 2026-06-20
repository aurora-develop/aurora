package util

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

const FixedUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"

// userAgentSpec 描述一个主流桌面浏览器的 User-Agent 模板
// 模板中可使用 %d 作为版本占位符
type userAgentSpec struct {
	Template   string
	MinVersion int
	MaxVersion int // 闭区间上界
	Family     string
}

// 保留 RandomUserAgent 用于其它场景(测试、临时抓包等)。生产路径走 FixedUserAgent。
var userAgentSpecs = []userAgentSpec{
	{
		Template:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.0.0.0 Safari/537.36",
		MinVersion: 146,
		MaxVersion: 148,
		Family:     "Chrome-Win",
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

	// 数一下模板里 %d 的个数,按个数填充 version
	placeholders := strings.Count(spec.Template, "%d")
	switch placeholders {
	case 1:
		return fmt.Sprintf(spec.Template, version)
	default:
		// 0 个或 2+ 个(兼容老 Edge 模板): 都用 version 填充
		return fmt.Sprintf(strings.Replace(spec.Template, "%d", "%v", -1), version)
	}
}
