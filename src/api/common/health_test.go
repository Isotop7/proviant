package common

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"codeberg.org/isotop7/proviant/api"

	"github.com/gin-gonic/gin"
)

func TestGetHealth(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	req, _ := http.NewRequest("GET", "/health", http.NoBody)
	ctx.Request = req

	GetHealth(ctx)

	if w.Code != http.StatusOK {
		t.Errorf("Status = %v, want %v", w.Code, http.StatusOK)
	}

	var response api.APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("unmarshal error: %v", err)
	}
	if response.Message != "ok" {
		t.Errorf("Message = %v, want 'ok'", response.Message)
	}
}

func TestGetHealthResponseFormat(t *testing.T) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	req, _ := http.NewRequest("GET", "/health", http.NoBody)
	ctx.Request = req

	GetHealth(ctx)

	if w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %v, want 'application/json; charset=utf-8'", w.Header().Get("Content-Type"))
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("unmarshal error: %v", err)
	}
	if response["message"] == nil {
		t.Error("expected 'message' key in response")
	}
	if response["message"] != "ok" {
		t.Errorf("message = %v, want 'ok'", response["message"])
	}
}