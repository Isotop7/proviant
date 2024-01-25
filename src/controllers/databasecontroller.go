package controllers

import (
	"fmt"
	"time"

	"gitlab.com/Isotop7/expiro/errors"
	"gitlab.com/Isotop7/expiro/models/authentication"
	"gitlab.com/Isotop7/expiro/models/database"
	"gitlab.com/Isotop7/expiro/models/webparts"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// DatabaseController is the object struct for interacting with the gorm-backed database
type DatabaseController struct {
	DBHandle *gorm.DB
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
	// Gets first user with matching username
	selectErr := dbc.DBHandle.First(&user, userID)
	return user, selectErr.Error
}

// UserExistsByUsername returns if a given user object exists in the database based on the property 'username'
func (dbc DatabaseController) UserExistsByUsername(user authentication.User) bool {
	var dbUser authentication.User
	// Try to get first object with matching username
	selectErr := dbc.DBHandle.First(&dbUser, "username = ?", user.Username)
	// If no user is found, return false
	return selectErr.Error != gorm.ErrRecordNotFound
}

// UserExistsByMailAddress returns if a given user object exists in the database based on the property 'mailAddress'
func (dbc DatabaseController) UserExistsByMailAddress(user authentication.User) bool {
	var dbUser authentication.User
	// Try to get first object with matching mailAddress
	selectErr := dbc.DBHandle.First(&dbUser, "mail_address = ?", user.MailAddress)
	// If no user is found, return false
	return selectErr.Error != gorm.ErrRecordNotFound
}

// GetNextUserID returns the next available user ID
func (dbc DatabaseController) GetNextUserID() uint {
	// TODO: Do we really need this or can't we use db-based mechanisms
	// Get next user id from database
	var lastUser authentication.User
	dbc.DBHandle.Order("id DESC").Limit(1).Find(&lastUser)
	return (lastUser.ID + 1)
}

// CreateUser creates a new user based on a given user object
// Before creation, the user password is hashed with brcypt
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) CreateUser(user *authentication.User) error {
	// Create new database user
	hashedPassword, hashError := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if hashError != nil {
		return hashError
	}
	user.Password = string(hashedPassword)
	createResult := dbc.DBHandle.Create(user)
	return createResult.Error
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

// UserIsProductOwner checks if user (based on user ID) is the matching owner of a product (based on product ID)
func (dbc DatabaseController) UserIsProductOwner(userID uint, productID int) bool {
	// Check for invalid product IDs
	if productID <= 0 {
		return false
	}

	// Get single product by ID
	var product database.Product
	getError := dbc.DBHandle.First(&product, productID)
	// Failsafe - If error is found, return false
	if getError.Error != nil {
		return false
	}

	// Return if given userID matches database assigned userID
	return product.UserID == userID
}

// GetUserMailAddressByID returns the mail address of a user by his ID
func (dbc DatabaseController) GetUserMailAddressByID(userID uint) (string, error) {
	// Get user object from database
	user, userErr := dbc.GetUserByID(userID)
	// Check for database error
	if userErr != nil {
		return "", userErr
	}

	// Check if mail address of user is empty or return it
	if user.MailAddress != "" {
		return user.MailAddress, nil
	} else {
		return "", errors.ErrUserHasNoMailAddress
	}
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
	// Get user with products preloaded
	var userWithData authentication.User
	findErr := dbc.DBHandle.Preload("Products", "user_id = ?", user.ID).Find(&userWithData, user.ID)
	if findErr.Error != nil {
		return []database.Product{}, findErr.Error
	}

	// If no limit is supplied, return full set
	// If limit is supplied, return limited set
	if limit <= 0 {
		return userWithData.Products, nil
	} else {
		return userWithData.Products[:limit], nil
	}
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

	// Check if error occured while getting produc
	if getError.Error != nil {
		// Return empty set and database error
		return database.Product{}, getError.Error
	}

	// Check if userID of database product matches the userID of the current user
	if product.UserID != userID {
		// Return empty set and custom error
		return database.Product{}, errors.ErrMismatcherUserID
	}

	// Return database product
	return product, nil
}

// CreateProduct creates a product in the database and connects it to the user
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) CreateProduct(userID uint, product *database.Product) error {
	// Get user object from database
	user, userErr := dbc.GetUserByID(userID)
	if userErr != nil {
		return userErr
	}
	user.Products = append(user.Products, *product)
	// Create new product
	saveErr := dbc.DBHandle.Save(&user)
	return saveErr.Error
}

