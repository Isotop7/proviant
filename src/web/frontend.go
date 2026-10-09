package web

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/api"
	v1 "codeberg.org/isotop7/proviant/api/v1"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
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

func (frontend *Frontend) mustGetPageContext(ctx *gin.Context) (*zerolog.Logger, *database.RepositoryContainer, uint, bool) {
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserIDFromToken.Error())
		return nil, nil, 0, false
	}
	repos, ok := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrDatabaseContextNotFound)
		return nil, nil, 0, false
	}
	return logger, repos, userID, true
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
	logger, repos, userID, ok := frontend.mustGetPageContext(ctx)
	if !ok {
		return
	}

	var hasHousehold bool
	userHouseholdID, userErr := repos.Users.GetUserHouseholdByID(userID)
	if userErr != nil {
		logger.Error().Msg(userErr.Error())
	}
	hasHousehold = userHouseholdID > 0

	demoMode := false
	if cfgVal, ok := ctx.Get(util.ContextKeyProviantConfig); ok {
		if cfg, ok := cfgVal.(*configuration.ProviantConfiguration); ok {
			demoMode = cfg.Server.DemoMode
		}
	}

	// Setup page data
	pageData := map[string]any{
		"InviteToken":  ctx.Query("invite_token"),
		"Title":        "Home",
		"HasHousehold": hasHousehold,
		"Household":    userHouseholdID,
		"DemoMode":     demoMode,
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
	minLength := 12
	requireUppercase := false
	requireDigit := false
	requireSpecial := false

	if cfgVal, ok := ctx.Get(util.ContextKeyProviantConfig); ok {
		if cfg, ok := cfgVal.(*configuration.ProviantConfiguration); ok {
			minLength = cfg.Server.Authentication.PasswordMinLength
			requireUppercase = cfg.Server.Authentication.PasswordRequireUppercase
			requireDigit = cfg.Server.Authentication.PasswordRequireDigit
			requireSpecial = cfg.Server.Authentication.PasswordRequireSpecial
		}
	}

	pageData := map[string]any{
		"InviteToken":              ctx.Query("invite_token"),
		"Title":                    "Authentication",
		"PasswordMinLength":        minLength,
		"PasswordRequireUppercase": requireUppercase,
		"PasswordRequireDigit":     requireDigit,
		"PasswordRequireSpecial":   requireSpecial,
		"RouteForgotPassword":      util.RouteForgotPassword,
		"BrandHeadline":            authBrandHeadline,
		"BrandSub":                 authBrandSub,
		"BrandFeatures":            authBrandFeatures,
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
	logger, repos, userID, ok := frontend.mustGetPageContext(ctx)
	if !ok {
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

	minLength := 12
	requireUppercase := false
	requireDigit := false
	requireSpecial := false

	receiptScanFeatureEnabled := false
	receiptScanDefaultTimeout := util.ReceiptScanDefaultTimeout
	receiptScanDefaults := map[string]any{
		"Endpoint":         "",
		"Model":            "",
		"Timeout":          receiptScanDefaultTimeout,
		"APIKeyConfigured": false,
	}

	if cfgVal, ok := ctx.Get(util.ContextKeyProviantConfig); ok {
		if cfg, ok := cfgVal.(*configuration.ProviantConfiguration); ok {
			minLength = cfg.Server.Authentication.PasswordMinLength
			requireUppercase = cfg.Server.Authentication.PasswordRequireUppercase
			requireDigit = cfg.Server.Authentication.PasswordRequireDigit
			requireSpecial = cfg.Server.Authentication.PasswordRequireSpecial
			receiptScanFeatureEnabled = cfg.OCR.Receipt.Enabled && strings.TrimSpace(cfg.OCR.Receipt.Model) != ""
			receiptScanDefaultTimeout = v1.ReceiptScanEffectiveTimeout(cfg.OCR.Receipt.Timeout)
			receiptScanDefaults = map[string]any{
				"Endpoint":         cfg.OCR.Receipt.Endpoint,
				"Model":            cfg.OCR.Receipt.Model,
				"Timeout":          receiptScanDefaultTimeout,
				"APIKeyConfigured": cfg.OCR.Receipt.APIKey != "",
			}
		}
	}

	pageData := map[string]any{
		"InviteToken":               ctx.Query("invite_token"),
		"Title":                     "User Settings",
		"User":                      user,
		"Household":                 household,
		"IsAdmin":                   isAdmin,
		"ShowAuditLog":              isAdmin,
		"Members":                   members,
		"PendingApplications":       pendingApplications,
		"MyApplications":            myApplications,
		"TelegramConfigured":        telegramConfigured,
		"Locations":                 locations,
		"PasswordMinLength":         minLength,
		"PasswordRequireUppercase":  requireUppercase,
		"PasswordRequireDigit":      requireDigit,
		"PasswordRequireSpecial":    requireSpecial,
		"ReceiptScanFeatureEnabled": receiptScanFeatureEnabled,
		"ReceiptScanDefaults":       receiptScanDefaults,
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

// productsPageSize bounds how many full rows one products-page request
// materialises. The candidate list is still every matching row — the
// effective-expiry order cannot be expressed in portable SQL — but only this
// many rows are fetched whole and rendered.
const productsPageSize = 50

// fetchProductProjections returns the candidate rows for the products page,
// narrowed to productProjectionColumns. Nothing here is a page yet: the caller
// classifies, filters, orders, slices one page out of the result and hydrates
// just those rows.
//
// The search path keeps its SQL ORDER BY (the caller picked that column); every
// other path comes back in id order for the caller to sort.
func fetchProductProjections(
	repos *database.RepositoryContainer,
	userID uint,
	params *productQueryParams,
) ([]dbModel.Product, error) {
	if params.statusFilter == "archived" {
		return repos.Products.GetArchivedProductProjections(userID)
	}
	switch {
	case params.locationFilter != "":
		locationID, err := strconv.ParseUint(params.locationFilter, 10, 64)
		if err != nil {
			return nil, nil
		}
		return repos.Products.GetActiveProductProjections(userID, uint(locationID)) //nolint:gosec
	case params.queryParam != "" && params.queryValue != "":
		enumParam := database.SearchParameterEnumFromString(params.queryParam)
		if enumParam == database.InvalidParameter {
			return nil, errors.ErrProductSearchInvalidQuery
		}
		return repos.Products.SearchProductProjections(enumParam, params.queryValue, params.sort, params.order, userID)
	default:
		return repos.Products.GetActiveProductProjections(userID, 0)
	}
}

// renderProductsFetchError writes the response for a failed products fetch and
// reports whether it did, so the caller can return immediately. It is shared by
// the projection pass and the hydration pass because they fail the same way for
// the user.
func renderProductsFetchError(ctx *gin.Context, frontend *Frontend, logger *zerolog.Logger, err error) bool {
	switch err {
	case nil:
		return false
	case errors.ErrProductSearchInvalidQuery:
		logger.Error().Msg(err.Error())
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, err.Error())
		return true
	case errors.ErrDatabaseInvalidSortParameter:
		logger.Warn().Msg(err.Error())
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, err.Error())
		return true
	default:
		logger.Error().Msgf("Error getting products of user: %s", err)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrUserNoProductsFound.Error())
		return true
	}
}

// pageURL renders the current filters with the page number swapped in. The
// value slices are shared with params on purpose: only the page key is added,
// and Set replaces that entry rather than touching any other key's slice.
func pageURL(params url.Values, page int) string {
	paged := make(url.Values, len(params)+1)
	for key, value := range params {
		paged[key] = value
	}
	paged.Set("page", strconv.Itoa(page))
	return "?" + paged.Encode()
}

// hydrateProductsPage fetches the full rows for the chosen page IDs and returns
// them in exactly that order. The projection pass has already decided the
// ordering (SQL for a search, effective expiry otherwise) while the hydration
// query orders by id, so the caller's order has to be restored rather than
// assumed. A row that disappeared between the two queries is dropped instead of
// shifting the rest of the page.
func hydrateProductsPage(repos *database.RepositoryContainer, userID uint, ids []uint, archived bool) ([]dbModel.Product, error) {
	if len(ids) == 0 {
		return []dbModel.Product{}, nil
	}

	var rows []dbModel.Product
	var err error
	if archived {
		rows, err = repos.Products.GetUserArchivedProductsByIDs(userID, ids)
	} else {
		rows, err = repos.Products.GetUserProductsByIDs(userID, ids)
	}
	if err != nil {
		return nil, err
	}

	byID := make(map[uint]dbModel.Product, len(rows))
	for i := range rows {
		byID[rows[i].ID] = rows[i]
	}
	ordered := make([]dbModel.Product, 0, len(ids))
	for _, id := range ids {
		if row, ok := byID[id]; ok {
			ordered = append(ordered, row)
		}
	}
	return ordered, nil
}

// sliceProductPage picks the page of candidate rows one request renders: the
// ordered IDs to hydrate, the 1-based page actually shown, and how many pages
// the candidate list has in total.
//
// The requested page is clamped into range: a stale link or a list that shrank
// since it was written lands on a valid page instead of rendering an empty one
// that reads like data loss.
func sliceProductPage(candidates []dbModel.Product, rawPage string) (ids []uint, page, totalPages int) {
	totalPages = (len(candidates) + productsPageSize - 1) / productsPageSize
	if totalPages < 1 {
		totalPages = 1
	}

	page = 1
	if requested, err := strconv.Atoi(rawPage); err == nil && requested > 1 {
		page = requested
	}
	if page > totalPages {
		page = totalPages
	}

	start := (page - 1) * productsPageSize
	end := start + productsPageSize
	if end > len(candidates) {
		end = len(candidates)
	}
	ids = make([]uint, 0, end-start)
	for i := start; i < end; i++ {
		ids = append(ids, candidates[i].ID)
	}
	return ids, page, totalPages
}

// The expiry window boundaries use calendar-day arithmetic (AddDate) to stay
// consistent with GetActiveExpiryCounts, which classifies the same products
// for the sidebar badge on the archived view.
func productExpiryStatus(p *dbModel.Product, now time.Time, criticalDays, soonDays int) string {
	effective := p.EffectiveExpireAt()
	switch {
	case effective.IsZero():
		return "nodate"
	case effective.Before(now):
		return "expired"
	case effective.Before(now.AddDate(0, 0, criticalDays)):
		return "critical"
	case effective.Before(now.AddDate(0, 0, soonDays)):
		return "soon"
	default:
		return "fresh"
	}
}

func computeExpiryStats(products []dbModel.Product, now time.Time, criticalDays int) (expired, critical int) {
	for i := range products {
		p := &products[i]
		effective := p.EffectiveExpireAt()
		if !effective.IsZero() && effective.Before(now) {
			expired++
		} else if !effective.IsZero() && effective.Before(now.AddDate(0, 0, criticalDays)) {
			critical++
		}
	}
	return
}

func filterProductsByStatus(products []dbModel.Product, status string, now time.Time, criticalDays, soonDays int) []dbModel.Product {
	filtered := make([]dbModel.Product, 0, len(products))
	for i := range products {
		p := &products[i]
		if productExpiryStatus(p, now, criticalDays, soonDays) == status {
			filtered = append(filtered, *p)
		}
	}
	return filtered
}

func (frontend *Frontend) Products(ctx *gin.Context) {
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusInternalServerError, errors.ErrDatabaseContextNotFound.Error())
		return
	}

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusUnauthorized, errors.ErrUserIDFromToken.Error())
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

	productQuery := &productQueryParams{
		statusFilter:   statusFilter,
		locationFilter: locationFilter,
		queryParam:     queryParam,
		queryValue:     queryValue,
		sort:           sort,
		order:          order,
	}
	projections, productErr := fetchProductProjections(repos, userID, productQuery)
	if renderProductsFetchError(ctx, frontend, logger, productErr) {
		return
	}

	now := time.Now()

	proviantConfig, _ := ctx.MustGet(util.ContextKeyProviantConfig).(*configuration.ProviantConfiguration)
	if proviantConfig == nil {
		logger.Error().Msg("failed to get proviant configuration from context")
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusInternalServerError, "Internal error")
		return
	}
	criticalDays := proviantConfig.Expiry.CriticalThresholdDays
	if criticalDays <= 0 {
		criticalDays = 3
	}
	receiptScanEnabled := proviantConfig.OCR.Receipt.Enabled && strings.TrimSpace(proviantConfig.OCR.Receipt.Model) != ""
	soonDays := proviantConfig.Expiry.SoonThresholdDays
	if soonDays <= 0 {
		soonDays = 7
	}

	// The sidebar badge counts expiring stock, so it must always be derived from
	// the active list. On the archived view the rendered rows are all consumed
	// and therefore all past their expiry date, which would badge every one of
	// them as urgent, so that view asks the repository for the counts directly
	// instead of loading the active rows it would immediately throw away.
	var expiredCount, criticalCount int
	if statusFilter == "archived" {
		if archivedExpired, archivedCritical, statsErr := repos.Products.GetActiveExpiryCounts(userID, now, criticalDays); statsErr != nil {
			logger.Warn().Msgf("Error getting active product expiry stats: %s", statsErr)
		} else {
			expiredCount, criticalCount = archivedExpired, archivedCritical
		}
	} else {
		expiredCount, criticalCount = computeExpiryStats(projections, now, criticalDays)
	}

	// ProductCount keeps its old meaning: every row the view selected, before the
	// status filter narrows it to what is on screen.
	totalMatched := len(projections)

	if statusFilter != "all" && statusFilter != "archived" {
		projections = filterProductsByStatus(projections, statusFilter, now, criticalDays, soonDays)
	}

	// A search arrives already ordered by the column the caller picked. Every
	// other active view is ordered by effective expiry; the archived view keeps
	// the id order its projection came back in.
	isSearch := productQuery.queryParam != "" && productQuery.queryValue != ""
	if statusFilter != "archived" && !isSearch {
		database.SortByEffectiveExpiry(projections)
	}

	pageIDs, page, totalPages := sliceProductPage(projections, ctx.Query("page"))

	products, hydrateErr := hydrateProductsPage(repos, userID, pageIDs, statusFilter == "archived")
	if renderProductsFetchError(ctx, frontend, logger, hydrateErr) {
		return
	}

	params := url.Values{}
	for k, v := range ctx.Request.URL.Query() {
		params[k] = v
	}
	// Filter and view links start over at page 1: carrying ?page= along would
	// mean a page number that no longer refers to anything under the new filter.
	delete(params, "page")

	// Pagination links keep every current filter and swap only the page number.
	var prevPageURL, nextPageURL string
	if page > 1 {
		prevPageURL = pageURL(params, page-1)
	}
	if page < totalPages {
		nextPageURL = pageURL(params, page+1)
	}

	pageData := map[string]any{
		"InviteToken":        ctx.Query("invite_token"),
		"Title":              "Products",
		"Products":           products,
		"QueryParam":         queryParam,
		"QueryValue":         queryValue,
		"Sort":               sort,
		"Order":              order,
		"Locations":          locations,
		"StorageLocations":   locations,
		"LocationFilter":     locationFilter,
		"View":               view,
		"StatusFilter":       statusFilter,
		"ProductCount":       totalMatched,
		"ExpiredCount":       expiredCount,
		"CriticalCount":      criticalCount,
		"UrgentCount":        expiredCount + criticalCount,
		"Params":             params,
		"Page":               page,
		"TotalPages":         totalPages,
		"PrevPageURL":        prevPageURL,
		"NextPageURL":        nextPageURL,
		"CurrentUserID":      userID,
		"ReceiptScanEnabled": receiptScanEnabled,
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

// ProductsScanReceipt renders the receipt photo scan page (issue #61).
// Storage locations are prefetched server-side so the review form can offer
// the household's locations without an extra API round-trip.
// @Summary      Receipt scan page
// @Description  Renders the receipt photo scanning page
// @Tags         web
// @Produce      html
// @Success      200  {string}  html
// @Failure      500  {object}  api.APIResponse
// @Router       /web/products/scan-receipt [get]
func (frontend *Frontend) ProductsScanReceipt(ctx *gin.Context) {
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	// Gate on the feature flag server-side: without this a user with the URL
	// uploads a photo and only then hits a 403 from the scan API. Mirrors the
	// products-page entry-point condition, including the model check — with
	// enabled:true but an empty model the page would render and every scan
	// would then fail at request time.
	proviantConfig, _ := ctx.MustGet(util.ContextKeyProviantConfig).(*configuration.ProviantConfiguration)
	if proviantConfig == nil || !proviantConfig.OCR.Receipt.Enabled || strings.TrimSpace(proviantConfig.OCR.Receipt.Model) == "" {
		ctx.Redirect(http.StatusFound, "/web/products")
		return
	}

	repos, ok := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(errors.ErrDatabaseContextNotFound.Error())
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusInternalServerError, "Internal error")
		return
	}
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusUnauthorized, errors.ErrUserIDFromToken.Error())
		return
	}

	locations, locErr := repos.StorageLocations.GetByHousehold(userID)
	if locErr != nil {
		logger.Warn().Msgf("Receipt scan page: storage locations: %s", locErr)
	}

	// ResolveReceiptScanConfiguration already folds in the configured default,
	// and the else branch returns, so there is nothing to assign up front.
	var scanTimeoutSeconds int
	if user, userErr := repos.Users.GetUserByID(userID); userErr == nil {
		effective := v1.ResolveReceiptScanConfiguration(&proviantConfig.OCR.Receipt, user.ReceiptScanPreferences)
		scanTimeoutSeconds = v1.ReceiptScanEffectiveTimeout(effective.Timeout)
	} else {
		logger.Error().Msgf("Receipt scan page: failed to load user settings: %s", userErr)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusInternalServerError, errors.ErrInternalServer.Error())
		return
	}

	pageData := map[string]any{
		"Title":              "Scan Receipt",
		"StorageLocations":   locations,
		"MaxItems":           util.ReceiptBulkMaxItems,
		"ScanTimeoutSeconds": scanTimeoutSeconds,
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "productsScanReceipt.tmpl", pageData)
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
	var err error
	if productIDRaw, err = strconv.ParseUint(idParam, 10, 64); err != nil {
		logger.Warn().Msgf(errors.FormatInvalidRequestId, idParam)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, err.Error())
		return
	}

	// Get database instance from context
	repos, ok := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusInternalServerError, errors.ErrDatabaseContextNotFound.Error())
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusUnauthorized, errors.ErrUserIDFromToken.Error())
		return
	}

	product, productErr := repos.Products.GetArchivedProductByID(uint(productIDRaw), userID) //nolint:gosec
	if productErr != nil {
		logger.Error().Msgf("Error getting product: %s", productErr)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusNotFound, errors.ErrUserNoProductsFound.Error())
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
	var err error
	if productIDRaw, err = strconv.ParseUint(idParam, 10, 64); err != nil {
		logger.Warn().Msgf(errors.FormatInvalidRequestId, idParam)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, err.Error())
		return
	}

	// Get database instance from context
	repos, ok := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
	if !ok {
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusInternalServerError, errors.ErrDatabaseContextNotFound.Error())
		return
	}

	// Extract JWT claims from context
	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims[static.TokenIdentityKey].(float64))
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusUnauthorized, errors.ErrUserIDFromToken.Error())
		return
	}

	product, productErr := repos.Products.GetArchivedProductByID(uint(productIDRaw), userID) //nolint:gosec
	if productErr != nil {
		logger.Error().Msgf("Error getting product: %s", productErr)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusNotFound, errors.ErrUserNoProductsFound.Error())
		return
	}

	user, userErr := repos.Users.GetUserByID(userID)
	if userErr != nil {
		logger.Error().Msgf("Error getting user: %s", userErr)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusBadRequest, errors.ErrInvalidUserData.Error())
		return
	}

	locations, _ := repos.StorageLocations.GetByHousehold(userID)

	pageData := map[string]any{
		"InviteToken":                     ctx.Query("invite_token"),
		"Title":                           "Products",
		"Product":                         product,
		"Locations":                       locations,
		"IsArchived":                      product.DeletedAt.Valid,
		"GlobalNotificationThresholdDays": user.NotificationPreferences.NotificationThresholdDays,
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
	logger, repos, userID, ok := frontend.mustGetPageContext(ctx)
	if !ok {
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
	_, repos, userID, ok := frontend.mustGetPageContext(ctx)
	if !ok {
		return
	}

	householdID, err := repos.Users.GetUserHouseholdByID(userID)
	if err != nil || householdID == 0 {
		householdID = 0
	}

	pageData := map[string]any{
		"Title":        "Recipes",
		"HasHousehold": householdID > 0,
		"HouseholdID":  householdID,
	}

	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "recipes.tmpl", pageData)
}

