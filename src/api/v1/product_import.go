// v1 implements version 1 of the proviant API
package v1

import (
	"bufio"
	"context"
	"encoding/csv"
	stderrors "errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

const (
	fmtErrImportRead          = "ImportProducts: read: %s"
	fmtErrImportWriteProducts = "ImportProducts: CreateProductsBulk: %s"
	fmtErrImportActivity      = "ImportProducts: activity log: %s"
	fmtErrImportLocations     = "ImportProducts: GetByHousehold: %s"
	fmtErrImportDupeLookup    = "ImportProducts: GetUserProductsBulkByBarcodes: %s"
	fmtImportOffMiss          = "name is required and no Open Food Facts entry found"
	fmtImportOffBudget        = "name is required and the Open Food Facts lookup budget for this import is exhausted"

	// The name of the multipart form field carrying the CSV.
	importFormField = "file"

	// Imported locations get a neutral icon; the CSV carries no icon column.
	importLocationIcon = "📦"

	// A row without an explicit quantity is a single unit.
	importDefaultQuantity = 1
)

// importRow is one parsed CSV data line, with its values already trimmed.
type importRow struct {
	line     int
	name     string
	barcode  string
	quantity string
	unit     string
	category string
	location string
	expiry   string
}

// importCell returns the trimmed value of the named column, or "" when the file
// lacks that column or the row is short.
func importCell(record []string, columns map[string]int, name string) string {
	index, ok := columns[name]
	if !ok || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

// ImportProducts imports products from an uploaded CSV file
// @Summary      Import products from CSV
// @Description  Creates products from an uploaded CSV file. Insert-only: a barcode that already exists among the household's active products is reported as a row error and never modified. Unknown storage locations are created. Rows without a name are completed from the Open Food Facts cache, and at most a bounded number of them may additionally trigger a live Open Food Facts lookup.
// @Tags         import
// @Accept       multipart/form-data
// @Produce      json
// @Param        file  formData  file  true  "CSV file"
// @Success      200   {object}  apiModel.ImportProductsResponse
// @Failure      400   {object}  apiModel.ImportProductsResponse
// @Failure      500   {object}  apiModel.ImportProductsResponse
// @Router       /api/v1/products/import [post]
func ImportProducts(ctx *gin.Context, appCtx *AppContext) {
	logger := appCtx.Logger

	configValue, configOk := ctx.Get(util.ContextKeyProviantConfig)
	proviantConfig, isConfig := configValue.(*configuration.ProviantConfiguration)
	if !configOk || !isConfig || proviantConfig == nil {
		logger.Error().Msg("proviant config not found in context")
		api.RespondError(ctx, http.StatusInternalServerError, errors.ErrInternalServer)
		return
	}

	maxFileBytes := int64(proviantConfig.Server.MaxUploadSizeMB) * 1024 * 1024
	// ctx.FormFile parses the whole multipart body before the file part is
	// returned, so the configured cap has to be enforced on the body itself.
	// Without this, Gin buffers up to engine.MaxMultipartMemory and spills the
	// remainder to a temp file no matter how small MaxUploadSizeMB is.
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxFileBytes+util.CsvImportMultipartSlackBytes)

	file, err := ctx.FormFile(importFormField)
	if err != nil {
		logger.Warn().Msgf("Import form file error: %s", err.Error())
		if importBodyTooLarge(err) {
			api.RespondError(ctx, http.StatusBadRequest, errors.ErrFileTooLarge)
			return
		}
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrImportMissingFile)
		return
	}
	if file.Size > maxFileBytes {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrFileTooLarge)
		return
	}

	src, openErr := file.Open()
	if openErr != nil {
		logger.Error().Msgf(fmtErrImportRead, openErr.Error())
		api.RespondError(ctx, http.StatusInternalServerError, errors.ErrInternalServer)
		return
	}
	defer func() {
		if closeErr := src.Close(); closeErr != nil {
			logger.Warn().Msgf("Error closing file: %s", closeErr.Error())
		}
	}()

	rows, columns, readErr := readImportCSV(src, logger)
	if readErr != nil {
		// A body that trips the transport cap surfaces here rather than at
		// FormFile, so it has to map onto the same 400.
		if importBodyTooLarge(readErr) {
			api.RespondError(ctx, http.StatusBadRequest, errors.ErrFileTooLarge)
			return
		}
		api.RespondError(ctx, http.StatusBadRequest, readErr)
		return
	}
	if _, hasBarcode := columns[util.CsvColumnBarcode]; !hasBarcode {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrImportNoBarcodeColumn)
		return
	}
	if len(rows) == 0 {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrImportEmptyFile)
		return
	}

	userID := appCtx.UserID
	repos := appCtx.Repos
	barcodes := importBarcodes(rows)

	// A barcode that already belongs to an active household product is rejected
	// rather than merged. Archived barcodes are restorable, so they do not count
	// as duplicates.
	existing, dupeErr := repos.Products.GetUserProductsBulkByBarcodes(userID, barcodes)
	if dupeErr != nil {
		logger.Error().Msgf(fmtErrImportDupeLookup, dupeErr)
		api.RespondError(ctx, http.StatusInternalServerError, errors.ErrInternalServer)
		return
	}
	knownBarcodes := make(map[string]bool, len(existing))
	for i := range existing {
		knownBarcodes[existing[i].Barcode] = true
	}

	locations, resolverErr := newLocationResolver(userID, repos, logger)
	if resolverErr != nil {
		api.RespondError(ctx, http.StatusInternalServerError, resolverErr)
		return
	}
	names := newImportNameResolver(repos, barcodes, importOffDatasetGetter(ctx), proviantConfig.OpenFoodFacts.CacheEnabled, logger)

	products := make([]database.ImportedProduct, 0, len(rows))
	rowErrors := make([]apiModel.ImportRowError, 0)
	seen := make(map[string]bool, len(rows))

	for i := range rows {
		row := &rows[i]
		product, reason := parseImportRow(row, knownBarcodes, names)
		if reason == "" && seen[row.barcode] {
			// Only a row that was otherwise importable consumes the barcode, so
			// one bad cell does not silently drop a later valid row carrying the
			// same code.
			reason = "duplicate barcode within file"
		}
		if reason != "" {
			logger.Debug().Msgf(errors.ErrImportRowWrapper, row.line, reason)
			rowErrors = append(rowErrors, apiModel.ImportRowError{
				Row: row.line, Name: row.name, Barcode: row.barcode, Reason: reason,
			})
			continue
		}
		seen[row.barcode] = true

		// The location is only named here, never written: CreateProductsBulk
		// creates it inside the same transaction as the product insert, so a
		// failed write leaves no orphaned location behind.
		locationID, newLocationName := locations.resolve(row.location)
		product.StorageLocationID = locationID
		products = append(products, database.ImportedProduct{
			Product:         product,
			NewLocationName: newLocationName,
			NewLocationIcon: importLocationIcon,
		})
	}

	response := apiModel.ImportProductsResponse{
		TotalRows: len(rows),
		Imported:  len(products),
		Failed:    len(rowErrors),
		Errors:    rowErrors,
	}

	if len(products) == 0 {
		// Nothing was written. The client asked for a write that did not
		// happen, so this is a bad request rather than a partial success.
		response.Message = "no products were imported"
		ctx.JSON(http.StatusBadRequest, response)
		return
	}

	createdLocations, createErr := repos.Products.CreateProductsBulk(userID, products)
	if createErr != nil {
		// Too many distinct new storage locations is the file's fault, not the
		// server's, so it is a bad request rather than a write failure.
		if stderrors.Is(createErr, errors.ErrImportTooManyLocations) {
			logger.Warn().Msgf("Import rejected: %s", createErr)
			response.Imported = 0
			response.Failed = response.TotalRows
			response.Message = fmt.Sprintf("the CSV names more than %d new storage locations", util.CsvImportMaxNewLocations)
			ctx.JSON(http.StatusBadRequest, response)
			return
		}
		logger.Error().Msgf(fmtErrImportWriteProducts, createErr)
		// Keep totalRows == imported + failed on every status code.
		response.Imported = 0
		response.Failed = response.TotalRows
		response.Message = "failed to write imported products"
		ctx.JSON(http.StatusInternalServerError, response)
		return
	}
	response.CreatedLocations = createdLocations

	logImportActivity(repos, logger, userID, len(products))

	if response.Failed > 0 {
		response.Message = fmt.Sprintf("%d imported, %d failed", response.Imported, response.Failed)
	} else {
		response.Message = fmt.Sprintf("%d products imported", response.Imported)
	}
	ctx.JSON(http.StatusOK, response)
}

