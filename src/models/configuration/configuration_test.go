package configuration

import (
	"html/template"
	"testing"

	"codeberg.org/isotop7/proviant/controllers/database"
	proviantErrors "codeberg.org/isotop7/proviant/errors"
)

func TestDatabaseMariaDBConfigurationStruct(t *testing.T) {
	t.Run("can create DatabaseMariaDBConfiguration", func(t *testing.T) {
		config := DatabaseMariaDBConfiguration{
			Host:     "localhost",
			Port:     3306,
			Name:     "proviant",
			User:     "proviant_user",
			Password: "secret",
		}

		if config.Host != "localhost" {
			t.Errorf("Host = %v, want localhost", config.Host)
		}
		if config.Port != 3306 {
			t.Errorf("Port = %v, want 3306", config.Port)
		}
		if config.Name != "proviant" {
			t.Errorf("Name = %v, want proviant", config.Name)
		}
		if config.User != "proviant_user" {
			t.Errorf("User = %v, want proviant_user", config.User)
		}
		if config.Password != "secret" {
			t.Errorf("Password = %v, want secret", config.Password)
		}
	})
}

func TestDatabaseSQLiteConfigurationStruct(t *testing.T) {
	t.Run("can create DatabaseSQLiteConfiguration", func(t *testing.T) {
		config := DatabaseSQLiteConfiguration{
			Filepath: "/path/to/database.db",
		}

		if config.Filepath != "/path/to/database.db" {
			t.Errorf("Filepath = %v, want /path/to/database.db", config.Filepath)
		}
	})
}

func TestDatabaseConfigurationStruct(t *testing.T) {
	t.Run("can create DatabaseConfiguration", func(t *testing.T) {
		config := DatabaseConfiguration{
			Engine: "sqlite",
			SQLite: DatabaseSQLiteConfiguration{
				Filepath: "/path/to/database.db",
			},
			SelectedEngine: database.SQLite,
		}

		if config.Engine != "sqlite" {
			t.Errorf("Engine = %v, want sqlite", config.Engine)
		}
		if config.SelectedEngine != database.SQLite {
			t.Errorf("SelectedEngine = %v, want SQLite", config.SelectedEngine)
		}
	})
}

func TestAuthenticationConfigurationStruct(t *testing.T) {
	t.Run("can create AuthenticationConfiguration", func(t *testing.T) {
		config := AuthenticationConfiguration{
			TokenPassword: "secret-key-12345",
			TokenLifetime: 24,
		}

		if config.TokenPassword != "secret-key-12345" {
			t.Errorf("TokenPassword = %v, want secret-key-12345", config.TokenPassword)
		}
		if config.TokenLifetime != 24 {
			t.Errorf("TokenLifetime = %v, want 24", config.TokenLifetime)
		}
	})
}

func TestCorsConfigurationStruct(t *testing.T) {
	t.Run("can create CorsConfiguration", func(t *testing.T) {
		config := CorsConfiguration{
			AllowAllOrigins: true,
			AllowedOrigins:  []string{"http://localhost:3000", "https://example.com"},
		}

		if !config.AllowAllOrigins {
			t.Errorf("AllowAllOrigins = %v, want true", config.AllowAllOrigins)
		}
		if len(config.AllowedOrigins) != 2 {
			t.Errorf("AllowedOrigins length = %v, want 2", len(config.AllowedOrigins))
		}
	})
}

func TestServerConfigurationStruct(t *testing.T) {
	t.Run("can create ServerConfiguration", func(t *testing.T) {
		config := ServerConfiguration{
			Port: 5114,
			Authentication: AuthenticationConfiguration{
				TokenPassword: "secret",
				TokenLifetime: 24,
			},
			CORS: CorsConfiguration{
				AllowAllOrigins: true,
			},
		}

		if config.Port != 5114 {
			t.Errorf("Port = %v, want 5114", config.Port)
		}
		if config.Authentication.TokenPassword != "secret" {
			t.Errorf("Authentication.TokenPassword mismatch")
		}
		if !config.CORS.AllowAllOrigins {
			t.Errorf("CORS.AllowAllOrigins = %v, want true", config.CORS.AllowAllOrigins)
		}
	})
}

