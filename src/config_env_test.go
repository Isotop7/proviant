package main

import (
	"os"
	"path/filepath"
	"testing"
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
