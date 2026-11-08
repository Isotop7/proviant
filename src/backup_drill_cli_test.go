package main

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/models/configuration"
	"github.com/Isotop7/proviant/util"
)

// TestBackupDrillCommandVerifiesADocument covers the drill command on a
// document that restores: it reports the counts and leaves the exit code at
// zero, so a scheduled job sees a healthy backup.
func TestBackupDrillCommandVerifiesADocument(t *testing.T) {
	code := runBackupDrillWithoutConfig([]string{"--file", backupDrillFixture(t, "household-1.json")})
	if code != controllers.BackupDrillExitOK {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitOK)
	}
}

// TestBackupDrillCommandFailsOnASchemaMismatch keeps a document written
// against another schema apart from a broken one: the drill exits with the
// schema code, not the failure code.
func TestBackupDrillCommandFailsOnASchemaMismatch(t *testing.T) {
	code := runBackupDrillWithoutConfig([]string{"--file", backupDrillFixture(t, "household-1-schema-mismatch.json")})
	if code != controllers.BackupDrillExitSchemaMismatch {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitSchemaMismatch)
	}
}

// TestBackupDrillCommandFailsOnABrokenDocument covers the truncated file: the
// drill exits with the failure code.
func TestBackupDrillCommandFailsOnABrokenDocument(t *testing.T) {
	code := runBackupDrillWithoutConfig([]string{"--file", backupDrillFixture(t, "household-1-truncated.json")})
	if code != controllers.BackupDrillExitFailed {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitFailed)
	}
}

// TestBackupDrillCommandFailsWithoutABackup covers a backup directory that
// holds nothing: there is nothing to drill.
func TestBackupDrillCommandFailsWithoutABackup(t *testing.T) {
	code := runBackupDrillWithoutConfig([]string{"--directory", t.TempDir()})
	if code != controllers.BackupDrillExitUnavailable {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitUnavailable)
	}
}

// TestBackupDrillCommandReportsAMissingBackupDirectory keeps a directory that
// does not exist apart from one that holds no document: the first is a
// misconfiguration, the second is a backup job that never ran.
func TestBackupDrillCommandReportsAMissingBackupDirectory(t *testing.T) {
	code := runBackupDrillWithoutConfig([]string{"--directory", filepath.Join(t.TempDir(), "missing")})
	if code != controllers.BackupDrillExitUnavailable {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitUnavailable)
	}
}

// TestBackupDrillCommandRejectsAnUnknownFlag covers a usage error: it is
// neither a broken backup nor a broken schema.
func TestBackupDrillCommandRejectsAnUnknownFlag(t *testing.T) {
	code := runBackupDrillWithoutConfig([]string{"--nonsense"})
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

// TestBackupDrillCommandDrillsEveryHousehold covers the everyday drill: the
// backup directory holds one document per household, and every one of them is
// restored and compared.
func TestBackupDrillCommandDrillsEveryHousehold(t *testing.T) {
	code := runBackupDrillWithoutConfig([]string{"--directory", "testdata/backup-drill/directory", "--max-age", "0"})
	if code != controllers.BackupDrillExitOK {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitOK)
	}
}

// TestBackupDrillCommandFailsOnAStaleBackup covers the second silent failure:
// the backup restores, but the job that writes the backups stopped.
func TestBackupDrillCommandFailsOnAStaleBackup(t *testing.T) {
	code := runBackupDrillWithoutConfig([]string{"--directory", "testdata/backup-drill/directory", "--max-age", "1h"})
	if code != controllers.BackupDrillExitStale {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitStale)
	}
}

// TestBackupDrillCommandRejectsAnInvalidMaxAge covers a usage error: a bound
// that is not a duration is neither a broken backup nor a broken schema.
func TestBackupDrillCommandRejectsAnInvalidMaxAge(t *testing.T) {
	code := runBackupDrillWithoutConfig([]string{"--max-age", "yesterday"})
	if code != controllers.BackupDrillExitUnavailable {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitUnavailable)
	}
}

