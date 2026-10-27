package controllers

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Isotop7/proviant/controllers/database"
	"github.com/Isotop7/proviant/errors"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/authentication"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// backupBarcodePattern is the compiled form of util.CsvImportBarcodePattern.
// A restore is an untrusted document, so its barcodes are held to the same
// path/query-safe character set as the CSV import and Open Food Facts lookup.
var backupBarcodePattern = regexp.MustCompile(util.CsvImportBarcodePattern)

// BuildFullExport assembles the versioned household backup document shared by
// the export endpoint, the restore endpoint and the scheduled backup job.
// includePrivate lifts the per-user privacy scope: the user-facing export keeps
// it, while a backup must carry every member's stock or a restore would silently
// lose private products.
func BuildFullExport(
	db *gorm.DB,
	logger *zerolog.Logger,
	householdID uint,
	viewerID uint,
	includePrivate bool,
) (apiModel.FullExportResponse, error) {
	if db == nil {
		return apiModel.FullExportResponse{}, errors.ErrDatabaseContextNotFound
	}
	productRepo := database.NewProductRepository(db)
	locationRepo := database.NewStorageLocationRepository(db)

	var household dbModel.Household
	if err := db.First(&household, householdID).Error; err != nil {
		return apiModel.FullExportResponse{}, err
	}

	var members []authentication.User
	if err := db.Where(util.QueryHouseholdId, householdID).Find(&members).Error; err != nil {
		return apiModel.FullExportResponse{}, err
	}

	locations, err := locationRepo.GetByHouseholdID(householdID)
	if err != nil {
		return apiModel.FullExportResponse{}, err
	}

	var activeProducts, archivedProducts []dbModel.Product
	if includePrivate {
		activeProducts, err = productRepo.GetHouseholdProductsForBackup(householdID)
		if err != nil {
			return apiModel.FullExportResponse{}, err
		}
		archivedProducts, err = productRepo.GetHouseholdArchivedProductsForBackup(householdID)
		if err != nil {
			return apiModel.FullExportResponse{}, err
		}
	} else {
		activeProducts, err = productRepo.GetUserActiveProductsFiltered(viewerID, nil, nil)
		if err != nil {
			return apiModel.FullExportResponse{}, err
		}
		archivedProducts, err = productRepo.GetUserArchivedProductsFiltered(viewerID, nil, nil)
		if err != nil {
			return apiModel.FullExportResponse{}, err
		}
	}

	ownerNames := make(map[uint]string, len(members))
	exportMembers := make([]apiModel.FullExportMember, len(members))
	for i := range members {
		ownerNames[members[i].ID] = members[i].Username
		exportMembers[i] = apiModel.FullExportMember{
			Username:    members[i].Username,
			MailAddress: members[i].MailAddress,
			Role:        members[i].Role,
		}
	}

	activeExport := apiModel.NewExportProductResponses(activeProducts)
	archivedExport := apiModel.NewExportProductResponses(archivedProducts)
	apiModel.AssignExportProductOwners(activeExport, ownerNames)
	apiModel.AssignExportProductOwners(archivedExport, ownerNames)

	viewerThreshold := 0
	for i := range members {
		if members[i].ID == viewerID {
			viewerThreshold = members[i].NotificationPreferences.NotificationThresholdDays
			break
		}
	}

	response := apiModel.FullExportResponse{
		Version: apiModel.BackupSchemaVersion,
		Household: &apiModel.FullExportHousehold{
			Name:                    household.Name,
			CreatedAt:               household.CreatedAt.Format(util.DefaultDateFormatParseStr),
			Description:             household.Description,
			MonthlyWasteGoalType:    household.MonthlyWasteGoalType,
			MonthlyWasteGoalCount:   household.MonthlyWasteGoalCount,
			MonthlyWasteGoalPercent: household.MonthlyWasteGoalPercent,
		},
		Members:          exportMembers,
		StorageLocations: apiModel.NewStorageLocationResponses(locations),
		Products: apiModel.FullExportProducts{
			Active:   activeExport,
			Archived: archivedExport,
		},
		Stats:      buildFullExportStats(activeProducts, archivedProducts, viewerThreshold),
		ExportedAt: time.Now().Format(time.RFC3339),
	}
	if logger != nil {
		logger.Debug().Msgf("Built household backup document for household %d (%d active, %d archived)", householdID, len(activeProducts), len(archivedProducts))
	}
	return response, nil
}

