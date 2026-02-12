package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"satvos/internal/service"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	authService          service.AuthService
	registrationService  service.RegistrationService
	passwordResetService service.PasswordResetService
	socialAuthService    service.SocialAuthService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authService service.AuthService, registrationService service.RegistrationService, passwordResetService service.PasswordResetService, socialAuthService service.SocialAuthService) *AuthHandler {
	return &AuthHandler{authService: authService, registrationService: registrationService, passwordResetService: passwordResetService, socialAuthService: socialAuthService}
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var input service.LoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	tokenPair, err := h.authService.Login(c.Request.Context(), input)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, tokenPair)
}

// RefreshToken handles POST /api/v1/auth/refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var input service.RefreshInput
	if err := c.ShouldBindJSON(&input); err != nil {
		RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	tokenPair, err := h.authService.RefreshToken(c.Request.Context(), input.RefreshToken)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, tokenPair)
}

// Register handles POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	if h.registrationService == nil {
		RespondError(c, http.StatusNotFound, "NOT_FOUND", "registration is not enabled")
		return
	}

	var input service.RegisterInput
	if err := c.ShouldBindJSON(&input); err != nil {
		RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	output, err := h.registrationService.Register(c.Request.Context(), input)
	if err != nil {
		HandleError(c, err)
		return
	}

	RespondCreated(c, output)
}

// VerifyEmail handles GET /api/v1/auth/verify-email?token=...
func (h *AuthHandler) VerifyEmail(c *gin.Context) {
	if h.registrationService == nil {
		RespondError(c, http.StatusNotFound, "NOT_FOUND", "registration is not enabled")
		return
	}

	token := c.Query("token")
	if token == "" {
		RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", "token query parameter is required")
		return
	}

	if err := h.registrationService.VerifyEmail(c.Request.Context(), token); err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, gin.H{"message": "email verified successfully"})
}

// ForgotPassword handles POST /api/v1/auth/forgot-password
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	if h.passwordResetService == nil {
		RespondError(c, http.StatusNotFound, "NOT_FOUND", "password reset is not enabled")
		return
	}

	var input service.ForgotPasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	if err := h.passwordResetService.ForgotPassword(c.Request.Context(), input); err != nil {
		// Never leak information — always return 200
		log.Printf("forgot-password internal error: %v", err)
	}

	RespondOK(c, gin.H{"message": "if an account with that email exists, a password reset link has been sent"})
}

// ResetPassword handles POST /api/v1/auth/reset-password
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	if h.passwordResetService == nil {
		RespondError(c, http.StatusNotFound, "NOT_FOUND", "password reset is not enabled")
		return
	}

	var input service.ResetPasswordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	if err := h.passwordResetService.ResetPassword(c.Request.Context(), input); err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, gin.H{"message": "password has been reset successfully"})
}

// SocialLogin handles POST /api/v1/auth/social-login
func (h *AuthHandler) SocialLogin(c *gin.Context) {
	if h.socialAuthService == nil {
		RespondError(c, http.StatusNotFound, "NOT_FOUND", "social login is not enabled")
		return
	}

	var input service.SocialLoginInput
	if err := c.ShouldBindJSON(&input); err != nil {
		RespondError(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	output, err := h.socialAuthService.SocialLogin(c.Request.Context(), input)
	if err != nil {
		HandleError(c, err)
		return
	}

	if output.IsNewUser {
		RespondCreated(c, output)
	} else {
		RespondOK(c, output)
	}
}

// ResendVerification handles POST /api/v1/auth/resend-verification
func (h *AuthHandler) ResendVerification(c *gin.Context) {
	if h.registrationService == nil {
		RespondError(c, http.StatusNotFound, "NOT_FOUND", "registration is not enabled")
		return
	}

	tenantID, userID, _, ok := extractAuthContext(c)
	if !ok {
		return
	}

	if err := h.registrationService.ResendVerification(c.Request.Context(), tenantID, userID); err != nil {
		HandleError(c, err)
		return
	}

	RespondOK(c, gin.H{"message": "verification email sent"})
}
