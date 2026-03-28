package apperror

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type appHandler func(w http.ResponseWriter, r *http.Request) error

func JWTMiddleware(jwtSecret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Printf("DEBUG JWT MIDDLEWARE: Checking auth for %s %s\n", r.Method, r.URL.Path)
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			fmt.Printf("DEBUG JWT MIDDLEWARE: Missing token\n")
			http.Error(w, "Missing token", http.StatusUnauthorized)
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		fmt.Printf("DEBUG JWT MIDDLEWARE: Token: %s...\n", tokenString[:10])
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		if err != nil {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			http.Error(w, "Invalid claims", http.StatusUnauthorized)
			return
		}
		studentID := int64(claims["user_id"].(float64))
		ctx := context.WithValue(r.Context(), "studentID", studentID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

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
