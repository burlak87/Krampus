package middleware

import (
	"krampus/internal/domain"
	"krampus/internal/storage/redis"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(redisStorage *redis.SessionStorage, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Извлекаем токен
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token format"})
			c.Abort()
			return
		}

		// 2. ✅ ПРОВЕРЯЕМ РЕДИС КЭШ (100x быстрее JWT парсинга)
		cachedSession, err := redisStorage.GetAccessToken(tokenString)
		if err == nil {
			// ✅ ХИТ! Токен валиден в Redis
			c.Set("userID", cachedSession.UserID)
			c.Next()
			return
		}

		// 3. Cache MISS - парсим JWT
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token claims"})
			c.Abort()
			return
		}

		userIDFloat, ok := claims["user_id"].(float64)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid user_id"})
			c.Abort()
			return
		}

		userID := int64(userIDFloat)

		// 4. ✅ КЭШИРУЕМ ВАЛИДНЫЙ ТОКЕН
		session := domain.CachedSession{
			UserID:       userID,
			AccessToken:  tokenString,
			CreatedAt:    time.Now(),
			LastActivity: time.Now(),
			ExpiresAt:    time.Now().Add(15 * time.Minute),
		}
		if err := redisStorage.SetAccessToken(tokenString, session); err != nil {
			// Логируем, но продолжаем
		}

		c.Set("userID", userID)
		c.Next()
	}
}
