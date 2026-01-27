package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"satvos/internal/service"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	authService service.AuthService
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
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
