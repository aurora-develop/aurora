package main

import (
	"aurora/internal/proxys"
	"aurora/internal/tokens"
	"bufio"
	"fmt"
	"net/url"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var (
	HOST          string
	PORT          string
	ACCESS_TOKENS tokens.AccessToken
	ProxyIP       proxys.IProxy
)

func init() {
	_ = godotenv.Load(".env")

	HOST = os.Getenv("SERVER_HOST")
	PORT = os.Getenv("SERVER_PORT")

	if HOST == "" {
		HOST = "0.0.0.0"
	}
	if PORT == "" {
		PORT = "8080"
	}
	checkProxy()
	readAccessToken()
}

func checkProxy() {
	proxies := []string{}
	PROXY_URL := os.Getenv("PROXY_URL")
	if PROXY_URL != "" {
		proxies = append(proxies, PROXY_URL)
	}
	if _, err := os.Stat("proxies.txt"); err == nil {
		file, _ := os.Open("proxies.txt")
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			proxy := scanner.Text()
			parsedURL, _ := url.Parse(proxy)
			if err != nil {
				fmt.Println("无法解析URL:", err)
				return
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
	ProxyIP = proxys.NewIProxyIP(proxies)

}

func main() {
	router := gin.Default()

	router.Use(cors)

	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Hello, world!",
		})
	})

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	router.OPTIONS("/v1/chat/completions", optionsHandler)
	router.POST("/v1/chat/completions", nightmare)
	router.GET("/v1/models", engines_handler)

	router.Run(HOST + ":" + PORT)

}
