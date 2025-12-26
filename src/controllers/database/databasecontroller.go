package database

import (
	"fmt"
	"time"

	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/models/webparts"

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
func (dbc DatabaseController) SearchProducts(queryParam SearchParameterEnum, queryValue, sort, order string, userID uint) ([]database.Product, error) {
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
	preloadedDataset = preloadedDataset.Order(fmt.Sprintf("%s %s", sort, order))

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
	if productCountErr != nil {
		return homeTiles, productCountErr
	}
	// Check for products
	productCount := len(productList)
	// If no products are assigned, there is nothing to show
	if productCount == 0 {
		return homeTiles, nil
	}
	// Create tile
	homeTiles = append(homeTiles, webparts.Tile{
		Title:  "Amount of your products",
		Hero:   fmt.Sprint(productCount),
		Body:   fmt.Sprintf("You currently have %d products assigned", productCount),
		Footer: fmt.Sprintf(GeneratedPrefix, time.Now().Format(PreferredTimeFormat)),
	})

	// Get last inserted product
	var lastProduct database.Product
	getError := dbc.DBHandle.
		Where("household_id = ?", user.HouseholdID).
		Where("deleted_at IS NULL").
		Order("created_at DESC").
		Limit(1).
		Find(&lastProduct)
	if getError.Error != nil {
		return homeTiles, getError.Error
	}
	// Create tile
	homeTiles = append(homeTiles, webparts.Tile{
		Title:  "Last inserted product",
		Hero:   lastProduct.ProductName,
		Body:   fmt.Sprintf("'%s' is the most recent product with barcode #%s", lastProduct.ProductName, lastProduct.Barcode),
		Footer: fmt.Sprintf(GeneratedPrefix, time.Now().Format(PreferredTimeFormat)),
	})

	// Last notification
	var lastNotifiedProduct database.Product
	getNotifiedError := dbc.DBHandle.
		Where("household_id = ?", user.HouseholdID).
		Where("deleted_at IS NULL").
		Order("notified_at DESC").
		Limit(1).
		Find(&lastNotifiedProduct)
	if getNotifiedError.Error != nil {
		return homeTiles, getNotifiedError.Error
	}
	// Create tile
	homeTiles = append(homeTiles, webparts.Tile{
		Title:  "Last notification",
		Hero:   lastNotifiedProduct.NotifiedAt.Format(PreferredTimeFormat),
		Body:   fmt.Sprintf("You received the last notfication for product with barcode #%s at %s", lastProduct.Barcode, lastNotifiedProduct.NotifiedAt.Format(PreferredTimeFormat)),
		Footer: fmt.Sprintf(GeneratedPrefix, time.Now().Format(PreferredTimeFormat)),
	})

	return homeTiles, nil
}
