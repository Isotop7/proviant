package database

// OffProduct is one row of the imported OpenFoodFacts bulk dump. Only the
// fields Proviant actually uses are kept, serialized as JSON in Data, which
// makes the store far smaller than the raw dump.
type OffProduct struct {
	Barcode string `gorm:"primaryKey;size:32" json:"barcode"`
	Data    string `gorm:"type:longtext;not null" json:"data"`
}

// OffDumpState is a key/value row tracking dump import progress: the last full
// import time and the timestamp of the last applied delta. It lives beside the
// dump products so a re-import can decide what to fetch without touching the
// main database.
type OffDumpState struct {
	StateKey string `gorm:"primaryKey;size:64" json:"stateKey"`
	Value    string `gorm:"not null" json:"value"`
}
