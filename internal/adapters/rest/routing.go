package adapters

import (
	"github.com/gin-gonic/gin"
)

type UserHandler interface {
	SignUp(c *gin.Context)
	SignIn(c *gin.Context)
	Logout(c *gin.Context)
}

type RefreshTokenHandler interface {
	Refresh(c *gin.Context)
	// SendEmailToken(c *gin.Context)
}

type TwoFAHandler interface {
	VerifyCode(c *gin.Context)
	EnableTwoFA(c *gin.Context)
	// DisableTwoFA(c *gin.Context)
}

func RegisterRoutes(router *gin.RouterGroup, userHandler UserHandler, refreshTokenHandler RefreshTokenHandler, twoFAHandler TwoFAHandler) {
	auth := router.Group("/auth")
	{
		auth.POST("/register", userHandler.SignUp)
		auth.POST("/login", userHandler.SignIn)
		auth.POST("/logout", userHandler.Logout)

		auth.POST("/refresh", refreshTokenHandler.Refresh)
		// auth.POST("/send-code", refreshTokenHandler.SendEmailToken)

		auth.POST("/verify-code", twoFAHandler.VerifyCode)
		auth.POST("/enable-2fa", twoFAHandler.EnableTwoFA)
		// auth.POST("/disable-2fa", twoFAHandler.DisableTwoFA)
	}
}
