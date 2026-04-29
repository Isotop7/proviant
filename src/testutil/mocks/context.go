// Package mocks provides test utilities that import controllers/database.
// It is a subpackage of testutil to avoid an import cycle
// (controllers/database tests → testutil → controllers/database).
package mocks

import (
	"net/http/httptest"

	"codeberg.org/isotop7/proviant/controllers/database"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// SetupGinContextWithDB creates a Gin test context with both dbHandle and a live
// RepositoryContainer backed by the provided GORM connection.
// Use this in handler integration tests instead of testutil.SetupGinContext when the
// handler under test reads from ctx.Get("repos").
func SetupGinContextWithDB(db *gorm.DB) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	mockLogger := zerolog.Nop()
	ctx.Set("logger", &mockLogger)
	ctx.Set("dbHandle", db)
	ctx.Set("repos", database.NewRepositoryContainer(db))

	return ctx, w
}

// SetupGinContextWithMocks creates a Gin test context backed by mock repositories.
// No database connection is needed; use this for handler unit tests.
func SetupGinContextWithMocks(m *MockRepositoryContainer) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)

	mockLogger := zerolog.Nop()
	ctx.Set("logger", &mockLogger)
	ctx.Set("repos", m.ToRepositoryContainer())

	return ctx, w
}
