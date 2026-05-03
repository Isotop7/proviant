package web

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/templates"
	"codeberg.org/isotop7/proviant/util"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

type Frontend struct {
	TemplateCache map[string]*template.Template
}

const (
	AcceptInviteFileName  = "acceptInvite.tmpl"
	AcceptInvitationTitle = "Accept Invitation"
)

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
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	// Extract user id
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserIDFromToken.Error())
		return
	}

	// Get database instance from context
	repos, ok := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	var hasHousehold bool
	userHouseholdID, userErr := repos.Users.GetUserHouseholdByID(userID)
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
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	// Extract user id
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserIDFromToken.Error())
		return
	}

	// Get database instance from context
	repos, ok := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	user, userErr := repos.Users.GetUserByID(userID)
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

	household, householdErr := repos.Households.GetHouseholdByID(user.HouseholdID)
	if householdErr != nil {
		logger.Error().Msg(api.ResponseErrInvalidUserData.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrInvalidUserData.Error())
		return
	}

	isAdmin := household.AdminID == userID

	members, _ := repos.Households.GetHouseholdMembers(user.HouseholdID)

	pendingApplications, _ := repos.Households.GetPendingApplicationsForAdmin(userID)

	myApplications, _ := repos.Households.GetPendingApplicationsForApplicant(userID)

	locations, _ := repos.StorageLocations.GetByHousehold(userID)

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
		"Locations":           locations,
	}

	if isAdmin {
		invitations, _ := repos.Invitations.GetInvitationsForHousehold(user.HouseholdID, userID)
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

type productQueryParams struct {
	statusFilter   string
	locationFilter string
	queryParam     string
	queryValue     string
	sort           string
	order          string
}

func fetchProducts(
	repos *database.RepositoryContainer,
	userID uint,
	params *productQueryParams,
) ([]dbModel.Product, error) {
	if params.statusFilter == "archived" {
		return repos.Products.GetUserArchivedProductsBulk(userID, -1)
	}
	switch {
	case params.locationFilter != "":
		locationID, parseErr := strconv.ParseUint(params.locationFilter, 10, 64)
		if parseErr != nil {
			return nil, nil
		}
		return repos.Products.GetUserProductsByLocation(userID, uint(locationID)) //nolint:gosec
	case params.queryParam != "" && params.queryValue != "":
		enumParam := database.SearchParameterEnumFromString(params.queryParam)
		if enumParam == database.InvalidParameter {
			return nil, errors.ErrProductSearchInvalidQuery
		}
		return repos.Products.SearchProducts(enumParam, params.queryValue, params.sort, params.order, userID)
	default:
		return repos.Products.GetUserProductsBulk(userID, -1)
	}
}

func productExpiryStatus(p *dbModel.Product, now time.Time, criticalDur, soonDur time.Duration) string {
	switch {
	case p.ExpireAt.IsZero():
		return "nodate"
	case p.ExpireAt.Before(now):
		return "expired"
	case p.ExpireAt.Before(now.Add(criticalDur)):
		return "critical"
	case p.ExpireAt.Before(now.Add(soonDur)):
		return "soon"
	default:
		return "fresh"
	}
}

func computeExpiryStats(products []dbModel.Product, now time.Time, criticalDur time.Duration) (expired, critical int) {
	for i := range products {
		p := &products[i]
		if !p.ExpireAt.IsZero() && p.ExpireAt.Before(now) {
			expired++
		} else if !p.ExpireAt.IsZero() && p.ExpireAt.Before(now.Add(criticalDur)) {
			critical++
		}
	}
	return
}

func filterProductsByStatus(products []dbModel.Product, status string, now time.Time, criticalDur, soonDur time.Duration) []dbModel.Product {
	filtered := make([]dbModel.Product, 0, len(products))
	for i := range products {
		p := &products[i]
		if productExpiryStatus(p, now, criticalDur, soonDur) == status {
			filtered = append(filtered, *p)
		}
	}
	return filtered
}

func (frontend *Frontend) Products(ctx *gin.Context) {
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, dbErr := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
	if !dbErr {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrDatabaseContextNotFound.Error())
		return
	}

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserIDFromToken.Error())
		return
	}

	queryParam := ctx.Query("queryParam")
	queryValue := ctx.Query("queryValue")
	sort := ctx.DefaultQuery("sort", "created_at")
	order := ctx.DefaultQuery("order", "asc")
	locationFilter := ctx.Query("locationId")
	view := ctx.DefaultQuery("view", "list")
	statusFilter := ctx.DefaultQuery("status", "all")

	locations, _ := repos.StorageLocations.GetByHousehold(userID)

	products, productErr := fetchProducts(repos, userID, &productQueryParams{
		statusFilter:   statusFilter,
		locationFilter: locationFilter,
		queryParam:     queryParam,
		queryValue:     queryValue,
		sort:           sort,
		order:          order,
	})
	if productErr == errors.ErrProductSearchInvalidQuery {
		logger.Error().Msg(productErr.Error())
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, productErr.Error())
		return
	}
	if productErr != nil {
		logger.Error().Msgf("Error getting products of user: %s", productErr)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserNoProductsFound.Error())
		return
	}

	allProducts := products
	now := time.Now()

	proviantConfig, _ := ctx.MustGet(util.ContextKeyProviantConfig).(*configuration.ProviantConfiguration)
	criticalDays := proviantConfig.Expiry.CriticalThresholdDays
	if criticalDays <= 0 {
		criticalDays = 3
	}
	soonDays := proviantConfig.Expiry.SoonThresholdDays
	if soonDays <= 0 {
		soonDays = 7
	}
	criticalDuration := time.Duration(criticalDays) * 24 * time.Hour
	soonDuration := time.Duration(soonDays) * 24 * time.Hour

	expiredCount, criticalCount := computeExpiryStats(allProducts, now, criticalDuration)

	if statusFilter != "all" && statusFilter != "archived" {
		products = filterProductsByStatus(allProducts, statusFilter, now, criticalDuration, soonDuration)
	}

	params := url.Values{}
	for k, v := range ctx.Request.URL.Query() {
		params[k] = v
	}

	pageData := map[string]any{
		"InviteToken":      ctx.Query("invite_token"),
		"Title":            "Products",
		"Products":         products,
		"QueryParam":       queryParam,
		"QueryValue":       queryValue,
		"Sort":             sort,
		"Order":            order,
		"Locations":        locations,
		"StorageLocations": locations,
		"LocationFilter":   locationFilter,
		"View":             view,
		"StatusFilter":     statusFilter,
		"ProductCount":     len(allProducts),
		"ExpiredCount":     expiredCount,
		"CriticalCount":    criticalCount,
		"UrgentCount":      expiredCount + criticalCount,
		"Params":           params,
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "products.tmpl", pageData)
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
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	// Get and parse parameter id
	idParam := ctx.Param("id")
	var productIDRaw uint64
	var convErr error
	if productIDRaw, convErr = strconv.ParseUint(idParam, 10, 64); convErr != nil {
		logger.Warn().Msgf(errors.FormatInvalidRequestId, idParam)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, convErr.Error())
		return
	}

	// Get database instance from context
	repos, dbErr := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
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

	product, productErr := repos.Products.GetArchivedProductByID(uint(productIDRaw), userID) //nolint:gosec
	if productErr != nil {
		logger.Error().Msgf("Error getting product: %s", productErr)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserNoProductsFound.Error())
		return
	}

	pageData := map[string]any{
		"InviteToken": ctx.Query("invite_token"),
		"Title":       "Products",
		"Product":     product,
		"IsArchived":  product.DeletedAt.Valid,
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
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	// Get and parse parameter id
	idParam := ctx.Param("id")
	var productIDRaw uint64
	var convErr error
	if productIDRaw, convErr = strconv.ParseUint(idParam, 10, 64); convErr != nil {
		logger.Warn().Msgf(errors.FormatInvalidRequestId, idParam)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, convErr.Error())
		return
	}

	// Get database instance from context
	repos, dbErr := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
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

	product, productErr := repos.Products.GetArchivedProductByID(uint(productIDRaw), userID) //nolint:gosec
	if productErr != nil {
		logger.Error().Msgf("Error getting product: %s", productErr)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserNoProductsFound.Error())
		return
	}

	locations, _ := repos.StorageLocations.GetByHousehold(userID)

	pageData := map[string]any{
		"InviteToken": ctx.Query("invite_token"),
		"Title":       "Products",
		"Product":     product,
		"Locations":   locations,
		"IsArchived":  product.DeletedAt.Valid,
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
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)
	token := ctx.Query("token")

	if token == "" {
		templates.Render(ctx, frontend.TemplateCache, http.StatusBadRequest, "base", AcceptInviteFileName, map[string]any{
			"Title": AcceptInvitationTitle,
			"Error": "No invitation token provided.",
		})
		return
	}

	repos, ok := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		templates.Render(ctx, frontend.TemplateCache, http.StatusInternalServerError, "base", AcceptInviteFileName, map[string]any{
			"Title": AcceptInvitationTitle,
			"Error": "Internal server error.",
		})
		return
	}

	invitation, invErr := repos.Invitations.GetInvitationByToken(token)
	if invErr != nil {
		if invErr == errors.ErrInvitationNotFound {
			templates.Render(ctx, frontend.TemplateCache, http.StatusNotFound, "base", AcceptInviteFileName, map[string]any{
				"Title": AcceptInvitationTitle,
				"Error": "This invitation does not exist or has been deleted.",
			})
			return
		}
		logger.Error().Msg(invErr.Error())
		templates.Render(ctx, frontend.TemplateCache, http.StatusInternalServerError, "base", AcceptInviteFileName, map[string]any{
			"Title": AcceptInvitationTitle,
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
		templates.Render(ctx, frontend.TemplateCache, http.StatusGone, "base", AcceptInviteFileName, map[string]any{
			"Title": AcceptInvitationTitle,
			"Error": msg,
		})
		return
	}

	if time.Now().After(invitation.ExpiresAt) {
		_ = repos.Invitations.MarkInvitationExpired(invitation.ID)
		templates.Render(ctx, frontend.TemplateCache, http.StatusGone, "base", AcceptInviteFileName, map[string]any{
			"Title": AcceptInvitationTitle,
			"Error": "This invitation has expired.",
		})
		return
	}

	household, householdErr := repos.Households.GetHouseholdByID(invitation.HouseholdID)
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
			templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", AcceptInviteFileName, map[string]any{
				"Title":           AcceptInvitationTitle,
				"ConfirmAccept":   true,
				"Token":           token,
				"HouseholdName":   householdName,
				"InvitationEmail": invitation.Email,
			})
			return
		}
	}

	// User is not logged in — redirect to auth with token
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "baseAuth", AcceptInviteFileName, map[string]any{
		"Title":           AcceptInvitationTitle,
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
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserIDFromToken.Error())
		return
	}

	user, userErr := repos.Users.GetUserByID(userID)
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

// Recipes renders the recipe suggestions page
// @Summary      Recipes page
// @Description  Shows recipe suggestions for expiring products
// @Tags         web
// @Produce      html
// @Success      200  {string}  html
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /web/recipes [get]
func (frontend *Frontend) Recipes(ctx *gin.Context) {
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	claims := jwt.ExtractClaims(ctx)
	userID64, ok := claims[static.TokenIdentityKey].(float64)
	if !ok {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserIDFromToken.Error())
		return
	}
	userID := uint(userID64)

	// Verify user has household (optional, page can show empty state if none)
	repos, ok := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return
	}

	householdID, err := repos.Users.GetUserHouseholdByID(userID)
	if err != nil || householdID == 0 {
		// No household, still render page with empty state (frontend will handle)
		householdID = 0
	}

	pageData := map[string]any{
		"Title":        "Recipes",
		"HasHousehold": householdID > 0,
		"HouseholdID":  householdID,
	}

	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "recipes.tmpl", pageData)
}
