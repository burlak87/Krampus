package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"crampus/internal/domain"
	"crampus/pkg/apperror"
	"crampus/pkg/logging"
)

type UserService interface {
	UserRegister(users domain.User) (domain.User, error)
	UserLogin(users domain.User) (domain.TokenResponse, domain.TwoFaCodes, error)
	UserLogout(userID int64, password string) error
	// SaveSession() ()
	UserRefresh(token string) (domain.TokenResponse, error)
	UserSendEmailCode(tempToken string) error
	VerifyCode(code domain.Code) (domain.TokenResponse, error)
	EnableTwoFA(userID int64) error
	DisableTwoFA(userID int64, passqord string) error
}

type UsersHandler struct {
	service UserService
	logger  *logging.Logger
}

func NewUsersHandler(s UserService, l *logging.Logger) *UsersHandler {
	return &UsersHandler{
		service: s,
		logger: l,
	}
}

func (h *UsersHandler) RegisterRoutes(router *gin.RouterGroup, jwtSecret string) {
	auth := router.Group("/auth")
	{
		auth.POST("/register", h.signUp)
		auth.POST("/login", h.signIn)
		auth.POST("/logout", h.logout)
		auth.POST("/refresh", h.refresh)
		auth.POST("/send-code", h.sendEmailToken)
		auth.POST("/verify-code", h.verifyCode)
		auth.POST("/enable-2fa", h.enableTwoFA)
		auth.POST("/disable-2fa", h.disableTwoFA)
	}
}

func (h *UsersHandler) signUp(c *gin.Context) {
	var user domain.User
	if err := c.ShouldBindJSON(&user); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}
	
	createdUser, err := h.service.UserRegister(user)
	if err != nil {
		h.logger.Error("Failed to register user: " + err.Error())
		appErr, ok := err.(*apperror.AppError)
		if ok {
			c.JSON(http.StatusBadRequest, appErr)
		} else {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Failed to register user",
			})
		}
		return
	}
	
	c.JSON(http.StatusCreated, createdUser)
}

func (h *UsersHandler) signIn(c *gin.Context) {
	var user domain.User
	if err := c.ShouldBindJSON(&user); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}
	
	accessToken, tempToken, err := h.service.UserLogin(user)
	if err != nil {
		h.logger.Error("Failed to login user: " + err.Error())
		appErr, ok := err.(*apperror.AppError)
		if ok {
			c.JSON(http.StatusUnauthorized, appErr)
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication failed",
			})
		}
		return
	}
	
	if tempToken.RequiresTwoFa {
		c.JSON(http.StatusOK, tempToken)
		return
	}
	
	c.JSON(http.StatusOK, accessToken)
}

func (h *UsersHandler) logout(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unauthorized",
		})
		return
	}
	
	var req domain.TwoFaToggleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request body",
		})
		return
	}
	
	err := h.service.UserLogout(userID.(int64), req.Password)
	if err != nil {
		h.logger.Error("Failed to logout: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
				"error": "Failed to logout",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (h *UsersHandler) refresh(c *gin.Context) {
	var req struct{ RefreshToken string `json:"refresh_token"` }
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}
	
	token, err := h.service.UserRefresh(req.RefreshToken)
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

func (h *UsersHandler) sendEmailToken(c *gin.Context) {
	var req struct{ TempToken string `json:"temp_token"` }
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}
	
	err := h.service.UserSendEmailCode(req.TempToken)
	if err != nil {
		h.logger.Error("Failed to send verification code: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
				"error": "Failed to send verification code",
		})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

func (h *UsersHandler) verifyCode(c *gin.Context) {
	var code domain.Code
	if err := c.ShouldBindJSON(&code); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}
	
	tokenRes, err := h.service.VerifyCode(code)
	if err != nil {
		h.logger.Error("Failed to verify code: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
				"error": "Verification failed",
		})
		return
	}
	
	c.JSON(http.StatusOK, tokenRes)
}

func (h *UsersHandler) enableTwoFA(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unauthorized",
		})
		return
	}
	
	err := h.service.EnableTwoFA(userID.(int64))
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

func (h *UsersHandler) disableTwoFA(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Unauthorized",
		})
		return
	}
	
	var req domain.TwoFaToggleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Failed to bind JSON: " + err.Error())
		c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid request body",
		})
		return
	}
	
	err := h.service.DisableTwoFA(userID.(int64), req.Password)
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