// TestBackupDrillCommandReportsAMissingDocument covers a document that is not
// there: there is nothing to drill, which is not the same as a broken
// backup.
func TestBackupDrillCommandReportsAMissingDocument(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "proviant-backup-household-1-20260101T000000.000Z.json")

	code := runBackupDrillWithoutConfig([]string{"--file", missing})
	if code != controllers.BackupDrillExitUnavailable {
		t.Errorf("exit code = %d, want %d", code, controllers.BackupDrillExitUnavailable)
	}
}

// TestBackupDrillMaxAge covers the bound the drill applies when the operator
// names none: twice the backup interval, so one missed run is tolerated and
// two is a failure. It is the only part of the drill that decides whether a
// stopped backup job is caught.
func TestBackupDrillMaxAge(t *testing.T) {
	hourly := &configuration.ProviantConfiguration{}
	hourly.Backup.Enabled = true
	hourly.Backup.IntervalHours = 1

	unset := &configuration.ProviantConfiguration{}
	unset.Backup.Enabled = true

	disabled := &configuration.ProviantConfiguration{}

	cases := []struct {
		name      string
		flag      string
		config    *configuration.ProviantConfiguration
		configErr error
		want      time.Duration
	}{
		{name: "the flag wins", flag: "30m", config: hourly, want: 30 * time.Minute},
		{name: "the flag switches the check off", flag: "0", config: hourly, want: 0},
		{name: "the interval decides", flag: "", config: hourly, want: 2 * time.Hour},
		{name: "the default interval decides", flag: "", config: unset, want: 2 * time.Duration(util.DefaultBackupIntervalHours) * time.Hour},
		{name: "a disabled backup has no bound", flag: "", config: disabled, want: 0},
		{name: "an unreadable configuration has no bound", flag: "", config: nil, configErr: stderrors.New("no configuration"), want: 0},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := backupDrillMaxAge(testCase.flag, testCase.config, testCase.configErr)
			if err != nil {
				t.Fatalf("backupDrillMaxAge: %v", err)
			}
			if got != testCase.want {
				t.Errorf("max age = %s, want %s", got, testCase.want)
			}
		})
	}
}

// TestBackupDrillMaxAgeRejectsAForeignDuration covers a usage error: a bound
// that is not a duration is neither a broken backup nor a broken schema.
func TestBackupDrillMaxAgeRejectsAForeignDuration(t *testing.T) {
	if _, err := backupDrillMaxAge("yesterday", nil, stderrors.New("no configuration")); err == nil {
		t.Fatal("backupDrillMaxAge error = nil, want a usage error")
	}
}

// TestBackupDrillCommandUsesTheConfiguredInterval covers the production path
// the other tests bypass: the drill derives its staleness bound from the
// backup interval in the configuration.
func TestBackupDrillCommandUsesTheConfiguredInterval(t *testing.T) {
	directory := t.TempDir()
	payload, err := os.ReadFile(backupDrillFixture(t, "household-1.json"))
	if err != nil {
		t.Fatalf("read the fixture: %v", err)
	}
	name := filepath.Join(directory, util.BackupDocumentName(1, time.Now().Add(-3*time.Hour)))
	if err := os.WriteFile(name, payload, 0o600); err != nil {
		t.Fatalf("write the backup document: %v", err)
	}

	hourly := &configuration.ProviantConfiguration{}
	hourly.Backup.Enabled = true
	hourly.Backup.IntervalHours = 1
	hourly.Backup.Directory = directory

	daily := &configuration.ProviantConfiguration{}
	daily.Backup.Enabled = true
	daily.Backup.IntervalHours = 24
	daily.Backup.Directory = directory

	if code := runBackupDrillWith([]string{"--directory", directory}, hourly, nil); code != controllers.BackupDrillExitStale {
		t.Errorf("exit code = %d, want %d for a backup older than twice the interval", code, controllers.BackupDrillExitStale)
	}
	if code := runBackupDrillWith([]string{"--directory", directory}, daily, nil); code != controllers.BackupDrillExitOK {
		t.Errorf("exit code = %d, want %d for a backup within twice the interval", code, controllers.BackupDrillExitOK)
	}
}

// runBackupDrillWithoutConfig runs the drill command without a configuration
// on disk. The command reads the configuration of the working directory, and a
// developer's own config.yaml must not decide what the drill does.
func runBackupDrillWithoutConfig(args []string) int {
	return runBackupDrillWith(args, nil, stderrors.New("no configuration was read"))
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
