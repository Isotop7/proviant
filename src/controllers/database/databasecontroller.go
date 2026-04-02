package database

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/models/webparts"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// DatabaseController is the object struct for interacting with the gorm-backed database
type DatabaseController struct {
	DBHandle *gorm.DB
}

// SearchParameterEnum is a int value specifying a valid search parameter
type SearchParameterEnum int

const (
	InvalidParameter SearchParameterEnum = iota
	ProductName
	Barcode
)

// SearchParameterEnumFromString parses and converts a given string to the matching enum value
// If the enum value can't be matched, enum value 'InvalidParameter' is used
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

// BulkOperationError is an error type for bulk operations
type BulkOperationError struct {
	productID int
	error     error
}

// PreferredTimeFormat is the preferred time format for database operations
const PreferredTimeFormat = "02.01.2006 15:04"

const GeneratedPrefix = "Generated @ %s"

// Error returns a string representation of the error
func (b *BulkOperationError) Error() string {
	return fmt.Sprintf("Error bulk deleting product '%d', error: %v", b.productID, b.error)
}

// GetUserByUsername uses a given username and returns the matching user object
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) GetUserByUsername(username string) (authentication.User, error) {
	var user authentication.User
	// Gets first user with matching username
	selectErr := dbc.DBHandle.First(&user, "username = ?", username)
	return user, selectErr.Error
}

// GetUserByID uses a given user ID and returns the matching user object
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) GetUserByID(userID uint) (authentication.User, error) {
	var user authentication.User
	// Gets first user with matching id
	selectErr := dbc.DBHandle.First(&user, userID)
	return user, selectErr.Error
}

// GetUserHouseholdByID uses a given user ID and returns the connected household id
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) GetUserHouseholdByID(userID uint) (uint, error) {
	var user authentication.User
	// Gets first user with matching id
	selectErr := dbc.DBHandle.First(&user, userID)
	return user.HouseholdID, selectErr.Error
}

// GetHouseholdByID uses a given household ID and returns the matching household object
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) GetHouseholdByID(householdID uint) (database.Household, error) {
	var household database.Household
	// Gets first household with matching id
	selectErr := dbc.DBHandle.First(&household, householdID)
	return household, selectErr.Error
}

// UserExistsByUsername returns if a given user object exists in the database based on the property 'username'
func (dbc DatabaseController) UserExistsByUsername(user *authentication.User) bool {
	var dbUser authentication.User
	// Try to get first object with matching username
	selectErr := dbc.DBHandle.First(&dbUser, "username = ?", user.Username)
	// If no user is found, return false
	return selectErr.Error != gorm.ErrRecordNotFound
}

// UserExistsByMailAddress returns if a given user object exists in the database based on the property 'mailAddress'
func (dbc DatabaseController) UserExistsByMailAddress(user *authentication.User) bool {
	var dbUser authentication.User
	// Try to get first object with matching mailAddress
	selectErr := dbc.DBHandle.First(&dbUser, "mail_address = ?", user.MailAddress)
	// If no user is found, return false
	return selectErr.Error != gorm.ErrRecordNotFound
}

// GetNextUserID returns the next available user ID
func (dbc DatabaseController) GetNextUserID() uint {
	// Get next user id from database
	var maxID uint
	dbc.DBHandle.Model(&authentication.User{}).Select("MAX(id)").Scan(&maxID)
	return (maxID + 1)
}

// CreateUser creates a new user based on a given user object
// Before creation, the user password is hashed with brcypt
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) CreateUser(user *authentication.User) error {
	// Start a database transaction
	tx := dbc.DBHandle.Begin()

	// Create a new household
	household := database.Household{
		Name: fmt.Sprintf("%s's Household", user.Username),
	}
	if err := tx.Create(&household).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Create new database user
	hashedPassword, hashError := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if hashError != nil {
		return hashError
	}
	user.Password = string(hashedPassword)
	user.HouseholdID = household.ID
	if err := tx.Create(&user).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Set created user as household admin
	if err := tx.Model(&household).Update("admin_id", user.ID).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Commit the transaction
	tx.Commit()
	return nil
}

// UpdateUser gets a user (based on user ID) and updates its contents with the contents of a supplied reference to the updated user
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) UpdateUser(userID uint, user *authentication.User) error {
	// Check if id is valid
	if userID <= 0 {
		return gorm.ErrNotImplemented
	}

	// Try to get user
	var dbUser authentication.User
	getError := dbc.DBHandle.First(&dbUser, userID)

	// If database operation returned error, return it to the caller
	if getError.Error != nil {
		return getError.Error
	}
	// Check if supplied user matches the userID in the database object
	if dbUser.ID != userID {
		return errors.ErrMismatcherUserID
	}

	// Update values
	dbUser.Username = user.Username
	dbUser.MailAddress = user.MailAddress
	dbUser.NotificationPreferences = user.NotificationPreferences

	// Save updated product
	saveResult := dbc.DBHandle.Save(&dbUser)
	// Return error if save did not work
	if saveResult.Error != nil {
		return saveResult.Error
	} else {
		return nil
	}
}

// UpdateUserPassword gets a user (based on user ID) and updates its password with the contents of a supplied reference to the updated login data
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) UpdateUserPassword(userID uint, login *authentication.Login) error {
	// Check if id is valid
	if userID <= 0 {
		return gorm.ErrNotImplemented
	}

	// Try to get user
	var dbUser authentication.User
	getError := dbc.DBHandle.First(&dbUser, userID)

	// If database operation returned error, return it to the caller
	if getError.Error != nil {
		return getError.Error
	}
	// Check if supplied user matches the userID in the database object
	if dbUser.ID != userID {
		return errors.ErrMismatcherUserID
	}

	// Check if login is equivalent to database user
	if dbUser.Username != login.Username {
		return errors.ErrMismatchedUsername
	}

	// Generate hash from password
	hashedPassword, hashError := bcrypt.GenerateFromPassword([]byte(login.Password), bcrypt.DefaultCost)
	if hashError != nil {
		return hashError
	}
	// Set password on database user
	dbUser.Password = string(hashedPassword)

	// Save updated user
	saveResult := dbc.DBHandle.Save(&dbUser)
	// Return error if save did not work
	if saveResult.Error != nil {
		return saveResult.Error
	} else {
		return nil
	}
}