// buildFullExportStats derives the dashboard numbers from the exported rows
// themselves. The user-facing stats endpoints answer from SQL with a privacy
// scope; a backup document has no single viewer once it is written to disk, so
// deriving from the rows it already carries keeps the file self-consistent.
func buildFullExportStats(active, archived []dbModel.Product, notificationThresholdDays int) apiModel.ProductStatsResponse {
	now := time.Now()
	expiringSoonDays := 7
	if notificationThresholdDays > 0 {
		expiringSoonDays = notificationThresholdDays
	}

	wasteCount := 0
	categories := make(map[string]int)
	monthCounts := make(map[string]int, 12)
	for i := range 12 {
		monthCounts[now.AddDate(0, i, 0).Format(util.DefaultDateFormatMonthStr)] = 0
	}

	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfWindow := startOfToday.AddDate(0, 0, expiringSoonDays+1).Add(-time.Nanosecond)
	expiringSoon := make([]apiModel.StatsExpiringProduct, 0)
	lastInserted := ""
	var lastInsertedAt time.Time

	for i := range active {
		product := &active[i]
		countFullExportCategory(categories, product.Categories)
		expiry := product.EffectiveExpireAt()
		if !expiry.IsZero() {
			if expiry.Before(now) {
				wasteCount++
			}
			month := expiry.Format(util.DefaultDateFormatMonthStr)
			if _, tracked := monthCounts[month]; tracked {
				monthCounts[month]++
			}
			if !expiry.Before(startOfToday) && !expiry.After(endOfWindow) {
				expiringSoon = append(expiringSoon, apiModel.StatsExpiringProduct{
					ProductName: product.ProductName,
					ExpireAt:    expiry.Format(util.DefaultDateFormatParseStr),
				})
			}
		}
		if product.CreatedAt.After(lastInsertedAt) {
			lastInsertedAt = product.CreatedAt
			lastInserted = product.ProductName
		}
	}

	uniqueArchived := make(map[string]bool, len(archived))
	for i := range archived {
		uniqueArchived[archived[i].Barcode] = true
	}

	wastePercent := 0.0
	if len(active) > 0 {
		wastePercent = float64(wasteCount) / float64(len(active)) * 100
	}

	sort.Slice(expiringSoon, func(i, j int) bool {
		return expiringSoon[i].ExpireAt < expiringSoon[j].ExpireAt
	})

	expiryTrend := make([]apiModel.StatsMonthlyCount, 0, 12)
	for i := range 12 {
		month := now.AddDate(0, i, 0).Format(util.DefaultDateFormatMonthStr)
		expiryTrend = append(expiryTrend, apiModel.StatsMonthlyCount{Month: month, Count: monthCounts[month]})
	}

	return apiModel.ProductStatsResponse{
		WasteCount:          wasteCount,
		WastePercent:        wastePercent,
		TotalActive:         len(active),
		TotalArchived:       len(archived),
		UniqueArchived:      len(uniqueArchived),
		LastInsertedProduct: lastInserted,
		ExpiringSoon:        expiringSoon,
		ExpiringSoonDays:    expiringSoonDays,
		Categories:          summarizeFullExportCategories(categories),
		ExpiryTrend:         expiryTrend,
	}
}

// summarizeFullExportCategories mirrors the stats endpoint's top-eight
// breakdown: the dashboard only renders that many labels, so the tail is folded
// into "Other" here too and the backup carries the same numbers the UI shows.
func summarizeFullExportCategories(counts map[string]int) map[string]int {
	const maxCategories = 8
	if len(counts) <= maxCategories {
		return counts
	}
	type categoryCount struct {
		name  string
		count int
	}
	sorted := make([]categoryCount, 0, len(counts))
	for name, count := range counts {
		sorted = append(sorted, categoryCount{name: name, count: count})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].count == sorted[j].count {
			return sorted[i].name < sorted[j].name
		}
		return sorted[i].count > sorted[j].count
	})

	summary := make(map[string]int, maxCategories+1)
	other := 0
	for i := range sorted {
		if i < maxCategories {
			summary[sorted[i].name] = sorted[i].count
			continue
		}
		other += sorted[i].count
	}
	if other > 0 {
		summary["Other"] = other
	}
	return summary
}

