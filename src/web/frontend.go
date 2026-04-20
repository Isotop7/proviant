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

// Root renders the home page for authenticated users
// @Summary      Home page
// @Description  Renders the home page showing product dashboard
// @Tags         web
// @Produce      html
// @Success      200  {string}  html
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /web [get]
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

	userRepo := database.NewUserRepository(dbHandle)
	var hasHousehold bool
	userHouseholdID, userErr := userRepo.GetUserHouseholdByID(userID)
	if userErr != nil {
		logger.Error().Msg(userErr.Error())
	}
	hasHousehold = userHouseholdID > 0

	// Setup page data
	pageData := map[string]any{
		"InviteToken":  ctx.Query("invite_token"),
		"Title":        "Home",
		"HasHousehold": hasHousehold,
		"Household":    userHouseholdID,
	}

	// Render website
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "home.tmpl", pageData)
}

// Auth renders the authentication page
// @Summary      Auth page
// @Description  Renders the authentication page for login/signup
// @Tags         web
// @Produce      html
// @Success      200  {string}  html
// @Router       /web/auth [get]
func (frontend *Frontend) Auth(ctx *gin.Context) {
	pageData := map[string]any{
		"InviteToken": ctx.Query("invite_token"),
		"Title":       "Authentication",
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "baseAuth", "auth.tmpl", pageData)
}

// User renders the user page
// @Summary      User page
// @Description  Renders the user page showing user info
// @Tags         web
// @Produce      html
// @Success      200  {string}  html
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /web/user [get]
func (frontend *Frontend) User(ctx *gin.Context) {
	pageData := map[string]any{
		"InviteToken": ctx.Query("invite_token"),
		"Title":       "User",
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "user.tmpl", pageData)
}

// UserSettings renders the user settings page
// @Summary      User settings page
// @Description  Renders the user settings page with household management
// @Tags         web
// @Produce      html
// @Success      200  {string}  html
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /web/user/settings [get]
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

	userRepo := database.NewUserRepository(dbHandle)
	householdRepo := database.NewHouseholdRepository(dbHandle)
	invitationRepo := database.NewInvitationRepository(dbHandle)

	user, userErr := userRepo.GetUserByID(userID)
	if userErr != nil {
		logger.Error().Msg(api.ResponseErrInvalidUserData.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrInvalidUserData.Error())
		return
	}
	user.NotificationPreferences.TelegramLinked = user.NotificationPreferences.TelegramChatID != ""
	if user.NotificationPreferences.TelegramLinked && !user.NotificationPreferences.TelegramEnabled {
		user.NotificationPreferences.TelegramEnabled = true
	}

	telegramConfigured := user.NotificationPreferences.TelegramBotToken != ""

	household, householdErr := householdRepo.GetHouseholdByID(user.HouseholdID)
	if householdErr != nil {
		logger.Error().Msg(api.ResponseErrInvalidUserData.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrInvalidUserData.Error())
		return
	}

	isAdmin := household.AdminID == userID

	members, _ := householdRepo.GetHouseholdMembers(user.HouseholdID)

	pendingApplications, _ := householdRepo.GetPendingApplicationsForAdmin(userID)

	myApplications, _ := householdRepo.GetPendingApplicationsForApplicant(userID)

	pageData := map[string]any{
		"InviteToken":         ctx.Query("invite_token"),
		"Title":               "User Settings",
		"User":                user,
		"Household":           household,
		"IsAdmin":             isAdmin,
		"Members":             members,
		"PendingApplications": pendingApplications,
		"MyApplications":      myApplications,
		"TelegramConfigured":  telegramConfigured,
	}

	if isAdmin {
		invitations, _ := invitationRepo.GetInvitationsForHousehold(user.HouseholdID, userID)
		pageData["Invitations"] = invitations
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "userSettings.tmpl", pageData)
}