// UserHasProductAccess checks if user (based on user ID) is the matching owner of a product (based on product ID)
func (dbc DatabaseController) UserHasProductAccess(userID uint, productID int) bool {
	// Check for invalid product IDs
	if productID <= 0 {
		return false
	}

	// Get single product by ID
	var product database.Product
	getError := dbc.DBHandle.Unscoped().First(&product, productID)
	// Failsafe - If error is found, return false
	if getError.Error != nil {
		return false
	}

	// Get user by ID
	var user authentication.User
	getError = dbc.DBHandle.First(&user, userID)
	// Failsafe - If error is found, return false
	if getError.Error != nil {
		return false
	}

	return user.HouseholdID == product.HouseholdID
}

// GetHouseholdMembersMailAddressesByID returns the mail addresses of all users of a household
func (dbc DatabaseController) GetHouseholdMembersMailAddressesByID(householdID uint) ([]string, error) {
	var mailAddresses []string
	// Check for household
	_, householdErr := dbc.GetHouseholdByID(householdID)
	// Check for database error
	if householdErr != nil {
		return mailAddresses, householdErr
	}

	// Find users with matching id
	var users []*authentication.User
	findErr := dbc.DBHandle.Where("household_id = ?", householdID).Find(&users)
	if findErr != nil {
		return mailAddresses, findErr.Error
	}

	// Loop through household members and add mail addresses
	for idx := range users {
		mailAddresses = append(mailAddresses, users[idx].MailAddress)
	}
	return mailAddresses, nil
}

// GetHouseholdMembersNotificationPreferences returns the notification preferences of all users of a household
func (dbc DatabaseController) GetHouseholdMembersNotificationPreferences(householdID uint) ([]models.NotificationRecipientInfo, error) {
	var preferences []models.NotificationRecipientInfo

	// Check for household
	_, householdErr := dbc.GetHouseholdByID(householdID)
	if householdErr != nil {
		return preferences, householdErr
	}

	// Find users with matching household ID
	var users []*authentication.User
	findErr := dbc.DBHandle.Where("household_id = ?", householdID).Find(&users)
	if findErr.Error != nil {
		return preferences, findErr.Error
	}

	// Loop through household members and collect notification preferences
	for idx := range users {
		user := users[idx]
		preferences = append(preferences, models.NotificationRecipientInfo{
			EmailAddress: user.MailAddress,
			NtfyURL:      user.NotificationPreferences.NtfyURL,
			NtfyTopic:    user.NotificationPreferences.NtfyTopic,
			NtfyToken:    user.NotificationPreferences.NtfyToken,
		})
	}

	return preferences, nil
}

// GetUserProductsBulk returns an array of products of a user (based on user ID)
// The returned dataset can be limitied by supplying 'limit'
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) GetUserProductsBulk(userID uint, limit int) ([]database.Product, error) {
	// Get user object from database
	user, userErr := dbc.GetUserByID(userID)
	if userErr != nil {
		return []database.Product{}, userErr
	}

	if user.HouseholdID == 0 {
		return []database.Product{}, errors.ErrInvalidUserData
	}

	// Get household with products preloaded
	var products []database.Product
	query := dbc.DBHandle.Where("household_id = ?", user.HouseholdID)

	// Apply limit if specified
	if limit > 0 {
		query = query.Limit(limit)
	}

	// Execute query
	queryErr := query.Find(&products).Error
	if queryErr != nil {
		return []database.Product{}, queryErr
	}
	return products, nil
}

// GetUserArchivedProductsBulk returns an array of archived products of a user (based on user ID)
// The returned dataset can be limitied by supplying 'limit'
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) GetUserArchivedProductsBulk(userID uint, limit int) ([]database.Product, error) {
	// Get user object from database
	user, userErr := dbc.GetUserByID(userID)
	if userErr != nil {
		return []database.Product{}, userErr
	}

	if user.HouseholdID == 0 {
		return []database.Product{}, errors.ErrInvalidUserData
	}

	// Get household with products preloaded
	var products []database.Product
	query := dbc.DBHandle.Unscoped().Where("deleted_at IS NOT NULL").Where("household_id = ?", user.HouseholdID)

	// Apply limit if specified
	if limit > 0 {
		query = query.Limit(limit)
	}

	// Execute query
	queryErr := query.Find(&products).Error
	if queryErr != nil {
		return []database.Product{}, queryErr
	}
	return products, nil
}

// GetUserProductsBulkByBarcode returns an array of products of a user (based on user ID) matching a barcode
// The returned dataset can be limitied by supplying 'limit'
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) GetUserProductsBulkByBarcode(userID uint, barcode int) ([]database.Product, error) {
	// Get user object from database
	user, userErr := dbc.GetUserByID(userID)
	if userErr != nil {
		return []database.Product{}, userErr
	}

	if user.HouseholdID == 0 {
		return []database.Product{}, errors.ErrInvalidUserData
	}

	// Get household with products preloaded
	var products []database.Product
	query := dbc.DBHandle.Where("household_id = ? and barcode = ?", user.HouseholdID, barcode)

	// Execute query
	queryErr := query.Find(&products).Error
	if queryErr != nil {
		return []database.Product{}, queryErr
	}
	return products, nil
}

