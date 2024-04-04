package main

import (
	"os"
	"strings"

	gin "github.com/gin-gonic/gin"
)

func cors(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "*")
	c.Header("Access-Control-Allow-Headers", "*")
	c.Next()
}

func Authorization(c *gin.Context) {
	customer_key := os.Getenv("Authorization")
	if customer_key != "" {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(401, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
		customAccessToken := strings.Replace(authHeader, "Bearer ", "", 1)
		if customer_key != customAccessToken {
			c.JSON(401, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
	}
	c.Next()
}
