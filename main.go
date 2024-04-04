package main

import (
	"aurora/internal/chatgpt"
	"bufio"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"aurora/internal/proxys"

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
	return router
}

func main() {
	router := registerRouter()

	_ = godotenv.Load(".env")
	host := os.Getenv("SERVER_HOST")
	port := os.Getenv("SERVER_PORT")
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
func (h *Handler) engines(c *gin.Context) {
	proxyUrl := h.proxy.GetProxyIP()
	secret := h.token.GetSecret()
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		customAccessToken := strings.Replace(authHeader, "Bearer ", "", 1)
		// Check if customAccessToken starts with sk-
		if strings.HasPrefix(customAccessToken, "eyJhbGciOiJSUzI1NiI") {
			secret = h.token.GenerateTempToken(customAccessToken)
		}
	}
	if secret == nil || secret.Token == "" {
		c.JSON(400, gin.H{"error": "Not Account Found."})
		return
	}

	resp, status, err := chatgpt.GETengines(secret, proxyUrl)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "error sending request",
		})
		return
	}

	type ResData struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int    `json:"created"`
		OwnedBy string `json:"owned_by"`
	}

	type JSONData struct {
		Object string    `json:"object"`
		Data   []ResData `json:"data"`
	}

	modelS := JSONData{
		Object: "list",
	}
	var resModelList []ResData
	if len(resp.Models) > 2 {
		res_data := ResData{
			ID:      "gpt-4-mobile",
			Object:  "model",
			Created: 1685474247,
			OwnedBy: "openai",
		}
		resModelList = append(resModelList, res_data)
	}
	for _, model := range resp.Models {
		res_data := ResData{
			ID:      model.Slug,
			Object:  "model",
			Created: 1685474247,
			OwnedBy: "openai",
		}
		if model.Slug == "text-davinci-002-render-sha" {
			res_data.ID = "gpt-3.5-turbo"
		}
		resModelList = append(resModelList, res_data)
	}
	modelS.Data = resModelList
	c.JSON(status, modelS)
}