// GetProductByID returns a product object (based on product ID) of a user (based on user ID)
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) GetProductByID(productID int, userID uint) (database.Product, error) {
	// Get single product by ID
	if productID <= 0 {
		return database.Product{}, gorm.ErrNotImplemented
	}
	// Parse product to var
	var product database.Product
	getError := dbc.DBHandle.First(&product, productID)

	// Check if error occured while getting product
	if getError.Error != nil {
		// Return empty set and database error
		return database.Product{}, getError.Error
	}

	// Get user object from database
	user, userErr := dbc.GetUserByID(userID)
	if userErr != nil {
		return database.Product{}, userErr
	}

	// Check if household id of the product is different than household id of the user
	if product.HouseholdID != user.HouseholdID {
		// Return empty set and custom error
		return database.Product{}, errors.ErrMismatcherUserID
	}
	// Return database product
	return product, nil
}

// GetArchivedProductByID returns an archived product object (based on product ID) of a user (based on user ID)
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) GetArchivedProductByID(productID int, userID uint) (database.Product, error) {
	// Get single product by ID
	if productID <= 0 {
		return database.Product{}, gorm.ErrNotImplemented
	}
	// Parse product to var
	var product database.Product
	getError := dbc.DBHandle.Unscoped().First(&product, productID)

	// Check if error occured while getting product
	if getError.Error != nil {
		// Return empty set and database error
		return database.Product{}, getError.Error
	}

	// Get user object from database
	user, userErr := dbc.GetUserByID(userID)
	if userErr != nil {
		return database.Product{}, userErr
	}

	// Check if household id of the product is different than household id of the user
	if product.HouseholdID != user.HouseholdID {
		// Return empty set and custom error
		return database.Product{}, errors.ErrMismatcherUserID
	}
	// Return database product
	return product, nil
}

// SearchProducts returns an array of products of a user matching a search paramater and a query
func (dbc DatabaseController) SearchProducts(queryParam SearchParameterEnum, queryValue, sortValue, orderValue string, userID uint) ([]database.Product, error) {
	// Get user object from database
	user, userErr := dbc.GetUserByID(userID)
	if userErr != nil {
		return []database.Product{}, userErr
	}

	// Get all user products
	var foundProducts []database.Product
	preloadedDataset := dbc.DBHandle.
		Where("household_id = ?", user.HouseholdID).
		Where("deleted_at IS NULL")

	// Transform search query
	queryValue = fmt.Sprintf("%%%s%%", queryValue)

	// Get matching products of preloaded set based on search parameter
	switch queryParam {
	case ProductName:
		preloadedDataset = preloadedDataset.Where("product_name LIKE ?", queryValue)
	case Barcode:
		preloadedDataset = preloadedDataset.Where("barcode LIKE ?", queryValue)
	default:
		return []database.Product{}, errors.ErrDatabaseInvalidSearchParameter
	}

	// Order dataset
	if sortValue == "" {
		sortValue = "product_name"
	}
	if orderValue == "" {
		orderValue = "ASC"
	}
	preloadedDataset = preloadedDataset.Order(fmt.Sprintf("%s %s", sortValue, orderValue))

	// Cast found set to returned array or return error
	findErr := preloadedDataset.Find(&foundProducts)
	if findErr.Error != nil {
		return []database.Product{}, findErr.Error
	} else {
		return foundProducts, nil
	}
}

// CreateProduct creates a product in the database and connects it to the user
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) CreateProduct(userID uint, product *database.Product) error {
	// Get user object from database
	user, userErr := dbc.GetUserByID(userID)
	if userErr != nil {
		return userErr
	}

	// Set household id
	product.HouseholdID = user.HouseholdID
	// Create new product
	createErr := dbc.DBHandle.Create(&product)
	return createErr.Error
}

// UpdateProduct gets a product (based on product ID) of a user (based on user ID) and updates its contents with the contents of a supplied reference to the updated product
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) UpdateProduct(productID int, userID uint, product *database.ProductDTOPatch) error {
	// Check if id is valid
	if productID <= 0 {
		return gorm.ErrNotImplemented
	}

	// Try to get product
	var dbProduct database.Product
	getError := dbc.DBHandle.First(&dbProduct, productID)

	// If database operation returned error, return it to the caller
	if getError.Error != nil {
		return getError.Error
	}

	// Try to get user object from database
	user, userErr := dbc.GetUserByID(userID)
	if userErr != nil {
		return userErr
	}

	// Check if supplied user is allowed to patch the product
	if dbProduct.HouseholdID != user.HouseholdID {
		return errors.ErrMismatcherUserID
	}

	// Update values
	dbProduct.ProductName = product.ProductName
	dbProduct.Categories = product.Categories
	dbProduct.Countries = product.Countries
	dbProduct.ImageURL = product.ImageURL
	dbProduct.ExpireAt = product.ExpireAt

	// Save updated product
	saveResult := dbc.DBHandle.Save(&dbProduct)
	// Return error if save did not work
	if saveResult.Error != nil {
		return saveResult.Error
	} else {
		return nil
	}
}

// DeleteProduct deletes a product (based on product ID) of a user (based on user ID)
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) DeleteProduct(productID int, userID uint, archiveOnly bool) error {
	// Get product and check for correct userID
	_, getError := dbc.GetArchivedProductByID(productID, userID)
	if getError != nil {
		return getError
	}

	var deleteResult *gorm.DB
	if archiveOnly {
		// Archive product by its id
		deleteResult = dbc.DBHandle.Delete(&database.Product{}, productID)
	} else {
		// Delete product by its id
		deleteResult = dbc.DBHandle.Unscoped().Delete(&database.Product{}, productID)
	}
	return deleteResult.Error
}

