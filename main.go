package main

import (
	"aurora/initialize"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/acheong08/endless"
	"github.com/joho/godotenv"
)

//go:embed web/*
var staticFiles embed.FS

func main() {
	gin.SetMode(gin.ReleaseMode)
	router := initialize.RegisterRouter()
	subFS, err := fs.Sub(staticFiles, "web")
	if err != nil {
		log.Fatal(err)
	}
	router.StaticFS("/web", http.FS(subFS))

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
