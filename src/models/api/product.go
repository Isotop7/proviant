package api

import (
	"fmt"
	"time"

	"github.com/Isotop7/proviant/models/authentication"
	dbModel "github.com/Isotop7/proviant/models/database"
)

// HouseholdUserResponse is the admin-facing view of a household member.
type HouseholdUserResponse struct {
	ID                 uint   `json:"id"`
	Username           string `json:"username"`
	DisplayName        string `json:"displayName"`
	MailAddress        string `json:"mailAddress"`
	Role               string `json:"role"`
	TelegramConfigured bool   `json:"telegramConfigured"`
	TelegramLinked     bool   `json:"telegramLinked"`
	TwoFactorEnabled   bool   `json:"twoFactorEnabled"`
}

// NewHouseholdUserResponse projects a user onto the admin-facing view.
func NewHouseholdUserResponse(user *authentication.User) HouseholdUserResponse {
	if user == nil {
		return HouseholdUserResponse{}
	}
	return HouseholdUserResponse{
		ID:                 user.ID,
		Username:           user.Username,
		DisplayName:        user.DisplayName,
		MailAddress:        user.MailAddress,
		Role:               user.Role,
		TelegramConfigured: user.NotificationPreferences.TelegramBotToken != "",
		TelegramLinked:     user.NotificationPreferences.TelegramChatID != "",
		TwoFactorEnabled:   user.TwoFactorEnabled,
	}
}

// NewHouseholdUserResponses projects a slice of users.
func NewHouseholdUserResponses[T authentication.User | *authentication.User](users []T) []HouseholdUserResponse {
	if users == nil {
		return nil
	}
	out := make([]HouseholdUserResponse, len(users))
	for i := range users {
		switch typed := any(users[i]).(type) {
		case authentication.User:
			out[i] = NewHouseholdUserResponse(&typed)
		case *authentication.User:
			out[i] = NewHouseholdUserResponse(typed)
		}
	}
	return out
}

// StorageLocationResponse is the client-facing view of a storage location.
type StorageLocationResponse struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Icon      string `json:"icon"`
	SortOrder int    `json:"sortOrder"`
}

// NewStorageLocationResponse projects a storage location onto the wire shape.
func NewStorageLocationResponse(location *dbModel.StorageLocation) StorageLocationResponse {
	if location == nil {
		return StorageLocationResponse{}
	}
	return StorageLocationResponse{
		ID:        location.ID,
		Name:      location.Name,
		Icon:      location.Icon,
		SortOrder: location.SortOrder,
	}
}

// NewStorageLocationResponses projects a slice of storage locations.
func NewStorageLocationResponses[T dbModel.StorageLocation | *dbModel.StorageLocation](locations []T) []StorageLocationResponse {
	if locations == nil {
		return nil
	}
	out := make([]StorageLocationResponse, len(locations))
	for i := range locations {
		switch typed := any(locations[i]).(type) {
		case dbModel.StorageLocation:
			out[i] = NewStorageLocationResponse(&typed)
		case *dbModel.StorageLocation:
			out[i] = NewStorageLocationResponse(typed)
		}
	}
	return out
}

