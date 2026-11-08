package util

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// BackupDocumentName builds the name of a household backup document. The
// name carries the household and the moment the document was written, so the
// drill can tell the households apart and order their documents by age
// without reading a single row.
func BackupDocumentName(householdID uint, writtenAt time.Time) string {
	return fmt.Sprintf("%shousehold-%d-%s.json",
		BackupFilePrefix,
		householdID,
		writtenAt.UTC().Format(BackupDocumentTimestampLayout),
	)
}

// BackupDocumentPrefix is the prefix every document of one household carries.
// Retention pruning matches on it, so one household's prune can never touch
// another's documents.
func BackupDocumentPrefix(householdID uint) string {
	return fmt.Sprintf("%shousehold-%d-", BackupFilePrefix, householdID)
}

// ParseBackupDocumentName reads the household and the write time out of a
// backup document's name. A name that carries neither is not a document this
// release wrote: the drill reports it without an expectation rather than
// guessing what it holds.
func ParseBackupDocumentName(name string) (householdID uint, writtenAt time.Time, ok bool) {
	rest, found := strings.CutPrefix(name, BackupFilePrefix)
	if !found {
		return 0, time.Time{}, false
	}
	rest, found = strings.CutPrefix(rest, "household-")
	if !found {
		return 0, time.Time{}, false
	}
	idPart, stampPart, found := strings.Cut(rest, "-")
	if !found {
		return 0, time.Time{}, false
	}
	id, err := strconv.ParseUint(idPart, 10, 32)
	if err != nil {
		return 0, time.Time{}, false
	}
	if !strings.HasSuffix(stampPart, ".json") {
		return 0, time.Time{}, false
	}
	stamp, err := time.Parse(BackupDocumentTimestampLayout, strings.TrimSuffix(stampPart, ".json"))
	if err != nil {
		return 0, time.Time{}, false
	}
	return uint(id), stamp, true
}
