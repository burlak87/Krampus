package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func JWTAuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header is required",
			})
			return
		}
		
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		fmt.Printf("DEBUG JWT MIDDLEWARE: Token: %s...\n", tokenString[:min(10, len(tokenString))])
		
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})
		
		if err != nil || token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			return
		}
		
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token claims",
			})
			return
		}
		
		userID := int64(claims["user_id"].(float64))
		c.Set("userID", userID)
		c.Set("token", tokenString)
		
		c.Next()
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}