// BulkDeleteProducts deletes a list of products (based on product ID) of a user (based on user ID) given as a slice of product IDs
// If the database operations return an error, the error is added to a wrapper slice which is returned at the end of the function
func (dbc DatabaseController) BulkDeleteProducts(productIDs []int, userID uint) []BulkOperationError {
	bulkErrors := []BulkOperationError{}
	for _, productID := range productIDs {
		// Get product and check for correct userID
		_, getError := dbc.GetArchivedProductByID(productID, userID)
		if getError != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, getError})
		}
		deleteResult := dbc.DBHandle.Unscoped().Delete(&database.Product{}, productID)
		if deleteResult.Error != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, getError})
		}
	}
	return bulkErrors
}

// BulkDeleteProducts deletes a list of products (based on product ID) of a user (based on user ID) given as a slice of product IDs
// If the database operations return an error, the error is added to a wrapper slice which is returned at the end of the function
func (dbc DatabaseController) BulkArchiveProducts(productIDs []int, userID uint) []BulkOperationError {
	bulkErrors := []BulkOperationError{}
	for _, productID := range productIDs {
		// Get product and check for correct userID
		_, getError := dbc.GetArchivedProductByID(productID, userID)
		if getError != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, getError})
		}
		deleteResult := dbc.DBHandle.Delete(&database.Product{}, productID)
		if deleteResult.Error != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, getError})
		}
	}
	return bulkErrors
}

// RestoreProduct restores a product (based on product ID) of a user (based on user ID)
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) RestoreProduct(productID int, userID uint) error {
	// Get product and check for correct userID
	product, getError := dbc.GetArchivedProductByID(productID, userID)
	if getError != nil {
		return getError
	}

	// Reset deletedAt field
	product.DeletedAt = gorm.DeletedAt{}

	// Save changes to db
	saveResult := dbc.DBHandle.Save(&product)
	return saveResult.Error
}

// BulkRestoreProducts restores a list of products (based on product ID) of a user (based on user ID) given as a slice of product IDs
// If the database operations return an error, the error is added to a wrapper slice which is returned at the end of the function
func (dbc DatabaseController) BulkRestoreProducts(productIDs []int, userID uint) []BulkOperationError {
	bulkErrors := []BulkOperationError{}
	for _, productID := range productIDs {
		// Get product and check for correct userID
		product, getError := dbc.GetArchivedProductByID(productID, userID)
		if getError != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, getError})
		}
		// Reset deletedAt field
		product.DeletedAt = gorm.DeletedAt{}
		// Save changes to db
		saveResult := dbc.DBHandle.Save(&product)
		if saveResult.Error != nil {
			bulkErrors = append(bulkErrors, BulkOperationError{productID, getError})
		}
	}
	return bulkErrors
}

// SetProductExpireAt updates the expiry date of a product (based on product ID) of a user (based on user ID)
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) SetProductExpireAt(productID int, userID uint, expireAt database.Timestamp) error {
	// Update ExpireAt date
	var dbProduct database.Product
	getError := dbc.DBHandle.First(&dbProduct, productID)
	if getError.Error != nil {
		return getError.Error
	}

	// Try to get user object from database
	user, userErr := dbc.GetUserByID(userID)
	if userErr != nil {
		return userErr
	}

	// Check if supplied user is allowed to update product
	if dbProduct.HouseholdID != user.HouseholdID {
		return errors.ErrMismatcherUserID
	}

	// Update values
	dbProduct.ExpireAt = time.Time(expireAt.Timestamp)
	saveResult := dbc.DBHandle.Save(&dbProduct)
	if saveResult.Error != nil {
		return saveResult.Error
	} else {
		return nil
	}
}

// SetProductNotifiedAt sets the notified_at timestamp to the current time
func (dbc DatabaseController) SetProductNotifiedAt(productID uint) error {
	// Update NotifiedAt date
	var dbProduct database.Product
	getError := dbc.DBHandle.First(&dbProduct, productID)
	if getError.Error != nil {
		return getError.Error
	}

	// Update value or return error
	dbProduct.NotifiedAt = time.Now()
	saveResult := dbc.DBHandle.Save(&dbProduct)
	if saveResult.Error != nil {
		return saveResult.Error
	} else {
		return nil
	}
}

// GetProductsExpired returns an array of products of a user (based on user ID) that are already expired
// If the database operations return an error, the error is also returned (otherwise nil)
// If the user has no products assigned, the function returns an empty dataset
func (dbc DatabaseController) GetProductsExpired(userID uint) ([]*database.Product, error) {
	// Get all user products
	userProducts, getBulkErr := dbc.GetUserProductsBulk(userID, 0)
	if getBulkErr != nil {
		return []*database.Product{}, getBulkErr
	}

	// Get all currently expired products
	var expiredProducts []*database.Product
	timestamp := time.Now()
	for idx := range userProducts {
		if userProducts[idx].ExpireAt.After(timestamp) {
			expiredProducts = append(expiredProducts, &userProducts[idx])
		}
	}
	return expiredProducts, nil
}

// GetProductsExpiredAndNotificationPending returns an array of products which are expired and have a pending notification
func (dbc DatabaseController) GetProductsExpiredAndNotificationPending(sleepInterval time.Duration) ([]database.Product, error) {
	// Get products with pending notification
	var notificationProducts []database.Product
	// Get expired products with pending notification
	getError := dbc.DBHandle.
		Where("expire_at > ?", time.Time{}).
		Where("expire_at < ?", time.Now()).
		Where("notified_at < ?", time.Now().Add(-(sleepInterval))).
		Find(&notificationProducts)

	// Check for error or return product list
	if getError.Error != nil {
		return []database.Product{}, getError.Error
	} else {
		return notificationProducts, nil
	}
}

