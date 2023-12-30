package controllers

import (
	"time"

	"gitlab.com/Isotop7/expiro/errors"
	"gitlab.com/Isotop7/expiro/models/authentication"
	"gitlab.com/Isotop7/expiro/models/database"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type DatabaseController struct {
	DB *gorm.DB
}

func (dbc DatabaseController) FindUserByUsername(username string) (authentication.User, error) {
	var user authentication.User
	// Gets first user with matching username
	selectErr := dbc.DB.First(&user, "username = ?", username)
	return user, selectErr.Error
}

func (dbc DatabaseController) GetUserByID(userID uint) (authentication.User, error) {
	var user authentication.User
	// Gets first user with matching username
	selectErr := dbc.DB.First(&user, userID)
	return user, selectErr.Error
}

func (dbc DatabaseController) UserExists(user authentication.User) bool {
	// Check if user with username exists
	var dbUser authentication.User
	// Username must be unique
	selectErr := dbc.DB.First(&dbUser, "username = ?", user.Username)
	return !(selectErr.Error == gorm.ErrRecordNotFound)
}

func (dbc DatabaseController) GetNextUserID() uint {
	// Get next user id from database
	var lastUser authentication.User
	dbc.DB.Order("id").Limit(1).Find(&lastUser)
	return (lastUser.ID + 1)
}

func (dbc DatabaseController) CreateUser(user *authentication.User) error {
	// Create new database user
	hashedPassword, hashError := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if hashError != nil {
		return hashError
	}
	user.Password = string(hashedPassword)
	createResult := dbc.DB.Create(user)
	return createResult.Error
}

func (dbc DatabaseController) UserIsProductOwner(userID uint, productID int) bool {
	// Check for invalid product IDs
	if productID <= 0 {
		return false
	}

	// Get single product by ID
	var product database.Product
	getError := dbc.DB.First(&product, productID)
	if getError.Error != nil {
		return false
	}

	// Return if given userID matches database assigned userID
	return product.UserID == userID
}

func (dbc DatabaseController) GetUserProductsBulk(userID uint, limit int) ([]database.Product, error) {
	// Get user object from database
	user, userErr := dbc.GetUserByID(userID)
	if userErr != nil {
		return []database.Product{}, userErr
	}
	// Get user with products preloaded
	var userWithData authentication.User
	findErr := dbc.DB.Preload("Products", "user_id = ?", user.ID).Find(&userWithData, user.ID)
	if findErr.Error != nil {
		return []database.Product{}, findErr.Error
	}

	// Apply optional limit
	if limit > 0 {
		return userWithData.Products[:limit], nil
	} else {
		return userWithData.Products, nil
	}
}

func (dbc DatabaseController) GetProductByID(productID int, userID uint) (database.Product, error) {
	// Get single product by ID
	if productID <= 0 {
		return database.Product{}, gorm.ErrNotImplemented
	}
	// Parse product to var
	var product database.Product
	getError := dbc.DB.First(&product, productID)

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

func (dbc DatabaseController) CreateProduct(userID uint, product *database.Product) error {
	// Get user object from database
	user, userErr := dbc.GetUserByID(userID)
	if userErr != nil {
		return userErr
	}
	user.Products = append(user.Products, *product)
	// Create new product
	saveErr := dbc.DB.Save(&user)
	return saveErr.Error
}

func (dbc DatabaseController) UpdateProduct(productID int, userID uint, product *database.Product) error {
	// Check if id is valid
	if productID <= 0 {
		return gorm.ErrNotImplemented
	}

	// Try to get product
	var dbProduct database.Product
	getError := dbc.DB.First(&dbProduct, productID)

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

	saveResult := dbc.DB.Save(&dbProduct)
	if saveResult.Error != nil {
		return saveResult.Error
	} else {
		return nil
	}
}

func (dbc DatabaseController) DeleteProduct(productID int, userID uint) error {
	// Get product and check for correct userID
	_, getError := dbc.GetProductByID(productID, userID)
	if getError != nil {
		return getError
	}

	// Delete product by its id
	deleteResult := dbc.DB.Delete(&database.Product{}, productID)
	return deleteResult.Error
}

func (dbc DatabaseController) SetProductExpireAt(productID int, userID uint, expireAt database.Timestamp) (string, error) {
	// Update ExpireAt date
	var dbProduct database.Product
	getError := dbc.DB.First(&dbProduct, productID)
	if getError.Error != nil {
		return "", getError.Error
	}
	// Check if supplied user matches the userID in the database object
	if dbProduct.UserID != userID {
		return "", errors.ErrMismatcherUserID
	}

	// Update values
	dbProduct.ExpireAt = time.Time(expireAt.Timestamp)
	saveResult := dbc.DB.Save(&dbProduct)
	if saveResult.Error != nil {
		return dbProduct.Barcode, saveResult.Error
	} else {
		return dbProduct.Barcode, nil
	}
}

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
