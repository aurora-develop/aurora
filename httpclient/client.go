package httpclient

import (
	"crypto/tls"
	browser "github.com/EDDYCJY/fake-useragent"
	"github.com/go-resty/resty/v2"
	"net/http"
	"time"
)

type RestyClient struct {
	Client *resty.Client
}

func NewStdClient() *RestyClient {
	client := &RestyClient{
		Client: resty.NewWithClient(&http.Client{
			Transport: &http.Transport{
				// 禁用长连接
				DisableKeepAlives: true,
				// 配置TLS设置，跳过证书验证
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
		}),
	}
	client.Client.SetTimeout(600 * time.Second)
	client.Client.SetHeader("user-agent", browser.Random()).
		SetHeader("accept", "*/*").
		SetHeader("accept-language", "en-US,en;q=0.9").
		SetHeader("cache-control", "no-cache").
		SetHeader("content-type", "application/json").
		SetHeader("oai-language", "en-US").
		SetHeader("pragma", "no-cache").
		SetHeader("sec-ch-ua", `"Google Chrome";v="123", "Not:A-Brand";v="8", "Chromium";v="123"`).
		SetHeader("sec-ch-ua-mobile", "?0").
		SetHeader("sec-ch-ua-platform", "Windows").
		SetHeader("sec-fetch-dest", "empty").
		SetHeader("sec-fetch-mode", "cors").
		SetHeader("sec-fetch-site", "same-origin")
	return client
}