// GetLastNotifiedProduct returns the last notified product for a user
func (dbc DatabaseController) GetLastNotifiedProduct(householdID uint) (database.Product, error) {
	var lastNotifiedProduct database.Product
	getNotifiedError := dbc.DBHandle.
		Where("household_id = ?", householdID).
		Where("deleted_at IS NULL").
		Order("notified_at DESC").
		Limit(1).
		Find(&lastNotifiedProduct)

	return lastNotifiedProduct, getNotifiedError.Error
}

func (dbc DatabaseController) GetLastInsertedProduct(householdID uint) (database.Product, error) {
	var lastProduct database.Product
	getError := dbc.DBHandle.
		Where("household_id = ?", householdID).
		Where("deleted_at IS NULL").
		Order("created_at DESC").
		Limit(1).
		Find(&lastProduct)

	return lastProduct, getError.Error
}

// GetExpiredProductsCount returns the count of expired products for a user
func (dbc DatabaseController) GetExpiredProductsCount(userID uint) (int, error) {
	userProducts, getBulkErr := dbc.GetUserProductsBulk(userID, 0)
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

// GetArchivedProductsGroupedByBarcode returns archived products grouped by barcode with counts
func (dbc DatabaseController) GetArchivedProductsGroupedByBarcode(userID uint) (map[string]int, error) {
	archivedProducts, err := dbc.GetUserArchivedProductsBulk(userID, -1)
	if err != nil {
		return nil, err
	}

	grouped := make(map[string]int)
	for i := range archivedProducts {
		grouped[archivedProducts[i].Barcode]++
	}
	return grouped, nil
}

// GetTopArchivedProducts returns the top N most frequently archived products
func (dbc DatabaseController) GetTopArchivedProducts(userID uint, limit int) ([]database.Product, error) {
	archivedProducts, err := dbc.GetUserArchivedProductsBulk(userID, -1)
	if err != nil {
		return nil, err
	}

	if len(archivedProducts) == 0 {
		return []database.Product{}, nil
	}

	// Count occurrences by barcode and store first product occurrence
	barcodeCounts := make(map[string]int)
	barcodeToProduct := make(map[string]database.Product)

	for i := range archivedProducts {
		product := &archivedProducts[i]
		barcodeCounts[product.Barcode]++
		// Store the first occurrence of each barcode
		if _, exists := barcodeToProduct[product.Barcode]; !exists {
			barcodeToProduct[product.Barcode] = *product
		}
	}

	// Create a slice of structs to sort
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

	// Sort by count (descending)
	sort.Slice(counts, func(i, j int) bool {
		return counts[i].count > counts[j].count
	})

	// Get top N products
	var result []database.Product
	for i := 0; i < len(counts) && i < limit; i++ {
		result = append(result, counts[i].product)
	}

	return result, nil
}

// GetUserHomeTiles creates a list of tiles with user statistics
func (dbc DatabaseController) GetUserHomeTiles(userID uint) ([]webparts.Tile, error) {
	// Create list of hometiles
	homeTiles := []webparts.Tile{}

	// Try to get user object from database
	user, userErr := dbc.GetUserByID(userID)
	if userErr != nil {
		return homeTiles, userErr
	}

	// Get count of products
	productList, productCountErr := dbc.GetUserProductsBulk(userID, -1)
	if productCountErr == nil && len(productList) > 0 {
		homeTiles = append(homeTiles, webparts.Tile{
			Title:  "Amount of your products",
			Hero:   fmt.Sprint(len(productList)),
			Body:   fmt.Sprintf("You currently have %d products assigned", len(productList)),
			Footer: fmt.Sprintf(GeneratedPrefix, time.Now().Format(PreferredTimeFormat)),
		})
	}

	// Get last inserted product
	lastInsertedProduct, getLastInsertedProductErr := dbc.GetLastInsertedProduct(user.HouseholdID)
	if getLastInsertedProductErr == nil && lastInsertedProduct.ID != 0 {
		homeTiles = append(homeTiles, webparts.Tile{
			Title:  "Last inserted product",
			Hero:   lastInsertedProduct.ProductName,
			Body:   fmt.Sprintf("'%s' is the most recent product with barcode #%s", lastInsertedProduct.ProductName, lastInsertedProduct.Barcode),
			Footer: fmt.Sprintf(GeneratedPrefix, time.Now().Format(PreferredTimeFormat)),
		})
	}

	// Last notified product
	lastNotifiedProduct, getNotifiedErr := dbc.GetLastNotifiedProduct(user.HouseholdID)
	if getNotifiedErr == nil && lastNotifiedProduct.ID != 0 {
		homeTiles = append(homeTiles, webparts.Tile{
			Title:  "Last notification",
			Hero:   lastNotifiedProduct.NotifiedAt.Format(PreferredTimeFormat),
			Body:   fmt.Sprintf("You received the last notfication for product with barcode #%s at %s", lastNotifiedProduct.Barcode, lastNotifiedProduct.NotifiedAt.Format(PreferredTimeFormat)),
			Footer: fmt.Sprintf(GeneratedPrefix, time.Now().Format(PreferredTimeFormat)),
		})
	}

	// Percentage of unopened expired products
	activeProducts, activeErr := dbc.GetUserProductsBulk(userID, -1)
	if activeErr == nil && len(activeProducts) > 0 {
		expiredCount, expiredErr := dbc.GetExpiredProductsCount(userID)
		if expiredErr == nil && expiredCount > 0 {
			percentage := float64(expiredCount) / float64(len(activeProducts)) * 100
			homeTiles = append(homeTiles, webparts.Tile{
				Title:   "Unopened Expired Products",
				Hero:    fmt.Sprintf("%.1f%%", percentage),
				Body:    fmt.Sprintf("%.1f%% of your %d active products are expired but not archived", percentage, len(activeProducts)),
				Footer:  fmt.Sprintf(GeneratedPrefix, time.Now().Format(PreferredTimeFormat)),
				Variant: "danger",
			})
		}
	}

	// Number of total archived products
	archivedProducts, archivedErr := dbc.GetUserArchivedProductsBulk(userID, -1)
	if archivedErr == nil {
		homeTiles = append(homeTiles, webparts.Tile{
			Title:  "Total Archived Products",
			Hero:   fmt.Sprint(len(archivedProducts)),
			Body:   fmt.Sprintf("You have archived %d products", len(archivedProducts)),
			Footer: fmt.Sprintf(GeneratedPrefix, time.Now().Format(PreferredTimeFormat)),
		})
	}

	// Number of total archived products grouped by barcode
	grouped, groupErr := dbc.GetArchivedProductsGroupedByBarcode(userID)
	if groupErr == nil {
		homeTiles = append(homeTiles, webparts.Tile{
			Title:  "Unique Archived Products",
			Hero:   fmt.Sprint(len(grouped)),
			Body:   fmt.Sprintf("%d different products have been archived", len(grouped)),
			Footer: fmt.Sprintf(GeneratedPrefix, time.Now().Format(PreferredTimeFormat)),
		})
	}

	// Top 3 archived products
	topArchived, topErr := dbc.GetTopArchivedProducts(userID, 3)
	if topErr == nil && len(topArchived) > 0 {
		var productNames []string
		for i := range topArchived {
			productNames = append(productNames, topArchived[i].ProductName)
		}

		homeTiles = append(homeTiles, webparts.Tile{
			Title:  "Top Archived Products",
			Hero:   strings.Join(productNames, ", "),
			Body:   fmt.Sprintf("Your most archived products: %s", strings.Join(productNames, ", ")),
			Footer: fmt.Sprintf(GeneratedPrefix, time.Now().Format(PreferredTimeFormat)),
		})
	}

	return homeTiles, nil
}

// GetHouseholdMemberCount returns how many users currently belong to a household
func (dbc DatabaseController) GetHouseholdMemberCount(householdID uint) (int64, error) {
	var count int64
	result := dbc.DBHandle.Model(&authentication.User{}).Where("household_id = ?", householdID).Count(&count)
	return count, result.Error
}

// LeaveHousehold creates a new personal household for the user, moves all products if they were the
// sole member, then updates the user's HouseholdID to the new household.
func (dbc DatabaseController) LeaveHousehold(userID uint) error {
	tx := dbc.DBHandle.Begin()

	var user authentication.User
	if err := tx.First(&user, userID).Error; err != nil {
		tx.Rollback()
		return err
	}

	oldHouseholdID := user.HouseholdID

	// Create new personal household
	newHousehold := database.Household{
		Name: fmt.Sprintf("%s's Household", user.Username),
	}
	if err := tx.Create(&newHousehold).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Model(&newHousehold).Update("admin_id", userID).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Move products only when the user is the sole member
	var memberCount int64
	tx.Model(&authentication.User{}).Where("household_id = ?", oldHouseholdID).Count(&memberCount)
	if memberCount == 1 {
		if err := tx.Model(&database.Product{}).Where("household_id = ?", oldHouseholdID).Update("household_id", newHousehold.ID).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// Update user to new household
	if err := tx.Model(&user).Update("household_id", newHousehold.ID).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// CreateAndSwitchHousehold creates a new named household and switches the user to it.
// Products are moved from the old household when the user was its sole member.
func (dbc DatabaseController) CreateAndSwitchHousehold(userID uint, name string) error {
	tx := dbc.DBHandle.Begin()

	var user authentication.User
	if err := tx.First(&user, userID).Error; err != nil {
		tx.Rollback()
		return err
	}

	oldHouseholdID := user.HouseholdID

	newHousehold := database.Household{Name: name}
	if err := tx.Create(&newHousehold).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Model(&newHousehold).Update("admin_id", userID).Error; err != nil {
		tx.Rollback()
		return err
	}

	var memberCount int64
	tx.Model(&authentication.User{}).Where("household_id = ?", oldHouseholdID).Count(&memberCount)
	if memberCount == 1 {
		if err := tx.Model(&database.Product{}).Where("household_id = ?", oldHouseholdID).Update("household_id", newHousehold.ID).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Model(&user).Update("household_id", newHousehold.ID).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// ApplyForHousehold creates a pending HouseholdApplication for the given user and target household.
// Returns ErrHouseholdNotFound if the target household does not exist,
// or ErrApplicationAlreadyPending if a pending application already exists.
func (dbc DatabaseController) ApplyForHousehold(applicantID, householdID uint) error {
	// Verify target household exists
	var household database.Household
	if err := dbc.DBHandle.First(&household, householdID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.ErrHouseholdNotFound
		}
		return err
	}

	// Check for an existing pending application
	var existing database.HouseholdApplication
	result := dbc.DBHandle.Where("applicant_id = ? AND household_id = ? AND status = ?", applicantID, householdID, database.ApplicationStatusPending).First(&existing)
	if result.Error == nil {
		return errors.ErrApplicationAlreadyPending
	}
	if result.Error != gorm.ErrRecordNotFound {
		return result.Error
	}

	application := database.HouseholdApplication{
		ApplicantID: applicantID,
		HouseholdID: householdID,
		Status:      database.ApplicationStatusPending,
	}
	return dbc.DBHandle.Create(&application).Error
}

// GetPendingApplicationsForAdmin returns all pending applications for the household the given user administrates.
// Returns ErrNotHouseholdAdmin if the user is not the admin of their household.
func (dbc DatabaseController) GetPendingApplicationsForAdmin(adminUserID uint) ([]database.HouseholdApplication, error) {
	var user authentication.User
	if err := dbc.DBHandle.First(&user, adminUserID).Error; err != nil {
		return nil, err
	}

	var household database.Household
	if err := dbc.DBHandle.First(&household, user.HouseholdID).Error; err != nil {
		return nil, err
	}
	if household.AdminID != adminUserID {
		return nil, errors.ErrNotHouseholdAdmin
	}

	var applications []database.HouseholdApplication
	err := dbc.DBHandle.Where("household_id = ? AND status = ?", household.ID, database.ApplicationStatusPending).Find(&applications).Error
	return applications, err
}

// ApproveApplication approves a household application: moves the applicant into the household.
// Only the household admin may call this.
func (dbc DatabaseController) ApproveApplication(applicationID, adminUserID uint) error {
	tx := dbc.DBHandle.Begin()

	var application database.HouseholdApplication
	if err := tx.First(&application, applicationID).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return errors.ErrApplicationNotFound
		}
		return err
	}

	// Verify caller is admin of target household
	var household database.Household
	if err := tx.First(&household, application.HouseholdID).Error; err != nil {
		tx.Rollback()
		return err
	}
	if household.AdminID != adminUserID {
		tx.Rollback()
		return errors.ErrNotHouseholdAdmin
	}

	// Move applicant to household
	if err := tx.Model(&authentication.User{}).Where("id = ?", application.ApplicantID).Update("household_id", application.HouseholdID).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Mark application approved
	if err := tx.Model(&application).Update("status", database.ApplicationStatusApproved).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// RejectApplication rejects a household application.
// Only the household admin may call this.
func (dbc DatabaseController) RejectApplication(applicationID, adminUserID uint) error {
	tx := dbc.DBHandle.Begin()

	var application database.HouseholdApplication
	if err := tx.First(&application, applicationID).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return errors.ErrApplicationNotFound
		}
		return err
	}

	var household database.Household
	if err := tx.First(&household, application.HouseholdID).Error; err != nil {
		tx.Rollback()
		return err
	}
	if household.AdminID != adminUserID {
		tx.Rollback()
		return errors.ErrNotHouseholdAdmin
	}

	if err := tx.Model(&application).Update("status", database.ApplicationStatusRejected).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// GetHouseholdMembers returns all users that belong to the given household.
func (dbc DatabaseController) GetHouseholdMembers(householdID uint) ([]authentication.User, error) {
	var users []authentication.User
	err := dbc.DBHandle.Where("household_id = ?", householdID).Find(&users).Error
	return users, err
}

// UpdateHouseholdName renames a household. The caller must be the household admin.
func (dbc DatabaseController) UpdateHouseholdName(householdID, adminUserID uint, name string) error {
	var household database.Household
	if err := dbc.DBHandle.First(&household, householdID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.ErrHouseholdNotFound
		}
		return err
	}
	if household.AdminID != adminUserID {
		return errors.ErrNotHouseholdAdmin
	}
	return dbc.DBHandle.Model(&household).Update("name", name).Error
}

// GetPendingApplicationsForApplicant returns all pending applications submitted by the given user.
func (dbc DatabaseController) GetPendingApplicationsForApplicant(applicantUserID uint) ([]database.HouseholdApplication, error) {
	var applications []database.HouseholdApplication
	err := dbc.DBHandle.
		Where("applicant_id = ? AND status = ?", applicantUserID, database.ApplicationStatusPending).
		Find(&applications).Error
	return applications, err
}

// CancelApplication cancels a pending application. The caller must be the applicant.
func (dbc DatabaseController) CancelApplication(applicationID, applicantUserID uint) error {
	var application database.HouseholdApplication
	if err := dbc.DBHandle.First(&application, applicationID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.ErrApplicationNotFound
		}
		return err
	}
	if application.ApplicantID != applicantUserID {
		return errors.ErrNotApplicationApplicant
	}
	if application.Status != database.ApplicationStatusPending {
		return errors.ErrApplicationNotFound
	}
	return dbc.DBHandle.Delete(&application).Error
}

