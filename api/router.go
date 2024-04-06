package api

import (
	"aurora/initialize"
	"github.com/gin-gonic/gin"
	"net/http"
)

var router *gin.Engine

func init() {
	// 初始化gin
	router = initialize.RegisterRouter()
}

func Listen(w http.ResponseWriter, r *http.Request) {
	router.ServeHTTP(w, r)
}
