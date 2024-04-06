package main

import (
	"aurora/initialize"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/acheong08/endless"
	"github.com/joho/godotenv"
)

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := initialize.RegisterRouter()

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
