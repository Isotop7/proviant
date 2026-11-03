package controllers

import (
	"slices"
	"strconv"
	"strings"
	"time"

	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/authentication"
	dbModel "github.com/Isotop7/proviant/models/database"

	"gorm.io/gorm"
)

// productDigest is the comparable shape of one product row: the fields the
// restore has to carry over unchanged. A count says how many rows came back;
// the digest says whether they are the right ones.
type productDigest struct {
	// key is the canonical rendering of the row. It is built once, when the
	// digest is built, and every comparison orders by it.
	key        string
	Barcode    string
	Archived   bool
	Name       string
	Categories string
	Quantity   int
	PackSize   string
	ExpireAt   time.Time
	IsPrivate  bool
	Owner      string
	Location   string
}

// The entities a mismatch can belong to.
const (
	// BackupDrillMismatchProduct is a product row.
	BackupDrillMismatchProduct = "product"
	// BackupDrillMismatchStorageLocation is a storage location.
	BackupDrillMismatchStorageLocation = "storage location"
	// BackupDrillMismatchHousehold is the household metadata.
	BackupDrillMismatchHousehold = "household"
)

// BackupDrillMismatch names one field that came back different from the
// document.
type BackupDrillMismatch struct {
	// Entity names what the mismatch belongs to: a product, a storage
	// location or the household.
	Entity string
	// Key names the row the mismatch belongs to: the barcode of a product,
	// the name of a location.
	Key string
	// Field is the name of the field that differs.
	Field string
	// Document is the value the document carries.
	Document string
	// Restored is the value the restore produced.
	Restored string
}

// compareProductRows compares every product of the document with the rows
// the restore produced. The counts say how many rows came back; this says
// whether they are the right ones: a restore that keeps the count and writes
// the wrong values is the failure a count cannot see.
func compareProductRows(
	document *apiModel.FullExportResponse,
	db *gorm.DB,
	household scratchHousehold,
) ([]BackupDrillMismatch, error) {
	expected := documentProductDigests(document, household)
	restored, err := restoredProductDigests(db, household.ID)
	if err != nil {
		return nil, err
	}

	slices.SortFunc(expected, func(a, b productDigest) int {
		return compareProductDigests(&a, &b)
	})
	slices.SortFunc(restored, func(a, b productDigest) int {
		return compareProductDigests(&a, &b)
	})

	mismatches := make([]BackupDrillMismatch, 0)
	for i := range expected {
		if i >= len(restored) {
			break
		}
		// The key is the canonical form of a row: two times for the same
		// instant in different zones are not equal as structs.
		if expected[i].key == restored[i].key {
			continue
		}
		mismatches = append(mismatches, expected[i].difference(&restored[i]))
	}
	return mismatches, nil
}

// documentProductDigests projects the document's products onto the comparable
// shape. The owner and the pack count are resolved the way the restore
// resolves them, so the comparison is against what the restore was asked to
// write and not against the document's raw wording.
func documentProductDigests(
	document *apiModel.FullExportResponse,
	household scratchHousehold,
) []productDigest {
	memberIDs := make(map[string]uint, len(household.Usernames))
	for id, username := range household.Usernames {
		memberIDs[strings.ToLower(username)] = id
	}

	digests := make([]productDigest, 0, len(document.Products.Active)+len(document.Products.Archived))
	for i := range document.Products.Active {
		digests = append(digests, documentProductDigest(&document.Products.Active[i], false, household, memberIDs))
	}
	for i := range document.Products.Archived {
		digests = append(digests, documentProductDigest(&document.Products.Archived[i], true, household, memberIDs))
	}
	return digests
}

// documentProductDigest projects one exported row onto the comparable shape.
func documentProductDigest(
	row *apiModel.ExportProductResponse,
	archived bool,
	household scratchHousehold,
	memberIDs map[string]uint,
) productDigest {
	quantity, packSize := restoreProductQuantity(row)
	owner := resolveBackupOwnerID(memberIDs, row.OwnerUsername, household.AdminUserID)

	location := ""
	if row.StorageLocation != nil {
		location = strings.ToLower(strings.TrimSpace(row.StorageLocation.Name))
	}
	digest := productDigest{
		Barcode:    row.Barcode,
		Archived:   archived,
		Name:       row.ProductName,
		Categories: row.Categories,
		Quantity:   quantity,
		PackSize:   packSize,
		ExpireAt:   row.ExpireAt,
		IsPrivate:  row.IsPrivate,
		Owner:      strings.ToLower(household.Usernames[owner]),
		Location:   location,
	}
	digest.key = productDigestKey(&digest)
	return digest
}

