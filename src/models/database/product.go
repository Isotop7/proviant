package database

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	RemovalReasonConsumed = "consumed"
	RemovalReasonWasted   = "wasted"

	// WasteCause names why a product was wasted. The cause is a property of the
	// waste action, not of RemovalReason: a wasted product keeps
	// RemovalReason == RemovalReasonWasted and carries the cause beside it.
	// An empty cause means the waste was not attributed.
	WasteCauseSpoiled    = "spoiled"
	WasteCauseOverbought = "overbought"
	WasteCauseForgotten  = "forgotten"
	WasteCauseDisliked   = "disliked"
	WasteCauseOther      = "other"

	// MaxDaysAfterOpening is the upper bound on Product.DaysAfterOpening,
	// enforced on every write (create and PATCH). The opened-shelf-life SQL
	// prunes (effectiveExpiryCandidateScope and the notification candidate
	// start) bound their OpenedAt floor with this same value, so the write
	// bound and the query bound must stay in sync: a stored value above it
	// can put a row inside a query window while sitting below the prune floor,
	// silently dropping it from expiring-soon lists and notifications.
	MaxDaysAfterOpening = 365
)

// wasteCauses is the fixed cause list in the order the UI offers it.
var wasteCauses = [...]string{
	WasteCauseSpoiled,
	WasteCauseOverbought,
	WasteCauseForgotten,
	WasteCauseDisliked,
	WasteCauseOther,
}

// WasteCauseList returns the fixed cause list in the order the UI offers it.
// It hands out a copy: the list is shared by every request that renders it,
// and one that could be mutated would leak into all of them.
func WasteCauseList() []string {
	list := make([]string, len(wasteCauses))
	copy(list, wasteCauses[:])
	return list
}

// IsValidWasteCause reports whether cause is one of the known causes. The empty
// cause is valid: a waste without a reason is still a waste.
func IsValidWasteCause(cause string) bool {
	if cause == "" {
		return true
	}
	for _, known := range wasteCauses {
		if cause == known {
			return true
		}
	}
	return false
}

// Product is the database model of a product. A product is counted in packs:
// Quantity is the number of packs on hand and PackSize is the size of one
// pack, a free-text label that is never converted and never computed with.
// PriceOverride is the price of one pack, so the savings of a product are
// the override times the number of packs.
type Product struct {
	gorm.Model
	Barcode           string           `gorm:"index:idx_products_barcode_household,priority:1" json:"barcode"`
	ProductName       string           `gorm:"index:idx_products_household_name_deleted,priority:1" json:"productName"`
	Categories        string           `json:"categories"`
	Countries         string           `json:"countries"`
	ImageURL          string           `json:"imageUrl"`
	ExpireAt          time.Time        `gorm:"index" json:"expireAt"`
	ScannedAt         time.Time        `json:"scannedAt"`
	NotifiedAt        time.Time        `json:"notifiedAt"`
	DeletedAt         gorm.DeletedAt   `gorm:"index:idx_products_household_deleted,priority:2"`
	HouseholdID       uint             `gorm:"index;index:idx_products_household_deleted,priority:1;index:idx_products_barcode_household,priority:2;index:idx_products_household_name_deleted,priority:2;index:idx_products_household_clientsyncid,priority:1;not null" json:"-"`
	Household         Household        `json:"-"`
	UserID            uint             `gorm:"index, not null" json:"-"`
	IsPrivate         bool             `gorm:"default:false" json:"isPrivate"`
	Quantity          int              `gorm:"default:1" json:"quantity"`
	PackSize          string           `json:"packSize"`
	DisplayQuantity   string           `gorm:"-"                            json:"-"`
	StorageLocationID *uint            `gorm:"index"                        json:"storageLocationId"`
	ClientSyncID      *string          `gorm:"index:idx_products_household_clientsyncid,unique,priority:2" json:"clientSyncId,omitempty"`
	StorageLocation   *StorageLocation `gorm:"foreignKey:StorageLocationID" json:"storageLocation,omitempty"`
	PriceOverride     *float64         `gorm:"default:null"                 json:"priceOverride,omitempty"`
	CO2KgPerKg        *float64         `gorm:"default:null"                 json:"co2KgPerKg,omitempty"`
	RemovalReason     string           `gorm:"default:''"                   json:"removalReason"`
	// WasteCause is why the product was wasted. It is set when the waste
	// archives the row and is empty for an unattributed waste.
	WasteCause           string     `gorm:"default:''"                   json:"wasteCause"`
	StorageHint          string     `gorm:"-"                            json:"-"`
	NotificationLeadDays *int       `gorm:"default:null"                 json:"notificationLeadDays,omitempty"`
	MinStockAmount       int        `gorm:"default:0"                   json:"minStockAmount"`
	OpenedAt             *time.Time `gorm:"default:null"                 json:"openedAt,omitempty"`
	DaysAfterOpening     *int       `gorm:"default:null"                 json:"daysAfterOpening,omitempty"`
	// CategoryKey is the canonical category the product belongs to. It is what
	// the household's shelf-life defaults are looked up by. Categories keeps
	// the free text Open Food Facts wrote, which the recipe suggestions read.
	CategoryKey *string `gorm:"index" json:"categoryKey,omitempty"`

	// AllowDuplicate says the caller has seen the duplicate refusal and wants
	// a row of its own anyway. It is a request flag and never stored.
	AllowDuplicate bool `gorm:"-" json:"allowDuplicate,omitempty"`
}

