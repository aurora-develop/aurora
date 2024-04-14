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
		tokenParts := strings.Split(strings.Replace(authHeader, "Bearer ", "", 1)," ")
		customAccessToken := tokenParts[0]
		if customer_key != customAccessToken {
			c.JSON(401, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}
		if len(tokenParts) > 1 {
			openaiAccessToken := tokenParts[1]
			c.Request.Header.Set("Authorization", "Bearer " + openaiAccessToken)
		}
	}
	c.Next()
}
