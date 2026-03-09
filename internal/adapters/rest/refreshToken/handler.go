package refreshToken

import (
	"krampus/internal/domain"
	"krampus/pkg/apperror"
	"krampus/pkg/logging"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RefreshTokenService interface {
	// SaveSession() ()
	UserRefresh(token string) (domain.TokenResponse, error)
	// UserSendEmailCode(tempToken string) error
}

type refreshTokenHandler struct {
	refreshTokenService RefreshTokenService
	logger              *logging.Logger
}

func NewRefreshTokenHandler(s RefreshTokenService, l *logging.Logger) *refreshTokenHandler {
	return &refreshTokenHandler{
		refreshTokenService: s,
		logger:              l,
	}
}

func (h *refreshTokenHandler) refresh(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	token, err := h.refreshTokenService.UserRefresh(req.RefreshToken)
	if err != nil {
		h.logger.Error("Failed to refresh user: " + err.Error())
		appErr, ok := err.(*apperror.AppError)
		if ok {
			c.JSON(http.StatusUnauthorized, appErr)
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Failed to refresh token",
			})
		}
		return
	}

	c.JSON(http.StatusOK, token)
}

// func (h *refreshTokenHandler) sendEmailToken(c *gin.Context) {
// 	var req struct {
// 		TempToken string `json:"temp_token"`
// 	}
// 	if err := c.ShouldBindJSON(&req); err != nil {
// 		h.logger.Error("Failed to bind JSON: " + err.Error())
// 		c.JSON(http.StatusBadRequest, gin.H{
// 			"error": "Invalid request body",
// 		})
// 		return
// 	}

// 	err := h.refreshTokenService.UserSendEmailCode(req.TempToken)
// 	if err != nil {
// 		h.logger.Error("Failed to send verification code: " + err.Error())
// 		c.JSON(http.StatusBadRequest, gin.H{
// 			"error": "Failed to send verification code",
// 		})
// 		return
// 	}

// 	c.JSON(http.StatusOK, gin.H{
// 		"success": true,
// 	})
// }
