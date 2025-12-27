package common

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/isotop7/proviant/api"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestGetHealth tests the health endpoint handler
func TestGetHealth(t *testing.T) {
	// Setup test context
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	// Create a minimal request
	req, _ := http.NewRequest("GET", "/health", http.NoBody)
	ctx.Request = req

	// Call the handler
	GetHealth(ctx)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response body
	var response api.APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Assert response content
	assert.Equal(t, "ok", response.Message)
}

// TestGetHealthResponseFormat tests that the health endpoint returns the correct response format
func TestGetHealthResponseFormat(t *testing.T) {
	// Setup test context
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	// Create a minimal request
	req, _ := http.NewRequest("GET", "/health", http.NoBody)
	ctx.Request = req

	// Call the handler
	GetHealth(ctx)

	// Assert response is JSON
	assert.Equal(t, "application/json; charset=utf-8", w.Header().Get("Content-Type"))

	// Parse and verify response structure
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Verify response has the expected structure
	assert.Contains(t, response, "message")
	assert.Equal(t, "ok", response["message"])
}
