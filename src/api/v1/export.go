// v1 implements version 1 of the proviant API
package v1

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
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

type FullExportResponse = apiModel.FullExportResponse
type FullExportHousehold = apiModel.FullExportHousehold
type FullExportMember = apiModel.FullExportMember
type FullExportProducts = apiModel.FullExportProducts

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
// @Success      200    {object}  []apiModel.ExportProductResponse
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
	ctx.JSON(http.StatusOK, apiModel.NewExportProductResponses(products))
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
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
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
// @Description  Returns a versioned household backup document including household info, members, storage locations, products, and statistics
// @Tags         export
// @Produce      application/json
// @Success      200  {object}  FullExportResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Router       /api/v1/products/export/full.json [get]
func ExportFullJSON(ctx *gin.Context, appCtx *AppContext) {
	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetUserHouseholdByID: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	response, err := controllers.BuildFullExport(appCtx.DB, appCtx.Logger, householdID, appCtx.UserID, false)
	if err != nil {
		appCtx.Logger.Error().Msgf("BuildFullExport: %s", err)
		ctx.JSON(http.StatusInternalServerError, api.InternalError())
		return
	}

	ctx.Header(util.RequestHeaderContentType, "application/json")
	ctx.Header(util.RequestHeaderContentDisposition, "attachment; filename=\"full_export.json\"")
	ctx.JSON(http.StatusOK, response)
}
