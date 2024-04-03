package main

import (
	"aurora/internal/tokens"
	"os"

	"github.com/acheong08/endless"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var HOST string
var PORT string
var TLS_CERT string
var TLS_KEY string
var ACCESS_TOKENS tokens.AccessToken
var PROXY_URL string

func init() {
	_ = godotenv.Load(".env")

	HOST = os.Getenv("SERVER_HOST")
	PORT = os.Getenv("SERVER_PORT")
	TLS_CERT = os.Getenv("TLS_CERT")
	TLS_KEY = os.Getenv("TLS_KEY")
	PROXY_URL = os.Getenv("PROXY_URL")
	if HOST == "" {
		HOST = "0.0.0.0"
	}
	if PORT == "" {
		PORT = "8080"
	}
}
func main() {
	gin.SetMode(gin.ReleaseMode)
	router := gin.Default()

	router.Use(cors)

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "pong",
		})
	})

	router.OPTIONS("/v1/chat/completions", optionsHandler)
	router.POST("/v1/chat/completions", nightmare)

	if TLS_CERT != "" && TLS_KEY != "" {
		endless.ListenAndServeTLS(HOST+":"+PORT, TLS_CERT, TLS_KEY, router)
	} else {
		endless.ListenAndServe(HOST+":"+PORT, router)
	}
}
