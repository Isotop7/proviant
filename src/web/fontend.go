package web

import (
	"html/template"
	"net/http"
	"strconv"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gitlab.com/Isotop7/expiro/api"
	"gitlab.com/Isotop7/expiro/controllers/database"
	"gitlab.com/Isotop7/expiro/errors"
	"gitlab.com/Isotop7/expiro/models/configuration/static"
	"gitlab.com/Isotop7/expiro/templates"
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

	// Setup page data
	pageData := map[string]any{
		"Title": "Home",
		"Tiles": homeTiles,
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

	pageData := map[string]any{
		"Title": "User Settings",
		"User":  user,
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

	// Create database controller
	dbController := database.DatabaseController{DBHandle: dbHandle}
	// Get products of user from database with optional limit
	products, productBulkErr := dbController.GetUserProductsBulk(userID, -1)
	if productBulkErr != nil {
		logger.Error().Msgf("Error getting products of user: %s", productBulkErr)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserNoProductsFound.Error())
		return
	}

	pageData := map[string]any{
		"Title":    "Products",
		"Products": products,
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "products.tmpl", pageData)
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
		logger.Warn().Msgf("Requested ID '%s' is invalid", idParam)
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
		logger.Warn().Msgf("Requested ID '%s' is invalid", idParam)
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

func (frontend *Frontend) Search(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Helper variables
	searchParameterEnum := database.InvalidParameter
	var searchParameter string
	var searchQuery string
	// Parse all query parameters, get first, run function with it
	queryParams := ctx.Request.URL.Query()

	// Loop through params and check for valid param
	for param := range queryParams {
		enumParam := database.SearchParameterEnumFromString(param)
		if enumParam != database.InvalidParameter {
			// If valid parameter is found, assign vars and exit loop
			searchParameterEnum = enumParam
			searchParameter = param
			searchQuery = queryParams[param][0]
			break
		}
	}

	if searchParameterEnum == database.InvalidParameter {
		// If no supported parameter was found, exit
		logger.Error().Msg(errors.ErrProductSearchInvalidQuery.Error())
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrProductSearchInvalidQuery.Error())
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
	products, productErr := dbController.SearchProducts(searchQuery, searchParameterEnum, userID)
	if productErr != nil {
		logger.Error().Msgf("Error getting products: %s", productErr)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserNoProductsFound.Error())
		return
	}

	pageData := map[string]any{
		"Title":           "Search results",
		"SearchParameter": searchParameter,
		"SearchQuery":     searchQuery,
		"Products":        products,
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "search.tmpl", pageData)
}
