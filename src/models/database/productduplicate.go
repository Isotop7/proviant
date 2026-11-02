package database

import (
	"strings"
	"time"
)

// IsDuplicateOf reports whether two products describe the same stock: the
// same barcode in the same storage location with the same effective expiry.
//
// The effective expiry is the one Product.EffectiveExpireAt computes, not the
// printed date: an opened pack with three days of shelf life expires on the
// same day as a fresh one printed for that date, and those two rows are the
// same stock.
//
// The rule lives here so it cannot be copied: the repository, the handler and
// the client must agree on what a duplicate is.
func (p *Product) IsDuplicateOf(other *Product) bool {
	if p == nil || other == nil {
		return false
	}
	if NormalizeBarcode(p.Barcode) != NormalizeBarcode(other.Barcode) {
		return false
	}
	if !sameStorageLocation(p.StorageLocationID, other.StorageLocationID) {
		return false
	}
	return sameExpiryDay(p.EffectiveExpireAt(), other.EffectiveExpireAt())
}

// NormalizeBarcode is the barcode as the app stores and compares it: trimmed.
// A padded barcode is the same barcode, so a row written with padding must
// still match a row written without it.
func NormalizeBarcode(barcode string) string {
	return strings.TrimSpace(barcode)
}

// sameStorageLocation reports whether two products sit in the same place. A
// product without a location sits nowhere, and two of them sit in the same
// nowhere.
func sameStorageLocation(a, b *uint) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

// sameExpiryDay reports whether two effective expiries fall on the same
// calendar day.
//
// The stored expiry carries a time of day: the client sends the midnight of
// the day it picked, and an opened shelf life is that midnight plus whole
// days. Two rows therefore land on the same day at different instants, and
// only the day is what the user means. The day is read in the container's
// zone, because that is the zone the server renders dates in.
func sameExpiryDay(a, b time.Time) bool {
	if a.IsZero() || b.IsZero() {
		return a.IsZero() && b.IsZero()
	}
	return a.In(time.Local).Format("2006-01-02") == b.In(time.Local).Format("2006-01-02")
}