// RemoveMemberFromHousehold removes a member from the admin's household and assigns them a new personal household.
func (dbc DatabaseController) RemoveMemberFromHousehold(memberUserID, adminUserID uint) error {
	tx := dbc.DBHandle.Begin()

	var adminUser authentication.User
	if err := tx.First(&adminUser, adminUserID).Error; err != nil {
		tx.Rollback()
		return err
	}

	var household database.Household
	if err := tx.First(&household, adminUser.HouseholdID).Error; err != nil {
		tx.Rollback()
		return err
	}
	if household.AdminID != adminUserID {
		tx.Rollback()
		return errors.ErrNotHouseholdAdmin
	}

	var memberUser authentication.User
	if err := tx.First(&memberUser, memberUserID).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return errors.ErrMemberNotInHousehold
		}
		return err
	}
	if memberUser.HouseholdID != household.ID {
		tx.Rollback()
		return errors.ErrMemberNotInHousehold
	}
	if memberUserID == adminUserID {
		tx.Rollback()
		return errors.ErrCannotRemoveAdmin
	}

	newHousehold := database.Household{
		Name: fmt.Sprintf("%s's Household", memberUser.Username),
	}
	if err := tx.Create(&newHousehold).Error; err != nil {
		tx.Rollback()
		return err
	}
	if err := tx.Model(&newHousehold).Update("admin_id", memberUserID).Error; err != nil {
		tx.Rollback()
		return err
	}

	var memberCount int64
	tx.Model(&authentication.User{}).Where("household_id = ?", household.ID).Count(&memberCount)
	if memberCount == 1 {
		if err := tx.Model(&database.Product{}).Where("household_id = ?", household.ID).Update("household_id", newHousehold.ID).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	if err := tx.Model(&memberUser).Update("household_id", newHousehold.ID).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// CreateInvitation creates a new household invitation after verifying the inviter is a member
// and no pending invitation exists for the same email.
func (dbc DatabaseController) CreateInvitation(householdID, inviterID uint, email string) (database.HouseholdInvitation, error) {
	// Verify inviter is a member of the household
	var user authentication.User
	if err := dbc.DBHandle.Where("id = ? AND household_id = ?", inviterID, householdID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return database.HouseholdInvitation{}, errors.ErrInvitationNotAuthorized
		}
		return database.HouseholdInvitation{}, err
	}

	// Check no pending invitation exists for same householdID + email
	var existingInvitation database.HouseholdInvitation
	err := dbc.DBHandle.Where("household_id = ? AND email = ? AND status = ?", householdID, email, database.InvitationStatusPending).First(&existingInvitation).Error
	if err == nil {
		return database.HouseholdInvitation{}, errors.ErrDuplicateInvitation
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return database.HouseholdInvitation{}, err
	}

	// Generate token and set expiry
	token := uuid.New().String()
	expiresAt := time.Now().Add(7 * 24 * time.Hour)

	invitation := database.HouseholdInvitation{
		HouseholdID: householdID,
		InviterID:   inviterID,
		Email:       email,
		Token:       token,
		Status:      database.InvitationStatusPending,
		ExpiresAt:   expiresAt,
	}

	if err := dbc.DBHandle.Create(&invitation).Error; err != nil {
		return database.HouseholdInvitation{}, err
	}

	return invitation, nil
}

