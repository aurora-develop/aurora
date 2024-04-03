package main

import (
	gin "github.com/gin-gonic/gin"
	"os"
	"strings"
)

func cors(c *gin.Context) {
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Methods", "*")
	c.Header("Access-Control-Allow-Headers", "*")
	c.Next()
}

func Authorization(c *gin.Context) {
	authHeader := c.GetHeader("Authorization")
	customer_key := os.Getenv("Authorization")
	if authHeader != "" {
		customAccessToken := strings.Replace(authHeader, "Bearer ", "", 1)
		if customer_key != "" {
			if customer_key != customAccessToken {
				c.JSON(401, gin.H{"error": "Unauthorized"})
				c.Abort()
				return
			}
		}
	}
	c.Next()
}