// Products renders the products list page
// @Summary      Products page
// @Description  Renders the products list page with optional search
// @Tags         web
// @Produce      html
// @Success      200  {string}  html
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /web/products [get]
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
	productRepo := database.NewProductRepository(dbHandle)

	queryParam := ctx.Query("queryParam")
	queryValue := ctx.Query("queryValue")
	sort := ctx.DefaultQuery("sort", "created_at")
	order := ctx.DefaultQuery("order", "asc")

	var products []dbModel.Product
	var productErr error

	if queryParam != "" && queryValue != "" {
		enumParam := database.SearchParameterEnumFromString(queryParam)
		if enumParam == database.InvalidParameter {
			logger.Error().Msg(errors.ErrProductSearchInvalidQuery.Error())
			templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrProductSearchInvalidQuery.Error())
			return
		}
		products, productErr = productRepo.SearchProducts(enumParam, queryValue, sort, order, userID)
	} else {
		products, productErr = productRepo.GetUserProductsBulk(userID, -1)
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

// ProductsArchived renders the archived products page
// @Summary      Archived products page
// @Description  Renders the archived products list page
// @Tags         web
// @Produce      html
// @Success      200  {string}  html
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /web/products/archived [get]
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

	productRepo := database.NewProductRepository(dbHandle)
	archivedProducts, productBulkErr := productRepo.GetUserArchivedProductsBulk(userID, -1)
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

// ProductsCreate renders the product creation page
// @Summary      Create product page
// @Description  Renders the page for creating a new product
// @Tags         web
// @Produce      html
// @Success      200  {string}  html
// @Router       /web/products/create [get]
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

// ProductsView renders the product view page
// @Summary      Product view page
// @Description  Renders the product details page
// @Tags         web
// @Produce      html
// @Param        id   path      int  true  "Product ID"
// @Success      200  {string}  html
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /web/products/{id}/view [get]
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

	productRepo := database.NewProductRepository(dbHandle)
	product, productErr := productRepo.GetProductByID(productID, userID)
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

// ProductsEdit renders the product edit page
// @Summary      Product edit page
// @Description  Renders the page for editing a product
// @Tags         web
// @Produce      html
// @Param        id   path      int  true  "Product ID"
// @Success      200  {string}  html
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /web/products/{id}/edit [get]
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

	productRepo := database.NewProductRepository(dbHandle)
	product, productErr := productRepo.GetProductByID(productID, userID)
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
// @Summary      Accept invitation page
// @Description  Renders the page for accepting a household invitation
// @Tags         web
// @Produce      html
// @Param        token  query  string  false  "Invitation token"
// @Success      200    {string}  html
// @Failure      400    {object}  api.APIResponse
// @Failure      404    {object}  api.APIResponse
// @Failure      410    {object}  api.APIResponse
// @Failure      500    {object}  api.APIResponse
// @Router       /web/invite/accept [get]
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

	invitationRepo := database.NewInvitationRepository(dbHandle)
	householdRepo := database.NewHouseholdRepository(dbHandle)

	invitation, invErr := invitationRepo.GetInvitationByToken(token)
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

	if invitation.Status != dbModel.InvitationStatusPending {
		var msg string
		switch invitation.Status {
		case dbModel.InvitationStatusCancelled:
			msg = "This invitation has been cancelled by the sender."
		case dbModel.InvitationStatusExpired:
			msg = "This invitation has expired."
		default:
			msg = "This invitation has already been used."
		}
		templates.Render(ctx, frontend.TemplateCache, http.StatusGone, "base", "acceptInvite.tmpl", map[string]any{
			"Title": "Accept Invitation",
			"Error": msg,
		})
		return
	}

	if time.Now().After(invitation.ExpiresAt) {
		_ = dbHandle.Model(&dbModel.HouseholdInvitation{}).Where("id = ?", invitation.ID).Update("status", dbModel.InvitationStatusExpired)
		templates.Render(ctx, frontend.TemplateCache, http.StatusGone, "base", "acceptInvite.tmpl", map[string]any{
			"Title": "Accept Invitation",
			"Error": "This invitation has expired.",
		})
		return
	}

	household, householdErr := householdRepo.GetHouseholdByID(invitation.HouseholdID)
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

// VerifyEmail renders the email verification page
// @Summary      Verify email page
// @Description  Renders the email verification status page
// @Tags         web
// @Produce      html
// @Param        token  query  string  false  "Verification token"
// @Success      200    {string}  html
// @Failure      400    {object}  api.APIResponse
// @Router       /web/verify-email [get]
func (frontend *Frontend) VerifyEmail(ctx *gin.Context) {
	token := ctx.Query("token")

	if token == "" {
		templates.Render(ctx, frontend.TemplateCache, http.StatusBadRequest, "baseAuth", "verifyEmail.tmpl", map[string]any{
			"Title": "Verify Email",
			"Error": "No verification token provided.",
		})
		return
	}

	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "baseAuth", "verifyEmail.tmpl", map[string]any{
		"Title":     "Verify Email",
		"Verifying": true,
		"Token":     token,
	})
}

// Onboarding renders the post-signup onboarding wizard
// @Summary      Onboarding page
// @Description  Renders the onboarding wizard for new users
// @Tags         web
// @Produce      html
// @Success      200  {string}  html
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /web/onboarding [get]
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

	userRepo := database.NewUserRepository(dbHandle)
	user, userErr := userRepo.GetUserByID(userID)
	if userErr != nil {
		logger.Error().Msg(api.ResponseErrInvalidUserData.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrInvalidUserData.Error())
		return
	}

	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "baseAuth", "onboarding.tmpl", map[string]any{
		"Title":       "Onboarding",
		"Username":    user.Username,
		"DisplayName": user.DisplayName,
	})
}
