package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/models/configuration"
)

// TestBackupDrillCommandVerifiesADocument covers the drill command on a
// document that restores: it reports the counts and leaves the exit code at
// zero, so a scheduled job sees a healthy backup.
func TestBackupDrillCommandVerifiesADocument(t *testing.T) {
	code := runBackupDrill([]string{"--file", backupDrillFixture(t, "household-1.json")})
	if code != controllers.BackupDrillExitOK {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitOK)
	}
}

// TestBackupDrillCommandFailsOnASchemaMismatch keeps a document written
// against another schema apart from a broken one: the drill exits with the
// schema code, not the failure code.
func TestBackupDrillCommandFailsOnASchemaMismatch(t *testing.T) {
	code := runBackupDrill([]string{"--file", backupDrillFixture(t, "household-1-schema-mismatch.json")})
	if code != controllers.BackupDrillExitSchemaMismatch {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitSchemaMismatch)
	}
}

// TestBackupDrillCommandFailsOnABrokenDocument covers the truncated file: the
// drill exits with the failure code.
func TestBackupDrillCommandFailsOnABrokenDocument(t *testing.T) {
	code := runBackupDrill([]string{"--file", backupDrillFixture(t, "household-1-truncated.json")})
	if code != controllers.BackupDrillExitFailed {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitFailed)
	}
}

// TestBackupDrillCommandFailsWithoutABackup covers a backup directory that
// holds nothing: there is nothing to drill.
func TestBackupDrillCommandFailsWithoutABackup(t *testing.T) {
	code := runBackupDrill([]string{"--directory", t.TempDir()})
	if code != controllers.BackupDrillExitUnavailable {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitUnavailable)
	}
}

// TestBackupDrillCommandReportsAMissingBackupDirectory keeps a directory that
// does not exist apart from one that holds no document: the first is a
// misconfiguration, the second is a backup job that never ran.
func TestBackupDrillCommandReportsAMissingBackupDirectory(t *testing.T) {
	code := runBackupDrill([]string{"--directory", filepath.Join(t.TempDir(), "missing")})
	if code != controllers.BackupDrillExitUnavailable {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitUnavailable)
	}
}

// TestBackupDrillCommandRejectsAnUnknownFlag covers a usage error: it is
// neither a broken backup nor a broken schema.
func TestBackupDrillCommandRejectsAnUnknownFlag(t *testing.T) {
	code := runBackupDrill([]string{"--nonsense"})
	if code != controllers.BackupDrillExitUnavailable {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitUnavailable)
	}
}

// TestBackupDrillCommandSkipsADisabledBackup covers the deployment that does
// not run the scheduled backup: there is nothing to drill, and failing the
// scheduled job every night would be noise.
func TestBackupDrillCommandSkipsADisabledBackup(t *testing.T) {
	config := &configuration.ProviantConfiguration{}
	config.Backup.Enabled = false
	config.Backup.Directory = t.TempDir()

	code := runBackupDrillWith(nil, config, nil)
	if code != controllers.BackupDrillExitOK {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitOK)
	}
}

// TestBackupDrillCommandDrillsAnExplicitDirectoryOfADisabledBackup covers the
// other side: a directory the operator named is drilled even when the
// scheduled backup is off — they asked for it.
func TestBackupDrillCommandDrillsAnExplicitDirectoryOfADisabledBackup(t *testing.T) {
	config := &configuration.ProviantConfiguration{}
	config.Backup.Enabled = false

	code := runBackupDrillWith([]string{"--directory", t.TempDir()}, config, nil)
	if code != controllers.BackupDrillExitUnavailable {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitUnavailable)
	}
}

// TestBackupDrillCommandDrillsAnEnabledBackup covers the other side: with the
// backup job enabled, an empty directory is a failure.
func TestBackupDrillCommandDrillsAnEnabledBackup(t *testing.T) {
	config := &configuration.ProviantConfiguration{}
	config.Backup.Enabled = true

	code := runBackupDrillWith([]string{"--directory", t.TempDir()}, config, nil)
	if code != controllers.BackupDrillExitUnavailable {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitUnavailable)
	}
}

// backupDrillFixture returns the path of a backup document from the testdata
// directory.
func backupDrillFixture(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", "backup-drill", name)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the fixture %s is missing: %v", path, err)
	}
	return path
}