// UpdateProduct gets a product (based on product ID) of a user (based on user ID) and updates its contents with the contents of a supplied reference to the updated product
// If the database operations return an error, the error is also returned (otherwise nil)
func (dbc DatabaseController) UpdateProduct(productID int, userID uint, product *database.Product) error {
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
	// Check if supplied user matches the userID in the database object
	if dbProduct.UserID != userID {
		return errors.ErrMismatcherUserID
	}

	// Update values
	dbProduct.Barcode = product.Barcode
	dbProduct.ProductName = product.ProductName
	dbProduct.Categories = product.Categories
	dbProduct.Countries = product.Countries
	dbProduct.ImageURL = product.ImageURL
	dbProduct.ExpireAt = product.ExpireAt
	dbProduct.NotifiedAt = product.NotifiedAt

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
func (dbc DatabaseController) DeleteProduct(productID int, userID uint) error {
	// Get product and check for correct userID
	_, getError := dbc.GetProductByID(productID, userID)
	if getError != nil {
		return getError
	}

	// Delete product by its id
	deleteResult := dbc.DBHandle.Delete(&database.Product{}, productID)
	return deleteResult.Error
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
	// Check if supplied user matches the userID in the database object
	if dbProduct.UserID != userID {
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
func (dbc DatabaseController) GetProductsExpired(userID uint) ([]database.Product, error) {
	// Get all user products
	userProducts, getBulkErr := dbc.GetUserProductsBulk(userID, 0)
	if getBulkErr != nil {
		return []database.Product{}, getBulkErr
	}

	// Get all currently expired products
	expiredProducts := []database.Product{}
	timestamp := time.Now()
	for _, p := range userProducts {
		if p.ExpireAt.After(timestamp) {
			expiredProducts = append(expiredProducts, p)
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

	// Get count of products
	productList, productCountErr := dbc.GetUserProductsBulk(userID, -1)
	if productCountErr != nil {
		return homeTiles, productCountErr
	}
	// Create tile
	productCount := len(productList)
	homeTiles = append(homeTiles, webparts.Tile{
		Title:  "Amount of your products",
		Hero:   fmt.Sprint(productCount),
		Body:   fmt.Sprintf("You currently have %d products assigned", productCount),
		Footer: fmt.Sprintf("Generated @ %s", time.Now().Format("02.01.2006 15:04")),
	})

	// Get last inserted product
	var lastProduct database.Product
	getError := dbc.DBHandle.
		Where("user_id = ?", userID).
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
		Hero:   fmt.Sprint(lastProduct.ProductName),
		Body:   fmt.Sprintf("'%s' is the most recent product with barcode #%s", lastProduct.ProductName, lastProduct.Barcode),
		Footer: fmt.Sprintf("Generated @ %s", time.Now().Format("02.01.2006 15:04")),
	})

	// Last notification
	var lastNotifiedProduct database.Product
	getNotifiedError := dbc.DBHandle.
		Where("user_id = ?", userID).
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
		Hero:   fmt.Sprint(lastNotifiedProduct.NotifiedAt.Format("02.01.2006 15:04")),
		Body:   fmt.Sprintf("You received the last notfication for product with barcode #%s at %s", lastProduct.Barcode, lastNotifiedProduct.NotifiedAt.Format("02.01.2006 15:04")),
		Footer: fmt.Sprintf("Generated @ %s", time.Now().Format("02.01.2006 15:04")),
	})

	return homeTiles, nil
}
