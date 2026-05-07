package apperror

import (
	"fmt"
	"krampus/internal/user/domain"
	"krampus/internal/user/storage"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type appHandler func(w http.ResponseWriter, r *http.Request) error

func ErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		lastErr := c.Errors.Last()
		if appErr, ok := lastErr.Err.(*AppError); ok {
			c.JSON(getHTTPStatus(appErr.Code), gin.H{
				"error": gin.H{
					"code":              appErr.Code,
					"message":           appErr.Message,
					"developer_message": appErr.DeveloperMessage,
					"details":           appErr.Details,
				},
			})
			c.Abort()
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"code":    ErrInternal,
				"message": "internal server error",
			},
		})
		c.Abort()
	}
}

// CORSMiddleware — разрешает кросс-доменные запросы
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// AuthMiddleware — умная авторизация (Redis + JWT)
func AuthMiddleware(redisStorage storage.RedisSessionStorage, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")

		// Проверка формата Bearer
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.Error(New(ErrUnauthorized, "invalid auth format (bearer required)"))
			c.Abort()
			return
		}

		tokenString := authHeader[7:]

		// 1. Пытаемся взять из кэша Redis
		if any(redisStorage) != nil { // Измените на это
			if session, err := redisStorage.GetAccessToken(tokenString); err == nil {
				c.Set("user_id", fmt.Sprintf("%d", session.UserID))
				c.Next()
				return
			}
		}

		// 2. Если в кэше нет — парсим JWT
		token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})

		if err != nil || !token.Valid {
			c.Error(New(ErrUnauthorized, "token is invalid or expired"))
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.Error(New(ErrUnauthorized, "invalid token claims"))
			c.Abort()
			return
		}

		// Извлекаем userID (обычно float64 в JWT)
		userIDRaw, ok := claims["user_id"]
		if !ok {
			c.Error(New(ErrUnauthorized, "user_id not found in token"))
			c.Abort()
			return
		}

		userID := int64(userIDRaw.(float64))
		userIDStr := fmt.Sprintf("%d", userID)

		// 3. Кэшируем результат для следующих запросов
		if any(redisStorage) != nil { // И здесь тоже
			_ = redisStorage.SetAccessToken(tokenString, domain.CachedSession{
				UserID:    userID,
				ExpiresAt: time.Now().Add(15 * time.Minute),
			})
		}

		c.Set("user_id", userIDStr)
		c.Next()
	}
}

func getHTTPStatus(code ErrorCode) int {
	switch code {
	case ErrUnauthorized:
		return http.StatusUnauthorized
	case ErrForbidden:
		return http.StatusForbidden
	case ErrRateLimit:
		return http.StatusTooManyRequests
	case ErrRoomNotFound, ErrUserNotFound:
		return http.StatusNotFound
	case ErrValidation, ErrInvalidMessage, ErrPayloadTooLarge:
		return http.StatusBadRequest
	case ErrDuplicate:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