// ImportTemplateCSV streams the import column header plus one example row
// @Summary      Download the CSV import template
// @Description  Returns a CSV file with the import column header and one example row
// @Tags         import
// @Produce      text/csv
// @Success      200  {file}  binary "CSV file"
// @Router       /api/v1/products/import/template.csv [get]
func ImportTemplateCSV(ctx *gin.Context, _ *AppContext) {
	ctx.Header(util.RequestHeaderContentType, mimeTypeCSV)
	ctx.Header(util.RequestHeaderContentDisposition, "attachment; filename=\""+util.CsvImportTemplateFilename+"\"")

	writer := csv.NewWriter(ctx.Writer)
	if err := writer.Write([]string{
		util.CsvColumnName, util.CsvColumnBarcode, util.CsvColumnQuantity, util.CsvColumnUnit,
		util.CsvColumnCategory, util.CsvColumnStorageLocation, util.CsvColumnExpiryDate, util.CsvColumnAddedAt,
	}); err != nil {
		return
	}
	// The example carries an empty added_at: that column is accepted so an
	// export can be re-imported unchanged, but GORM owns the creation time.
	_ = writer.Write([]string{"Milk", "4006381333931", "2", "l", "Dairy", "Fridge", "2026-12-31", ""})
	writer.Flush()
}

