package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"satvos/internal/middleware"
)

func TestCORS_AllowedOrigin(t *testing.T) {
	r := gin.New()
	r.Use(middleware.CORS([]string{"https://app.example.com", "http://localhost:3000"}))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "https://app.example.com")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://app.example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORS_DisallowedOrigin(t *testing.T) {
	r := gin.New()
	r.Use(middleware.CORS([]string{"https://app.example.com"}))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "https://evil.com")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_PreflightAllowed(t *testing.T) {
	r := gin.New()
	r.Use(middleware.CORS([]string{"https://app.example.com"}))
	r.PUT("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodOptions, "/test", http.NoBody)
	req.Header.Set("Origin", "https://app.example.com")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "https://app.example.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "PUT")
	assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
}

func TestCORS_PreflightDisallowed(t *testing.T) {
	r := gin.New()
	r.Use(middleware.CORS([]string{"https://app.example.com"}))
	r.PUT("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodOptions, "/test", http.NoBody)
	req.Header.Set("Origin", "https://evil.com")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_NoOriginHeader(t *testing.T) {
	r := gin.New()
	r.Use(middleware.CORS([]string{"https://app.example.com"}))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", http.NoBody)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_MultipleOrigins(t *testing.T) {
	origins := []string{"https://app.example.com", "https://staging.example.com", "http://localhost:3000"}
	r := gin.New()
	r.Use(middleware.CORS(origins))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	for _, origin := range origins {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, "/test", http.NoBody)
		req.Header.Set("Origin", origin)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, origin, w.Header().Get("Access-Control-Allow-Origin"), "origin %s should be allowed", origin)
	}
}

func TestCORS_EmptyOriginsList(t *testing.T) {
	r := gin.New()
	r.Use(middleware.CORS([]string{}))
	r.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/test", http.NoBody)
	req.Header.Set("Origin", "https://app.example.com")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Access-Control-Allow-Origin"))
}
