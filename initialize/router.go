package initialize

import (
	"aurora/middlewares"

	"github.com/gin-gonic/gin"
)

func RegisterRouter() *gin.Engine {
	handler := NewHandle(
		checkProxy(),
		readAccessToken(),
	)

	// 初始化基础前置参数
	handler.InitBasicConfigForChatGPT()

	router := gin.Default()
	router.Use(middlewares.Cors)

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
	router.OPTIONS("/v1/responses", optionsHandler)
	router.OPTIONS("/v1/images/generations", optionsHandler)
	router.OPTIONS("/v1/images/edits", optionsHandler)
	router.OPTIONS("/v1/files", optionsHandler)

	authGroup := router.Group("").Use(middlewares.Authorization)
	authGroup.POST("/v1/chat/completions", handler.nightmare)
	authGroup.POST("/v1/responses", handler.responses)
	authGroup.POST("/v1/files", handler.files)
	authGroup.GET("/v1/models", handler.engines)
	authGroup.POST("/backend-api/conversation", handler.chatgptConversation)
	authGroup.POST("/v1/images/generations", handler.imageGenerations)
	// 改图 + 图生图(变体)统一入口:
	//   - 传 prompt     → 按 prompt 改图
	//   - 不传 prompt   → 自动注入默认指令,生成图像变体(图生图)
	authGroup.POST("/v1/images/edits", handler.imageEdits)
	authGroup.OPTIONS("/v1/audio/speech", optionsHandler)
	authGroup.POST("/v1/audio/speech", handler.tts)
	return router
}