// ProductDTOExpire is a simplified DTO for product expiration
type ProductDTOExpire struct {
	ID       uint   `json:"id"`
	Barcode  string `json:"barcode"`
	ExpireAt Date   `json:"expireAt"`
}

// ProductDTOBarcode is a simplified DTO only containing a barcode
type ProductDTOBarcode struct {
	Barcode string `json:"barcode"`
}

// ProductDTOPatch is a simplified DTO for the patchable elements. A field the
// request did not carry keeps its stored value; a field the request carried
// is written, so a scalar sent as null clears the field and a scalar sent
// with a value replaces it. PriceOverride is the price of one pack.
type ProductDTOPatch struct {
	ID                   uint       `json:"ID"`
	ProductName          string     `json:"productName"`
	Categories           string     `json:"categories"`
	Countries            string     `json:"countries"`
	ImageURL             string     `json:"imageUrl"`
	ExpireAt             time.Time  `json:"expireAt"`
	Quantity             int        `json:"quantity"`
	PackSize             string     `json:"packSize"`
	StorageLocationID    *uint      `json:"storageLocationId"`
	NotificationLeadDays *int       `json:"notificationLeadDays,omitempty"`
	MinStockAmount       int        `json:"minStockAmount"`
	IsPrivate            bool       `json:"isPrivate"`
	PriceOverride        *float64   `json:"priceOverride,omitempty"`
	OpenedAt             *time.Time `json:"openedAt,omitempty"`
	DaysAfterOpening     *int       `json:"daysAfterOpening,omitempty"`
	// CategoryKey is the canonical category of the product. It is sent as
	// null to clear it.
	CategoryKey *string `json:"categoryKey,omitempty"`

	present map[string]bool
}

// Has reports whether the request carried the given field. A patch that was
// not decoded from a request body reports every field as present, so a
// direct call keeps its full-replacement meaning.
func (p *ProductDTOPatch) Has(field string) bool {
	if p.present == nil {
		return true
	}
	return p.present[strings.ToLower(field)]
}

// MarkAllPresent makes every field count as carried, so a patch written to a
// product replaces it: a field the request did not carry is cleared instead of
// keeping its stored value.
func (p *ProductDTOPatch) MarkAllPresent() {
	p.present = nil
}

// UnmarshalJSON decodes the patch and remembers which fields the request
// carried, so an omitted field can be told apart from one sent as null.
//
// The keys are folded to lower case because the decoder matches a field name
// case-insensitively: a body that carries "Quantity" fills the field, so it
// must also count as carrying it. A key that only differs in case would
// otherwise be decoded and then dropped, and the field would keep its stored
// value.
func (p *ProductDTOPatch) UnmarshalJSON(data []byte) error {
	type patchFields ProductDTOPatch
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	var decoded patchFields
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*p = ProductDTOPatch(decoded)
	p.present = make(map[string]bool, len(raw))
	for field := range raw {
		p.present[strings.ToLower(field)] = true
	}
	return nil
}
