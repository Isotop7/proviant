package database

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"

	"gorm.io/gorm"
)

type ProductRepository struct {
	DB *gorm.DB
}

func NewProductRepository(db *gorm.DB) *ProductRepository {
	return &ProductRepository{DB: db}
}

type SearchParameterEnum int

const (
	InvalidParameter SearchParameterEnum = iota
	ProductName
	Barcode
)

func SearchParameterEnumFromString(str string) SearchParameterEnum {
	switch str {
	case "product_name":
		return ProductName
	case "barcode":
		return Barcode
	default:
		return InvalidParameter
	}
}

type BulkOperationError struct {
	productID int
	error     error
}

func (b *BulkOperationError) Error() string {
	return fmt.Sprintf("Error bulk deleting product '%d', error: %v", b.productID, b.error)
}

func (r *ProductRepository) GetUserProductsBulk(userID uint, limit int) ([]database.Product, error) {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return []database.Product{}, err
	}

	if user.HouseholdID == 0 {
		return []database.Product{}, errors.ErrInvalidUserData
	}

	var products []database.Product
	query := r.DB.Where("household_id = ?", user.HouseholdID)

	if limit > 0 {
		query = query.Limit(limit)
	}

	queryErr := query.Find(&products).Error
	if queryErr != nil {
		return []database.Product{}, queryErr
	}
	return products, nil
}

func (r *ProductRepository) GetUserArchivedProductsBulk(userID uint, limit int) ([]database.Product, error) {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return []database.Product{}, err
	}

	if user.HouseholdID == 0 {
		return []database.Product{}, errors.ErrInvalidUserData
	}

	var products []database.Product
	query := r.DB.Unscoped().Where("deleted_at IS NOT NULL").Where("household_id = ?", user.HouseholdID)

	if limit > 0 {
		query = query.Limit(limit)
	}

	queryErr := query.Find(&products).Error
	if queryErr != nil {
		return []database.Product{}, queryErr
	}
	return products, nil
}

func (r *ProductRepository) GetUserProductsBulkByBarcode(userID uint, barcode int) ([]database.Product, error) {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return []database.Product{}, err
	}

	if user.HouseholdID == 0 {
		return []database.Product{}, errors.ErrInvalidUserData
	}

	var products []database.Product
	queryErr := r.DB.Where("household_id = ? and barcode = ?", user.HouseholdID, barcode).Find(&products).Error
	if queryErr != nil {
		return []database.Product{}, queryErr
	}
	return products, nil
}

func (r *ProductRepository) GetProductByID(productID int, userID uint) (database.Product, error) {
	if productID <= 0 {
		return database.Product{}, gorm.ErrNotImplemented
	}

	var product database.Product
	getError := r.DB.First(&product, productID)
	if getError.Error != nil {
		return database.Product{}, getError.Error
	}

	var user authentication.User
	userErr := r.DB.First(&user, userID)
	if userErr.Error != nil {
		return database.Product{}, userErr.Error
	}

	if product.HouseholdID != user.HouseholdID {
		return database.Product{}, errors.ErrMismatcherUserID
	}
	return product, nil
}

func (r *ProductRepository) GetArchivedProductByID(productID int, userID uint) (database.Product, error) {
	if productID <= 0 {
		return database.Product{}, gorm.ErrNotImplemented
	}

	var product database.Product
	getError := r.DB.Unscoped().First(&product, productID)
	if getError.Error != nil {
		return database.Product{}, getError.Error
	}

	var user authentication.User
	userErr := r.DB.First(&user, userID)
	if userErr.Error != nil {
		return database.Product{}, userErr.Error
	}

	if product.HouseholdID != user.HouseholdID {
		return database.Product{}, errors.ErrMismatcherUserID
	}
	return product, nil
}

func (r *ProductRepository) SearchProducts(queryParam SearchParameterEnum, queryValue, sortValue, orderValue string, userID uint) ([]database.Product, error) {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return []database.Product{}, err
	}

	var foundProducts []database.Product
	preloadedDataset := r.DB.
		Where("household_id = ?", user.HouseholdID).
		Where("deleted_at IS NULL")

	queryValue = fmt.Sprintf("%%%s%%", queryValue)

	switch queryParam {
	case ProductName:
		preloadedDataset = preloadedDataset.Where("product_name LIKE ?", queryValue)
	case Barcode:
		preloadedDataset = preloadedDataset.Where("barcode LIKE ?", queryValue)
	default:
		return []database.Product{}, errors.ErrDatabaseInvalidSearchParameter
	}

	if sortValue == "" {
		sortValue = "product_name"
	}
	if orderValue == "" {
		orderValue = "ASC"
	}
	preloadedDataset = preloadedDataset.Order(fmt.Sprintf("%s %s", sortValue, orderValue))

	findErr := preloadedDataset.Find(&foundProducts)
	if findErr.Error != nil {
		return []database.Product{}, findErr.Error
	}
	return foundProducts, nil
}

