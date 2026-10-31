package v1

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/errors"
	apiModel "github.com/Isotop7/proviant/models/api"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

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

	linkedProduct := linkedProductOf(appCtx, req.ProductID)

	if req.Name == "" && linkedProduct != nil {
		req.Name = linkedProduct.ProductName
		if req.Category == "" {
			req.Category = linkedProduct.Categories
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
		CategoryKey: itemCategoryKey(appCtx, linkedProduct, req.Category),
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

// itemCategoryKey is the canonical category of a shopping list item.
//
// The wording of the item names its category: it is the item's own statement
// about itself, and it is what the household wrote. The canonical key of the
// product the item came from is the fallback for a wording that names no
// canonical category — an item keeps the classification of the product it
// was taken from. One order for every path, so the same wording always names
// the same category, however the item came to be.
//
// The order is one line: swap the two blocks to let the product's key win.
func itemCategoryKey(appCtx *AppContext, linked *dbModel.Product, freeText string) *string {
	resolver, err := appCtx.Repos.Savings.LoadCategoryResolver()
	if err != nil {
		appCtx.Logger.Warn().Msgf("Could not load the canonical categories: %s", err)
	} else if key, ok := resolver.ResolveKey(freeText); ok {
		return &key
	}
	if linked != nil {
		return linked.CategoryKey
	}
	return nil
}

// linkedProductOf is the product with the given id, or nil when there is
// none, when it is gone, or when it belongs to another household.
func linkedProductOf(appCtx *AppContext, productID *uint) *dbModel.Product {
	if productID == nil {
		return nil
	}
	product, err := appCtx.Repos.Products.GetProductByID(*productID, appCtx.UserID)
	if err != nil {
		return nil
	}
	return &product
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
	if req.Category != nil && *req.Category != item.Category {
		item.Category = *req.Category
		// The canonical category follows the new wording: a key that named
		// the old category would hand the purchase the wrong shelf life. The
		// product the item came from is the fallback for a wording that names
		// no canonical category.
		item.CategoryKey = itemCategoryKey(appCtx, linkedProductOf(appCtx, item.ProductID), item.Category)
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
			CategoryKey: product.CategoryKey,
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

// PurchaseShoppingListItem turns a shopping list item into a bought product
// @Summary      Buys a shopping list item
// @Description  Turns a shopping list item into a product of the household and checks the item. The pack count of the product is the number of packs bought, which is not the number the item asked for. The expiry is the one the request carries; without one the household's shelf life of the category fills it. An item that already points at a product gets the bought packs added to that product instead of a second row — that product keeps its own expiry, category, pack size and storage location, so the fields of the request that describe a product do not apply to it. The product and the check land together or not at all. A bought product that matches one the household already has is refused with 409 and the matched product in the body: the caller decides whether the packs belong to that row (addToProductId) or make a product of their own (allowDuplicate).
// @Tags         shopping-list
// @Accept       json
// @Produce      json
// @Param        id path int true "Shopping list item ID"
// @Param        request body apiModel.PurchaseShoppingListItemRequest true "The purchase: packs bought, expiry, category, storage location"
// @Success      201 {object} apiModel.PurchaseShoppingListItemResponse
// @Failure      400 {object} api.APIResponse
// @Failure      404 {object} api.APIResponse
// @Failure      409 {object} api.ProductConflictResponse
// @Failure      500 {object} api.APIResponse
// @Router       /api/v1/shopping-list/{id}/purchase [post]
// @Security     BearerAuth
func PurchaseShoppingListItem(ctx *gin.Context, appCtx *AppContext) {
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

	item, err := appCtx.Repos.ShoppingListItems.GetByID(id, householdID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting shopping list item: %s", err)
		api.RespondError(ctx, http.StatusNotFound, errors.ErrShoppingListItemNotFound)
		return
	}

	var req apiModel.PurchaseShoppingListItemRequest
	if !bindJSON(ctx, appCtx.Logger, &req) {
		return
	}

	// The pack count of the product is the number of packs bought. The item
	// asks for a number; the two are not the same number, so the request
	// carries the bought one and the item's is only the fallback.
	packs := item.Quantity
	if req.Quantity != nil {
		packs = *req.Quantity
	}
	packs = util.ClampProductQuantity(packs)

	// The item already names a product the household tracks: the bought packs
	// belong to it, so no second row is written. The product keeps its own
	// date, category, pack size and location — the purchase only adds packs.
	// A request may name another product: the caller resolved a same-item
	// conflict by hand. A product that is gone is no second row either: the
	// purchase creates a new one.
	targetProductID := item.ProductID
	namedByRequest := false
	if req.AddToProductID != nil {
		targetProductID = req.AddToProductID
		namedByRequest = true
	}
	if targetProductID != nil {
		linked, linkedErr := appCtx.Repos.Products.GetProductByID(*targetProductID, appCtx.UserID)
		if linkedErr == nil {
			if err := appCtx.Repos.ShoppingListItems.PurchaseIntoProduct(item, linked.ID, packs); err != nil {
				if err == errors.ErrShoppingListItemProductGone {
					appCtx.Logger.Warn().Msgf("The product of shopping list item %d is gone: %s", item.ID, err)
					api.RespondError(ctx, http.StatusNotFound, errors.ErrShoppingListItemProductGone)
					return
				}
				if err == errors.ErrShoppingListItemNotFound {
					appCtx.Logger.Warn().Msgf("Shopping list item %d is gone: %s", item.ID, err)
					api.RespondError(ctx, http.StatusNotFound, errors.ErrShoppingListItemNotFound)
					return
				}
				appCtx.Logger.Error().Msgf("Error buying shopping list item %d: %s", item.ID, err)
				ctx.JSON(http.StatusInternalServerError, api.CreateFailedError())
				return
			}
			updated, updatedErr := appCtx.Repos.Products.GetProductByID(linked.ID, appCtx.UserID)
			if updatedErr != nil {
				appCtx.Logger.Error().Msgf("Error reloading the bought product: %s", updatedErr)
				ctx.JSON(http.StatusInternalServerError, api.CreateFailedError())
				return
			}
			item.Checked = true
			fireQuantityChange(appCtx.Repos, appCtx.UserID, &updated, packs)
			product := apiModel.NewProductResponse(&updated)
			ctx.JSON(http.StatusCreated, apiModel.PurchaseShoppingListItemResponse{
				Item:    apiModel.NewShoppingListItemResponse(item),
				Product: &product,
				Created: false,
			})
			return
		}
		if namedByRequest {
			// The caller named a product this household cannot see. Buying a
			// different one would be a silent substitution, so the purchase
			// is refused. Unknown and foreign ids answer alike, so a caller
			// cannot probe which ids exist elsewhere.
			appCtx.Logger.Warn().Msgf("The product %d of shopping list item %d is not accessible: %s", *targetProductID, item.ID, linkedErr)
			ctx.JSON(http.StatusNotFound, api.APIResponse{
				Message: fmt.Sprintf(errors.FormatProductWithIDNotFound, *targetProductID),
				Action:  MsgCheckProductIdTryAgain,
			})
			return
		}
		appCtx.Logger.Warn().Msgf("The product of shopping list item %d is gone: %s", item.ID, linkedErr)
	}

	product := dbModel.Product{
		ProductName: item.Name,
		Categories:  item.Category,
		CategoryKey: item.CategoryKey,
		PackSize:    item.PackSize,
		Quantity:    packs,
		UserID:      appCtx.UserID,
	}
	if req.PackSize != nil {
		product.PackSize = *req.PackSize
	}
	if req.StorageLocationID != nil {
		// The location has to be one of the household's: a product may not
		// point at the storage of another one.
		if _, locationErr := appCtx.Repos.StorageLocations.GetByID(*req.StorageLocationID, appCtx.UserID); locationErr != nil {
			if locationErr == errors.ErrStorageLocationNotFound {
				api.RespondError(ctx, http.StatusNotFound, locationErr)
				return
			}
			if locationErr == errors.ErrStorageLocationNotOwned {
				api.RespondError(ctx, http.StatusForbidden, locationErr)
				return
			}
			appCtx.Logger.Error().Msgf("Error loading storage location %d: %s", *req.StorageLocationID, locationErr)
			ctx.JSON(http.StatusInternalServerError, api.InternalError())
			return
		}
		product.StorageLocationID = req.StorageLocationID
	}
	if req.Barcode != nil {
		product.Barcode = *req.Barcode
	}
	// A bought item that was not scanned must not claim it was.
	if req.Scanned {
		product.ScannedAt = time.Now()
	}
	// An explicit category replaces the category of the item. An empty key
	// clears it, and the free text goes with it so the resolution cannot
	// find the category of the item again.
	if req.CategoryKey != nil {
		product.CategoryKey = req.CategoryKey
		if strings.TrimSpace(*req.CategoryKey) == "" {
			product.Categories = ""
		}
	}
	if req.ExpireAt != nil && strings.TrimSpace(*req.ExpireAt) != "" {
		expireAt, parseErr := parseDraftExpiry(*req.ExpireAt)
		if parseErr != nil {
			appCtx.Logger.Warn().Msgf("Invalid expiry date '%s': %s", *req.ExpireAt, parseErr)
			api.RespondError(ctx, http.StatusBadRequest, errors.ErrInvalidExpiryDate)
			return
		}
		product.ExpireAt = expireAt
	}

	// The category and the shelf life are the rules of the add-product path:
	// the key is resolved the same way and the expiry is only filled while
	// the request carried none.
	if !applyHouseholdCategory(ctx, appCtx, &product) {
		return
	}

	// The same-item rule of the add-product path: a bought product that
	// matches one the household already has is refused instead of written.
	// The pack may belong to the product that is already there. A request
	// that names the product itself has resolved the conflict already.
	if !req.AllowDuplicate && req.AddToProductID == nil {
		duplicate, isDuplicate, duplicateErr := appCtx.Repos.Products.FindDuplicateProduct(appCtx.UserID, &product)
		if duplicateErr != nil {
			appCtx.Logger.Error().Msgf("Error looking up a duplicate product: %s", duplicateErr)
			ctx.JSON(http.StatusInternalServerError, api.CreateFailedError())
			return
		}
		if isDuplicate {
			appCtx.Logger.Info().Msgf("Purchase of shopping list item %d duplicates product #%d", item.ID, duplicate.ID)
			matched := apiModel.NewProductResponse(&duplicate)
			ctx.JSON(http.StatusConflict, api.ProductConflictResponse{
				APIResponse: api.APIResponse{
					Message: errors.ErrDuplicateProduct.Error(),
					Action:  "Add a pack to the existing product, create a separate one, or cancel",
				},
				Duplicate: &matched,
			})
			return
		}
	}

	// The product and the check land together: a failure between the two
	// writes would leave a bought product nobody checked off.
	if err := appCtx.Repos.ShoppingListItems.PurchaseWithProduct(item, &product); err != nil {
		if err == errors.ErrShoppingListItemNotFound {
			appCtx.Logger.Warn().Msgf("Shopping list item %d is gone: %s", item.ID, err)
			api.RespondError(ctx, http.StatusNotFound, errors.ErrShoppingListItemNotFound)
			return
		}
		appCtx.Logger.Error().Msgf("Error buying shopping list item %d: %s", item.ID, err)
		ctx.JSON(http.StatusInternalServerError, api.CreateFailedError())
		return
	}

	item.Checked = true
	item.ProductID = &product.ID

	fireProductCreated(appCtx.Repos, appCtx.UserID, &product, packs)

	created := apiModel.NewProductResponse(&product)
	ctx.JSON(http.StatusCreated, apiModel.PurchaseShoppingListItemResponse{
		Item:    apiModel.NewShoppingListItemResponse(item),
		Product: &created,
		Created: true,
	})
}