// parseImportRow converts one CSV line into a product, or returns the reason it
// was rejected. An empty reason means the row is importable. In-file duplicate
// detection has already run in the caller, so only the household check remains.
func parseImportRow(
	row *importRow,
	knownBarcodes map[string]bool,
	names *importNameResolver,
) (dbModel.Product, string) {
	var product dbModel.Product

	if reason := validateImportBarcode(row); reason != "" {
		return product, reason
	}
	if knownBarcodes[row.barcode] {
		return product, "barcode already exists in household"
	}

	product = dbModel.Product{
		ProductName: row.name,
		Barcode:     row.barcode,
		Amount:      importDefaultQuantity,
		Unit:        row.unit,
		Categories:  row.category,
	}

	if row.quantity != "" {
		quantity, parseErr := strconv.Atoi(row.quantity)
		if parseErr != nil {
			return product, fmt.Sprintf("quantity '%s' is not a number", row.quantity)
		}
		// Zero is accepted, not rejected. The CSV export writes Amount verbatim
		// and the model has no minimum, so an exported 0 must re-import instead
		// of dropping the row. The amount only ever moves by a delta, so a
		// negative value stays meaningless either way.
		if quantity < 0 {
			return product, "quantity must not be negative"
		}
		product.Amount = quantity
	}

	if row.expiry != "" {
		expireAt, dateErr := time.Parse(util.DefaultDateFormatParseStr, row.expiry)
		if dateErr != nil {
			return product, fmt.Sprintf("expiry_date '%s' is not a %s date", row.expiry, util.DefaultDateFormatParseStr)
		}
		product.ExpireAt = expireAt
	}

	if product.ProductName == "" {
		// A blank name renders as a blank row in the product list, so a
		// barcode-only row falls back to Open Food Facts. The resolver reads
		// its cache first and only spends a bounded number of live lookups, so
		// a file full of nameless rows cannot fan out without limit.
		resolved, reason := names.resolve(product.Barcode)
		if reason != "" {
			return product, reason
		}
		product.ProductName = resolved
	}

	return product, ""
}

// importBarcodePattern is the compiled form of util.CsvImportBarcodePattern. An
// imported barcode reaches the path and query string of the outbound Open Food
// Facts request, so it is restricted to the same characters the codebase
// already allows when it fetches a product image.
var importBarcodePattern = regexp.MustCompile(util.CsvImportBarcodePattern)