func (r *ProductRepository) CreateProduct(userID uint, product *database.Product) error {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return err
	}

	product.HouseholdID = user.HouseholdID
	createErr := r.DB.Create(&product)
	return createErr.Error
}

func (r *ProductRepository) UpdateProduct(productID int, userID uint, product *database.ProductDTOPatch) error {
	if productID <= 0 {
		return gorm.ErrNotImplemented
	}

	var dbProduct database.Product
	getError := r.DB.First(&dbProduct, productID)
	if getError.Error != nil {
		return getError.Error
	}

	var user authentication.User
	userErr := r.DB.First(&user, userID)
	if userErr.Error != nil {
		return userErr.Error
	}

	if dbProduct.HouseholdID != user.HouseholdID {
		return errors.ErrMismatcherUserID
	}

	dbProduct.ProductName = product.ProductName
	dbProduct.Categories = product.Categories
	dbProduct.Countries = product.Countries
	dbProduct.ImageURL = product.ImageURL
	dbProduct.ExpireAt = product.ExpireAt
	dbProduct.Amount = product.Amount
	dbProduct.Unit = product.Unit
	dbProduct.StorageLocation = product.StorageLocation

	saveResult := r.DB.Save(&dbProduct)
	return saveResult.Error
}

func (r *ProductRepository) UpdateProductAmount(productID int, userID uint, delta int) (bool, error) {
	if productID <= 0 {
		return false, gorm.ErrNotImplemented
	}

	var dbProduct database.Product
	getError := r.DB.First(&dbProduct, productID)
	if getError.Error != nil {
		return false, getError.Error
	}

	var user authentication.User
	userErr := r.DB.First(&user, userID)
	if userErr.Error != nil {
		return false, userErr.Error
	}

	if dbProduct.HouseholdID != user.HouseholdID {
		return false, errors.ErrMismatcherUserID
	}

	newAmount := dbProduct.Amount + delta
	if newAmount < 0 {
		newAmount = 0
	}
	dbProduct.Amount = newAmount

	if dbProduct.Amount <= 0 {
		deleteResult := r.DB.Unscoped().Delete(&database.Product{}, dbProduct.ID)
		return true, deleteResult.Error
	}

	saveResult := r.DB.Save(&dbProduct)
	if saveResult.Error != nil {
		return false, saveResult.Error
	}
	return false, nil
}

func (r *ProductRepository) DeleteProduct(productID int, userID uint, archiveOnly bool) error {
	_, getError := r.GetArchivedProductByID(productID, userID)
	if getError != nil {
		return getError
	}

	var deleteResult *gorm.DB
	if archiveOnly {
		deleteResult = r.DB.Delete(&database.Product{}, productID)
	} else {
		deleteResult = r.DB.Unscoped().Delete(&database.Product{}, productID)
	}
	return deleteResult.Error
}

func (r *ProductRepository) BulkDeleteProducts(productIDs []int, userID uint) []BulkOperationError {
	bulkErrors := []BulkOperationError{}
	for _, productID := range productIDs {
		_, getError := r.GetArchivedProductByID(productID, userID)
		if getError != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, getError})
		}
		deleteResult := r.DB.Unscoped().Delete(&database.Product{}, productID)
		if deleteResult.Error != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, getError})
		}
	}
	return bulkErrors
}

func (r *ProductRepository) BulkArchiveProducts(productIDs []int, userID uint) []BulkOperationError {
	bulkErrors := []BulkOperationError{}
	for _, productID := range productIDs {
		_, getError := r.GetArchivedProductByID(productID, userID)
		if getError != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, getError})
		}
		deleteResult := r.DB.Delete(&database.Product{}, productID)
		if deleteResult.Error != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, getError})
		}
	}
	return bulkErrors
}