// WasteAnalytics renders the waste analytics dashboard
// @Summary      Waste Analytics page
// @Description  Renders consumed-vs-wasted metrics, monthly breakdown, and most-wasted categories
// @Tags         web
// @Produce      html
// @Success      200  {string}  html
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /web/waste-analytics [get]
func (frontend *Frontend) WasteAnalytics(ctx *gin.Context) {
	_, repos, userID, ok := frontend.mustGetPageContext(ctx)
	if !ok {
		return
	}

	householdID, err := repos.Users.GetUserHouseholdByID(userID)
	if err != nil || householdID == 0 {
		householdID = 0
	}

	pageData := map[string]any{
		"Title":        "Waste Analytics",
		"HasHousehold": householdID > 0,
		"HouseholdID":  householdID,
	}

	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "wasteAnalytics.tmpl", pageData)
}

// ShoppingList renders the shopping list page
// @Summary      Shopping List page
// @Description  Renders the shared household shopping list with custom items and import banner
// @Tags         web
// @Produce      html
// @Success      200  {string}  html
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /web/shopping-list [get]
func (frontend *Frontend) ShoppingList(ctx *gin.Context) {
	logger, repos, userID, ok := frontend.mustGetPageContext(ctx)
	if !ok {
		return
	}

	householdID, err := repos.Users.GetUserHouseholdByID(userID)
	if err != nil {
		logger.Error().Msgf("Error getting household: %s", err)
		templates.RenderError(ctx, frontend.TemplateCache, http.StatusInternalServerError, "Error loading shopping list")
		return
	}

	items, _ := repos.ShoppingListItems.ListByHousehold(householdID)

	autoProducts, _ := repos.Products.GetSubThresholdProducts(userID)

	autoProductIDs := make(map[uint]bool)
	for i := range items {
		if items[i].ProductID != nil {
			autoProductIDs[*items[i].ProductID] = true
		}
	}

	var filteredAutoProducts []dbModel.Product
	for i := range autoProducts {
		if !autoProductIDs[autoProducts[i].ID] {
			filteredAutoProducts = append(filteredAutoProducts, autoProducts[i])
		}
	}

	unchecked := make([]dbModel.ShoppingListItem, 0)
	checked := make([]dbModel.ShoppingListItem, 0)
	for i := range items {
		if items[i].Checked {
			checked = append(checked, items[i])
		} else {
			unchecked = append(unchecked, items[i])
		}
	}

	sort.Slice(unchecked, func(i, j int) bool {
		return unchecked[i].Name < unchecked[j].Name
	})
	sort.Slice(checked, func(i, j int) bool {
		return checked[i].Name < checked[j].Name
	})

	unchecked = append(unchecked, checked...)

	pageData := map[string]any{
		"InviteToken":   ctx.Query("invite_token"),
		"Title":         "Shopping List",
		"Items":         items,
		"ItemsSorted":   unchecked,
		"ItemCount":     len(items),
		"AutoItems":     filteredAutoProducts,
		"AutoItemCount": len(filteredAutoProducts),
		"HasAutoItems":  len(filteredAutoProducts) > 0,
		"ItemsJSON":     toJSON(unchecked),
	}
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "shoppingList.tmpl", pageData)
}

