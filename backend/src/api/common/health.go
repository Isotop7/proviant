// common implements non-specifc handlers
package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gitlab.com/Isotop7/expiro/api"
)

// GetHealth returns the health status of the backend
// @Summary      	Gets health
// @Description  	Gets health status of the backend
// @Tags         	common
// @Accept			json
// @Produce      	json
// @Success      	200  {object}  api.APIResponse
// @Router       	/api/health [get]
// GET /api/health
func GetHealth(c *gin.Context) {
	c.JSON(http.StatusOK, api.APIResponse{Message: "ok"})
}