func (r *ProductRepository) RestoreProduct(productID int, userID uint) error {
	product, getError := r.GetArchivedProductByID(productID, userID)
	if getError != nil {
		return getError
	}

	product.DeletedAt = gorm.DeletedAt{}

	saveResult := r.DB.Save(&product)
	return saveResult.Error
}

func (r *ProductRepository) BulkRestoreProducts(productIDs []int, userID uint) []BulkOperationError {
	bulkErrors := []BulkOperationError{}
	for _, productID := range productIDs {
		product, getError := r.GetArchivedProductByID(productID, userID)
		if getError != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, getError})
		}
		product.DeletedAt = gorm.DeletedAt{}
		saveResult := r.DB.Save(&product)
		if saveResult.Error != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, getError})
		}
	}
	return bulkErrors
}

func (r *ProductRepository) SetProductExpireAt(productID int, userID uint, expireAt database.Timestamp) error {
	var dbProduct database.Product
	getError := r.DB.First(&dbProduct, productID)
	if getError.Error != nil {
		return getError.Error
	}

	var user authentication.User
	userErr := r.DB.First(&user, userID)
	if userErr.Error != nil {
		return userErr.Error
	}

	if dbProduct.HouseholdID != user.HouseholdID {
		return errors.ErrMismatcherUserID
	}

	dbProduct.ExpireAt = time.Time(expireAt.Timestamp)
	saveResult := r.DB.Save(&dbProduct)
	return saveResult.Error
}

func (r *ProductRepository) SetProductNotifiedAt(productID uint) error {
	var dbProduct database.Product
	getError := r.DB.First(&dbProduct, productID)
	if getError.Error != nil {
		return getError.Error
	}

	dbProduct.NotifiedAt = time.Now()
	saveResult := r.DB.Save(&dbProduct)
	return saveResult.Error
}

func (r *ProductRepository) GetProductsExpired(userID uint) ([]*database.Product, error) {
	userProducts, getBulkErr := r.GetUserProductsBulk(userID, 0)
	if getBulkErr != nil {
		return []*database.Product{}, getBulkErr
	}

	var expiredProducts []*database.Product
	timestamp := time.Now()
	for idx := range userProducts {
		if userProducts[idx].ExpireAt.After(timestamp) {
			expiredProducts = append(expiredProducts, &userProducts[idx])
		}
	}
	return expiredProducts, nil
}

func (r *ProductRepository) GetExpiredProductsCount(userID uint) (int, error) {
	userProducts, getBulkErr := r.GetUserProductsBulk(userID, 0)
	if getBulkErr != nil {
		return 0, getBulkErr
	}

	count := 0
	timestamp := time.Now()
	for i := range userProducts {
		if userProducts[i].ExpireAt.Before(timestamp) {
			count++
		}
	}
	return count, nil
}

func (r *ProductRepository) GetArchivedProductsGroupedByBarcode(userID uint) (map[string]int, error) {
	archivedProducts, err := r.GetUserArchivedProductsBulk(userID, -1)
	if err != nil {
		return nil, err
	}

	grouped := make(map[string]int)
	for i := range archivedProducts {
		grouped[archivedProducts[i].Barcode]++
	}
	return grouped, nil
}

func (r *ProductRepository) GetTopArchivedProducts(userID uint, limit int) ([]database.Product, error) {
	archivedProducts, err := r.GetUserArchivedProductsBulk(userID, -1)
	if err != nil {
		return nil, err
	}

	if len(archivedProducts) == 0 {
		return []database.Product{}, nil
	}

	barcodeCounts := make(map[string]int)
	barcodeToProduct := make(map[string]database.Product)

	for i := range archivedProducts {
		product := &archivedProducts[i]
		barcodeCounts[product.Barcode]++
		if _, exists := barcodeToProduct[product.Barcode]; !exists {
			barcodeToProduct[product.Barcode] = *product
		}
	}

	type barcodeCount struct {
		barcode string
		count   int
		product database.Product
	}

	var counts []barcodeCount
	for barcode, count := range barcodeCounts {
		counts = append(counts, barcodeCount{
			barcode: barcode,
			count:   count,
			product: barcodeToProduct[barcode],
		})
	}

	sort.Slice(counts, func(i, j int) bool {
		return counts[i].count > counts[j].count
	})

	var result []database.Product
	for i := 0; i < len(counts) && i < limit; i++ {
		result = append(result, counts[i].product)
	}

	return result, nil
}

