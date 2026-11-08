package util

import (
	"testing"
	"time"
)

// TestBackupDocumentNameCarriesTheHouseholdAndTheWriteTime covers the name
// the backup job writes: it names the household and the moment the document
// was written, so the drill can tell two households apart and order their
// documents by age.
func TestBackupDocumentNameCarriesTheHouseholdAndTheWriteTime(t *testing.T) {
	writtenAt := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

	name := BackupDocumentName(7, writtenAt)

	if name != "proviant-backup-household-7-20261103T120000.000Z.json" {
		t.Errorf("name = %q, want %q", name, "proviant-backup-household-7-20261103T120000.000Z.json")
	}
}

// TestParseBackupDocumentNameReadsBackWhatTheNameCarries covers the other
// side of the round trip: the drill reads the household and the write time
// out of the name the writer produced.
func TestParseBackupDocumentNameReadsBackWhatTheNameCarries(t *testing.T) {
	writtenAt := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

	householdID, parsed, ok := ParseBackupDocumentName(BackupDocumentName(7, writtenAt))

	if !ok {
		t.Fatal("ParseBackupDocumentName ok = false, want true")
	}
	if householdID != 7 {
		t.Errorf("household id = %d, want 7", householdID)
	}
	if !parsed.Equal(writtenAt) {
		t.Errorf("written at = %s, want %s", parsed, writtenAt)
	}
}

// TestParseBackupDocumentNameReadsAHouseholdWithTwoDigits covers a household
// id that is longer than one digit: the name has to split the id from the
// timestamp on the right dash.
func TestParseBackupDocumentNameReadsAHouseholdWithTwoDigits(t *testing.T) {
	writtenAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	householdID, parsed, ok := ParseBackupDocumentName(BackupDocumentName(10, writtenAt))

	if !ok {
		t.Fatal("ParseBackupDocumentName ok = false, want true")
	}
	if householdID != 10 {
		t.Errorf("household id = %d, want 10", householdID)
	}
	if !parsed.Equal(writtenAt) {
		t.Errorf("written at = %s, want %s", parsed, writtenAt)
	}
}

// TestParseBackupDocumentNameRejectsAForeignName covers a name this release
// did not write: the drill reports such a document without an expectation
// rather than guessing what it holds.
func TestParseBackupDocumentNameRejectsAForeignName(t *testing.T) {
	for _, name := range []string{
		"household-1.json",
		"proviant-backup-household-1.json",
		"proviant-backup-household-one-20261103T120000.000Z.json",
		"proviant-backup-household-1-20261103T120000.000Z.txt",
		"proviant-backup-household-1-yesterday.json",
		"proviant-backup-manual.json",
	} {
		if _, _, ok := ParseBackupDocumentName(name); ok {
			t.Errorf("ParseBackupDocumentName(%q) ok = true, want false", name)
		}
	}
}

// TestBackupDocumentPrefixNamesOneHousehold covers the prefix retention
// pruning matches on: it names one household and no other.
func TestBackupDocumentPrefixNamesOneHousehold(t *testing.T) {
	prefix := BackupDocumentPrefix(10)

	if prefix != "proviant-backup-household-10-" {
		t.Errorf("prefix = %q, want %q", prefix, "proviant-backup-household-10-")
	}
}