// countFullExportCategory mirrors the stats endpoint's category normalisation:
// the first comma-separated entry, with a "prefix:" label stripped, or
// "Uncategorized".
func countFullExportCategory(counts map[string]int, raw string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		counts["Uncategorized"]++
		return
	}
	first, _, _ := strings.Cut(trimmed, ",")
	first = strings.TrimSpace(first)
	if idx := strings.Index(first, ":"); idx != -1 {
		first = strings.TrimSpace(first[idx+1:])
	}
	if first == "" {
		first = "Uncategorized"
	}
	counts[first]++
}

// RestoreHouseholdBackup replaces a household's inventory with the rows in a
// backup document inside one transaction. Accounts are never recreated: the
// document carries usernames only, so product ownership is mapped back onto
// members that still exist and falls back to the restoring admin.
//
// The replacement is deliberately destructive and all-or-nothing. A restore
// that merged into the current stock could not reproduce a past state, and a
// partial write would leave a household with half a backup in it.
func RestoreHouseholdBackup(
	db *gorm.DB,
	householdID uint,
	adminUserID uint,
	backup *apiModel.FullExportResponse,
) (apiModel.RestoreResult, error) {
	if backup == nil {
		return apiModel.RestoreResult{}, errors.ErrBackupInvalidDocument
	}
	if db == nil {
		return apiModel.RestoreResult{}, errors.ErrDatabaseContextNotFound
	}
	if !apiModel.SupportsBackupVersion(backup.Version) {
		return apiModel.RestoreResult{}, fmt.Errorf("%w: got %d, want %d", errors.ErrBackupUnsupportedVersion, backup.Version, apiModel.BackupSchemaVersion)
	}
	if len(backup.Products.Active)+len(backup.Products.Archived) > util.BackupRestoreMaxProducts {
		return apiModel.RestoreResult{}, fmt.Errorf("%w: %d", errors.ErrBackupTooManyProducts, len(backup.Products.Active)+len(backup.Products.Archived))
	}

	memberIDs := make(map[string]uint)
	var members []authentication.User
	if err := db.Where(util.QueryHouseholdId, householdID).Find(&members).Error; err != nil {
		return apiModel.RestoreResult{}, err
	}
	for i := range members {
		memberIDs[strings.ToLower(members[i].Username)] = members[i].ID
	}

	result := apiModel.RestoreResult{Version: apiModel.BackupSchemaVersion}
	transactionErr := db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&dbModel.ShoppingListItem{}).Where(util.QueryHouseholdId, householdID).Update("product_id", nil).Error; err != nil {
			return err
		}
		if err := tx.Model(&dbModel.RecipeIngredient{}).
			Where("recipe_id IN (?)", tx.Model(&dbModel.Recipe{}).Select("id").Where(util.QueryHouseholdId, householdID)).
			Update("product_id", nil).Error; err != nil {
			return err
		}
		if err := tx.Model(&dbModel.ExpiryScan{}).
			Where("product_id IN (?)", tx.Model(&dbModel.Product{}).Select("id").Where(util.QueryHouseholdId, householdID)).
			Update("product_id", nil).Error; err != nil {
			return err
		}
		if err := tx.Model(&dbModel.ActivityLog{}).Where(util.QueryHouseholdId, householdID).Update("product_id", 0).Error; err != nil {
			return err
		}
		if err := tx.Model(&dbModel.SavingsRecord{}).Where(util.QueryHouseholdId, householdID).Update("product_id", 0).Error; err != nil {
			return err
		}

		if err := tx.Unscoped().Where(util.QueryHouseholdId, householdID).Delete(&dbModel.Product{}).Error; err != nil {
			return err
		}
		if err := tx.Unscoped().Where(util.QueryHouseholdId, householdID).Delete(&dbModel.StorageLocation{}).Error; err != nil {
			return err
		}

		locationIDs, createdLocations, err := recreateBackupStorageLocations(tx, householdID, backup)
		if err != nil {
			return err
		}
		result.LocationsCreated = createdLocations

		if backup.Household != nil {
			updates := map[string]any{
				"name":                       backup.Household.Name,
				"description":                backup.Household.Description,
				"monthly_waste_goal_type":    backup.Household.MonthlyWasteGoalType,
				"monthly_waste_goal_count":   backup.Household.MonthlyWasteGoalCount,
				"monthly_waste_goal_percent": backup.Household.MonthlyWasteGoalPercent,
			}
			if err := tx.Model(&dbModel.Household{}).Where(util.QueryId, householdID).Updates(updates).Error; err != nil {
				return err
			}
		}

		products := make([]dbModel.Product, 0, len(backup.Products.Active)+len(backup.Products.Archived))
		for i := range backup.Products.Active {
			product, rowErr := restoreProductFromRow(&backup.Products.Active[i], false, householdID, memberIDs, adminUserID, locationIDs)
			if rowErr != nil {
				return rowErr
			}
			products = append(products, product)
		}
		for i := range backup.Products.Archived {
			product, rowErr := restoreProductFromRow(&backup.Products.Archived[i], true, householdID, memberIDs, adminUserID, locationIDs)
			if rowErr != nil {
				return rowErr
			}
			products = append(products, product)
		}
		if len(products) > 0 {
			// The canonical category of every restored product, resolved once
			// for the whole backup: the free text the backup carries names it.
			resolver, resolverErr := database.LoadCategoryResolver(tx)
			if resolverErr != nil {
				return resolverErr
			}
			defaults, defaultsErr := database.NewHouseholdCategoryRepository(tx).Defaults(householdID)
			if defaultsErr != nil {
				return defaultsErr
			}
			for i := range products {
				if products[i].CategoryKey == nil {
					if key, ok := resolver.ResolveKey(products[i].Categories); ok {
						products[i].CategoryKey = &key
					}
				}
				// A restored product that names no date gets the shelf life
				// the household gave its category, the same rule the create
				// paths follow: the restore re-establishes the household's
				// pantry, so its rules apply to it.
				database.ApplyHouseholdShelfLife(&products[i], defaults, products[i].CategoryKey)
			}
			if err := insertRestoreProducts(tx, restoreProductRowMaps(products)); err != nil {
				return err
			}
		}

		result.ProductsImported = len(backup.Products.Active)
		result.ProductsArchived = len(backup.Products.Archived)
		return nil
	})
	if transactionErr != nil {
		return apiModel.RestoreResult{}, transactionErr
	}

	result.Message = fmt.Sprintf("%d products restored (%d active, %d archived)", result.ProductsImported+result.ProductsArchived, result.ProductsImported, result.ProductsArchived)
	return result, nil
}

