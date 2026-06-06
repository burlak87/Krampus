package adapters

import (
	"context"
	"krampus/internal/user/domain"
	"krampus/pkg/apperror"
	"krampus/pkg/logging"
	"net/http"

	"github.com/gin-gonic/gin"
)

type RefreshTokenService interface {
	UserRefresh(ctx context.Context, token string) (domain.TokenResponse, error)
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

func (h *refreshTokenHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/refresh", h.refresh)
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

	token, err := h.refreshTokenService.UserRefresh(c.Request.Context(), req.RefreshToken)
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