// validateImportBarcode rejects a row whose barcode cell is unusable. The other
// columns are validated in parseImportRow, where the reason can name them.
func validateImportBarcode(row *importRow) string {
	if row.barcode == "" {
		return "barcode is required"
	}
	if len(row.barcode) > util.CsvImportMaxBarcodeLength {
		return fmt.Sprintf("barcode must be at most %d characters", util.CsvImportMaxBarcodeLength)
	}
	if len(row.barcode) < util.CsvImportMinBarcodeLength {
		return fmt.Sprintf("barcode must be at least %d characters", util.CsvImportMinBarcodeLength)
	}
	if !importBarcodePattern.MatchString(row.barcode) {
		return "barcode must contain only letters, digits, hyphen or underscore"
	}
	return ""
}

// readImportCSV parses an uploaded CSV into trimmed rows plus a column index
// map. The delimiter is sniffed from the header line because the export writes
// commas while German-locale spreadsheets write semicolons.
func readImportCSV(src io.Reader, logger *zerolog.Logger) ([]importRow, map[string]int, error) {
	index := &importNewlineIndex{reader: src}
	buffered := bufio.NewReader(index)
	headerLine, lineErr := buffered.ReadString('\n')
	if headerLine == "" {
		if lineErr != nil {
			logger.Warn().Msgf("Import file is empty: %s", lineErr.Error())
		}
		return nil, nil, errors.ErrImportEmptyFile
	}

	reader := csv.NewReader(io.MultiReader(strings.NewReader(headerLine), buffered))
	if strings.Contains(headerLine, ";") && !strings.Contains(headerLine, ",") {
		reader.Comma = ';'
	}
	// Spreadsheet exports routinely contain ragged rows and stray quotes inside
	// free-text names, so both are absorbed instead of rejecting the file.
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	header, headerErr := reader.Read()
	if headerErr != nil {
		logger.Warn().Msgf("Import header read error: %s", headerErr.Error())
		return nil, nil, errors.ErrImportEmptyFile
	}

	columns := importColumnIndex(header)

	var rows []importRow
	line := 0
	for {
		record, readErr := reader.Read()
		if readErr != nil {
			if stderrors.Is(readErr, io.EOF) {
				break
			}
			// LazyQuotes absorbs stray quotes, so a parse error here means the
			// file is structurally broken. Rejecting it beats importing a
			// silently truncated prefix.
			logger.Warn().Msgf("Import row read error: %s", readErr.Error())
			return nil, nil, readErr
		}
		// InputOffset is the byte just past this record, so the line feeds
		// before it land on the physical line the record ends on. The csv
		// reader skips blank lines and folds a newline inside a quoted field
		// into the same record, so neither a record count nor the source's
		// newline count would report the right line.
		line = index.lineAt(reader.InputOffset(), line)
		if isBlankRecord(record) {
			continue
		}
		if len(rows) >= util.CsvImportMaxRows {
			return nil, nil, errors.ErrImportTooManyRows
		}
		rows = append(rows, importRow{
			line:     line,
			name:     importCell(record, columns, util.CsvColumnName),
			barcode:  importCell(record, columns, util.CsvColumnBarcode),
			quantity: importCell(record, columns, util.CsvColumnQuantity),
			unit:     importCell(record, columns, util.CsvColumnUnit),
			category: importCell(record, columns, util.CsvColumnCategory),
			location: importCell(record, columns, util.CsvColumnStorageLocation),
			expiry:   importCell(record, columns, util.CsvColumnExpiryDate),
		})
	}
	return rows, columns, nil
}

// importNewlineIndex records the absolute stream offset of every line feed in
// the uploaded file. Counting line feeds as the source produces them is not
// enough: encoding/csv reads through a bufio.Reader of its own, so a counter
// sitting under the source reports up to a full buffer ahead of the parse
// position. Pairing these offsets with csv.Reader.InputOffset puts every row on
// the physical line the parser actually finished reading.
type importNewlineIndex struct {
	reader   io.Reader
	offsets  []int64
	position int64
}

func (i *importNewlineIndex) Read(buffer []byte) (int, error) {
	read, err := i.reader.Read(buffer)
	if read > 0 {
		for position, char := range buffer[:read] {
			if char == '\n' {
				i.offsets = append(i.offsets, i.position+int64(position))
			}
		}
		i.position += int64(read)
	}
	return read, err
}

