package database

import (
	"github.com/Isotop7/proviant/metrics"
	"github.com/Isotop7/proviant/models/database"

	"gorm.io/gorm"
)

// OpenFoodFactsCacheRepositoryInterface is the Open Food Facts cache: the
// entries a barcode lookup resolves from and the maintenance reads that fill
// in the storage hints and the images.
type OpenFoodFactsCacheRepositoryInterface interface {
	GetOpenFoodFactsCacheByBarcode(barcode string) (database.OpenFoodFactsCache, error)
	GetOpenFoodFactsCachesByBarcodes(barcodes []string) ([]database.OpenFoodFactsCache, error)
	CreateOpenFoodFactsCache(entry *database.OpenFoodFactsCache) error
	UpdateOpenFoodFactsCacheImageURL(barcode, imageURL string) error
	GetOpenFoodFactsCacheWithoutStorageHint() ([]database.OpenFoodFactsCache, error)
	UpdateOpenFoodFactsCacheStorageHint(barcode, storageHint string) error
	GetOpenFoodFactsCacheWithRemoteImageURL() ([]database.OpenFoodFactsCache, error)
}

var _ OpenFoodFactsCacheRepositoryInterface = (*OpenFoodFactsCacheRepository)(nil)

// OpenFoodFactsCacheRepository owns the Open Food Facts cache.
type OpenFoodFactsCacheRepository struct {
	DB *gorm.DB
}

func NewOpenFoodFactsCacheRepository(db *gorm.DB) *OpenFoodFactsCacheRepository {
	return &OpenFoodFactsCacheRepository{DB: db}
}

func (r *OpenFoodFactsCacheRepository) GetOpenFoodFactsCacheByBarcode(barcode string) (database.OpenFoodFactsCache, error) {
	var entry database.OpenFoodFactsCache
	result := r.DB.Where("barcode = ?", barcode).First(&entry)
	metrics.RecordCacheLookup(result.Error == nil)
	return entry, result.Error
}

// GetOpenFoodFactsCachesByBarcodes returns every cached Open Food Facts entry
// whose barcode is in the given set. It is the batch form of
// GetOpenFoodFactsCacheByBarcode, so a bulk import can resolve all of its
// fallback names with one query instead of one lookup per row.
func (r *OpenFoodFactsCacheRepository) GetOpenFoodFactsCachesByBarcodes(barcodes []string) ([]database.OpenFoodFactsCache, error) {
	if len(barcodes) == 0 {
		return []database.OpenFoodFactsCache{}, nil
	}
	var entries []database.OpenFoodFactsCache
	for _, chunk := range chunkStrings(barcodes, bulkQueryChunkSize) {
		var batch []database.OpenFoodFactsCache
		queryErr := r.DB.Where("barcode IN ?", chunk).Find(&batch).Error
		if queryErr != nil {
			hits, misses := countCacheHits(barcodes, nil)
			metrics.RecordCacheLookupBatch(hits, misses)
			return []database.OpenFoodFactsCache{}, queryErr
		}
		entries = append(entries, batch...)
	}
	hits, misses := countCacheHits(barcodes, entries)
	metrics.RecordCacheLookupBatch(hits, misses)
	return entries, nil
}

// countCacheHits returns how many of the queried barcodes the cache holds and
// how many it misses. A barcode listed twice is one lookup.
func countCacheHits(barcodes []string, entries []database.OpenFoodFactsCache) (int, int) {
	found := make(map[string]bool, len(entries))
	for i := range entries {
		found[entries[i].Barcode] = true
	}
	seen := make(map[string]bool, len(barcodes))
	hits := 0
	for _, barcode := range barcodes {
		if seen[barcode] {
			continue
		}
		seen[barcode] = true
		if found[barcode] {
			hits++
		}
	}
	return hits, len(seen) - hits
}

func (r *OpenFoodFactsCacheRepository) CreateOpenFoodFactsCache(entry *database.OpenFoodFactsCache) error {
	return r.DB.Create(entry).Error
}

func (r *OpenFoodFactsCacheRepository) UpdateOpenFoodFactsCacheImageURL(barcode, imageURL string) error {
	return r.DB.Model(&database.OpenFoodFactsCache{}).Where("barcode = ?", barcode).Update("image_url", imageURL).Error
}

func (r *OpenFoodFactsCacheRepository) GetOpenFoodFactsCacheWithoutStorageHint() ([]database.OpenFoodFactsCache, error) {
	var entries []database.OpenFoodFactsCache
	err := r.DB.Where("storage_hint = '' OR storage_hint IS NULL").Find(&entries).Error
	return entries, err
}

func (r *OpenFoodFactsCacheRepository) UpdateOpenFoodFactsCacheStorageHint(barcode, storageHint string) error {
	return r.DB.Model(&database.OpenFoodFactsCache{}).Where("barcode = ?", barcode).Update("storage_hint", storageHint).Error
}

func (r *OpenFoodFactsCacheRepository) GetOpenFoodFactsCacheWithRemoteImageURL() ([]database.OpenFoodFactsCache, error) {
	var entries []database.OpenFoodFactsCache
	err := r.DB.Where("image_url != '' AND image_url NOT LIKE '/product-images/%'").Find(&entries).Error
	return entries, err
}