// restoredProductDigests projects the restored rows onto the comparable
// shape. The owner and the storage location are instance-local, so they are
// resolved by name.
func restoredProductDigests(db *gorm.DB, householdID uint) ([]productDigest, error) {
	var products []dbModel.Product
	if err := db.Unscoped().Where("household_id = ?", householdID).Find(&products).Error; err != nil {
		return nil, err
	}

	var locations []dbModel.StorageLocation
	if err := db.Unscoped().Where("household_id = ?", householdID).Find(&locations).Error; err != nil {
		return nil, err
	}
	locationNames := make(map[uint]string, len(locations))
	for i := range locations {
		locationNames[locations[i].ID] = strings.ToLower(strings.TrimSpace(locations[i].Name))
	}

	var accounts []authentication.User
	if err := db.Where("household_id = ?", householdID).Find(&accounts).Error; err != nil {
		return nil, err
	}
	usernames := make(map[uint]string, len(accounts))
	for i := range accounts {
		usernames[accounts[i].ID] = strings.ToLower(accounts[i].Username)
	}

	digests := make([]productDigest, 0, len(products))
	for i := range products {
		product := &products[i]
		location := ""
		if product.StorageLocationID != nil {
			location = locationNames[*product.StorageLocationID]
		}
		digest := productDigest{
			Barcode:    product.Barcode,
			Archived:   product.DeletedAt.Valid,
			Name:       product.ProductName,
			Categories: product.Categories,
			Quantity:   product.Quantity,
			PackSize:   product.PackSize,
			ExpireAt:   product.ExpireAt,
			IsPrivate:  product.IsPrivate,
			Owner:      usernames[product.UserID],
			Location:   location,
		}
		digest.key = productDigestKey(&digest)
		digests = append(digests, digest)
	}
	return digests, nil
}

// compareProductDigests orders two rows so that equal rows sit next to each
// other. The whole row is the key: two products that share a barcode but not
// a count are two different rows, and a swap between them is not a mismatch.
func compareProductDigests(a, b *productDigest) int {
	return strings.Compare(a.key, b.key)
}

// productDigestKey renders a row as the string the comparison orders by.
func productDigestKey(digest *productDigest) string {
	return strings.Join([]string{
		digest.Barcode,
		strconv.FormatBool(digest.Archived),
		digest.Name,
		digest.Categories,
		strconv.Itoa(digest.Quantity),
		digest.PackSize,
		strconv.FormatInt(digest.ExpireAt.UTC().UnixNano(), 10),
		strconv.FormatBool(digest.IsPrivate),
		digest.Owner,
		digest.Location,
	}, "\x00")
}

// difference reports the first field in which two rows differ.
func (d *productDigest) difference(other *productDigest) BackupDrillMismatch {
	mismatch := BackupDrillMismatch{Entity: BackupDrillMismatchProduct, Key: d.Barcode}
	switch {
	case d.Barcode != other.Barcode:
		mismatch.Field = "barcode"
		mismatch.Document = d.Barcode
		mismatch.Restored = other.Barcode
	case d.Archived != other.Archived:
		mismatch.Field = "archived"
		mismatch.Document = strconv.FormatBool(d.Archived)
		mismatch.Restored = strconv.FormatBool(other.Archived)
	case d.Name != other.Name:
		mismatch.Field = "name"
		mismatch.Document = d.Name
		mismatch.Restored = other.Name
	case d.Categories != other.Categories:
		mismatch.Field = "categories"
		mismatch.Document = d.Categories
		mismatch.Restored = other.Categories
	case d.Quantity != other.Quantity:
		mismatch.Field = "quantity"
		mismatch.Document = strconv.Itoa(d.Quantity)
		mismatch.Restored = strconv.Itoa(other.Quantity)
	case d.PackSize != other.PackSize:
		mismatch.Field = "pack size"
		mismatch.Document = d.PackSize
		mismatch.Restored = other.PackSize
	case !d.ExpireAt.Equal(other.ExpireAt):
		mismatch.Field = "expiry"
		mismatch.Document = d.ExpireAt.UTC().Format(time.RFC3339)
		mismatch.Restored = other.ExpireAt.UTC().Format(time.RFC3339)
	case d.IsPrivate != other.IsPrivate:
		mismatch.Field = "private"
		mismatch.Document = strconv.FormatBool(d.IsPrivate)
		mismatch.Restored = strconv.FormatBool(other.IsPrivate)
	case d.Owner != other.Owner:
		mismatch.Field = "owner"
		mismatch.Document = d.Owner
		mismatch.Restored = other.Owner
	case d.Location != other.Location:
		mismatch.Field = "storage location"
		mismatch.Document = d.Location
		mismatch.Restored = other.Location
	}
	return mismatch
}