func (r *ProductRepository) GetActiveProductsCount(userID uint) (int, error) {
	products, err := r.GetUserProductsBulk(userID, 0)
	if err != nil {
		return 0, err
	}
	return len(products), nil
}

func (r *ProductRepository) GetProductCategoryBreakdown(userID uint) (map[string]int, error) {
	products, err := r.GetUserProductsBulk(userID, 0)
	if err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for i := range products {
		raw := strings.TrimSpace(products[i].Categories)
		if raw == "" {
			counts["Uncategorized"]++
			continue
		}
		first := strings.SplitN(raw, ",", 2)[0]
		first = strings.TrimSpace(first)
		if idx := strings.Index(first, ":"); idx != -1 {
			first = strings.TrimSpace(first[idx+1:])
		}
		if first == "" {
			first = "Uncategorized"
		}
		counts[first]++
	}

	const maxCategories = 8
	if len(counts) <= maxCategories {
		return counts, nil
	}

	type catCount struct {
		name  string
		count int
	}
	cats := make([]catCount, 0, len(counts))
	for n, c := range counts {
		cats = append(cats, catCount{n, c})
	}
	sort.Slice(cats, func(i, j int) bool {
		return cats[i].count > cats[j].count
	})

	result := make(map[string]int, maxCategories+1)
	other := 0
	for i, c := range cats {
		if i < maxCategories {
			result[c.name] = c.count
		} else {
			other += c.count
		}
	}
	if other > 0 {
		result["Other"] = other
	}
	return result, nil
}

func (r *ProductRepository) GetExpiryTrend(userID uint) ([]apiModel.StatsMonthlyCount, error) {
	products, err := r.GetUserProductsBulk(userID, 0)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	monthCounts := make(map[string]int, 12)
	for i := 0; i < 12; i++ {
		monthCounts[now.AddDate(0, i, 0).Format("2006-01")] = 0
	}
	for i := range products {
		month := products[i].ExpireAt.Format("2006-01")
		if _, ok := monthCounts[month]; ok {
			monthCounts[month]++
		}
	}

	result := make([]apiModel.StatsMonthlyCount, 0, 12)
	for i := 0; i < 12; i++ {
		month := now.AddDate(0, i, 0).Format("2006-01")
		result = append(result, apiModel.StatsMonthlyCount{Month: month, Count: monthCounts[month]})
	}
	return result, nil
}

func (r *ProductRepository) GetExpiringSoonProducts(userID uint, days int) ([]apiModel.StatsExpiringProduct, error) {
	products, err := r.GetUserProductsBulk(userID, 0)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfWindow := time.Date(now.Year(), now.Month(), now.Day()+days, 23, 59, 59, 999999999, now.Location())

	var result []apiModel.StatsExpiringProduct
	for i := range products {
		e := products[i].ExpireAt
		if !e.Before(startOfToday) && !e.After(endOfWindow) {
			result = append(result, apiModel.StatsExpiringProduct{
				ProductName: products[i].ProductName,
				ExpireAt:    e.Format("2006-01-02"),
			})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ExpireAt < result[j].ExpireAt
	})
	return result, nil
}

func (r *ProductRepository) GetLastNotifiedProduct(householdID uint) (database.Product, error) {
	var lastNotifiedProduct database.Product
	getNotifiedError := r.DB.
		Where("household_id = ?", householdID).
		Where("deleted_at IS NULL").
		Order("notified_at DESC").
		Limit(1).
		Find(&lastNotifiedProduct)

	return lastNotifiedProduct, getNotifiedError.Error
}

func (r *ProductRepository) GetExpiringInDays(userID uint, days int) ([]database.Product, error) {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return []database.Product{}, err
	}

	if user.HouseholdID == 0 {
		return []database.Product{}, errors.ErrInvalidUserData
	}

	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfWindow := time.Date(now.Year(), now.Month(), now.Day()+days, 23, 59, 59, 999999999, now.Location())

	var products []database.Product
	err := r.DB.
		Where("household_id = ?", user.HouseholdID).
		Where("deleted_at IS NULL").
		Where("expire_at >= ?", startOfToday).
		Where("expire_at <= ?", endOfWindow).
		Order("expire_at ASC").
		Find(&products).Error
	if err != nil {
		return []database.Product{}, err
	}
	return products, nil
}

