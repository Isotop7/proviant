package api

// BackupSchemaVersion is the version of the household backup JSON document
// produced by the full export, written by the scheduled backup job and accepted
// by the restore endpoint. It is bumped whenever a field is removed, renamed or
// changes meaning; additive fields keep the version and are restored as
// best-effort. The restore endpoint requires this exact value so an old or
// foreign document cannot silently lose fields the current schema carries.
const BackupSchemaVersion = 3

// BackupSchemaVersionNetContent is the version that stored a product's net
// content instead of a pack count. A document of that version is still
// accepted: its measured rows describe one pack of that size.
const BackupSchemaVersionNetContent = 2

// SupportsBackupVersion reports whether the restore endpoint can read a
// document of the given version.
func SupportsBackupVersion(version int) bool {
	return version == BackupSchemaVersion || version == BackupSchemaVersionNetContent
}

// FullExportHousehold is the household metadata carried in a backup document.
// The goal fields are included because they are household configuration, not
// derived statistics, so a restore can put them back.
type FullExportHousehold struct {
	Name                    string   `json:"name"`
	CreatedAt               string   `json:"createdAt"`
	Description             string   `json:"description,omitempty"`
	MonthlyWasteGoalType    string   `json:"monthlyWasteGoalType,omitempty"`
	MonthlyWasteGoalCount   *int     `json:"monthlyWasteGoalCount,omitempty"`
	MonthlyWasteGoalPercent *float64 `json:"monthlyWasteGoalPercent,omitempty"`
}

// FullExportMember is one household member as far as a backup can carry it.
// Credentials are deliberately absent: a restore never recreates accounts, it
// only maps product ownership back onto members that still exist.
type FullExportMember struct {
	Username    string `json:"username"`
	MailAddress string `json:"email"`
	Role        string `json:"role,omitempty"`
}

// FullExportProducts splits the household stock into the two states the UI
// already distinguishes.
type FullExportProducts struct {
	Active   []ExportProductResponse `json:"active"`
	Archived []ExportProductResponse `json:"archived"`
}

// FullExportResponse is the versioned household backup document. The same
// shape is returned by GET /api/v1/products/export/full.json, written by the
// scheduled backup job and consumed by the restore endpoint, so the three
// paths cannot drift apart.
type FullExportResponse struct {
	Version          int                       `json:"version"`
	Household        *FullExportHousehold      `json:"household"`
	Members          []FullExportMember        `json:"members"`
	StorageLocations []StorageLocationResponse `json:"storageLocations,omitempty"`
	Products         FullExportProducts        `json:"products"`
	Stats            ProductStatsResponse      `json:"stats"`
	ExportedAt       string                    `json:"exportedAt"`
}

// RestoreResult reports what a successful restore replaced. The counts are the
// values an operator needs to sanity-check a backup file; the message mirrors
// the CSV import response style.
type RestoreResult struct {
	Message          string `json:"message"`
	Version          int    `json:"version"`
	ProductsImported int    `json:"productsImported"`
	ProductsArchived int    `json:"productsArchived"`
	LocationsCreated int    `json:"locationsCreated"`
}