// locationDigest is the comparable shape of one storage location.
type locationDigest struct {
	Name      string
	Icon      string
	SortOrder int
}

// compareStorageLocations compares the locations the restore created with the
// locations the document carries. The count says how many came back; this
// says whether they are the right ones, including the places no product
// refers to.
func compareStorageLocations(
	document *apiModel.FullExportResponse,
	db *gorm.DB,
	householdID uint,
) ([]BackupDrillMismatch, error) {
	drafts := backupLocationDrafts(document)
	expected := make([]locationDigest, 0, len(drafts))
	for _, draft := range drafts {
		expected = append(expected, locationDigest{
			Name:      strings.ToLower(strings.TrimSpace(draft.name)),
			Icon:      draft.icon,
			SortOrder: draft.sortOrder,
		})
	}

	var locations []dbModel.StorageLocation
	if err := db.Unscoped().Where("household_id = ?", householdID).Find(&locations).Error; err != nil {
		return nil, err
	}
	restored := make([]locationDigest, 0, len(locations))
	for i := range locations {
		restored = append(restored, locationDigest{
			Name:      strings.ToLower(strings.TrimSpace(locations[i].Name)),
			Icon:      locations[i].Icon,
			SortOrder: locations[i].SortOrder,
		})
	}

	slices.SortFunc(expected, compareLocationDigests)
	slices.SortFunc(restored, compareLocationDigests)

	mismatches := make([]BackupDrillMismatch, 0)
	for i := range expected {
		if i >= len(restored) {
			break
		}
		if expected[i] == restored[i] {
			continue
		}
		mismatches = append(mismatches, expected[i].difference(&restored[i]))
	}
	return mismatches, nil
}

// compareLocationDigests orders two locations by their name, so equal
// locations sit next to each other.
func compareLocationDigests(a, b locationDigest) int {
	return strings.Compare(a.Name, b.Name)
}

// difference reports the first field in which two locations differ.
func (d locationDigest) difference(other *locationDigest) BackupDrillMismatch {
	mismatch := BackupDrillMismatch{Entity: BackupDrillMismatchStorageLocation, Key: d.Name}
	switch {
	case d.Name != other.Name:
		mismatch.Field = "name"
		mismatch.Document = d.Name
		mismatch.Restored = other.Name
	case d.Icon != other.Icon:
		mismatch.Field = "icon"
		mismatch.Document = d.Icon
		mismatch.Restored = other.Icon
	case d.SortOrder != other.SortOrder:
		mismatch.Field = "sort order"
		mismatch.Document = strconv.Itoa(d.SortOrder)
		mismatch.Restored = strconv.Itoa(other.SortOrder)
	}
	return mismatch
}

// compareHouseholdMetadata compares the household the restore wrote with the
// household the document carries. The restore replaces the household's name,
// description and waste goals; a restore that drops them leaves a household
// that no longer knows itself.
func compareHouseholdMetadata(
	document *apiModel.FullExportResponse,
	db *gorm.DB,
	householdID uint,
) ([]BackupDrillMismatch, error) {
	if document.Household == nil {
		return nil, nil
	}

	var household dbModel.Household
	if err := db.Unscoped().Where("id = ?", householdID).First(&household).Error; err != nil {
		return nil, err
	}

	expected := document.Household
	mismatches := make([]BackupDrillMismatch, 0)
	compare := func(field, documentValue, restoredValue string) {
		if documentValue == restoredValue {
			return
		}
		mismatches = append(mismatches, BackupDrillMismatch{
			Entity:   BackupDrillMismatchHousehold,
			Key:      household.Name,
			Field:    field,
			Document: documentValue,
			Restored: restoredValue,
		})
	}

	compare("name", expected.Name, household.Name)
	compare("description", expected.Description, household.Description)
	compare("monthly waste goal type", expected.MonthlyWasteGoalType, household.MonthlyWasteGoalType)
	compare("monthly waste goal count", renderOptionalInt(expected.MonthlyWasteGoalCount), renderOptionalInt(household.MonthlyWasteGoalCount))
	compare("monthly waste goal percent", renderOptionalFloat(expected.MonthlyWasteGoalPercent), renderOptionalFloat(household.MonthlyWasteGoalPercent))
	return mismatches, nil
}

// renderOptionalInt renders an optional number for the comparison report.
func renderOptionalInt(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

// renderOptionalFloat renders an optional number for the comparison report.
func renderOptionalFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}
