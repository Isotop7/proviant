package controllers

import (
	"expiro/backend/models/auth"
	"expiro/backend/models/database"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type DatabaseController struct {
	DB *gorm.DB
}

func (dbc DatabaseController) FindUserByUsername(username string) (auth.User, error) {
	var user auth.User
	// Gets first user with matching username
	selectErr := dbc.DB.First(&user, "username = ?", username)
	return user, selectErr.Error
}

func (dbc DatabaseController) UserExists(user auth.User) bool {
	// Check if user with username exists
	var dbUser auth.User
	// Username must be unique
	selectErr := dbc.DB.First(&dbUser, "username = ?", user.Username)
	return !(selectErr.Error == gorm.ErrRecordNotFound)
}

func (dbc DatabaseController) GetNextUserID() uint {
	// Get next user id from database
	var lastUser auth.User
	dbc.DB.Order("id").Limit(1).Find(&lastUser)
	return (lastUser.ID + 1)
}

func (dbc DatabaseController) CreateUser(user *auth.User) error {
	// Create new database user
	hashedPassword, hashError := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if hashError != nil {
		return hashError
	}
	user.Password = string(hashedPassword)
	createResult := dbc.DB.Create(user)
	return createResult.Error
}

func (dbc DatabaseController) GetProductsBulk(limit int) []database.Product {
	// Get products, optional: set limit on returned dataset
	var products []database.Product
	if limit > 0 {
		dbc.DB.Limit(limit).Find(&products)
	} else {
		dbc.DB.Find(&products)
	}
	return products
}

func (dbc DatabaseController) GetProductByID(id int) (database.Product, error) {
	// Get single product by ID
	if id <= 0 {
		return database.Product{}, gorm.ErrNotImplemented
	}
	var product database.Product
	getError := dbc.DB.First(&product, id)

	if getError.Error != nil {
		return product, nil
	} else {
		return database.Product{}, getError.Error
	}
}

func (dbc DatabaseController) CreateProduct(product *database.Product) error {
	// Create new product
	createResult := dbc.DB.Create(&product)
	return createResult.Error
}

func (dbc DatabaseController) UpdateProduct(product *database.Product, id int) error {
	// Check if id is valid
	if id <= 0 {
		return gorm.ErrNotImplemented
	}

	// Try to get product
	var dbProduct database.Product
	getError := dbc.DB.First(&dbProduct, id)

	if getError.Error != nil {
		return getError.Error
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

func (dbc DatabaseController) DeleteProduct(id int) error {
	// Delete product by its id
	deleteResult := dbc.DB.Delete(&database.Product{}, id)
	return deleteResult.Error
}

func (dbc DatabaseController) SetProductExpireAt(id int, expireAt database.Timestamp) (string, error) {
	// Update ExpireAt date
	var dbProduct database.Product
	getError := dbc.DB.First(&dbProduct, id)
	if getError.Error != nil {
		return "", getError.Error
	}

	dbProduct.ExpireAt = time.Time(expireAt.Timestamp)
	saveResult := dbc.DB.Save(&dbProduct)
	if saveResult.Error != nil {
		return "", saveResult.Error
	} else {
		return dbProduct.Barcode, nil
	}
}

func (dbc DatabaseController) GetProductsExpired() []database.Product {
	// Get all currently expired products
	var products []database.Product
	dbc.DB.Where("expire_at < ?", time.Now()).Find(&products)
	return products
}