// AuditLogResponse is the client-facing view of an audit log entry.
type AuditLogResponse struct {
	ID          uint      `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	UserID      *uint     `json:"userId"`
	HouseholdID *uint     `json:"householdId"`
	Action      string    `json:"action"`
	IPAddress   string    `json:"ipAddress"`
	Details     string    `json:"details"`
	RequestID   string    `json:"requestId"`
}

// NewAuditLogResponse projects an audit log entry onto the wire shape.
func NewAuditLogResponse(log *dbModel.AuditLog) AuditLogResponse {
	if log == nil {
		return AuditLogResponse{}
	}
	return AuditLogResponse{
		ID:          log.ID,
		Timestamp:   log.Timestamp,
		UserID:      log.UserID,
		HouseholdID: log.HouseholdID,
		Action:      log.Action,
		IPAddress:   log.IPAddress,
		Details:     log.Details,
		RequestID:   log.RequestID,
	}
}

// NewAuditLogResponses projects a slice of audit log entries.
func NewAuditLogResponses[T dbModel.AuditLog | *dbModel.AuditLog](logs []T) []AuditLogResponse {
	if logs == nil {
		return nil
	}
	out := make([]AuditLogResponse, len(logs))
	for i := range logs {
		switch typed := any(logs[i]).(type) {
		case dbModel.AuditLog:
			out[i] = NewAuditLogResponse(&typed)
		case *dbModel.AuditLog:
			out[i] = NewAuditLogResponse(typed)
		}
	}
	return out
}

// productLabelTimeFormat replaces proviant.formatDate, deleted in #396. Rendered
// server-side, so it follows the container's TZ rather than the browser's; the
// Dockerfiles install tzdata because an unresolvable zone falls back to UTC.
const productLabelTimeFormat = "2006-01-02 15:04:05"

// ProductResponse is the client-facing view of a product. ID is serialized as
// the lowercase wire field "id".
type ProductResponse struct {
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
	Quantity             int                      `json:"quantity"`
	PackSize             string                   `json:"packSize"`
	StorageLocationID    *uint                    `json:"storageLocationId"`
	StorageLocation      *StorageLocationResponse `json:"storageLocation,omitempty"`
	PriceOverride        *float64                 `json:"priceOverride,omitempty"`
	CO2KgPerKg           *float64                 `json:"co2KgPerKg,omitempty"`
	RemovalReason        string                   `json:"removalReason"`
	WasteCause           string                   `json:"wasteCause"`
	NotificationLeadDays *int                     `json:"notificationLeadDays,omitempty"`
	MinStockAmount       int                      `json:"minStockAmount"`
	OpenedAt             *time.Time               `json:"openedAt,omitempty"`
	DaysAfterOpening     *int                     `json:"daysAfterOpening,omitempty"`
	CategoryKey          *string                  `json:"categoryKey,omitempty"`

	// Label is set only by NewLabelledProductResponse, to keep the format cost
	// off list responses.
	Label string `json:"label,omitempty"`
}

// NewProductResponse projects a product onto the wire shape, without a label.
func NewProductResponse(product *dbModel.Product) ProductResponse {
	if product == nil {
		return ProductResponse{}
	}
	resp := ProductResponse{
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
		Quantity:             product.Quantity,
		PackSize:             product.PackSize,
		StorageLocationID:    product.StorageLocationID,
		PriceOverride:        product.PriceOverride,
		CO2KgPerKg:           product.CO2KgPerKg,
		RemovalReason:        product.RemovalReason,
		WasteCause:           product.WasteCause,
		NotificationLeadDays: product.NotificationLeadDays,
		MinStockAmount:       product.MinStockAmount,
		OpenedAt:             product.OpenedAt,
		DaysAfterOpening:     product.DaysAfterOpening,
		CategoryKey:          product.CategoryKey,
	}
	if product.StorageLocation != nil {
		location := NewStorageLocationResponse(product.StorageLocation)
		resp.StorageLocation = &location
	}
	return resp
}

// NewProductResponses projects a slice of products. The type parameter covers the
// two repository shapes: []*dbModel.Product and []dbModel.Product.
func NewProductResponses[T dbModel.Product | *dbModel.Product](products []T) []ProductResponse {
	if products == nil {
		return nil
	}
	out := make([]ProductResponse, len(products))
	for i := range products {
		switch typed := any(products[i]).(type) {
		case dbModel.Product:
			out[i] = NewProductResponse(&typed)
		case *dbModel.Product:
			out[i] = NewProductResponse(typed)
		}
	}
	return out
}

// FrequentProduct is one of the products a household buys repeatedly: the
// barcode the household's purchases are grouped by, the naming and the pack
// size the household itself used, and how often and how recently it bought
// the product.
//
// The category and the storage location are the household's own as well, so a
// suggestion reflects what this household actually buys rather than what the
// product is called somewhere else.
type FrequentProduct struct {
	Barcode           string    `json:"barcode"`
	ProductName       string    `json:"productName"`
	PackSize          string    `json:"packSize,omitempty"`
	CategoryKey       *string   `json:"categoryKey,omitempty"`
	StorageLocationID *uint     `json:"storageLocationId,omitempty"`
	ImageURL          string    `json:"imageUrl,omitempty"`
	PurchaseCount     int       `json:"purchaseCount"`
	LastBoughtAt      time.Time `json:"lastBoughtAt"`
}

// NewLabelledProductResponse projects a product and renders its dropdown label.
func NewLabelledProductResponse(product *dbModel.Product) ProductResponse {
	if product == nil {
		return ProductResponse{}
	}
	resp := NewProductResponse(product)
	resp.Label = fmt.Sprintf("%s; Created: %s; Expire at: %s; Notified at: %s",
		product.ProductName,
		product.CreatedAt.Format(productLabelTimeFormat),
		product.ExpireAt.Format(productLabelTimeFormat),
		product.NotifiedAt.Format(productLabelTimeFormat))
	return resp
}

// NewLabelledProductResponses projects a slice of products with labels rendered.
func NewLabelledProductResponses[T dbModel.Product | *dbModel.Product](products []T) []ProductResponse {
	if products == nil {
		return nil
	}
	out := make([]ProductResponse, len(products))
	for i := range products {
		switch typed := any(products[i]).(type) {
		case dbModel.Product:
			out[i] = NewLabelledProductResponse(&typed)
		case *dbModel.Product:
			out[i] = NewLabelledProductResponse(typed)
		}
	}
	return out
}

// ShoppingListItemResponse is the client-facing view of a shopping list entry.
type ShoppingListItemResponse struct {
	ID        uint   `json:"id"`
	ProductID *uint  `json:"productId"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	// CategoryKey is the canonical category the entry belongs to. It is what
	// the household's shelf-life defaults are looked up by.
	CategoryKey *string `json:"categoryKey,omitempty"`
	Quantity    int     `json:"quantity"`
	// PackSize is the size of one pack of the linked product, empty when the
	// entry is not linked to a product or the product has no known size.
	PackSize  string `json:"packSize,omitempty"`
	Checked   bool   `json:"checked"`
	Notes     string `json:"notes"`
	CreatedBy uint   `json:"createdBy"`
}

