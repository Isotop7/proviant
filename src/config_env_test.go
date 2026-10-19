package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Isotop7/proviant/util"
)

const fixtureConfigYAML = `server:
  authentication:
    tokenPassword: "from-file"
`

// TestLoadConfigEnvironmentOverride guards the configuration path the compose
// files and the README depend on: PROVIANT_* variables must reach the
// unmarshalled struct. viper.AutomaticEnv only applies inside Get(), while
// Unmarshal walks AllKeys() → Get(), so if that chain ever changes the override
// disappears silently — and with it the only way to supply the required JWT
// secret in a container.
//
// The fixture is written to a temp dir because the real config.yaml is
// gitignored: a fresh clone (and therefore CI) has none.
func TestLoadConfigEnvironmentOverride(t *testing.T) {
	const key = "PROVIANT_SERVER_AUTHENTICATION_TOKENPASSWORD"

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(fixtureConfigYAML), 0o600); err != nil {
		t.Fatalf("failed to write fixture config: %v", err)
	}

	t.Run("file value is used when no environment variable is set", func(t *testing.T) {
		t.Setenv(key, "") // viper treats an empty value as unset

		config := loadConfig(dir)
		if config.Server.Authentication.TokenPassword != "from-file" {
			t.Errorf("TokenPassword = %q, want %q",
				config.Server.Authentication.TokenPassword, "from-file")
		}
	})

	t.Run("environment variable overrides the file", func(t *testing.T) {
		t.Setenv(key, "env-supplied-secret")

		config := loadConfig(dir)
		if config.Server.Authentication.TokenPassword != "env-supplied-secret" {
			t.Errorf("TokenPassword = %q, want %q",
				config.Server.Authentication.TokenPassword, "env-supplied-secret")
		}
	})
}

// The offline sync cap must survive a config that omits the
// block: the outbox client splits its batches at the value
// injected into the page, so an unset cap has to resolve to
// the built-in default rather than to zero.
func TestLoadConfigOfflineSyncDefault(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(fixtureConfigYAML), 0o600); err != nil {
		t.Fatalf("failed to write fixture config: %v", err)
	}

	config := loadConfig(dir)
	if config.OfflineSync.MaxBatchBytes != util.ReceiptBulkMaxBodyBytes {
		t.Errorf("OfflineSync.MaxBatchBytes = %d, want %d (the viper default)",
			config.OfflineSync.MaxBatchBytes, util.ReceiptBulkMaxBodyBytes)
	}
}

// An operator-set cap reaches the struct verbatim — the value
// the endpoint enforces and the page injects is this one, not
// the default.
func TestLoadConfigOfflineSyncExplicit(t *testing.T) {
	const yaml = `offlineSync:
  maxBatchBytes: 2048
`
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatalf("failed to write fixture config: %v", err)
	}

	config := loadConfig(dir)
	if config.OfflineSync.MaxBatchBytes != 2048 {
		t.Errorf("OfflineSync.MaxBatchBytes = %d, want 2048", config.OfflineSync.MaxBatchBytes)
	}
}

// The dump import must resolve every knob to its built-in default when the
// block is absent, so an operator only flips dumpImportEnabled to opt in.
func TestLoadConfigDumpDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(fixtureConfigYAML), 0o600); err != nil {
		t.Fatalf("failed to write fixture config: %v", err)
	}

	config := loadConfig(dir)
	if config.OpenFoodFacts.DumpImportEnabled {
		t.Errorf("DumpImportEnabled = true, want false (opt-in feature)")
	}
	if config.OpenFoodFacts.DumpImportExpiryDays != util.DefaultOpenFoodFactsDumpExpiryDays {
		t.Errorf("DumpImportExpiryDays = %d, want %d",
			config.OpenFoodFacts.DumpImportExpiryDays, util.DefaultOpenFoodFactsDumpExpiryDays)
	}
	if config.OpenFoodFacts.DumpURL != util.OpenFoodFactsDumpURL {
		t.Errorf("DumpURL = %q, want %q", config.OpenFoodFacts.DumpURL, util.OpenFoodFactsDumpURL)
	}
	if config.OpenFoodFacts.DumpDeltaWindowDays != util.DefaultOpenFoodFactsDeltaWindowDays {
		t.Errorf("DumpDeltaWindowDays = %d, want %d",
			config.OpenFoodFacts.DumpDeltaWindowDays, util.DefaultOpenFoodFactsDeltaWindowDays)
	}
	if config.OpenFoodFacts.DumpDatabase != util.OffDumpDefaultDatabase {
		t.Errorf("DumpDatabase = %q, want %q", config.OpenFoodFacts.DumpDatabase, util.OffDumpDefaultDatabase)
	}
	if config.OpenFoodFacts.DumpStagingPath != util.OffDumpStagingDir {
		t.Errorf("DumpStagingPath = %q, want %q", config.OpenFoodFacts.DumpStagingPath, util.OffDumpStagingDir)
	}
}

// Environment overrides must reach the nested dump keys, matching the documented
// PROVIANT_OPENFOODFACTS_DUMP* variables.
func TestLoadConfigDumpEnvironmentOverride(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(fixtureConfigYAML), 0o600); err != nil {
		t.Fatalf("failed to write fixture config: %v", err)
	}

	t.Setenv("PROVIANT_OPENFOODFACTS_DUMPIMPORTENABLED", "true")
	t.Setenv("PROVIANT_OPENFOODFACTS_DUMPIMPORTEXPIRYDAYS", "3")
	t.Setenv("PROVIANT_OPENFOODFACTS_DUMPDATABASE", "custom_dump")

	config := loadConfig(dir)
	if !config.OpenFoodFacts.DumpImportEnabled {
		t.Errorf("DumpImportEnabled = false, want true")
	}
	if config.OpenFoodFacts.DumpImportExpiryDays != 3 {
		t.Errorf("DumpImportExpiryDays = %d, want 3", config.OpenFoodFacts.DumpImportExpiryDays)
	}
	if config.OpenFoodFacts.DumpDatabase != "custom_dump" {
		t.Errorf("DumpDatabase = %q, want custom_dump", config.OpenFoodFacts.DumpDatabase)
	}
}
