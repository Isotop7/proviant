package web

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

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
		"InviteToken":  ctx.Query("invite_token"),
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
		"InviteToken": ctx.Query("invite_token"),
		"Title":       "Authentication",
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "baseAuth", "auth.tmpl", pageData)
}

func (frontend *Frontend) User(ctx *gin.Context) {
	pageData := map[string]any{
		"InviteToken": ctx.Query("invite_token"),
		"Title":       "User",
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

	members, _ := dbController.GetHouseholdMembers(user.HouseholdID)

	pendingApplications, _ := dbController.GetPendingApplicationsForAdmin(userID)

	myApplications, _ := dbController.GetPendingApplicationsForApplicant(userID)

	pageData := map[string]any{
		"InviteToken":         ctx.Query("invite_token"),
		"Title":               "User Settings",
		"User":                user,
		"Household":           household,
		"IsAdmin":             isAdmin,
		"Members":             members,
		"PendingApplications": pendingApplications,
		"MyApplications":      myApplications,
	}

	// Only admins can see and manage invitations
	if isAdmin {
		invitations, _ := dbController.GetInvitationsForHousehold(user.HouseholdID, userID)
		pageData["Invitations"] = invitations
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
		"InviteToken": ctx.Query("invite_token"),
		"Title":       "Products",
		"Products":    products,
		"QueryParam":  queryParam,
		"QueryValue":  queryValue,
		"Sort":        sort,
		"Order":       order,
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
		"InviteToken": ctx.Query("invite_token"),
		"Title":       "ArchivedProducts",
		"Products":    archivedProducts,
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "productsArchived.tmpl", pageData)
}

func (frontend *Frontend) ProductsCreate(ctx *gin.Context) {
	pageData := map[string]any{
		"InviteToken": ctx.Query("invite_token"),
		"Title":       "Create product",
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "productsCreate.tmpl", pageData)
}

func (frontend *Frontend) ProductsScan(ctx *gin.Context) {
	pageData := map[string]any{
		"InviteToken": ctx.Query("invite_token"),
		"Title":       "Scan Product",
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
		"InviteToken": ctx.Query("invite_token"),
		"Title":       "Products",
		"Product":     product,
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
		"InviteToken": ctx.Query("invite_token"),
		"Title":       "Products",
		"Product":     product,
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "productsEdit.tmpl", pageData)
}

// AcceptInvite renders the invitation acceptance page
func (frontend *Frontend) AcceptInvite(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)
	token := ctx.Query("token")

	if token == "" {
		templates.Render(ctx, frontend.TemplateCache, http.StatusBadRequest, "base", "acceptInvite.tmpl", map[string]any{
			"Title": "Accept Invitation",
			"Error": "No invitation token provided.",
		})
		return
	}

	// Get database handle
	dbHandle, ok := ctx.MustGet("dbHandle").(*gorm.DB)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		templates.Render(ctx, frontend.TemplateCache, http.StatusInternalServerError, "base", "acceptInvite.tmpl", map[string]any{
			"Title": "Accept Invitation",
			"Error": "Internal server error.",
		})
		return
	}

	dbController := database.DatabaseController{DBHandle: dbHandle}

	// Look up the invitation
	invitation, invErr := dbController.GetInvitationByToken(token)
	if invErr != nil {
		if invErr == errors.ErrInvitationNotFound {
			templates.Render(ctx, frontend.TemplateCache, http.StatusNotFound, "base", "acceptInvite.tmpl", map[string]any{
				"Title": "Accept Invitation",
				"Error": "This invitation does not exist or has been deleted.",
			})
			return
		}
		logger.Error().Msg(invErr.Error())
		templates.Render(ctx, frontend.TemplateCache, http.StatusInternalServerError, "base", "acceptInvite.tmpl", map[string]any{
			"Title": "Accept Invitation",
			"Error": "An error occurred while processing this invitation.",
		})
		return
	}

	// Check if invitation is still valid
	if invitation.Status != dbModel.InvitationStatusPending {
		msg := "This invitation has already been used."
		if invitation.Status == dbModel.InvitationStatusCancelled {
			msg = "This invitation has been cancelled by the sender."
		} else if invitation.Status == dbModel.InvitationStatusExpired {
			msg = "This invitation has expired."
		}
		templates.Render(ctx, frontend.TemplateCache, http.StatusGone, "base", "acceptInvite.tmpl", map[string]any{
			"Title": "Accept Invitation",
			"Error": msg,
		})
		return
	}

	if time.Now().After(invitation.ExpiresAt) {
		// Mark as expired
		_ = dbController.DBHandle.Model(&dbModel.HouseholdInvitation{}).Where("id = ?", invitation.ID).Update("status", dbModel.InvitationStatusExpired)
		templates.Render(ctx, frontend.TemplateCache, http.StatusGone, "base", "acceptInvite.tmpl", map[string]any{
			"Title": "Accept Invitation",
			"Error": "This invitation has expired.",
		})
		return
	}

	// Get household name for display
	household, householdErr := dbController.GetHouseholdByID(invitation.HouseholdID)
	householdName := fmt.Sprintf("Household #%d", invitation.HouseholdID)
	if householdErr == nil {
		householdName = household.Name
	}

	// Check if user is already authenticated
	claims := jwt.ExtractClaims(ctx)
	if claims != nil {
		userID, ok := claims[static.TokenIdentityKey].(float64)
		if ok && uint(userID) > 0 {
			// User is logged in — show confirmation
			templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "acceptInvite.tmpl", map[string]any{
				"Title":           "Accept Invitation",
				"ConfirmAccept":   true,
				"Token":           token,
				"HouseholdName":   householdName,
				"InvitationEmail": invitation.Email,
			})
			return
		}
	}

	// User is not logged in — redirect to auth with token
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "baseAuth", "acceptInvite.tmpl", map[string]any{
		"Title":           "Accept Invitation",
		"NeedsAuth":       true,
		"Token":           token,
		"HouseholdName":   householdName,
		"InvitationEmail": invitation.Email,
	})
}

// Onboarding renders the post-signup onboarding wizard
func (frontend *Frontend) Onboarding(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	dbHandle, ok := ctx.MustGet("dbHandle").(*gorm.DB)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusInternalServerError, errors.ErrDatabaseContextNotFound.Error())
		return
	}

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserIDFromToken.Error())
		return
	}

	dbController := database.DatabaseController{DBHandle: dbHandle}
	user, userErr := dbController.GetUserByID(userID)
	if userErr != nil {
		logger.Error().Msg(api.ResponseErrInvalidUserData.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrInvalidUserData.Error())
		return
	}

	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "baseAuth", "onboarding.tmpl", map[string]any{
		"Title":    "Onboarding",
		"Username": user.Username,
	})
}