// NewShoppingListItemResponse projects a shopping list entry onto the wire shape.
func NewShoppingListItemResponse[T dbModel.ShoppingListItem | *dbModel.ShoppingListItem](item T) ShoppingListItemResponse {
	switch typed := any(item).(type) {
	case dbModel.ShoppingListItem:
		return NewShoppingListItemResponse(&typed)
	case *dbModel.ShoppingListItem:
		if typed == nil {
			return ShoppingListItemResponse{}
		}
		return ShoppingListItemResponse{
			ID:          typed.ID,
			ProductID:   typed.ProductID,
			Name:        typed.Name,
			Category:    typed.Category,
			CategoryKey: typed.CategoryKey,
			Quantity:    typed.Quantity,
			PackSize:    typed.PackSize,
			Checked:     typed.Checked,
			Notes:       typed.Notes,
			CreatedBy:   typed.CreatedBy,
		}
	}
	return ShoppingListItemResponse{}
}

// NewShoppingListItemResponses projects a slice of shopping list entries.
func NewShoppingListItemResponses[T dbModel.ShoppingListItem | *dbModel.ShoppingListItem](items []T) []ShoppingListItemResponse {
	if items == nil {
		return nil
	}
	out := make([]ShoppingListItemResponse, len(items))
	for i := range items {
		out[i] = NewShoppingListItemResponse(items[i])
	}
	return out
}

// HouseholdInvitationResponse is the client-facing view of a pending invitation.
type HouseholdInvitationResponse struct {
	ID           uint       `json:"id"`
	Email        string     `json:"email"`
	Status       string     `json:"status"`
	ExpiresAt    time.Time  `json:"expiresAt"`
	SentAt       *time.Time `json:"sentAt,omitempty"`
	SendAttempts int        `json:"sendAttempts"`
}

