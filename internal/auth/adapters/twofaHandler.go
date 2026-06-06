package adapters

import (
	"context"
	"krampus/internal/user/domain"
	"krampus/pkg/logging"
	"net/http"

	"github.com/gin-gonic/gin"
)

type TwoFAService interface {
	VerifyCode(ctx context.Context, code domain.Code) (domain.TokenResponse, error)
	EnableTwoFA(ctx context.Context, userID int64) error
}

type twoFAHandler struct {
	twoFAService TwoFAService
	logger       *logging.Logger
}

func NewTwoFAHandler(s TwoFAService, l *logging.Logger) *twoFAHandler {
	return &twoFAHandler{
		twoFAService: s,
		logger:       l,
	}
}

func (h *twoFAHandler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/verify", h.VerifyCode)
	rg.POST("/enable", h.EnableTwoFA)
}

func (h *twoFAHandler) VerifyCode(c *gin.Context) {
	var code domain.Code
	if err := c.ShouldBindJSON(&code); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	tokenRes, err := h.twoFAService.VerifyCode(c.Request.Context(), code)
	if err != nil {
		h.logger.Error("Failed to verify code: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Verification failed",
		})
		return
	}

	c.JSON(http.StatusOK, tokenRes)
}

func (h *twoFAHandler) EnableTwoFA(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unauthorized",
		})
		return
	}

	err := h.twoFAService.EnableTwoFA(c.Request.Context(), userID.(int64))
	if err != nil {
		h.logger.Error("Failed to enable 2FA: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to enable 2FA",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}