// GetInvitationsForHousehold returns all non-deleted invitations for the household, ordered by CreatedAt DESC
func (dbc DatabaseController) GetInvitationsForHousehold(householdID uint) ([]database.HouseholdInvitation, error) {
	var invitations []database.HouseholdInvitation
	err := dbc.DBHandle.Where("household_id = ?", householdID).Order("created_at DESC").Find(&invitations).Error
	return invitations, err
}

// GetInvitationByToken looks up an invitation by its token
func (dbc DatabaseController) GetInvitationByToken(token string) (database.HouseholdInvitation, error) {
	var invitation database.HouseholdInvitation
	if err := dbc.DBHandle.Where("token = ?", token).First(&invitation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return database.HouseholdInvitation{}, errors.ErrInvitationNotFound
		}
		return database.HouseholdInvitation{}, err
	}
	return invitation, nil
}

// AcceptInvitation processes an invitation acceptance, updating the user's household and marking the invitation as accepted
func (dbc DatabaseController) AcceptInvitation(token, email string, userID uint) error {
	var invitation database.HouseholdInvitation
	if err := dbc.DBHandle.Where("token = ?", token).First(&invitation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.ErrInvitationNotFound
		}
		return err
	}

	// Check status
	switch invitation.Status {
	case database.InvitationStatusAccepted:
		return errors.ErrInvitationAlreadyUsed
	case database.InvitationStatusCancelled:
		return errors.ErrInvitationCancelled
	}

	// Check expiry
	if time.Now().After(invitation.ExpiresAt) {
		dbc.DBHandle.Model(&invitation).Update("status", database.InvitationStatusExpired)
		return errors.ErrInvitationExpired
	}

	// Check email match
	if invitation.Email != email {
		return errors.ErrInvitationEmailMismatch
	}

	// Use transaction to update user's household and mark invitation as accepted
	tx := dbc.DBHandle.Begin()
	if err := tx.Model(&authentication.User{}).Where("id = ?", userID).Update("household_id", invitation.HouseholdID).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Model(&invitation).Update("status", database.InvitationStatusAccepted).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