// NewHouseholdInvitationResponses projects a slice of invitations.
func NewHouseholdInvitationResponses[T dbModel.HouseholdInvitation | *dbModel.HouseholdInvitation](invitations []T) []HouseholdInvitationResponse {
	if invitations == nil {
		return nil
	}
	out := make([]HouseholdInvitationResponse, len(invitations))
	for i := range invitations {
		switch typed := any(invitations[i]).(type) {
		case dbModel.HouseholdInvitation:
			out[i] = newHouseholdInvitationResponse(&typed)
		case *dbModel.HouseholdInvitation:
			out[i] = newHouseholdInvitationResponse(typed)
		}
	}
	return out
}

func newHouseholdInvitationResponse(invitation *dbModel.HouseholdInvitation) HouseholdInvitationResponse {
	if invitation == nil {
		return HouseholdInvitationResponse{}
	}
	return HouseholdInvitationResponse{
		ID:           invitation.ID,
		Email:        invitation.Email,
		Status:       invitation.Status,
		ExpiresAt:    invitation.ExpiresAt,
		SentAt:       invitation.SentAt,
		SendAttempts: invitation.SendAttempts,
	}
}

// HouseholdApplicationResponse is the client-facing view of a household application.
type HouseholdApplicationResponse struct {
	ID          uint      `json:"id"`
	ApplicantID uint      `json:"applicantId"`
	HouseholdID uint      `json:"householdId"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

// NewHouseholdApplicationResponses projects a slice of household applications.
func NewHouseholdApplicationResponses[T dbModel.HouseholdApplication | *dbModel.HouseholdApplication](applications []T) []HouseholdApplicationResponse {
	if applications == nil {
		return nil
	}
	out := make([]HouseholdApplicationResponse, len(applications))
	for i := range applications {
		switch typed := any(applications[i]).(type) {
		case dbModel.HouseholdApplication:
			out[i] = newHouseholdApplicationResponse(&typed)
		case *dbModel.HouseholdApplication:
			out[i] = newHouseholdApplicationResponse(typed)
		}
	}
	return out
}

func newHouseholdApplicationResponse(application *dbModel.HouseholdApplication) HouseholdApplicationResponse {
	if application == nil {
		return HouseholdApplicationResponse{}
	}
	return HouseholdApplicationResponse{
		ID:          application.ID,
		ApplicantID: application.ApplicantID,
		HouseholdID: application.HouseholdID,
		Status:      application.Status,
		CreatedAt:   application.CreatedAt,
	}
}

// OpenFoodFactsCacheResponse is the client-facing view of a cached OpenFoodFacts
// lookup.
type OpenFoodFactsCacheResponse struct {
	Barcode     string   `json:"barcode"`
	ProductName string   `json:"productName"`
	Categories  string   `json:"categories"`
	Countries   string   `json:"countries"`
	ImageURL    string   `json:"imageUrl"`
	PackSize    string   `json:"packSize,omitempty"`
	CO2KgPerKg  *float64 `json:"co2KgPerKg,omitempty"`
	StorageHint string   `json:"storageHint,omitempty"`
}

// NewOpenFoodFactsCacheResponse projects a cache row onto the wire shape.
func NewOpenFoodFactsCacheResponse(entry *dbModel.OpenFoodFactsCache) OpenFoodFactsCacheResponse {
	if entry == nil {
		return OpenFoodFactsCacheResponse{}
	}
	return OpenFoodFactsCacheResponse{
		Barcode:     entry.Barcode,
		ProductName: entry.ProductName,
		Categories:  entry.Categories,
		Countries:   entry.Countries,
		ImageURL:    entry.ImageURL,
		PackSize:    entry.PackSize,
		CO2KgPerKg:  entry.CO2KgPerKg,
		StorageHint: entry.StorageHint,
	}
}
