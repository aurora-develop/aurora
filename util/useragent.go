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

// 模板限定为 Edge（Windows）一族。
//
// 为什么不能用其他浏览器：internal/chatgpt 的 createBaseHeaderForState 同时
// 写死了 sec-ch-ua = "Microsoft Edge";v="146"，如果 user-agent 跟 sec-ch-ua
// 不一致，Cloudflare/ChatGPT 一眼就能看出是脚本客户端。版本号在
// [MinVersion, MaxVersion] 闭区间内随机，仍然保留一定的指纹多样性。
var userAgentSpecs = []userAgentSpec{
	{
		Template:   "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.0.0.0 Safari/537.36 Edg/%d.0.0.0",
		MinVersion: 120,
		MaxVersion: 147,
		Family:     "Edge-Win",
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
