package main

import (
	"aurora/internal/proxys"
	"bufio"
	"embed"
	"io/fs"
	"log"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/acheong08/endless"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
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

//go:embed web/*
var staticFiles embed.FS

func registerRouter() *gin.Engine {
	handler := NewHandle(
		checkProxy(),
		readAccessToken(),
	)

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

	router.POST("/auth/session", handler.session)
	router.POST("/auth/refresh", handler.refresh)
	router.OPTIONS("/v1/chat/completions", optionsHandler)

	authGroup := router.Group("").Use(Authorization)
	authGroup.POST("/v1/chat/completions", handler.nightmare)
	authGroup.GET("/v1/models", handler.engines)

	subFS, err := fs.Sub(staticFiles, "web")
	if err != nil {
		log.Fatal(err)
	}
	router.StaticFS("/web", http.FS(subFS))

	return router
}

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := registerRouter()

	_ = godotenv.Load(".env")
	host := os.Getenv("SERVER_HOST")
	port := os.Getenv("PORT") // 在heroku中部署，无法指定端口，必须获取PORT环境变量作为web端口
	tlsCert := os.Getenv("TLS_CERT")
	tlsKey := os.Getenv("TLS_KEY")

	if host == "" {
		host = "0.0.0.0"
	}
	if port == "" {
		port = "8080"
	}

	if tlsCert != "" && tlsKey != "" {
		_ = endless.ListenAndServeTLS(host+":"+port, tlsCert, tlsKey, router)
	} else {
		_ = endless.ListenAndServe(host+":"+port, router)
	}
}