// lineAt returns the 1-based physical line the record ending at the given input
// offset finishes on, advancing counted past every line feed before it.
// InputOffset is the byte just past the record, so each feed before it is a
// completed line and the number of feeds counted is the line the record ends on,
// with the header counted as line 1. counted is the cursor the previous record
// left behind, which keeps the walk linear over a long file; it is only a
// cursor, since counting every feed before end yields the same line either way.
func (i *importNewlineIndex) lineAt(end int64, counted int) int {
	for counted < len(i.offsets) && i.offsets[counted] < end {
		counted++
	}
	return counted
}

// importColumnIndex maps the recognised header cells to their field index.
// Unknown columns are ignored, and a repeated canonical column keeps the first
// occurrence so a later alias cannot shadow the primary one.
func importColumnIndex(header []string) map[string]int {
	columns := make(map[string]int, len(header))
	for i, cell := range header {
		name := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(cell, "\ufeff")))
		canonical, known := util.CsvImportColumnAliases[name]
		if !known {
			continue
		}
		if _, duplicate := columns[canonical]; duplicate {
			continue
		}
		columns[canonical] = i
	}
	return columns
}

func isBlankRecord(record []string) bool {
	for i := range record {
		if strings.TrimSpace(record[i]) != "" {
			return false
		}
	}
	return true
}

func importBarcodes(rows []importRow) []string {
	barcodes := make([]string, 0, len(rows))
	for i := range rows {
		if rows[i].barcode != "" {
			barcodes = append(barcodes, rows[i].barcode)
		}
	}
	return barcodes
}

// locationResolver maps a CSV storage_location cell to an existing location ID,
// or to a name the household does not have yet. A fresh household only has
// Fridge, Freezer and Pantry, so a realistic CSV names many locations that do
// not exist and dropping them would lose most of the file.
//
// Matching is case-insensitive and repeated spellings collapse to one name, so
// one location is created per distinct name. Nothing is written here: the
// repository creates the names inside the transaction that inserts the products,
// which is what keeps a failed import from leaving locations behind.
type locationResolver struct {
	existing map[string]uint
	pending  map[string]struct{}
}

func newLocationResolver(userID uint, repos *database.RepositoryContainer, logger *zerolog.Logger) (*locationResolver, error) {
	locations, err := repos.StorageLocations.GetByHousehold(userID)
	if err != nil {
		logger.Error().Msgf(fmtErrImportLocations, err)
		return nil, errors.ErrInternalServer
	}
	existing := make(map[string]uint, len(locations))
	for i := range locations {
		existing[strings.ToLower(strings.TrimSpace(locations[i].Name))] = locations[i].ID
	}
	return &locationResolver{existing: existing, pending: make(map[string]struct{})}, nil
}

// resolve returns the id of an existing location, or the name of a location
// that has to be created alongside the products.
func (r *locationResolver) resolve(name string) (*uint, string) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, ""
	}
	key := strings.ToLower(trimmed)
	if locationID, ok := r.existing[key]; ok {
		return &locationID, ""
	}
	if _, duplicate := r.pending[key]; !duplicate {
		r.pending[key] = struct{}{}
	}
	return nil, trimmed
}

// importNameResolver supplies the product name of a row that carries only a
// barcode. It resolves the whole file from the Open Food Facts cache with one
// query and then spends at most util.CsvImportOpenFoodFactsLookups live requests,
// bounded by util.CsvImportOpenFoodFactsBudget, so a CSV of barcode-only rows
// cannot turn one request into thousands of sequential outbound calls.
type importNameResolver struct {
	getter   controllers.DatasetGetter
	cache    map[string]string
	lookups  int
	budget   time.Duration
	deadline time.Time
	logger   *zerolog.Logger
}