func toJSON(v any) string {
	buf := &bytes.Buffer{}
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(true)
	_ = enc.Encode(v)
	return strings.TrimSpace(buf.String())
}

// Unsubscribe handles one-click unsubscribe from email digests.
// @Summary      Unsubscribe from email digests
// @Description  Handles unsubscribe token and disables digest for user
// @Tags         web
// @Produce      html
// @Param        token  query  string  true  "Unsubscribe token"
// @Success      200    {string}  html
// @Failure      400    {object}  api.APIResponse
// @Failure      404    {object}  api.APIResponse
// @Router       /web/unsubscribe [get]
func (frontend *Frontend) Unsubscribe(ctx *gin.Context) {
	token := ctx.Query("token")
	if token == "" {
		templates.Render(ctx, frontend.TemplateCache, http.StatusBadRequest, "baseAuth", "unsubscribe.tmpl", map[string]any{
			"Title": "Unsubscribe",
			"Error": "No unsubscribe token provided.",
		})
		return
	}

	repos, ok := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
	if !ok {
		logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)
		logger.Error().Msg(api.ResponseErrDatabaseContextNotFound.Message)
		templates.Render(ctx, frontend.TemplateCache, http.StatusInternalServerError, "baseAuth", "unsubscribe.tmpl", map[string]any{
			"Title": "Unsubscribe",
			"Error": "Internal server error.",
		})
		return
	}

	user, userErr := repos.Notifications.GetUserByMailDigestUnsubscribeToken(token)
	if userErr != nil {
		templates.Render(ctx, frontend.TemplateCache, http.StatusNotFound, "baseAuth", "unsubscribe.tmpl", map[string]any{
			"Title": "Unsubscribe",
			"Error": "Invalid or expired unsubscribe link.",
		})
		return
	}

	if err := repos.Notifications.DeleteMailDigestUnsubscribeToken(token); err != nil {
		logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)
		logger.Warn().Msgf("Unsubscribe: failed to delete token: %s", err)
	}

	user.NotificationPreferences.MailDigestFrequency = authentication.MailDigestFrequencyDisabled
	if updateErr := repos.Users.UpdateUser(user.ID, &user); updateErr != nil {
		logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)
		logger.Error().Msgf("Unsubscribe: failed to update user: %s", updateErr)
		templates.Render(ctx, frontend.TemplateCache, http.StatusInternalServerError, "baseAuth", "unsubscribe.tmpl", map[string]any{
			"Title": "Unsubscribe",
			"Error": "Failed to update preferences.",
		})
		return
	}

	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "baseAuth", "unsubscribe.tmpl", map[string]any{
		"Title":         "Unsubscribe",
		"Success":       true,
		"HouseholdName": user.Household.Name,
	})
}

