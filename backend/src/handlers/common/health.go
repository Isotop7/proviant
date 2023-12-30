// common implements non-specifc handlers
package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetHealth returns the health status of the backend
// GET /api/health
func GetHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}
