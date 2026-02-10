package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"satvos/internal/service"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	authService         service.AuthService
	registrationService service.RegistrationService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authService service.AuthService, registrationService service.RegistrationService) *AuthHandler {
	return &AuthHandler{authService: authService, registrationService: registrationService}
}

// Login handles POST /api/v1/auth/login
// @Summary Login to get access token
// @Description Authenticate with tenant slug, email and password to receive JWT tokens
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login credentials"
// @Success 200 {object} Response{data=TokenResponse} "Successfully authenticated"
// @Failure 400 {object} ErrorResponseBody "Validation error"
// @Failure 401 {object} ErrorResponseBody "Invalid credentials"
// @Failure 403 {object} ErrorResponseBody "Tenant or user inactive"
// @Router /auth/login [post]
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
// @Summary Refresh access token
// @Description Exchange a refresh token for a new access token pair
// @Tags auth
// @Accept json
// @Produce json
// @Param request body RefreshRequest true "Refresh token"
// @Success 200 {object} Response{data=TokenResponse} "New token pair"
// @Failure 400 {object} ErrorResponseBody "Validation error"
// @Failure 401 {object} ErrorResponseBody "Invalid or expired refresh token"
// @Router /auth/refresh [post]
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