// restoreProductFromRow converts one exported row into a product model. The
// exported ids are ignored: a restore into another instance cannot reuse them,
// and the row id is bookkeeping rather than data. Location and owner references
// are resolved by name because both are instance-local.
func restoreProductFromRow(
	row *apiModel.ExportProductResponse,
	archived bool,
	householdID uint,
	memberIDs map[string]uint,
	adminUserID uint,
	locationIDs map[string]uint,
) (dbModel.Product, error) {
	if err := database.ValidateOpenedLifecycle(row.OpenedAt, row.DaysAfterOpening); err != nil {
		return dbModel.Product{}, err
	}
	if err := validateBackupBarcode(row.Barcode); err != nil {
		return dbModel.Product{}, err
	}
	quantity, packSize := restoreProductQuantity(row)
	product := dbModel.Product{
		Barcode:              row.Barcode,
		ProductName:          row.ProductName,
		Categories:           row.Categories,
		Countries:            row.Countries,
		ImageURL:             row.ImageURL,
		ExpireAt:             row.ExpireAt,
		ScannedAt:            row.ScannedAt,
		NotifiedAt:           row.NotifiedAt,
		IsPrivate:            row.IsPrivate,
		Quantity:             quantity,
		PackSize:             packSize,
		PriceOverride:        row.PriceOverride,
		CO2KgPerKg:           row.CO2KgPerKg,
		RemovalReason:        row.RemovalReason,
		WasteCause:           row.WasteCause,
		NotificationLeadDays: row.NotificationLeadDays,
		MinStockAmount:       row.MinStockAmount,
		OpenedAt:             row.OpenedAt,
		DaysAfterOpening:     row.DaysAfterOpening,
		HouseholdID:          householdID,
		UserID:               resolveBackupOwnerID(memberIDs, row.OwnerUsername, adminUserID),
	}
	product.CreatedAt = row.CreatedAt
	if row.StorageLocation != nil {
		locationName := strings.ToLower(strings.TrimSpace(row.StorageLocation.Name))
		if locationID, ok := locationIDs[locationName]; ok {
			product.StorageLocationID = &locationID
		}
	}
	if archived {
		archivedAt := time.Now()
		if row.ArchivedAt != nil {
			archivedAt = *row.ArchivedAt
		}
		product.DeletedAt = gorm.DeletedAt{Time: archivedAt, Valid: true}
	}
	return product, nil
}

