package v1

import (
	"net/http"
	"strconv"

	"codeberg.org/isotop7/proviant/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"
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

func CreateShoppingListItem(ctx *gin.Context, appCtx *AppContext) {
	var req struct {
		ProductID *uint  `json:"productId"`
		Name      string `json:"name"`
		Category  string `json:"category"`
		Quantity  int    `json:"quantity"`
		Unit      string `json:"unit"`
		Notes     string `json:"notes"`
	}
	if !bindJSON(ctx, appCtx.Logger, &req) {
		return
	}

	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error creating shopping list item"})
		return
	}

	if req.Name == "" && req.ProductID != nil {
		product, err := appCtx.Repos.Products.GetProductByID(*req.ProductID, appCtx.UserID)
		if err == nil {
			req.Name = product.ProductName
			if req.Category == "" {
				req.Category = product.Categories
			}
		}
	}

	if req.Name == "" {
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "Name is required"})
		return
	}

	item := &dbModel.ShoppingListItem{
		HouseholdID: householdID,
		ProductID:   req.ProductID,
		Name:        req.Name,
		Category:    req.Category,
		Quantity:    req.Quantity,
		Unit:        req.Unit,
		Notes:       req.Notes,
		CreatedBy:   appCtx.UserID,
	}

	if err := appCtx.Repos.ShoppingListItems.Create(item); err != nil {
		appCtx.Logger.Error().Msgf("Error creating shopping list item: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error creating shopping list item"})
		return
	}

	ctx.JSON(http.StatusCreated, item)
}

func ListShoppingListItems(ctx *gin.Context, appCtx *AppContext) {
	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error loading shopping list"})
		return
	}

	items, err := appCtx.Repos.ShoppingListItems.ListByHousehold(householdID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error listing shopping list items: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error loading shopping list"})
		return
	}

	ctx.JSON(http.StatusOK, items)
}

func UpdateShoppingListItem(ctx *gin.Context, appCtx *AppContext) {
	id, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	var req struct {
		Name     *string `json:"name"`
		Category *string `json:"category"`
		Quantity *int    `json:"quantity"`
		Unit     *string `json:"unit"`
		Checked  *bool   `json:"checked"`
		Notes    *string `json:"notes"`
	}
	if !bindJSON(ctx, appCtx.Logger, &req) {
		return
	}

	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error updating shopping list item"})
		return
	}

	item, err := appCtx.Repos.ShoppingListItems.GetByID(id, householdID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting shopping list item: %s", err)
		ctx.JSON(http.StatusNotFound, api.APIResponse{Message: "Shopping list item not found"})
		return
	}

	if req.Name != nil {
		item.Name = *req.Name
	}
	if req.Category != nil {
		item.Category = *req.Category
	}
	if req.Quantity != nil {
		item.Quantity = *req.Quantity
	}
	if req.Unit != nil {
		item.Unit = *req.Unit
	}
	if req.Checked != nil {
		item.Checked = *req.Checked
	}
	if req.Notes != nil {
		item.Notes = *req.Notes
	}

	if err := appCtx.Repos.ShoppingListItems.Update(item); err != nil {
		appCtx.Logger.Error().Msgf("Error updating shopping list item: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error updating shopping list item"})
		return
	}

	ctx.JSON(http.StatusOK, item)
}

func DeleteShoppingListItem(ctx *gin.Context, appCtx *AppContext) {
	id, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error deleting shopping list item"})
		return
	}

	if err := appCtx.Repos.ShoppingListItems.Delete(id, householdID); err != nil {
		appCtx.Logger.Error().Msgf("Error deleting shopping list item: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error deleting shopping list item"})
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Shopping list item deleted"})
}

func ToggleShoppingListItem(ctx *gin.Context, appCtx *AppContext) {
	id, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error toggling shopping list item"})
		return
	}

	if err := appCtx.Repos.ShoppingListItems.ToggleChecked(id, householdID); err != nil {
		appCtx.Logger.Error().Msgf("Error toggling shopping list item: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error toggling shopping list item"})
		return
	}

	item, _ := appCtx.Repos.ShoppingListItems.GetByID(id, householdID)
	ctx.JSON(http.StatusOK, item)
}

func ImportAutoListToShoppingList(ctx *gin.Context, appCtx *AppContext) {
	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error importing auto shopping list"})
		return
	}

	autoProducts, err := appCtx.Repos.Products.GetSubThresholdProducts(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting sub-threshold products: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error importing auto shopping list"})
		return
	}

	if len(autoProducts) == 0 {
		ctx.JSON(http.StatusOK, api.APIResponse{Message: "No products below threshold"})
		return
	}

	existingItems, _ := appCtx.Repos.ShoppingListItems.ListByHousehold(householdID)
	existingProductIDs := make(map[uint]bool)
	for i := range existingItems {
		if existingItems[i].ProductID != nil {
			existingProductIDs[*existingItems[i].ProductID] = true
		}
	}

	var imported int
	for i := range autoProducts {
		product := &autoProducts[i]
		if existingProductIDs[product.ID] {
			continue
		}

		item := &dbModel.ShoppingListItem{
			HouseholdID: householdID,
			ProductID:   &product.ID,
			Name:        product.ProductName,
			Category:    product.Categories,
			Quantity:    product.MinStockAmount - product.Amount,
		}
		if item.Quantity <= 0 {
			item.Quantity = 1
		}
		item.Unit = product.Unit
		item.CreatedBy = appCtx.UserID

		if err := appCtx.Repos.ShoppingListItems.Create(item); err != nil {
			appCtx.Logger.Warn().Msgf("Error creating shopping list item for product %d: %s", product.ID, err)
			continue
		}
		imported++
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: strconv.Itoa(imported) + " items imported"})
}
