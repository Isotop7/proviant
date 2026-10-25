package database

import (
	"testing"
	"time"
)

// The duplicate rule is what stands between the user and a second row for the
// same stock. It keys on barcode, storage location and effective expiry, and
// nothing else: two yogurts of one brand with two best-before dates are two
// products, not a duplicate.
func TestIsDuplicateOf(t *testing.T) {
	expiry := time.Date(2026, time.October, 29, 0, 0, 0, 0, time.UTC)
	location := uint(4)
	otherLocation := uint(5)
	openedAt := time.Date(2026, time.October, 20, 0, 0, 0, 0, time.UTC)
	shelfLife := 9

	base := Product{
		Barcode:           "4006381333931",
		ProductName:       "Yogurt",
		ExpireAt:          expiry,
		StorageLocationID: &location,
		Quantity:          1,
	}

	tests := []struct {
		name  string
		other Product
		want  bool
	}{
		{
			name:  "the same product is a duplicate",
			other: base,
			want:  true,
		},
		{
			name: "a different expiry is not a duplicate",
			other: Product{
				Barcode:           "4006381333931",
				ExpireAt:          expiry.AddDate(0, 0, 1),
				StorageLocationID: &location,
			},
			want: false,
		},
		{
			name: "a different location is not a duplicate",
			other: Product{
				Barcode:           "4006381333931",
				ExpireAt:          expiry,
				StorageLocationID: &otherLocation,
			},
			want: false,
		},
		{
			name: "a different barcode is not a duplicate",
			other: Product{
				Barcode:           "1234567890123",
				ExpireAt:          expiry,
				StorageLocationID: &location,
			},
			want: false,
		},
		{
			name: "a padded barcode is the same barcode",
			other: Product{
				Barcode:           " 4006381333931 ",
				ExpireAt:          expiry,
				StorageLocationID: &location,
			},
			want: true,
		},
		{
			name: "a later time on the same day is a duplicate",
			other: Product{
				Barcode:           "4006381333931",
				ExpireAt:          expiry.Add(12 * time.Hour),
				StorageLocationID: &location,
			},
			want: true,
		},
		{
			name: "an undated product is not a duplicate of a dated one",
			other: Product{
				Barcode: "4006381333931",
			},
			want: false,
		},
		{
			name: "an opened pack matches a fresh one expiring the same day",
			other: Product{
				Barcode:           "4006381333931",
				ExpireAt:          expiry.AddDate(0, 0, 30),
				OpenedAt:          &openedAt,
				DaysAfterOpening:  &shelfLife,
				StorageLocationID: &location,
			},
			want: true,
		},
		{
			name: "an opened pack does not match a fresh one expiring later",
			other: Product{
				Barcode:           "4006381333931",
				ExpireAt:          expiry.AddDate(0, 0, 1),
				StorageLocationID: &location,
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := base.IsDuplicateOf(&test.other); got != test.want {
				t.Errorf("IsDuplicateOf = %t, want %t", got, test.want)
			}
		})
	}
}

// Two products without a location sit in the same nowhere, so they are
// duplicates of each other.
func TestIsDuplicateOf_WithoutLocation(t *testing.T) {
	expiry := time.Date(2026, time.October, 29, 0, 0, 0, 0, time.UTC)
	base := Product{Barcode: "4006381333931", ExpireAt: expiry}
	other := Product{Barcode: "4006381333931", ExpireAt: expiry}

	if !base.IsDuplicateOf(&other) {
		t.Error("IsDuplicateOf = false, want true for two products without a location")
	}

	location := uint(4)
	withLocation := Product{Barcode: "4006381333931", ExpireAt: expiry, StorageLocationID: &location}
	if base.IsDuplicateOf(&withLocation) {
		t.Error("IsDuplicateOf = true, want false for a product with and without a location")
	}
}

// An undated product is not a duplicate of a dated one: one of them expires,
// the other never does.
func TestIsDuplicateOf_UndatedAgainstDated(t *testing.T) {
	expiry := time.Date(2026, time.October, 29, 0, 0, 0, 0, time.UTC)
	undated := Product{Barcode: "4006381333931"}
	dated := Product{Barcode: "4006381333931", ExpireAt: expiry}

	if undated.IsDuplicateOf(&dated) {
		t.Error("IsDuplicateOf = true, want false for an undated and a dated product")
	}
	if dated.IsDuplicateOf(&undated) {
		t.Error("IsDuplicateOf = true, want false for a dated and an undated product")
	}
}

// A nil product is not a duplicate of anything.
func TestIsDuplicateOf_Nil(t *testing.T) {
	product := Product{Barcode: "4006381333931"}
	if product.IsDuplicateOf(nil) {
		t.Error("IsDuplicateOf = true, want false for a nil product")
	}
	if (*Product)(nil).IsDuplicateOf(&product) {
		t.Error("IsDuplicateOf = true, want false for a nil receiver")
	}
}