// restoreProductQuantity reads the pack count and pack size of an exported
// row. A document written before products were counted in packs carries the
// net content instead, and its measured rows describe one pack of that size.
func restoreProductQuantity(row *apiModel.ExportProductResponse) (int, string) {
	if row.Quantity > 0 {
		return row.Quantity, row.PackSize
	}
	return util.PackSizeFromLegacyAmount(row.Amount, row.Unit)
}

// restoreProductColumns is the explicit column list used by the restore insert.
// Keeping it alongside restoreProductRowMaps makes a schema change fail loudly
// instead of silently dropping a field from restored products.
var restoreProductColumns = []string{
	"barcode",
	"product_name",
	"categories",
	"countries",
	"image_url",
	"expire_at",
	"scanned_at",
	"notified_at",
	"is_private",
	"quantity",
	"pack_size",
	"storage_location_id",
	"price_override",
	"co2_kg_per_kg",
	"removal_reason",
	"waste_cause",
	"notification_lead_days",
	"min_stock_amount",
	"opened_at",
	"days_after_opening",
	"category_key",
	"household_id",
	"user_id",
	"created_at",
	"updated_at",
	"deleted_at",
}

// restoreProductMaxParameters keeps one INSERT under SQLite's historical
// 999-variable ceiling even as the product schema grows.
const restoreProductMaxParameters = 900

// insertRestoreProducts writes restored products with explicit column batches.
// GORM's struct batch insert emits the SQL DEFAULT token for a zero-valued
// column with a DB default whenever another row carries a non-zero value, and
// SQLite rejects DEFAULT inside VALUES.
func insertRestoreProducts(tx *gorm.DB, rows []map[string]any) error {
	statement := &gorm.Statement{DB: tx}
	if err := statement.Parse(&dbModel.Product{}); err != nil {
		return err
	}
	tableName := statement.Schema.Table
	batchRows := max(restoreProductMaxParameters/len(restoreProductColumns), 1)
	for start := 0; start < len(rows); start += batchRows {
		end := min(start+batchRows, len(rows))

		var insert strings.Builder
		insert.WriteString("INSERT INTO ")
		insert.WriteString(tableName)
		insert.WriteString(" (")
		insert.WriteString(strings.Join(restoreProductColumns, ", "))
		insert.WriteString(") VALUES ")
		arguments := make([]any, 0, (end-start)*len(restoreProductColumns))
		for row := start; row < end; row++ {
			if row > start {
				insert.WriteString(", ")
			}
			insert.WriteString("(")
			for column := range restoreProductColumns {
				if column > 0 {
					insert.WriteString(", ")
				}
				insert.WriteString("?")
				value, exists := rows[row][restoreProductColumns[column]]
				if !exists {
					return fmt.Errorf("missing product column %q", restoreProductColumns[column])
				}
				arguments = append(arguments, value)
			}
			insert.WriteString(")")
		}

		if err := tx.Exec(insert.String(), arguments...).Error; err != nil {
			return err
		}
	}
	return nil
}

// restoreProductRowMaps converts restored products into explicit column maps.
// A map insert carries concrete zero values, so GORM cannot replace them with
// the SQL DEFAULT token that SQLite rejects inside a batched VALUES clause.
func restoreProductRowMaps(products []dbModel.Product) []map[string]any {
	rows := make([]map[string]any, 0, len(products))
	now := time.Now()
	for i := range products {
		product := &products[i]
		createdAt := product.CreatedAt
		if createdAt.IsZero() {
			createdAt = now
		}
		var deletedAt any
		if product.DeletedAt.Valid {
			deletedAt = product.DeletedAt.Time
		}
		rows = append(rows, map[string]any{
			"barcode":                product.Barcode,
			"product_name":           product.ProductName,
			"categories":             product.Categories,
			"countries":              product.Countries,
			"image_url":              product.ImageURL,
			"expire_at":              product.ExpireAt,
			"scanned_at":             product.ScannedAt,
			"notified_at":            product.NotifiedAt,
			"is_private":             product.IsPrivate,
			"quantity":               product.Quantity,
			"pack_size":              product.PackSize,
			"storage_location_id":    nullable(product.StorageLocationID),
			"price_override":         nullable(product.PriceOverride),
			"co2_kg_per_kg":          nullable(product.CO2KgPerKg),
			"removal_reason":         product.RemovalReason,
			"waste_cause":            product.WasteCause,
			"notification_lead_days": nullable(product.NotificationLeadDays),
			"min_stock_amount":       product.MinStockAmount,
			"opened_at":              nullable(product.OpenedAt),
			"days_after_opening":     nullable(product.DaysAfterOpening),
			"category_key":           nullable(product.CategoryKey),
			"household_id":           product.HouseholdID,
			"user_id":                product.UserID,
			"created_at":             createdAt,
			"updated_at":             now,
			"deleted_at":             deletedAt,
		})
	}
	return rows
}

