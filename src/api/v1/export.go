// v1 implements version 1 of the proviant API
package v1

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const (
	GetUserActiveProductsFiltered    = "GetUserActiveProductsFiltered: %s"
	fmtGetUserArchivedProductsFilter = "GetUserArchivedProductsFiltered: %s"
	mimeTypeCSV                      = "text/csv"
)

func parseDateRange(ctx *gin.Context) (*time.Time, *time.Time) {
	var from, to *time.Time

	if fromStr := ctx.Query("from"); fromStr != "" {
		if t, err := time.Parse(util.DefaultDateFormatParseStr, fromStr); err == nil {
			from = &t
		}
	}

	if toStr := ctx.Query("to"); toStr != "" {
		if t, err := time.Parse(util.DefaultDateFormatParseStr, toStr); err == nil {
			endOfDay := t.Add(24*time.Hour - time.Nanosecond)
			to = &endOfDay
		}
	}

	return from, to
}

func productToExportRow(p *dbModel.Product) []string {
	expireAt := ""
	if !p.ExpireAt.IsZero() {
		expireAt = p.ExpireAt.Format(util.DefaultDateFormatParseStr)
	}
	addedAt := ""
	if !p.CreatedAt.IsZero() {
		addedAt = p.CreatedAt.Format(util.DefaultDateFormatParseStr)
	}
	return []string{
		p.ProductName,
		p.Barcode,
		fmt.Sprintf("%d", p.Amount),
		p.Unit,
		p.Categories,
		func() string {
			if p.StorageLocation != nil {
				return p.StorageLocation.Name
			}
			return ""
		}(),
		expireAt,
		addedAt,
	}
}

type FullExportResponse struct {
	Household  *FullExportHousehold          `json:"household"`
	Members    []FullExportMember            `json:"members"`
	Products   FullExportProducts            `json:"products"`
	Stats      apiModel.ProductStatsResponse `json:"stats"`
	ExportedAt string                        `json:"exported_at"`
}

type FullExportHousehold struct {
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type FullExportMember struct {
	Username    string `json:"username"`
	MailAddress string `json:"email"`
}

type FullExportProducts struct {
	Active   []dbModel.Product `json:"active"`
	Archived []dbModel.Product `json:"archived"`
}

// ExportProductsCSV exports the user's active products as CSV.
// @Summary      Export products as CSV
// @Description  Returns a CSV file with all active products for the user
// @Tags         export
// @Produce      text/csv
// @Param        from   query  string  false  "From date (2006-01-02)"
// @Param        to     query  string  false  "To date (2006-01-02)"
// @Success      200    {file}  binary "CSV file"
// @Failure      400    {object}  api.APIResponse
// @Failure      500    {object}  api.APIResponse
// @Router       /api/v1/products/export/products.csv [get]
func ExportProductsCSV(ctx *gin.Context, appCtx *AppContext) {
	from, to := parseDateRange(ctx)

	products, err := appCtx.Repos.Products.GetUserActiveProductsFiltered(appCtx.UserID, from, to)
	if err != nil {
		appCtx.Logger.Error().Msgf(GetUserActiveProductsFiltered, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrGettingProducts})
		return
	}

	ctx.Header(util.RequestHeaderContentType, mimeTypeCSV)
	ctx.Header(util.RequestHeaderContentDisposition, "attachment; filename=\"products.csv\"")

	writer := csv.NewWriter(ctx.Writer)
	if err := writer.Write([]string{"name", "barcode", "quantity", "unit", "category", "storage_location", "expiry_date", "added_at"}); err != nil {
		appCtx.Logger.Error().Msgf(errors.ErrExportCSVWriteWrapper, err)
		return
	}

	for i := range products {
		if err := writer.Write(productToExportRow(&products[i])); err != nil {
			appCtx.Logger.Error().Msgf(errors.ErrExportCSVWriteWrapper, err)
			return
		}
	}
	writer.Flush()
}