func TestLoggingConfigurationStruct(t *testing.T) {
	t.Run("can create LoggingConfiguration", func(t *testing.T) {
		config := LoggingConfiguration{
			Enabled: true,
			File:    "/var/log/proviant.log",
		}

		if !config.Enabled {
			t.Errorf("Enabled = %v, want true", config.Enabled)
		}
		if config.File != "/var/log/proviant.log" {
			t.Errorf("File = %v, want /var/log/proviant.log", config.File)
		}
	})
}

func TestSMTPConfigurationStruct(t *testing.T) {
	t.Run("can create SMTPConfiguration", func(t *testing.T) {
		config := SMTPConfiguration{
			Host:     "smtp.example.com",
			Port:     587,
			SSL:      true,
			User:     "user@example.com",
			Password: "secret",
		}

		if config.Host != "smtp.example.com" {
			t.Errorf("Host = %v, want smtp.example.com", config.Host)
		}
		if config.Port != 587 {
			t.Errorf("Port = %v, want 587", config.Port)
		}
		if !config.SSL {
			t.Errorf("SSL = %v, want true", config.SSL)
		}
		if config.User != "user@example.com" {
			t.Errorf("User = %v, want user@example.com", config.User)
		}
		if config.Password != "secret" {
			t.Errorf("Password mismatch")
		}
	})
}

func TestNotificationConfigurationStruct(t *testing.T) {
	t.Run("can create NotificationConfiguration", func(t *testing.T) {
		config := NotificationConfiguration{
			Enabled:  true,
			Interval: 24,
			SMTP: SMTPConfiguration{
				Host:        "smtp.example.com",
				Port:        587,
				FromAddress: "notifications@example.com",
			},
		}

		if !config.Enabled {
			t.Errorf("Enabled = %v, want true", config.Enabled)
		}
		if config.Interval != 24 {
			t.Errorf("Interval = %v, want 24", config.Interval)
		}
		if config.SMTP.FromAddress != "notifications@example.com" {
			t.Errorf("FromAddress = %v, want notifications@example.com", config.SMTP.FromAddress)
		}
		if config.SMTP.Host != "smtp.example.com" {
			t.Errorf("SMTP.Host = %v, want smtp.example.com", config.SMTP.Host)
		}
	})
}

func TestOpenFoodFactsConfigurationStruct(t *testing.T) {
	t.Run("can create OpenFoodFactsConfiguration", func(t *testing.T) {
		config := OpenFoodFactsConfiguration{
			URL:     "https://world.openfoodfacts.org",
			Timeout: 10,
		}

		if config.URL != "https://world.openfoodfacts.org" {
			t.Errorf("URL = %v, want https://world.openfoodfacts.org", config.URL)
		}
		if config.Timeout != 10 {
			t.Errorf("Timeout = %v, want 10", config.Timeout)
		}
	})
}

func TestProviantConfigurationStruct(t *testing.T) {
	config := ProviantConfiguration{
		Database: DatabaseConfiguration{
			Engine: "sqlite",
			SQLite: DatabaseSQLiteConfiguration{
				Filepath: "/path/to/database.db",
			},
		},
		Server: ServerConfiguration{
			Port: 5114,
		},
		Logging: LoggingConfiguration{
			Enabled: true,
		},
		Notification: NotificationConfiguration{
			Enabled: true,
		},
		OpenFoodFacts: OpenFoodFactsConfiguration{
			URL: "https://world.openfoodfacts.org",
		},
		TemplateCache: make(map[string]*template.Template),
	}

	t.Run("database engine", func(t *testing.T) {
		if config.Database.Engine != "sqlite" {
			t.Errorf("Database.Engine = %v, want sqlite", config.Database.Engine)
		}
	})
	t.Run("server port", func(t *testing.T) {
		if config.Server.Port != 5114 {
			t.Errorf("Server.Port = %v, want 5114", config.Server.Port)
		}
	})
	t.Run("logging enabled", func(t *testing.T) {
		if !config.Logging.Enabled {
			t.Errorf("Logging.Enabled = %v, want true", config.Logging.Enabled)
		}
	})
	t.Run("notification enabled", func(t *testing.T) {
		if !config.Notification.Enabled {
			t.Errorf("Notification.Enabled = %v, want true", config.Notification.Enabled)
		}
	})
	t.Run("openfoodfacts url", func(t *testing.T) {
		if config.OpenFoodFacts.URL != "https://world.openfoodfacts.org" {
			t.Errorf("OpenFoodFacts.URL = %v, want https://world.openfoodfacts.org", config.OpenFoodFacts.URL)
		}
	})
}