// ForgotPassword renders the forgot-password page (form to request a reset link).
// @Summary      Forgot password page
// @Description  Renders the page that lets users request a password reset link via email.
// @Tags         web
// @Produce      html
// @Success      200  {string}  html
// @Router       /web/forgot-password [get]
func (frontend *Frontend) ForgotPassword(ctx *gin.Context) {
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "baseAuth", "forgotPassword.tmpl", map[string]any{
		"Title":               "Forgot Password",
		"RouteForgotPassword": util.RouteForgotPassword,
		"RouteAuth":           util.RouteAuth,
		"BrandHeadline":       forgotPasswordBrandHeadline,
		"BrandSub":            forgotPasswordBrandSub,
		"BrandFeatures":       forgotPasswordBrandFeatures,
	})
}

// ResetPassword renders the password reset page (form to set a new password via token).
// @Summary      Reset password page
// @Description  Renders the page that lets users set a new password using a reset token.
// @Tags         web
// @Produce      html
// @Param        token  query  string  false  "Reset token"
// @Success      200    {string}  html
// @Router       /web/reset-password [get]
func (frontend *Frontend) ResetPassword(ctx *gin.Context) {
	token := ctx.Query("token")

	data := map[string]any{
		"Title":               "Reset Password",
		"RouteAuth":           util.RouteAuth,
		"RouteForgotPassword": util.RouteForgotPassword,
		"BrandHeadline":       resetPasswordBrandHeadline,
		"BrandSub":            resetPasswordBrandSub,
		"BrandFeatures":       resetPasswordBrandFeatures,
	}

	if token == "" {
		data["Error"] = "Please request a new reset link to continue."
	} else {
		data["Token"] = token
	}

	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "baseAuth", "resetPassword.tmpl", data)
}

