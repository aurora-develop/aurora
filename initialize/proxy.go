package initialize

import (
	"aurora/internal/proxys"
	"bufio"
	"log/slog"
	"net/url"
	"os"
)

func checkProxy() *proxys.IProxy {
	var proxies []string
	proxyUrl := os.Getenv("PROXY_URL")
	if proxyUrl != "" {
		proxies = append(proxies, proxyUrl)
	}

	if _, err := os.Stat("proxies.txt"); err == nil {
		file, _ := os.Open("proxies.txt")
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			proxy := scanner.Text()
			parsedURL, err := url.Parse(proxy)
			if err != nil {
				slog.Warn("proxy url is invalid", "url", proxy, "err", err)
				continue
			}

			// 如果缺少端口信息，不是完整的代理链接
			if parsedURL.Port() != "" {
				proxies = append(proxies, proxy)
			} else {
				continue
			}
		}
	}

	if len(proxies) == 0 {
		proxy := os.Getenv("http_proxy")
		if proxy != "" {
			proxies = append(proxies, proxy)
		}
	}

	proxyIP := proxys.NewIProxyIP(proxies)
	return &proxyIP
}
