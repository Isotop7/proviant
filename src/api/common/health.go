// common implements non-specifc handlers
package common

import (
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"github.com/gin-gonic/gin"
)

// GetHealth returns the health status of the API
// @Summary      	Gets health
// @Description  	Gets health status of the API
// @Tags         	common
// @Accept			json
// @Produce      	json
// @Success      	200  {object}  api.APIResponse
// @Router       	/api/health [get]
func GetHealth(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, api.APIResponse{Message: "ok"})
}