// BrandFeature is a single icon+text row in the brand panel of the
// split-panel auth pages. Consumed by the partials/loginBrand.tmpl template.
type BrandFeature struct {
	Icon string
	Text string
}

var (
	authBrandHeadline = template.HTML("Less waste.<br>More meals.")
	authBrandSub      = "Track what's in your pantry, get notified before things expire, and discover recipes with what you already have."
	authBrandFeatures = []BrandFeature{
		{Icon: "bi-upc-scan", Text: "Scan barcodes to add products instantly"},
		{Icon: "bi-bell", Text: "Get notified before items expire"},
		{Icon: "bi-journal-richtext", Text: "Recipe suggestions from your pantry"},
		{Icon: "bi-graph-down-arrow", Text: "Track savings and reduce food waste"},
	}
	forgotPasswordBrandHeadline = "Forgot your password?"
	forgotPasswordBrandSub      = "Enter your email and we'll send you a link to choose a new password."
	forgotPasswordBrandFeatures = []BrandFeature{
		{Icon: "bi-shield-lock", Text: "Your account stays protected"},
		{Icon: "bi-envelope", Text: "A secure reset link in your inbox"},
		{Icon: "bi-arrow-counterclockwise", Text: "Back to your pantry in minutes"},
	}
	resetPasswordBrandHeadline = "Choose a new password"
	resetPasswordBrandSub      = "Pick something strong. We'll get you back to your pantry in no time."
	resetPasswordBrandFeatures = []BrandFeature{
		{Icon: "bi-shield-check", Text: "Encrypted in transit and at rest"},
		{Icon: "bi-key", Text: "12+ characters keeps it strong"},
		{Icon: "bi-check2-circle", Text: "Then you're back in"},
	}
)