func newImportNameResolver(
	repos *database.RepositoryContainer,
	barcodes []string,
	getter controllers.DatasetGetter,
	cacheEnabled bool,
	logger *zerolog.Logger,
) *importNameResolver {
	resolver := &importNameResolver{
		getter: getter,
		cache:  make(map[string]string, len(barcodes)),
		budget: util.CsvImportOpenFoodFactsBudget,
		logger: logger,
	}
	if resolver.budget > 0 {
		resolver.deadline = time.Now().Add(resolver.budget)
	}

	// The cache is opt-in everywhere else: GetOpenFoodFactsData and
	// storeCacheEntry both gate on this flag, and the backfill does too, so
	// nothing writes or refreshes the table while it is off. Reading it anyway
	// would name rows from entries the operator disabled and that may be years
	// old.
	if !cacheEnabled {
		logger.Debug().Msg("Open Food Facts cache disabled, resolving every name live")
		return resolver
	}

	cached, cacheErr := repos.Products.GetOpenFoodFactsCachesByBarcodes(barcodes)
	if cacheErr != nil {
		// A cache miss costs a live lookup, not correctness: fall through with
		// an empty cache instead of failing the whole import.
		logger.Warn().Msgf("Import Open Food Facts cache lookup: %s", cacheErr)
		return resolver
	}
	for i := range cached {
		if name := strings.TrimSpace(cached[i].ProductName); name != "" {
			resolver.cache[cached[i].Barcode] = name
		}
	}
	return resolver
}

// resolve returns the name for a barcode, or the reason the row must be
// rejected: fmtImportOffMiss when Open Food Facts has no entry, fmtImportOffBudget
// once this import has spent its lookup allowance. A barcode that resolves to
// nothing is remembered too, so a file repeating one unknown code cannot make
// the same outbound request once per row.
func (r *importNameResolver) resolve(barcode string) (string, string) {
	if name, cached := r.cache[barcode]; cached {
		if name == "" {
			return "", fmtImportOffMiss
		}
		return name, ""
	}
	if r.getter == nil {
		r.cache[barcode] = ""
		return "", fmtImportOffMiss
	}
	if r.lookups >= util.CsvImportOpenFoodFactsLookups || (r.budget > 0 && time.Now().After(r.deadline)) {
		return "", fmtImportOffBudget
	}
	r.lookups++

	dataset, offErr := r.getter.GetDataset(barcode)
	if offErr != nil {
		r.logger.Debug().Msgf("Import Open Food Facts lookup for barcode '%s': %s", barcode, offErr)
		r.cache[barcode] = ""
		return "", fmtImportOffMiss
	}
	name := strings.TrimSpace(dataset.ProductName)
	r.cache[barcode] = name
	if name == "" {
		return "", fmtImportOffMiss
	}
	return name, ""
}

// importOffDatasetGetter returns the Open Food Facts controller wired into the
// request, or nil when none is available — in which case a barcode-only row
// cannot be completed.
func importOffDatasetGetter(ctx *gin.Context) controllers.DatasetGetter {
	value, exists := ctx.Get("offacntrl")
	if !exists {
		return nil
	}
	getter, ok := value.(controllers.DatasetGetter)
	if !ok {
		return nil
	}
	return getter
}

// importBodyTooLarge reports whether a multipart parse failure was the
// transport-level body cap tripping rather than a missing file part. The cap is
// derived from the configured upload size, so any http.MaxBytesError means the
// request exceeded it.
func importBodyTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return stderrors.As(err, &maxBytesErr)
}

// logImportActivity records the whole run as a single feed entry. Per-product
// entries would flood the feed for a large file, and the webhook is deliberately
// skipped for the same reason.
func logImportActivity(repos *database.RepositoryContainer, logger *zerolog.Logger, userID uint, count int) {
	if repos.ActivityLogs == nil {
		return
	}
	go func() {
		householdID, householdErr := repos.Users.GetUserHouseholdByID(userID)
		if householdErr != nil {
			// An entry with HouseholdID 0 would be orphaned; dropping it beats
			// writing a feed row no feed ever renders.
			logger.Warn().Msgf(fmtErrImportActivity, householdErr)
			return
		}
		userName := ""
		if user, err := repos.Users.GetUserByID(userID); err == nil {
			userName = user.DisplayName
		}
		// ProductID stays 0: the feed links a row only when a product is
		// attached, and an aggregate entry has no single product.
		logEntry := &dbModel.ActivityLog{
			HouseholdID: householdID,
			UserID:      &userID,
			UserName:    userName,
			Action:      dbModel.ActivityActionImport,
			ProductName: fmt.Sprintf("%d products imported", count),
			Quantity:    count,
			Timestamp:   time.Now(),
		}
		ctxBg, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := repos.ActivityLogs.Create(ctxBg, logEntry); err != nil {
			logger.Warn().Msgf(fmtErrImportActivity, err)
		}
	}()
}