// ExportProductsJSON exports the user's active products as JSON.
// @Summary      Export products as JSON
// @Description  Returns a JSON file with all active products for the user
// @Tags         export
// @Produce      application/json
// @Param        from   query  string  false  "From date (2006-01-02)"
// @Param        to     query  string  false  "To date (2006-01-02)"
// @Success      200    {file}  binary "JSON file"
// @Failure      400    {object}  api.APIResponse
// @Failure      500    {object}  api.APIResponse
// @Router       /api/v1/products/export/products.json [get]
func ExportProductsJSON(ctx *gin.Context, appCtx *AppContext) {
	from, to := parseDateRange(ctx)

	products, err := appCtx.Repos.Products.GetUserActiveProductsFiltered(appCtx.UserID, from, to)
	if err != nil {
		appCtx.Logger.Error().Msgf(GetUserActiveProductsFiltered, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrGettingProducts})
		return
	}

	ctx.Header(util.RequestHeaderContentType, "application/json")
	ctx.Header(util.RequestHeaderContentDisposition, "attachment; filename=\"products.json\"")
	ctx.JSON(http.StatusOK, products)
}

// ExportArchiveCSV exports the user's archived products as CSV.
// @Summary      Export archived products as CSV
// @Description  Returns a CSV file with all archived products for the user
// @Tags         export
// @Produce      text/csv
// @Param        from   query  string  false  "From date (2006-01-02)"
// @Param        to     query  string  false  "To date (2006-01-02)"
// @Success      200    {file}  binary "CSV file"
// @Failure      400    {object}  api.APIResponse
// @Failure      500    {object}  api.APIResponse
// @Router       /api/v1/products/export/archive.csv [get]
func ExportArchiveCSV(ctx *gin.Context, appCtx *AppContext) {
	from, to := parseDateRange(ctx)

	products, err := appCtx.Repos.Products.GetUserArchivedProductsFiltered(appCtx.UserID, from, to)
	if err != nil {
		appCtx.Logger.Error().Msgf(fmtGetUserArchivedProductsFilter, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting archived products"})
		return
	}

	ctx.Header(util.RequestHeaderContentType, mimeTypeCSV)
	ctx.Header(util.RequestHeaderContentDisposition, "attachment; filename=\"archive.csv\"")

	writer := csv.NewWriter(ctx.Writer)
	if err := writer.Write([]string{"name", "barcode", "quantity", "unit", "category", "storage_location", "expiry_date", "added_at", "archived_at"}); err != nil {
		appCtx.Logger.Error().Msgf(errors.ErrExportCSVWriteWrapper, err)
		return
	}

	for i := range products {
		product := &products[i]
		row := productToExportRow(product)
		if !product.DeletedAt.Valid {
			row = append(row, "")
		} else {
			row = append(row, product.DeletedAt.Time.Format(util.DefaultDateFormatParseStr))
		}
		if err := writer.Write(row); err != nil {
			appCtx.Logger.Error().Msgf(errors.ErrExportCSVWriteWrapper, err)
			return
		}
	}
	writer.Flush()
}

// ExportFullJSON exports all household data as JSON.
// @Summary      Export all household data as JSON
// @Description  Returns a comprehensive JSON export including household info, members, products, and statistics
// @Tags         export
// @Produce      application/json
// @Success      200  {object}  FullExportResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/products/export/full.json [get]
func ExportFullJSON(ctx *gin.Context, appCtx *AppContext) {
	user, err := appCtx.Repos.Products.GetUserByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetUserByID: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting user"})
		return
	}

	household, err := appCtx.Repos.Products.GetHouseholdByID(user.HouseholdID)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetHouseholdByID: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting household"})
		return
	}

	members, err := appCtx.Repos.Products.GetUsersByHouseholdID(user.HouseholdID)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetUsersByHouseholdID: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting household members"})
		return
	}

	activeProducts, err := appCtx.Repos.Products.GetUserActiveProductsFiltered(appCtx.UserID, nil, nil)
	if err != nil {
		appCtx.Logger.Error().Msgf(GetUserActiveProductsFiltered, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting active products"})
		return
	}

	archivedProducts, err := appCtx.Repos.Products.GetUserArchivedProductsFiltered(appCtx.UserID, nil, nil)
	if err != nil {
		appCtx.Logger.Error().Msgf(fmtGetUserArchivedProductsFilter, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting archived products"})
		return
	}

	stats, ok := buildExportStats(ctx, appCtx.Repos, appCtx.UserID, user.HouseholdID, user.NotificationPreferences.NotificationThresholdDays, len(archivedProducts), appCtx.Logger)
	if !ok {
		return
	}

	exportedAt := time.Now().Format(time.RFC3339)

	exportMembers := make([]FullExportMember, len(members))
	for i := range members {
		exportMembers[i] = FullExportMember{
			Username:    members[i].Username,
			MailAddress: members[i].MailAddress,
		}
	}

	response := FullExportResponse{
		Household: &FullExportHousehold{
			Name:      household.Name,
			CreatedAt: household.CreatedAt.Format(util.DefaultDateFormatParseStr),
		},
		Members: exportMembers,
		Products: FullExportProducts{
			Active:   activeProducts,
			Archived: archivedProducts,
		},
		Stats:      stats,
		ExportedAt: exportedAt,
	}

	ctx.Header(util.RequestHeaderContentType, "application/json")
	ctx.Header(util.RequestHeaderContentDisposition, "attachment; filename=\"full_export.json\"")
	ctx.JSON(http.StatusOK, response)
}

func buildExportStats(ctx *gin.Context, repos *database.RepositoryContainer, userID uint, householdID uint, notificationThresholdDays int, archivedProductsLen int, logger *zerolog.Logger) (apiModel.ProductStatsResponse, bool) {
	totalActive, err := repos.Products.GetActiveProductsCount(userID)
	if err != nil {
		logger.Error().Msgf(errors.FmtErrGetActiveProductsCount, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingActiveCount})
		return apiModel.ProductStatsResponse{}, false
	}

	wasteCount, err := repos.Products.GetExpiredProductsCount(userID)
	if err != nil {
		logger.Error().Msgf(errors.FmtErrGetExpiredProductsCount, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingWasteCount})
		return apiModel.ProductStatsResponse{}, false
	}

	var wastePercent float64
	if totalActive > 0 {
		wastePercent = float64(wasteCount) / float64(totalActive) * 100
	}

	expiringSoonDays := 7
	if notificationThresholdDays > 0 {
		expiringSoonDays = notificationThresholdDays
	}

	expiringSoon, err := repos.Products.GetExpiringSoonProducts(userID, expiringSoonDays)
	if err != nil {
		logger.Error().Msgf(errors.FmtErrGetExpiringSoonProducts, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingExpiringSoon})
		return apiModel.ProductStatsResponse{}, false
	}

	categories, err := repos.Products.GetProductCategoryBreakdown(userID)
	if err != nil {
		logger.Error().Msgf(errors.FmtErrGetProductCategoryBreakdown, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingCategoryBreakdown})
		return apiModel.ProductStatsResponse{}, false
	}

	expiryTrend, err := repos.Products.GetExpiryTrend(userID)
	if err != nil {
		logger.Error().Msgf(errors.FmtErrGetExpiryTrend, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingExpiryTrend})
		return apiModel.ProductStatsResponse{}, false
	}

	uniqueArchived, err := repos.Products.GetUniqueArchivedProductsCount(userID)
	if err != nil {
		logger.Error().Msgf(errors.FmtErrGetUniqueArchivedCount, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: errors.MsgErrComputingUniqueArchivedCount})
		return apiModel.ProductStatsResponse{}, false
	}

	var lastInsertedProduct string
	lastProduct, lastErr := repos.Products.GetLastInsertedProduct(householdID)
	if lastErr == nil && lastProduct.ID != 0 {
		lastInsertedProduct = lastProduct.ProductName
	}

	return apiModel.ProductStatsResponse{
		WasteCount:          wasteCount,
		WastePercent:        wastePercent,
		TotalActive:         totalActive,
		TotalArchived:       archivedProductsLen,
		UniqueArchived:      uniqueArchived,
		LastInsertedProduct: lastInsertedProduct,
		ExpiringSoon:        expiringSoon,
		ExpiringSoonDays:    expiringSoonDays,
		Categories:          categories,
		ExpiryTrend:         expiryTrend,
	}, true
}