func assertValidationError(t *testing.T, err error, want error) {
	t.Helper()
	if want == nil {
		if err != nil {
			t.Errorf("expected no error but got: %v", err)
		}
		return
	}
	if err == nil {
		t.Errorf("expected error but got none")
		return
	}
	if err.Error() != want.Error() {
		t.Errorf("error = %v, want %v", err.Error(), want.Error())
	}
}

func assertEngineSet(t *testing.T, config *ProviantConfiguration) {
	t.Helper()
	if config.Database.SelectedEngine != database.MariaDB && config.Database.SelectedEngine != database.SQLite {
		t.Errorf("SelectedEngine not set correctly")
	}
}

func TestValidateOpenFoodFactsConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		config  *ProviantConfiguration
		wantErr error
	}{
		{
			name: "valid configuration",
			config: &ProviantConfiguration{
				OpenFoodFacts: OpenFoodFactsConfiguration{
					URL:     "https://world.openfoodfacts.org",
					Timeout: 10,
				},
			},
			wantErr: nil,
		},
		{
			name: "valid configuration with minimum timeout",
			config: &ProviantConfiguration{
				OpenFoodFacts: OpenFoodFactsConfiguration{
					URL:     "https://world.openfoodfacts.org",
					Timeout: 1,
				},
			},
			wantErr: nil,
		},
		{
			name: "empty URL",
			config: &ProviantConfiguration{
				OpenFoodFacts: OpenFoodFactsConfiguration{
					URL:     "",
					Timeout: 10,
				},
			},
			wantErr: proviantErrors.ErrOpenFoodFactsAPIEmptyURL,
		},
		{
			name: "zero timeout",
			config: &ProviantConfiguration{
				OpenFoodFacts: OpenFoodFactsConfiguration{
					URL:     "https://world.openfoodfacts.org",
					Timeout: 0,
				},
			},
			wantErr: proviantErrors.ErrOpenFoodFactsAPIInvalidTimeout,
		},
		{
			name: "negative timeout",
			config: &ProviantConfiguration{
				OpenFoodFacts: OpenFoodFactsConfiguration{
					URL:     "https://world.openfoodfacts.org",
					Timeout: -1,
				},
			},
			wantErr: proviantErrors.ErrOpenFoodFactsAPIInvalidTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateOpenFoodFactsConfiguration()
			assertValidationError(t, err, tt.wantErr)
		})
	}
}

func TestValidateDatabaseConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		config  *ProviantConfiguration
		wantErr error
	}{
		{
			name: "valid SQLite configuration",
			config: &ProviantConfiguration{
				Database: DatabaseConfiguration{
					Engine: "sqlite",
					SQLite: DatabaseSQLiteConfiguration{
						Filepath: "/path/to/database.db",
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "valid MariaDB configuration",
			config: &ProviantConfiguration{
				Database: DatabaseConfiguration{
					Engine: "mariadb",
					MariaDB: DatabaseMariaDBConfiguration{
						Host:     "localhost",
						Port:     3306,
						Name:     "proviant",
						User:     "proviant_user",
						Password: "secret",
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "valid MariaDB configuration with minimum port",
			config: &ProviantConfiguration{
				Database: DatabaseConfiguration{
					Engine: "mariadb",
					MariaDB: DatabaseMariaDBConfiguration{
						Host:     "localhost",
						Port:     1,
						Name:     "proviant",
						User:     "user",
						Password: "pass",
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "invalid engine",
			config: &ProviantConfiguration{
				Database: DatabaseConfiguration{
					Engine: "postgresql",
				},
			},
			wantErr: proviantErrors.ErrDatabaseInvalidEngine,
		},
		{
			name: "empty engine",
			config: &ProviantConfiguration{
				Database: DatabaseConfiguration{
					Engine: "",
				},
			},
			wantErr: proviantErrors.ErrDatabaseInvalidEngine,
		},
		{
			name: "MariaDB with empty host",
			config: &ProviantConfiguration{
				Database: DatabaseConfiguration{
					Engine: "mariadb",
					MariaDB: DatabaseMariaDBConfiguration{
						Host:     "",
						Port:     3306,
						Name:     "proviant",
						User:     "user",
						Password: "pass",
					},
				},
			},
			wantErr: proviantErrors.ErrDatabaseMariaDBEmptyHost,
		},
		{
			name: "MariaDB with empty user",
			config: &ProviantConfiguration{
				Database: DatabaseConfiguration{
					Engine: "mariadb",
					MariaDB: DatabaseMariaDBConfiguration{
						Host:     "localhost",
						Port:     3306,
						Name:     "proviant",
						User:     "",
						Password: "pass",
					},
				},
			},
			wantErr: proviantErrors.ErrDatabaseMariaDBEmptyUser,
		},
		{
			name: "MariaDB with empty password",
			config: &ProviantConfiguration{
				Database: DatabaseConfiguration{
					Engine: "mariadb",
					MariaDB: DatabaseMariaDBConfiguration{
						Host:     "localhost",
						Port:     3306,
						Name:     "proviant",
						User:     "user",
						Password: "",
					},
				},
			},
			wantErr: proviantErrors.ErrDatabaseMariaDBEmptyPassword,
		},
		{
			name: "MariaDB with empty database name",
			config: &ProviantConfiguration{
				Database: DatabaseConfiguration{
					Engine: "mariadb",
					MariaDB: DatabaseMariaDBConfiguration{
						Host:     "localhost",
						Port:     3306,
						Name:     "",
						User:     "user",
						Password: "pass",
					},
				},
			},
			wantErr: proviantErrors.ErrDatabaseMariaDBEmptyName,
		},
		{
			name: "MariaDB with zero port",
			config: &ProviantConfiguration{
				Database: DatabaseConfiguration{
					Engine: "mariadb",
					MariaDB: DatabaseMariaDBConfiguration{
						Host:     "localhost",
						Port:     0,
						Name:     "proviant",
						User:     "user",
						Password: "pass",
					},
				},
			},
			wantErr: proviantErrors.ErrDatabaseMariaDBInvalidPort,
		},
		{
			name: "MariaDB with negative port",
			config: &ProviantConfiguration{
				Database: DatabaseConfiguration{
					Engine: "mariadb",
					MariaDB: DatabaseMariaDBConfiguration{
						Host:     "localhost",
						Port:     -1,
						Name:     "proviant",
						User:     "user",
						Password: "pass",
					},
				},
			},
			wantErr: proviantErrors.ErrDatabaseMariaDBInvalidPort,
		},
		{
			name: "SQLite with empty filepath",
			config: &ProviantConfiguration{
				Database: DatabaseConfiguration{
					Engine: "sqlite",
					SQLite: DatabaseSQLiteConfiguration{
						Filepath: "",
					},
				},
			},
			wantErr: proviantErrors.ErrDatabaseSQLiteInvalidPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateDatabaseConfiguration()
			assertValidationError(t, err, tt.wantErr)
			if tt.wantErr == nil {
				assertEngineSet(t, tt.config)
			}
		})
	}
}
