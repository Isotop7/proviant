package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/isotop7/proviant/models/configuration/static"
	"codeberg.org/isotop7/proviant/util"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

func setupTestRouterWithRequestID(baseLogger *zerolog.Logger) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware(baseLogger))
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"request_id": c.GetString(util.ContextKeyRequestID)})
	})
	return r
}

func TestRequestIDMiddleware_GeneratesUUIDWhenAbsent(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	r := setupTestRouterWithRequestID(&logger)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	reqID := w.Header().Get(static.RequestIDHeader)
	assert.NotEmpty(t, reqID)

	_, err := uuid.Parse(reqID)
	assert.NoError(t, err)
	assert.Contains(t, w.Body.String(), reqID)
}

func TestRequestIDMiddleware_HonorsIncomingValidUUID(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	r := setupTestRouterWithRequestID(&logger)

	validUUID := uuid.New().String()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set(static.RequestIDHeader, validUUID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	respReqID := w.Header().Get(static.RequestIDHeader)
	assert.Equal(t, validUUID, respReqID)
}

func TestRequestIDMiddleware_RejectsInvalidIncoming(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	r := setupTestRouterWithRequestID(&logger)

	invalidUUID := "not-a-valid-uuid"
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set(static.RequestIDHeader, invalidUUID)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	respReqID := w.Header().Get(static.RequestIDHeader)
	assert.NotEqual(t, invalidUUID, respReqID)

	_, err := uuid.Parse(respReqID)
	assert.NoError(t, err)
}

func TestRequestIDMiddleware_LoggerCarriesField(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)
	r := gin.New()
	r.Use(RequestIDMiddleware(&logger))
	r.GET("/test", func(c *gin.Context) {
		logger, _ := c.MustGet(util.ContextKeyLogger).(*zerolog.Logger)
		logger.Info().Msg("test message")
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "request_id")
}

func TestUserContextLogger_AddsUserID(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(util.ContextKeyLogger, &logger)
		c.Next()
	})
	r.Use(func(c *gin.Context) {
		// Simulate JWT claims using jwt.MapClaims type
		claims := jwt.MapClaims{static.TokenIdentityKey: float64(123)}
		c.Set("JWT_PAYLOAD", claims)
		c.Next()
	})
	r.Use(UserContextLoggerMiddleware())
	r.GET("/test", func(c *gin.Context) {
		userID, exists := c.Get(util.ContextKeyUserID)
		assert.True(t, exists)
		assert.Equal(t, uint(123), userID)

		logger, _ := c.MustGet(util.ContextKeyLogger).(*zerolog.Logger)
		logger.Info().Msg("test with user")
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "user_id")
}

func TestZerologMiddleware_AccessLogContainsBothFields(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RequestIDMiddleware(&logger))
	r.Use(ZerologMiddleware(&logger))
	r.GET("/test", func(c *gin.Context) {
		c.Set(util.ContextKeyUserID, uint(456))
		c.Status(http.StatusOK)
	})

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "request_id")
	assert.Contains(t, logOutput, "user_id")
	assert.Contains(t, logOutput, "Request handled")
}

// TestZerologMiddleware_SkipsProbeTraffic covers the exclusion. An orchestrator
// probes on a fixed interval and Prometheus scrapes every 15s, so logging them
// buries real traffic under thousands of identical lines a day.
func TestZerologMiddleware_SkipsProbeTraffic(t *testing.T) {
	for _, route := range []string{util.RouteHealth, util.RouteHealthReady, util.RouteMetrics} {
		t.Run(route, func(t *testing.T) {
			var buf bytes.Buffer
			logger := zerolog.New(&buf)

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(ZerologMiddleware(&logger))
			r.GET(route, func(c *gin.Context) { c.Status(http.StatusOK) })

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, route, nil))

			assert.Equal(t, http.StatusOK, w.Code)
			assert.NotContains(t, buf.String(), "Request handled")
		})
	}
}

func TestParseRequestID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty string", "", ""},
		{"invalid uuid", "not-a-uuid", ""},
		{"valid uuid", uuid.New().String(), "valid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseRequestID(tt.input)
			if tt.input == "" || tt.input == "not-a-uuid" {
				assert.Equal(t, tt.expected, result)
			} else {
				_, err := uuid.Parse(result)
				assert.NoError(t, err)
			}
		})
	}
}

func TestRouterSetup_RequestIDMiddlewareWired(t *testing.T) {
	var buf bytes.Buffer
	logger := zerolog.New(&buf)

	middleware := RequestIDMiddleware(&logger)
	assert.NotNil(t, middleware)
}