func (r *ProductRepository) GetLastInsertedProduct(householdID uint) (database.Product, error) {
	var lastProduct database.Product
	getError := r.DB.
		Where("household_id = ?", householdID).
		Where("deleted_at IS NULL").
		Order("created_at DESC").
		Limit(1).
		Find(&lastProduct)

	return lastProduct, getError.Error
}

func (r *ProductRepository) UserHasProductAccess(userID uint, productID int) bool {
	if productID <= 0 {
		return false
	}

	var product database.Product
	getError := r.DB.Unscoped().First(&product, productID)
	if getError.Error != nil {
		return false
	}

	var user authentication.User
	getError = r.DB.First(&user, userID)
	if getError.Error != nil {
		return false
	}

	return user.HouseholdID == product.HouseholdID
}

func (r *ProductRepository) GetOpenFoodFactsCacheByBarcode(barcode string) (database.OpenFoodFactsCache, error) {
	var entry database.OpenFoodFactsCache
	result := r.DB.Where("barcode = ?", barcode).First(&entry)
	return entry, result.Error
}

func (r *ProductRepository) CreateOpenFoodFactsCache(entry *database.OpenFoodFactsCache) error {
	return r.DB.Create(entry).Error
}

func (r *ProductRepository) GetUserByID(userID uint) (authentication.User, error) {
	var user authentication.User
	selectErr := r.DB.First(&user, userID)
	return user, selectErr.Error
}

func (r *ProductRepository) GetUserHouseholdByID(userID uint) (uint, error) {
	var user authentication.User
	selectErr := r.DB.First(&user, userID)
	return user.HouseholdID, selectErr.Error
}

func (r *ProductRepository) GetHouseholdByID(householdID uint) (database.Household, error) {
	var household database.Household
	selectErr := r.DB.First(&household, householdID)
	return household, selectErr.Error
}

func (r *ProductRepository) GetUserActiveProductsFiltered(userID uint, from, to *time.Time) ([]database.Product, error) {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return []database.Product{}, err
	}

	if user.HouseholdID == 0 {
		return []database.Product{}, errors.ErrInvalidUserData
	}

	var products []database.Product
	query := r.DB.Where("household_id = ?", user.HouseholdID).Where("deleted_at IS NULL")

	if from != nil {
		query = query.Where("created_at >= ?", *from)
	}
	if to != nil {
		query = query.Where("created_at <= ?", *to)
	}

	queryErr := query.Find(&products).Error
	if queryErr != nil {
		return []database.Product{}, queryErr
	}
	return products, nil
}

func (r *ProductRepository) GetUserArchivedProductsFiltered(userID uint, from, to *time.Time) ([]database.Product, error) {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return []database.Product{}, err
	}

	if user.HouseholdID == 0 {
		return []database.Product{}, errors.ErrInvalidUserData
	}

	var products []database.Product
	query := r.DB.Unscoped().Where("deleted_at IS NOT NULL").Where("household_id = ?", user.HouseholdID)

	if from != nil {
		query = query.Where("deleted_at >= ?", *from)
	}
	if to != nil {
		query = query.Where("deleted_at <= ?", *to)
	}

	queryErr := query.Find(&products).Error
	if queryErr != nil {
		return []database.Product{}, queryErr
	}
	return products, nil
}

func (r *ProductRepository) GetUsersByHouseholdID(householdID uint) ([]authentication.User, error) {
	var users []authentication.User
	err := r.DB.Where("household_id = ?", householdID).Find(&users).Error
	return users, err
}

type CalendarTokenRepository struct {
	DB *gorm.DB
}

func NewCalendarTokenRepository(db *gorm.DB) *CalendarTokenRepository {
	return &CalendarTokenRepository{DB: db}
}

func (r *CalendarTokenRepository) GetByToken(token string) (authentication.CalendarToken, error) {
	var ct authentication.CalendarToken
	err := r.DB.Where("token = ?", token).First(&ct).Error
	return ct, err
}

func (r *CalendarTokenRepository) DeleteByUserID(userID uint) error {
	return r.DB.Where("user_id = ?", userID).Delete(&authentication.CalendarToken{}).Error
}

func (r *CalendarTokenRepository) GetByUserID(userID uint) (authentication.CalendarToken, error) {
	var ct authentication.CalendarToken
	err := r.DB.Where("user_id = ?", userID).First(&ct).Error
	return ct, err
}

func (r *CalendarTokenRepository) Create(ct *authentication.CalendarToken) error {
	return r.DB.Create(ct).Error
}

var _ = (*ProductRepository)(nil)
