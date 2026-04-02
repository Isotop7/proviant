package web

import (
	"html/template"
	"net/http"
	"strconv"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/templates"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type Frontend struct {
	TemplateCache map[string]*template.Template
}

func (frontend *Frontend) Root(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Extract user id
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserIDFromToken.Error())
		return
	}

	// Get database instance from context
	dbHandle, ok := ctx.MustGet("dbHandle").(*gorm.DB)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusInternalServerError, errors.ErrDatabaseContextNotFound.Error())
		return
	}

	// Create database controller object
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Get user tiles
	homeTiles, homeTileErr := dbController.GetUserHomeTiles(userID)
	if homeTileErr != nil {
		logger.Error().Msg(homeTileErr.Error())
	}
	// Get user household
	var hasHousehold bool
	userHouseholdID, userErr := dbController.GetUserHouseholdByID(userID)
	if userErr != nil {
		logger.Error().Msg(userErr.Error())
	}
	hasHousehold = userHouseholdID > 0

	// Setup page data
	pageData := map[string]any{
		"Title":        "Home",
		"Tiles":        homeTiles,
		"HasHousehold": hasHousehold,
		"Household":    userHouseholdID,
	}

	// Render website
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "home.tmpl", pageData)
}

func (frontend *Frontend) Auth(ctx *gin.Context) {
	pageData := map[string]any{
		"Title": "Authentication",
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "baseAuth", "auth.tmpl", pageData)
}

func (frontend *Frontend) User(ctx *gin.Context) {
	pageData := map[string]any{
		"Title": "User",
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "user.tmpl", pageData)
}

func (frontend *Frontend) UserSettings(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Extract user id
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserIDFromToken.Error())
		return
	}

	// Get database instance from context
	dbHandle, ok := ctx.MustGet("dbHandle").(*gorm.DB)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusInternalServerError, errors.ErrDatabaseContextNotFound.Error())
		return
	}

	// Create database controller object
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Get user object
	user, userErr := dbController.GetUserByID(userID)
	if userErr != nil {
		logger.Error().Msg(api.ResponseErrInvalidUserData.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrInvalidUserData.Error())
		return
	}
	// Get household object
	household, householdErr := dbController.GetHouseholdByID(user.HouseholdID)
	if householdErr != nil {
		logger.Error().Msg(api.ResponseErrInvalidUserData.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrInvalidUserData.Error())
		return
	}

	isAdmin := household.AdminID == userID

	// Fetch pending applications if the user is the household admin
	pendingApplications, appErr := dbController.GetPendingApplicationsForAdmin(userID)
	if appErr != nil {
		// Non-fatal: admin check failing just means no applications are shown
		pendingApplications = nil
	}

	pageData := map[string]any{
		"Title":               "User Settings",
		"User":                user,
		"Household":           household,
		"IsAdmin":             isAdmin,
		"PendingApplications": pendingApplications,
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "userSettings.tmpl", pageData)
}

func (frontend *Frontend) Products(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrDatabaseContextNotFound.Error())
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserIDFromToken.Error())
		return
	}

	// Get query parameters
	queryParam := ctx.Query("queryParam")
	queryValue := ctx.Query("queryValue")
	sort := ctx.DefaultQuery("sort", "created_at")
	order := ctx.DefaultQuery("order", "asc")

	dbController := database.DatabaseController{DBHandle: dbHandle}

	var products []dbModel.Product
	var productErr error

	if queryParam != "" && queryValue != "" {
		enumParam := database.SearchParameterEnumFromString(queryParam)
		if enumParam == database.InvalidParameter {
			logger.Error().Msg(errors.ErrProductSearchInvalidQuery.Error())
			templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrProductSearchInvalidQuery.Error())
			return
		}
		products, productErr = dbController.SearchProducts(enumParam, queryValue, sort, order, userID)
	} else {
		products, productErr = dbController.GetUserProductsBulk(userID, -1)
	}

	if productErr != nil {
		logger.Error().Msgf("Error getting products of user: %s", productErr)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserNoProductsFound.Error())
		return
	}

	pageData := map[string]any{
		"Title":      "Products",
		"Products":   products,
		"QueryParam": queryParam,
		"QueryValue": queryValue,
		"Sort":       sort,
		"Order":      order,
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "products.tmpl", pageData)
}

func (frontend *Frontend) ProductsArchived(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrDatabaseContextNotFound.Error())
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserIDFromToken.Error())
		return
	}

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Get archived products of user from database with optional limit
	archivedProducts, productBulkErr := dbController.GetUserArchivedProductsBulk(userID, -1)
	if productBulkErr != nil {
		logger.Error().Msgf("Error getting archivedproducts of user: %s", productBulkErr)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserNoProductsFound.Error())
		return
	}

	pageData := map[string]any{
		"Title":    "ArchivedProducts",
		"Products": archivedProducts,
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "productsArchived.tmpl", pageData)
}

func (frontend *Frontend) ProductsCreate(ctx *gin.Context) {
	pageData := map[string]any{
		"Title": "Create product",
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "productsCreate.tmpl", pageData)
}

func (frontend *Frontend) ProductsScan(ctx *gin.Context) {
	pageData := map[string]any{
		"Title": "Scan Product",
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "productsScan.tmpl", pageData)
}

func (frontend *Frontend) ProductsView(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get and parse parameter id
	idParam := ctx.Param("id")
	var productID int
	var convErr error
	if productID, convErr = strconv.Atoi(idParam); convErr != nil {
		logger.Warn().Msgf(errors.FormatInvalidRequestId, idParam)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, convErr.Error())
		return
	}

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrDatabaseContextNotFound.Error())
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserIDFromToken.Error())
		return
	}

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Get products of user from database with optional limit
	product, productErr := dbController.GetProductByID(productID, userID)
	if productErr != nil {
		logger.Error().Msgf("Error getting product: %s", productErr)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserNoProductsFound.Error())
		return
	}

	pageData := map[string]any{
		"Title":   "Products",
		"Product": product,
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "productsView.tmpl", pageData)
}

func (frontend *Frontend) ProductsEdit(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get and parse parameter id
	idParam := ctx.Param("id")
	var productID int
	var convErr error
	if productID, convErr = strconv.Atoi(idParam); convErr != nil {
		logger.Warn().Msgf(errors.FormatInvalidRequestId, idParam)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, convErr.Error())
		return
	}

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrDatabaseContextNotFound.Error())
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserIDFromToken.Error())
		return
	}

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Get products of user from database with optional limit
	product, productErr := dbController.GetProductByID(productID, userID)
	if productErr != nil {
		logger.Error().Msgf("Error getting product: %s", productErr)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserNoProductsFound.Error())
		return
	}

	pageData := map[string]any{
		"Title":   "Products",
		"Product": product,
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "productsEdit.tmpl", pageData)
}
