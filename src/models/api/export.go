package api

import (
	"time"

	dbModel "codeberg.org/isotop7/proviant/models/database"
)

// ExportProductResponse is the product shape in a JSON data export. Unlike
// ProductResponse it keeps createdAt and archivedAt: the CSV export carries the
// same two as added_at and archived_at, so a backup that dropped them would lose
// when a product was bought and when it was archived. UpdatedAt is the one GORM
// field left out — the bookkeeping timestamp says nothing to whoever reads the
// file. ID is serialized as the lowercase wire field "id".
type ExportProductResponse struct {
	ID                   uint                     `json:"id"`
	Barcode              string                   `json:"barcode"`
	ProductName          string                   `json:"productName"`
	Categories           string                   `json:"categories"`
	Countries            string                   `json:"countries"`
	ImageURL             string                   `json:"imageUrl"`
	ExpireAt             time.Time                `json:"expireAt"`
	ScannedAt            time.Time                `json:"scannedAt"`
	NotifiedAt           time.Time                `json:"notifiedAt"`
	IsPrivate            bool                     `json:"isPrivate"`
	Amount               int                      `json:"amount"`
	Unit                 string                   `json:"unit"`
	StorageLocationID    *uint                    `json:"storageLocationId"`
	StorageLocation      *StorageLocationResponse `json:"storageLocation,omitempty"`
	PriceOverride        *float64                 `json:"priceOverride,omitempty"`
	CO2KgPerKg           *float64                 `json:"co2KgPerKg,omitempty"`
	RemovalReason        string                   `json:"removalReason"`
	NotificationLeadDays *int                     `json:"notificationLeadDays,omitempty"`
	MinStockAmount       int                      `json:"minStockAmount"`
	OpenedAt             *time.Time               `json:"openedAt,omitempty"`
	DaysAfterOpening     *int                     `json:"daysAfterOpening,omitempty"`
	CreatedAt            time.Time                `json:"createdAt"`
	ArchivedAt           *time.Time               `json:"archivedAt,omitempty"`
}

// NewExportProductResponse projects a product onto the export shape. An archived
// product carries archivedAt; an active one omits the field entirely.
func NewExportProductResponse(product *dbModel.Product) ExportProductResponse {
	if product == nil {
		return ExportProductResponse{}
	}
	resp := ExportProductResponse{
		ID:                   product.ID,
		Barcode:              product.Barcode,
		ProductName:          product.ProductName,
		Categories:           product.Categories,
		Countries:            product.Countries,
		ImageURL:             product.ImageURL,
		ExpireAt:             product.ExpireAt,
		ScannedAt:            product.ScannedAt,
		NotifiedAt:           product.NotifiedAt,
		IsPrivate:            product.IsPrivate,
		Amount:               product.Amount,
		Unit:                 product.Unit,
		StorageLocationID:    product.StorageLocationID,
		PriceOverride:        product.PriceOverride,
		CO2KgPerKg:           product.CO2KgPerKg,
		RemovalReason:        product.RemovalReason,
		NotificationLeadDays: product.NotificationLeadDays,
		MinStockAmount:       product.MinStockAmount,
		OpenedAt:             product.OpenedAt,
		DaysAfterOpening:     product.DaysAfterOpening,
		CreatedAt:            product.CreatedAt,
	}
	if product.StorageLocation != nil {
		location := NewStorageLocationResponse(product.StorageLocation)
		resp.StorageLocation = &location
	}
	if product.DeletedAt.Valid {
		archivedAt := product.DeletedAt.Time
		resp.ArchivedAt = &archivedAt
	}
	return resp
}

// NewExportProductResponses projects a slice of products onto the export shape.
// The type parameter covers the []dbModel.Product shape both export queries return.
func NewExportProductResponses[T dbModel.Product | *dbModel.Product](products []T) []ExportProductResponse {
	if products == nil {
		return nil
	}
	out := make([]ExportProductResponse, len(products))
	for i := range products {
		switch typed := any(products[i]).(type) {
		case dbModel.Product:
			out[i] = NewExportProductResponse(&typed)
		case *dbModel.Product:
			out[i] = NewExportProductResponse(typed)
		}
	}
	return out
}
