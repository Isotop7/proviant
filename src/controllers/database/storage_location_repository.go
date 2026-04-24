package database

import (
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"
	"gorm.io/gorm"
)

type StorageLocationRepository struct {
	DB *gorm.DB
}

func NewStorageLocationRepository(db *gorm.DB) *StorageLocationRepository {
	return &StorageLocationRepository{DB: db}
}

func (r *StorageLocationRepository) GetByHousehold(userID uint) ([]database.StorageLocation, error) {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return nil, err
	}
	var locs []database.StorageLocation
	err := r.DB.Where("household_id = ?", user.HouseholdID).
		Order("sort_order ASC, name ASC").
		Find(&locs).Error
	return locs, err
}

func (r *StorageLocationRepository) GetByID(locationID, userID uint) (database.StorageLocation, error) {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return database.StorageLocation{}, err
	}
	var storageLocation database.StorageLocation
	if err := r.DB.First(&storageLocation, locationID).Error; err != nil {
		return database.StorageLocation{}, errors.ErrStorageLocationNotFound
	}
	if storageLocation.HouseholdID != user.HouseholdID {
		return database.StorageLocation{}, errors.ErrStorageLocationNotOwned
	}
	return storageLocation, nil
}

func (r *StorageLocationRepository) Create(userID uint, name, icon string, sortOrder int) (database.StorageLocation, error) {
	var user authentication.User
	if err := r.DB.First(&user, userID).Error; err != nil {
		return database.StorageLocation{}, err
	}
	storageLocation := database.StorageLocation{
		HouseholdID: user.HouseholdID,
		Name:        name,
		Icon:        icon,
		SortOrder:   sortOrder,
	}
	if err := r.DB.Create(&storageLocation).Error; err != nil {
		return database.StorageLocation{}, err
	}
	return storageLocation, nil
}

func (r *StorageLocationRepository) Update(locationID, userID uint, name, icon string, sortOrder int) (database.StorageLocation, error) {
	loc, err := r.GetByID(locationID, userID)
	if err != nil {
		return database.StorageLocation{}, err
	}
	loc.Name = name
	loc.Icon = icon
	loc.SortOrder = sortOrder
	if err := r.DB.Save(&loc).Error; err != nil {
		return database.StorageLocation{}, err
	}
	return loc, nil
}

func (r *StorageLocationRepository) Delete(locationID, userID uint) error {
	loc, err := r.GetByID(locationID, userID)
	if err != nil {
		return err
	}
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&database.Product{}).
			Where("storage_location_id = ?", loc.ID).
			Update("storage_location_id", nil).Error; err != nil {
			return err
		}
		return tx.Delete(&loc).Error
	})
}
