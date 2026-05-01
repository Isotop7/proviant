// v1 implements version 1 of the proviant API
package v1

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const GetUserActiveProductsFiltered = "GetUserActiveProductsFiltered: %s"


func parseDateRange(ctx *gin.Context) (*time.Time, *time.Time) {
	var from, to *time.Time

	if fromStr := ctx.Query("from"); fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
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
func ExportProductsCSV(ctx *gin.Context) {
	logger, loggerOk := ctx.MustGet("logger").(*zerolog.Logger)
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	from, to := parseDateRange(ctx)

	products, err := repos.Products.GetUserActiveProductsFiltered(userID, from, to)
	if err != nil {
		logger.Error().Msgf(GetUserActiveProductsFiltered, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting products"})
		return
	}

	ctx.Header(util.RequestHeaderContentType, "text/csv")
	ctx.Header(util.RequestHeaderContentDisposition, "attachment; filename=\"products.csv\"")

	writer := csv.NewWriter(ctx.Writer)
	if err := writer.Write([]string{"name", "barcode", "quantity", "unit", "category", "storage_location", "expiry_date", "added_at"}); err != nil {
		logger.Error().Msgf(errors.ErrExportCSVWriteWrapper, err)
		return
	}

	for i := range products {
		if err := writer.Write(productToExportRow(&products[i])); err != nil {
			logger.Error().Msgf(errors.ErrExportCSVWriteWrapper, err)
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
func ExportProductsJSON(ctx *gin.Context) {
	logger, loggerOk := ctx.MustGet("logger").(*zerolog.Logger)
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	from, to := parseDateRange(ctx)

	products, err := repos.Products.GetUserActiveProductsFiltered(userID, from, to)
	if err != nil {
		logger.Error().Msgf(GetUserActiveProductsFiltered, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting products"})
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
func ExportArchiveCSV(ctx *gin.Context) {
	logger, loggerOk := ctx.MustGet("logger").(*zerolog.Logger)
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	from, to := parseDateRange(ctx)

	products, err := repos.Products.GetUserArchivedProductsFiltered(userID, from, to)
	if err != nil {
		logger.Error().Msgf("GetUserArchivedProductsFiltered: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting archived products"})
		return
	}

	ctx.Header(util.RequestHeaderContentType, "text/csv")
	ctx.Header(util.RequestHeaderContentDisposition, "attachment; filename=\"archive.csv\"")

	writer := csv.NewWriter(ctx.Writer)
	if err := writer.Write([]string{"name", "barcode", "quantity", "unit", "category", "storage_location", "expiry_date", "added_at", "archived_at"}); err != nil {
		logger.Error().Msgf(errors.ErrExportCSVWriteWrapper, err)
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
			logger.Error().Msgf(errors.ErrExportCSVWriteWrapper, err)
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
func ExportFullJSON(ctx *gin.Context) {
	logger, loggerOk := ctx.MustGet("logger").(*zerolog.Logger)
	if !loggerOk {
		ctx.JSON(http.StatusInternalServerError, api.ResponseErrLoggerContextNotFound)
		return
	}

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	user, err := repos.Products.GetUserByID(userID)
	if err != nil {
		logger.Error().Msgf("GetUserByID: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting user"})
		return
	}

	household, err := repos.Products.GetHouseholdByID(user.HouseholdID)
	if err != nil {
		logger.Error().Msgf("GetHouseholdByID: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting household"})
		return
	}

	members, err := repos.Products.GetUsersByHouseholdID(user.HouseholdID)
	if err != nil {
		logger.Error().Msgf("GetUsersByHouseholdID: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting household members"})
		return
	}

	activeProducts, err := repos.Products.GetUserActiveProductsFiltered(userID, nil, nil)
	if err != nil {
		logger.Error().Msgf(GetUserActiveProductsFiltered, err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting active products"})
		return
	}

	archivedProducts, err := repos.Products.GetUserArchivedProductsFiltered(userID, nil, nil)
	if err != nil {
		logger.Error().Msgf("GetUserArchivedProductsFiltered: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error getting archived products"})
		return
	}

	totalActive, err := repos.Products.GetActiveProductsCount(userID)
	if err != nil {
		logger.Error().Msgf("GetActiveProductsCount: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing active product count"})
		return
	}

	wasteCount, err := repos.Products.GetExpiredProductsCount(userID)
	if err != nil {
		logger.Error().Msgf("GetExpiredProductsCount: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing waste count"})
		return
	}

	var wastePercent float64
	if totalActive > 0 {
		wastePercent = float64(wasteCount) / float64(totalActive) * 100
	}

	expiringSoonDays := 7
	if user.NotificationPreferences.NotificationThresholdDays > 0 {
		expiringSoonDays = user.NotificationPreferences.NotificationThresholdDays
	}

	expiringSoon, err := repos.Products.GetExpiringSoonProducts(userID, expiringSoonDays)
	if err != nil {
		logger.Error().Msgf("GetExpiringSoonProducts: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing expiring soon products"})
		return
	}

	categories, err := repos.Products.GetProductCategoryBreakdown(userID)
	if err != nil {
		logger.Error().Msgf("GetProductCategoryBreakdown: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing category breakdown"})
		return
	}

	expiryTrend, err := repos.Products.GetExpiryTrend(userID)
	if err != nil {
		logger.Error().Msgf("GetExpiryTrend: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing expiry trend"})
		return
	}

	uniqueArchivedMap, err := repos.Products.GetArchivedProductsGroupedByBarcode(userID)
	if err != nil {
		logger.Error().Msgf("GetArchivedProductsGroupedByBarcode: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.APIResponse{Message: "Error computing unique archived count"})
		return
	}

	var lastInsertedProduct string
	lastProduct, lastErr := repos.Products.GetLastInsertedProduct(user.HouseholdID)
	if lastErr == nil && lastProduct.ID != 0 {
		lastInsertedProduct = lastProduct.ProductName
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
		Stats: apiModel.ProductStatsResponse{
			WasteCount:          wasteCount,
			WastePercent:        wastePercent,
			TotalActive:         totalActive,
			TotalArchived:       len(archivedProducts),
			UniqueArchived:      len(uniqueArchivedMap),
			LastInsertedProduct: lastInsertedProduct,
			ExpiringSoon:        expiringSoon,
			ExpiringSoonDays:    expiringSoonDays,
			Categories:          categories,
			ExpiryTrend:         expiryTrend,
		},
		ExportedAt: exportedAt,
	}

	ctx.Header(util.RequestHeaderContentType, "application/json")
	ctx.Header(util.RequestHeaderContentDisposition, "attachment; filename=\"full_export.json\"")
	ctx.JSON(http.StatusOK, response)
}
