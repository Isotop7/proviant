package main

import (
	"strings"
	"testing"

	"github.com/Isotop7/proviant/controllers/database"
	"github.com/Isotop7/proviant/models/configuration"
	"github.com/Isotop7/proviant/util"
)

// TestPublishedPasswordWarningNamesTheVariableTheAppReads pins the advice of
// the startup warning: it must name PROVIANT_DATABASE_MARIADB_PASSWORD, the
// only variable the app reads. A deployment that does not run through compose
// has no other way to change the password, so a warning that names only the
// compose-level name sends the operator after a variable nothing reads.
func TestPublishedPasswordWarningNamesTheVariableTheAppReads(t *testing.T) {
	t.Run("the compose file names both variables", func(t *testing.T) {
		config := &configuration.ProviantConfiguration{
			Database: configuration.DatabaseConfiguration{
				SelectedEngine: database.MariaDB,
				MariaDB: configuration.DatabaseMariaDBConfiguration{
					User:     "proviant",
					Password: util.PublishedMariaDBUserPassword,
				},
			},
		}
		warning := publishedPasswordWarning(config)
		if warning == "" {
			t.Fatal("publishedPasswordWarning() = \"\", want a warning")
		}
		if !strings.Contains(warning, "PROVIANT_DATABASE_MARIADB_PASSWORD") {
			t.Errorf("warning %q does not name PROVIANT_DATABASE_MARIADB_PASSWORD", warning)
		}
		if !strings.Contains(warning, "PROVIANT_MARIADB_PASSWORD") {
			t.Errorf("warning %q does not name the compose-level PROVIANT_MARIADB_PASSWORD", warning)
		}
		if !strings.Contains(warning, util.MariaDBComposeFile) {
			t.Errorf("warning %q does not name the file that publishes the password", warning)
		}
	})

	t.Run("the config template names the app variable", func(t *testing.T) {
		config := &configuration.ProviantConfiguration{
			Database: configuration.DatabaseConfiguration{
				SelectedEngine: database.MariaDB,
				MariaDB: configuration.DatabaseMariaDBConfiguration{
					User:     "root",
					Password: util.TemplateMariaDBPassword,
				},
			},
		}
		warning := publishedPasswordWarning(config)
		if !strings.Contains(warning, "PROVIANT_DATABASE_MARIADB_PASSWORD") {
			t.Errorf("warning %q does not name PROVIANT_DATABASE_MARIADB_PASSWORD", warning)
		}
		if strings.Contains(warning, "PROVIANT_MARIADB_PASSWORD") {
			t.Errorf("warning %q names the compose-level variable for a config-file deployment", warning)
		}
		if !strings.Contains(warning, util.MariaDBTemplateFile) {
			t.Errorf("warning %q does not name the file that publishes the password", warning)
		}
	})

	t.Run("an own password warns about nothing", func(t *testing.T) {
		config := &configuration.ProviantConfiguration{
			Database: configuration.DatabaseConfiguration{
				SelectedEngine: database.MariaDB,
				MariaDB: configuration.DatabaseMariaDBConfiguration{
					User:     "proviant",
					Password: "an-own-secret",
				},
			},
		}
		if warning := publishedPasswordWarning(config); warning != "" {
			t.Errorf("publishedPasswordWarning() = %q, want an empty string", warning)
		}
	})

	t.Run("the published root password warns about nothing", func(t *testing.T) {
		config := &configuration.ProviantConfiguration{
			Database: configuration.DatabaseConfiguration{
				SelectedEngine: database.MariaDB,
				MariaDB: configuration.DatabaseMariaDBConfiguration{
					User:     "proviant",
					Password: util.PublishedMariaDBRootPassword,
				},
			},
		}
		if warning := publishedPasswordWarning(config); warning != "" {
			t.Errorf("publishedPasswordWarning() = %q, want an empty string", warning)
		}
	})
}
