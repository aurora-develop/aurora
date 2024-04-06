package middlewares

import (
	"github.com/gin-gonic/gin"
	"os"
	"strings"
)

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
