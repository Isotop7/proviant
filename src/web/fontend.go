package web

import (
	"html/template"
	"net/http"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gitlab.com/Isotop7/expiro/api"
	"gitlab.com/Isotop7/expiro/controllers"
	"gitlab.com/Isotop7/expiro/models/configuration/static"
	"gitlab.com/Isotop7/expiro/templates"
	"gorm.io/gorm"
)

type Frontend struct {
	TemplateCache map[string]*template.Template
}

func (frontend *Frontend) Root(ctx *gin.Context) {
	pageData := map[string]any{
		"Title": "Home",
	}
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
	pageData := map[string]any{
		"Title": "User Settings",
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "userSettings.tmpl", pageData)
}

func (frontend *Frontend) Products(ctx *gin.Context) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	// Get database instance from context
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		//TODO: Show error
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrDatabaseContextNotFound)
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		// TODO: Show error
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		ctx.JSON(http.StatusBadRequest, api.ResponseErrUserIDFromToken)
		return
	}

	// Create database controller
	dbController := controllers.DatabaseController{DBHandle: dbHandle}
	// Get products of user from database with optional limit
	products, productBulkErr := dbController.GetUserProductsBulk(userID, -1)
	if productBulkErr != nil {
		// TODO: Show error
		logger.Error().Msgf("Error getting products of user: %s", productBulkErr)
		ctx.JSON(http.StatusBadRequest, api.APIResponse{Message: "Error getting products of user"})
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
	pageData := map[string]any{
		"Title": "View Product",
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "productsView.tmpl", pageData)
}

func (frontend *Frontend) ProductsEdit(ctx *gin.Context) {
	pageData := map[string]any{
		"Title": "Edit Product",
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "productsEdit.tmpl", pageData)
}