func nullable[T any](value *T) any {
	if value == nil {
		return nil
	}
	return *value
}

// recreateBackupStorageLocations creates the document's storage locations and
// returns their new ids keyed by lowercased name. Product rows refer to
// locations by name because the exported ids are instance-local.
func recreateBackupStorageLocations(
	tx *gorm.DB,
	householdID uint,
	backup *apiModel.FullExportResponse,
) (map[string]uint, int, error) {
	type locationDraft struct {
		name      string
		icon      string
		sortOrder int
	}
	drafts := make([]locationDraft, 0, len(backup.StorageLocations))
	seen := make(map[string]bool, len(backup.StorageLocations))

	appendDraft := func(name, icon string, sortOrder int) {
		key := strings.ToLower(strings.TrimSpace(name))
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		drafts = append(drafts, locationDraft{name: strings.TrimSpace(name), icon: icon, sortOrder: sortOrder})
	}

	for i := range backup.StorageLocations {
		location := &backup.StorageLocations[i]
		appendDraft(location.Name, location.Icon, location.SortOrder)
	}
	for i := range backup.Products.Active {
		if location := backup.Products.Active[i].StorageLocation; location != nil {
			appendDraft(location.Name, location.Icon, location.SortOrder)
		}
	}
	for i := range backup.Products.Archived {
		if location := backup.Products.Archived[i].StorageLocation; location != nil {
			appendDraft(location.Name, location.Icon, location.SortOrder)
		}
	}
	if len(drafts) > util.BackupRestoreMaxStorageLocations {
		return nil, 0, fmt.Errorf("%w: %d", errors.ErrBackupTooManyLocations, len(drafts))
	}

	locations := make([]dbModel.StorageLocation, 0, len(drafts))
	for i := range drafts {
		icon := drafts[i].icon
		if icon == "" {
			icon = "📦"
		}
		locations = append(locations, dbModel.StorageLocation{
			HouseholdID: householdID,
			Name:        drafts[i].name,
			Icon:        icon,
			SortOrder:   drafts[i].sortOrder,
		})
	}
	if len(locations) == 0 {
		return map[string]uint{}, 0, nil
	}
	// One Create per row, not CreateInBatches: GORM's batch insert emits the
	// SQL DEFAULT token for a zero-valued column with a DB default whenever
	// another row in the batch carries a non-zero value, and SQLite rejects
	// DEFAULT inside VALUES. A restore routinely mixes sort orders and icons,
	// so batching it produces "near DEFAULT: syntax error".
	for i := range locations {
		if err := tx.Create(&locations[i]).Error; err != nil {
			return nil, 0, err
		}
	}

	ids := make(map[string]uint, len(locations))
	for i := range locations {
		ids[strings.ToLower(locations[i].Name)] = locations[i].ID
	}
	return ids, len(locations), nil
}

func validateBackupBarcode(barcode string) error {
	if len(barcode) > util.CsvImportMaxBarcodeLength {
		return fmt.Errorf("%w: invalid barcode length", errors.ErrBackupInvalidDocument)
	}
	if barcode == "" || !backupBarcodePattern.MatchString(barcode) {
		return fmt.Errorf("%w: invalid barcode characters", errors.ErrBackupInvalidDocument)
	}
	return nil
}

func resolveBackupOwnerID(memberIDs map[string]uint, ownerUsername string, fallback uint) uint {
	if id, ok := memberIDs[strings.ToLower(strings.TrimSpace(ownerUsername))]; ok {
		return id
	}
	return fallback
}
