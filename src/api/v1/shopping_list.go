package v1

import (
	"net/http"
	"strconv"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/errors"
	apiModel "github.com/Isotop7/proviant/models/api"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/gin-gonic/gin"
)

// GetAutoShoppingList returns the products whose stock fell below their minimum amount
// @Summary      Gets auto shopping list
// @Description  Returns all products of the caller whose pack count is below their minimum stock amount
// @Tags         shopping-list
// @Produce      json
// @Success      200 {array} apiModel.ProductResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/shopping-list/auto [get]
// @Security     BearerAuth
func GetAutoShoppingList(ctx *gin.Context, appCtx *AppContext) {
	products, err := appCtx.Repos.Products.GetSubThresholdProducts(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error loading auto shopping list: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, apiModel.NewProductResponses(products))
}

// CreateShoppingListItem creates a shopping list item for the caller's household
// @Summary      Creates a shopping list item
// @Description  Creates a shopping list item either from a product or with a free-text name
// @Tags         shopping-list
// @Accept       json
// @Produce      json
// @Param        request body apiModel.CreateShoppingListItemRequest true "Item: productId or name, optional category, quantity, notes"
// @Success      201 {object} apiModel.ShoppingListItemResponse
// @Failure      400 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/shopping-list [post]
// @Security     BearerAuth
func CreateShoppingListItem(ctx *gin.Context, appCtx *AppContext) {
	var req apiModel.CreateShoppingListItemRequest
	if !bindJSON(ctx, appCtx.Logger, &req) {
		return
	}

	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
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
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrShoppingListItemNameRequired)
		return
	}

	item := &dbModel.ShoppingListItem{
		HouseholdID: householdID,
		ProductID:   req.ProductID,
		Name:        req.Name,
		Category:    req.Category,
		Quantity:    req.Quantity,
		Notes:       req.Notes,
		CreatedBy:   appCtx.UserID,
	}

	if err := appCtx.Repos.ShoppingListItems.Create(item); err != nil {
		appCtx.Logger.Error().Msgf("Error creating shopping list item: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusCreated, apiModel.NewShoppingListItemResponse(item))
}

// ListShoppingListItems lists the shopping list items of the caller's household
// @Summary      Lists shopping list items
// @Description  Returns all shopping list items of the caller's household
// @Tags         shopping-list
// @Produce      json
// @Success      200 {array} apiModel.ShoppingListItemResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/shopping-list [get]
// @Security     BearerAuth
func ListShoppingListItems(ctx *gin.Context, appCtx *AppContext) {
	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	items, err := appCtx.Repos.ShoppingListItems.ListByHousehold(householdID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error listing shopping list items: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, apiModel.NewShoppingListItemResponses(items))
}

// UpdateShoppingListItem updates a shopping list item of the caller's household
// @Summary      Updates a shopping list item
// @Description  Updates name, category, quantity, checked state and notes of a shopping list item
// @Tags         shopping-list
// @Accept       json
// @Produce      json
// @Param        id path int true "Shopping list item ID"
// @Param        request body apiModel.UpdateShoppingListItemRequest true "Fields to update: name, category, quantity, checked, notes"
// @Success      200 {object} apiModel.ShoppingListItemResponse
// @Failure      400 {object} api.APIResponse
// @Failure      404 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/shopping-list/{id} [patch]
// @Security     BearerAuth
func UpdateShoppingListItem(ctx *gin.Context, appCtx *AppContext) {
	id, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	var req apiModel.UpdateShoppingListItemRequest
	if !bindJSON(ctx, appCtx.Logger, &req) {
		return
	}

	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	item, err := appCtx.Repos.ShoppingListItems.GetByID(id, householdID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting shopping list item: %s", err)
		api.RespondError(ctx, http.StatusNotFound, errors.ErrShoppingListItemNotFound)
		return
	}

	if req.Name != nil {
		item.Name = *req.Name
	}
	if req.Category != nil {
		item.Category = *req.Category
	}
	if req.Quantity != nil {
		// A row always stands for at least one pack.
		item.Quantity = max(1, *req.Quantity)
	}
	if req.Checked != nil {
		item.Checked = *req.Checked
	}
	if req.Notes != nil {
		item.Notes = *req.Notes
	}

	if err := appCtx.Repos.ShoppingListItems.Update(item); err != nil {
		appCtx.Logger.Error().Msgf("Error updating shopping list item: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, apiModel.NewShoppingListItemResponse(item))
}

// DeleteShoppingListItem deletes a shopping list item of the caller's household
// @Summary      Deletes a shopping list item
// @Description  Deletes the shopping list item with the given ID
// @Tags         shopping-list
// @Produce      json
// @Param        id path int true "Shopping list item ID"
// @Success      200 {object} api.APIResponse
// @Failure      400 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/shopping-list/{id} [delete]
// @Security     BearerAuth
func DeleteShoppingListItem(ctx *gin.Context, appCtx *AppContext) {
	id, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	if err := appCtx.Repos.ShoppingListItems.Delete(id, householdID); err != nil {
		appCtx.Logger.Error().Msgf("Error deleting shopping list item: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Shopping list item deleted"})
}

// ToggleShoppingListItem toggles the checked state of a shopping list item
// @Summary      Toggles a shopping list item
// @Description  Flips the checked state of the shopping list item with the given ID and returns it
// @Tags         shopping-list
// @Produce      json
// @Param        id path int true "Shopping list item ID"
// @Success      200 {object} apiModel.ShoppingListItemResponse
// @Failure      400 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/shopping-list/{id}/toggle [post]
// @Security     BearerAuth
func ToggleShoppingListItem(ctx *gin.Context, appCtx *AppContext) {
	id, ok := parseUintPathParam(ctx, appCtx.Logger, "id")
	if !ok {
		return
	}

	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	if err := appCtx.Repos.ShoppingListItems.ToggleChecked(id, householdID); err != nil {
		appCtx.Logger.Error().Msgf("Error toggling shopping list item: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	item, err := appCtx.Repos.ShoppingListItems.GetByID(id, householdID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error reloading toggled shopping list item: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}
	ctx.JSON(http.StatusOK, apiModel.NewShoppingListItemResponse(item))
}

// ImportAutoListToShoppingList copies the auto shopping list into the household shopping list
// @Summary      Imports auto shopping list
// @Description  Creates shopping list items for all sub-threshold products that are not on the list yet
// @Tags         shopping-list
// @Produce      json
// @Success      200 {object} api.APIResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/shopping-list/import-auto [post]
// @Security     BearerAuth
func ImportAutoListToShoppingList(ctx *gin.Context, appCtx *AppContext) {
	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting household: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	autoProducts, err := appCtx.Repos.Products.GetSubThresholdProducts(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting sub-threshold products: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
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
			Quantity:    product.MinStockAmount - product.Quantity,
		}
		if item.Quantity <= 0 {
			item.Quantity = 1
		}
		item.CreatedBy = appCtx.UserID

		if err := appCtx.Repos.ShoppingListItems.Create(item); err != nil {
			appCtx.Logger.Warn().Msgf("Error creating shopping list item for product %d: %s", product.ID, err)
			continue
		}
		imported++
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: strconv.Itoa(imported) + " items imported"})
}
