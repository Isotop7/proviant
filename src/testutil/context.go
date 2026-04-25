package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"

	"codeberg.org/isotop7/proviant/models/configuration/static"
	"github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func SetupGinContext(db *gorm.DB) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	mockLogger := zerolog.Nop()
	ctx.Set("logger", &mockLogger)
	ctx.Set("dbHandle", db)

	return ctx, w
}

func MockJWTClaims(ctx *gin.Context, userID uint) {
	ctx.Set("JWT_PAYLOAD", jwt.MapClaims{
		"id": float64(userID),
	})
}

func MockJWTClaimsWithKey(ctx *gin.Context, userID uint, key string) {
	ctx.Set("JWT_PAYLOAD", jwt.MapClaims{
		key: float64(userID),
	})
}

var TokenIdentityKey = static.TokenIdentityKey

func CreateTestRequest(ctx *gin.Context, body interface{}) {
	bodyBytes, _ := json.Marshal(body)
	ctx.Request = &http.Request{
		Body:          io.NopCloser(bytes.NewBuffer(bodyBytes)),
		Header:        make(http.Header),
		ContentLength: int64(len(bodyBytes)),
	}
	ctx.Request.Header.Set("Content-Type", "application/json")
}

func CreateJSONRequest(method string, path string, body interface{}) *http.Request {
	var bodyBytes []byte
	if body != nil {
		bodyBytes, _ = json.Marshal(body)
	}
	req, _ := http.NewRequest(method, path, bytes.NewBuffer(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	return req
}