// CancelInvitation cancels a pending invitation after verifying the caller is a member of the invitation's household
func (dbc DatabaseController) CancelInvitation(invitationID, userID uint) error {
	var invitation database.HouseholdInvitation
	if err := dbc.DBHandle.First(&invitation, invitationID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.ErrInvitationNotFound
		}
		return err
	}

	// Verify caller is a member of the invitation's household
	var user authentication.User
	if err := dbc.DBHandle.Where("id = ? AND household_id = ?", userID, invitation.HouseholdID).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return errors.ErrInvitationNotAuthorized
		}
		return err
	}

	return dbc.DBHandle.Model(&invitation).Update("status", database.InvitationStatusCancelled).Error
}

// GetPendingInvitationsNotSent returns all pending invitations that have not been successfully sent yet,
// or that failed and are due for a retry based on the given retry interval.
func (dbc DatabaseController) GetPendingInvitationsNotSent(retryInterval time.Duration) ([]database.HouseholdInvitation, error) {
	var invitations []database.HouseholdInvitation
	// Get invitations that are:
	// 1. Status is pending
	// 2. Not yet expired
	// 3. Either never sent (SentAt IS NULL) or last attempt was before the retry interval
	query := dbc.DBHandle.Where(
		"status = ? AND expires_at > ? AND (sent_at IS NULL OR sent_at < ?)",
		database.InvitationStatusPending,
		time.Now(),
		time.Now().Add(-retryInterval),
	)
	err := query.Order("created_at ASC").Find(&invitations).Error
	return invitations, err
}

// MarkInvitationSent marks an invitation as successfully sent
func (dbc DatabaseController) MarkInvitationSent(invitationID uint) error {
	now := time.Now()
	return dbc.DBHandle.Model(&database.HouseholdInvitation{}).
		Where("id = ?", invitationID).
		Updates(map[string]interface{}{
			"sent_at":       now,
			"send_attempts": gorm.Expr("send_attempts + 1"),
		}).Error
}

// MarkInvitationSendFailed increments the send attempt counter without marking as sent
func (dbc DatabaseController) MarkInvitationSendFailed(invitationID uint) error {
	return dbc.DBHandle.Model(&database.HouseholdInvitation{}).
		Where("id = ?", invitationID).
		Update("send_attempts", gorm.Expr("send_attempts + 1")).Error
}
