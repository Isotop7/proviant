package v1

import (
	"net/http"

	"codeberg.org/isotop7/proviant/api"

	"github.com/gin-gonic/gin"
)

func GetAutoShoppingList(ctx *gin.Context, appCtx *AppContext) {
	products, err := appCtx.Repos.Products.GetSubThresholdProducts(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error loading auto shopping list: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error loading auto shopping list"})
		return
	}

	ctx.JSON(http.StatusOK, products)
}
