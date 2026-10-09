package configuration

import (
	"html/template"
	"testing"

	"codeberg.org/isotop7/proviant/controllers/database"
	proviantErrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/util"
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

func TestRateLimitConfigurationStruct(t *testing.T) {
	t.Run("can create RateLimitConfiguration", func(t *testing.T) {
		config := RateLimitConfiguration{
			LoginPerMinute:  5,
			SignupPerMinute: 3,
			ExportPerMinute: 1,
		}

		if config.LoginPerMinute != 5 {
			t.Errorf("LoginPerMinute = %v, want 5", config.LoginPerMinute)
		}
		if config.SignupPerMinute != 3 {
			t.Errorf("SignupPerMinute = %v, want 3", config.SignupPerMinute)
		}
		if config.ExportPerMinute != 1 {
			t.Errorf("ExportPerMinute = %v, want 1", config.ExportPerMinute)
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

func TestValidateServerConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		config  *ProviantConfiguration
		wantErr error
	}{
		{
			name: "valid configuration",
			config: &ProviantConfiguration{
				Server: ServerConfiguration{
					Authentication: AuthenticationConfiguration{
						TokenPassword: "secret-key-12345",
						TokenLifetime: 24,
					},
					RateLimit: RateLimitConfiguration{
						LoginPerMinute:    5,
						SignupPerMinute:   3,
						ExportPerMinute:   1,
						PasswordPerMinute: 5,
						ScanPerMinute:     5,
						RecipesPerMinute:  6,
					},
				},
			},
			wantErr: nil,
		},
		{
			name: "zero login rate limit",
			config: &ProviantConfiguration{
				Server: ServerConfiguration{
					Authentication: AuthenticationConfiguration{
						TokenPassword: "secret-key-12345",
						TokenLifetime: 24,
					},
					RateLimit: RateLimitConfiguration{
						LoginPerMinute:  0,
						SignupPerMinute: 3,
						ExportPerMinute: 1,
					},
				},
			},
			wantErr: proviantErrors.ErrRateLimitInvalidValue,
		},
		{
			name: "zero signup rate limit",
			config: &ProviantConfiguration{
				Server: ServerConfiguration{
					Authentication: AuthenticationConfiguration{
						TokenPassword: "secret-key-12345",
						TokenLifetime: 24,
					},
					RateLimit: RateLimitConfiguration{
						LoginPerMinute:  5,
						SignupPerMinute: 0,
						ExportPerMinute: 1,
					},
				},
			},
			wantErr: proviantErrors.ErrRateLimitInvalidValue,
		},
		{
			name: "zero export rate limit",
			config: &ProviantConfiguration{
				Server: ServerConfiguration{
					Authentication: AuthenticationConfiguration{
						TokenPassword: "secret-key-12345",
						TokenLifetime: 24,
					},
					RateLimit: RateLimitConfiguration{
						LoginPerMinute:  5,
						SignupPerMinute: 3,
						ExportPerMinute: 0,
					},
				},
			},
			wantErr: proviantErrors.ErrRateLimitInvalidValue,
		},
		{
			name: "negative rate limit value",
			config: &ProviantConfiguration{
				Server: ServerConfiguration{
					Authentication: AuthenticationConfiguration{
						TokenPassword: "secret-key-12345",
						TokenLifetime: 24,
					},
					RateLimit: RateLimitConfiguration{
						LoginPerMinute:  -1,
						SignupPerMinute: 3,
						ExportPerMinute: 1,
					},
				},
			},
			wantErr: proviantErrors.ErrRateLimitInvalidValue,
		},
		{
			name: "empty token password",
			config: &ProviantConfiguration{
				Server: ServerConfiguration{
					Authentication: AuthenticationConfiguration{
						TokenPassword: "",
						TokenLifetime: 24,
					},
				},
			},
			wantErr: proviantErrors.ErrServerEmptyTokenPassword,
		},
		{
			name: "retired public default token password",
			config: &ProviantConfiguration{
				Server: ServerConfiguration{
					Authentication: AuthenticationConfiguration{
						TokenPassword: util.RetiredTokenPassword,
						TokenLifetime: 24,
					},
				},
			},
			wantErr: proviantErrors.ErrServerRetiredTokenPassword,
		},
		{
			name: "zero token lifetime",
			config: &ProviantConfiguration{
				Server: ServerConfiguration{
					Authentication: AuthenticationConfiguration{
						TokenPassword: "secret-key-12345",
						TokenLifetime: 0,
					},
				},
			},
			wantErr: proviantErrors.ErrServerInvalidTokenLifetime,
		},
		{
			name: "negative token lifetime",
			config: &ProviantConfiguration{
				Server: ServerConfiguration{
					Authentication: AuthenticationConfiguration{
						TokenPassword: "secret-key-12345",
						TokenLifetime: -1,
					},
				},
			},
			wantErr: proviantErrors.ErrServerInvalidTokenLifetime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateServerConfiguration()
			assertValidationError(t, err, tt.wantErr)
		})
	}
}

func TestValidateRecipeAPIConfiguration(t *testing.T) {
	tests := []struct {
		name    string
		config  *ProviantConfiguration
		wantErr error
	}{
		{
			name: "valid TheMealDB configuration",
			config: &ProviantConfiguration{
				RecipeAPI: RecipeAPIConfiguration{
					Provider: util.RecipeProviderThemealDB,
					URL:      "https://www.themealdb.com/api/json/v1/1",
					Timeout:  10,
					CacheTTL: 24,
				},
			},
			wantErr: nil,
		},
		{
			name: "valid Mealie configuration with API key",
			config: &ProviantConfiguration{
				RecipeAPI: RecipeAPIConfiguration{
					Provider: util.RecipeProviderMealie,
					URL:      "https://mealie.example.com",
					APIKey:   "token123",
					Timeout:  10,
					CacheTTL: 24,
				},
			},
			wantErr: nil,
		},
		{
			name: "valid Tandoor configuration with API key",
			config: &ProviantConfiguration{
				RecipeAPI: RecipeAPIConfiguration{
					Provider: util.RecipeProviderTandoor,
					URL:      "https://tandoor.example.com",
					APIKey:   "token123",
					Timeout:  10,
					CacheTTL: 24,
				},
			},
			wantErr: nil,
		},
		{
			name: "empty provider",
			config: &ProviantConfiguration{
				RecipeAPI: RecipeAPIConfiguration{
					URL:     "https://mealie.example.com",
					Timeout: 10,
				},
			},
			wantErr: proviantErrors.ErrRecipeInvalidProvider,
		},
		{
			name: "legacy provider value (spoonacular) falls back to themealdb",
			config: &ProviantConfiguration{
				RecipeAPI: RecipeAPIConfiguration{
					Provider: "spoonacular",
					URL:      "https://www.themealdb.com/api/json/v1/1",
					Timeout:  10,
				},
			},
			wantErr: nil,
		},
		{
			name: "case-insensitive provider normalization",
			config: &ProviantConfiguration{
				RecipeAPI: RecipeAPIConfiguration{
					Provider: "TheMealDB",
					URL:      "https://www.themealdb.com/api/json/v1/1",
					Timeout:  10,
				},
			},
			wantErr: nil,
		},
		{
			name: "legacy case-variant Mealie without API key is rejected",
			config: &ProviantConfiguration{
				RecipeAPI: RecipeAPIConfiguration{
					Provider: "Mealie",
					URL:      "https://www.themealdb.com/api/json/v1/1",
					Timeout:  10,
				},
			},
			wantErr: proviantErrors.ErrRecipeAPIMissingAPIKey,
		},
		{
			name: "unsupported provider falls back to themealdb",
			config: &ProviantConfiguration{
				RecipeAPI: RecipeAPIConfiguration{
					Provider: "kitchenowl",
					URL:      "https://kitchenowl.example.com",
					Timeout:  10,
				},
			},
			wantErr: nil,
		},
		{
			name: "Mealie without API key is rejected",
			config: &ProviantConfiguration{
				RecipeAPI: RecipeAPIConfiguration{
					Provider: util.RecipeProviderMealie,
					URL:      "https://mealie.example.com",
					Timeout:  10,
				},
			},
			wantErr: proviantErrors.ErrRecipeAPIMissingAPIKey,
		},
		{
			name: "Tandoor without API key is rejected",
			config: &ProviantConfiguration{
				RecipeAPI: RecipeAPIConfiguration{
					Provider: util.RecipeProviderTandoor,
					URL:      "https://tandoor.example.com",
					Timeout:  10,
				},
			},
			wantErr: proviantErrors.ErrRecipeAPIMissingAPIKey,
		},
		{
			name: "Mealie with whitespace-only API key is rejected",
			config: &ProviantConfiguration{
				RecipeAPI: RecipeAPIConfiguration{
					Provider: util.RecipeProviderMealie,
					URL:      "https://mealie.example.com",
					APIKey:   "  ",
					Timeout:  10,
				},
			},
			wantErr: proviantErrors.ErrRecipeAPIMissingAPIKey,
		},
		{
			name: "TheMealDB without API key is fine",
			config: &ProviantConfiguration{
				RecipeAPI: RecipeAPIConfiguration{
					Provider: util.RecipeProviderThemealDB,
					URL:      "https://www.themealdb.com/api/json/v1/1",
					Timeout:  10,
				},
			},
			wantErr: nil,
		},
		{
			name: "empty URL",
			config: &ProviantConfiguration{
				RecipeAPI: RecipeAPIConfiguration{
					Provider: util.RecipeProviderThemealDB,
					Timeout:  10,
				},
			},
			wantErr: proviantErrors.ErrRecipeAPIEmptyURL,
		},
		{
			name: "invalid timeout",
			config: &ProviantConfiguration{
				RecipeAPI: RecipeAPIConfiguration{
					Provider: util.RecipeProviderThemealDB,
					URL:      "https://www.themealdb.com/api/json/v1/1",
					Timeout:  0,
				},
			},
			wantErr: proviantErrors.ErrRecipeAPIInvalidTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateRecipeAPIConfiguration()
			assertValidationError(t, err, tt.wantErr)
		})
	}
}

func TestValidateRecipeAPIConfigurationSelfHostedFallback(t *testing.T) {
	t.Run("unknown provider falls back to themealdb and keeps URL", func(t *testing.T) {
		config := &ProviantConfiguration{
			RecipeAPI: RecipeAPIConfiguration{
				Provider: "spoonacular",
				URL:      "https://mealie.example.com",
				Timeout:  10,
			},
		}
		err := config.ValidateRecipeAPIConfiguration()
		assertValidationError(t, err, nil)
		if config.RecipeAPI.Provider != util.RecipeProviderThemealDB {
			t.Errorf("expected provider fallback to %q, got %q", util.RecipeProviderThemealDB, config.RecipeAPI.Provider)
		}
		// The configured URL must never be overwritten: the old builds called
		// TheMealDB endpoints against the configured URL, and a silent reset
		// would redirect inventory-derived queries to the public API.
		if config.RecipeAPI.URL != "https://mealie.example.com" {
			t.Errorf("expected URL to be preserved, got %q", config.RecipeAPI.URL)
		}
	})

	t.Run("self-hosted provider without API key is rejected", func(t *testing.T) {
		for _, provider := range []string{util.RecipeProviderMealie, util.RecipeProviderTandoor, "Mealie"} {
			config := &ProviantConfiguration{
				RecipeAPI: RecipeAPIConfiguration{
					Provider: provider,
					URL:      "https://mealie.example.com",
					Timeout:  10,
				},
			}
			err := config.ValidateRecipeAPIConfiguration()
			assertValidationError(t, err, proviantErrors.ErrRecipeAPIMissingAPIKey)
		}
	})

	// The provider-independent checks must run before the key check, so a
	// missing key can no longer return early and hide an empty URL, an invalid
	// timeout or an unnormalized cache TTL.
	t.Run("provider-independent problems are reported before the missing key", func(t *testing.T) {
		tests := []struct {
			name    string
			config  RecipeAPIConfiguration
			wantErr error
		}{
			{
				name:    "empty URL wins over missing key",
				config:  RecipeAPIConfiguration{Provider: util.RecipeProviderMealie, Timeout: 10},
				wantErr: proviantErrors.ErrRecipeAPIEmptyURL,
			},
			{
				name:    "invalid timeout wins over missing key",
				config:  RecipeAPIConfiguration{Provider: util.RecipeProviderMealie, URL: "https://mealie.example.com"},
				wantErr: proviantErrors.ErrRecipeAPIInvalidTimeout,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				config := &ProviantConfiguration{RecipeAPI: tt.config}
				err := config.ValidateRecipeAPIConfiguration()
				assertValidationError(t, err, tt.wantErr)
			})
		}
	})

	// The missing-key path must leave the config fully normalized so the caller
	// disabling the feature cannot end up with a dead cache or no timeout.
	t.Run("missing key still normalizes the shared fields", func(t *testing.T) {
		config := &ProviantConfiguration{
			RecipeAPI: RecipeAPIConfiguration{
				Provider: util.RecipeProviderMealie,
				URL:      "https://mealie.example.com",
				Timeout:  10,
			},
		}
		err := config.ValidateRecipeAPIConfiguration()
		assertValidationError(t, err, proviantErrors.ErrRecipeAPIMissingAPIKey)
		if config.RecipeAPI.CacheTTL != 24 {
			t.Errorf("CacheTTL = %d, want 24 (normalized before the key check)", config.RecipeAPI.CacheTTL)
		}
		// The provider must stay as configured: the caller disables the feature
		// rather than pairing this URL with a different provider dialect.
		if config.RecipeAPI.Provider != util.RecipeProviderMealie {
			t.Errorf("Provider = %q, want %q (unchanged)", config.RecipeAPI.Provider, util.RecipeProviderMealie)
		}
	})
}

func TestValidateRecipeAPIConfigurationExplicitThemealDBKeepsURL(t *testing.T) {
	config := &ProviantConfiguration{
		RecipeAPI: RecipeAPIConfiguration{
			Provider: util.RecipeProviderThemealDB,
			URL:      "https://themealdb-proxy.example.com/api",
			Timeout:  10,
		},
	}
	if err := config.ValidateRecipeAPIConfiguration(); err != nil {
		t.Fatalf("ValidateRecipeAPIConfiguration() error = %v", err)
	}
	if config.RecipeAPI.URL != "https://themealdb-proxy.example.com/api" {
		t.Errorf("explicit themealdb URL was overwritten, got %q", config.RecipeAPI.URL)
	}
}

// TestValidateRecipeAPIConfigurationRejectsPublicThemealDBURL covers the API key
// leak: a self-hosted provider paired with the public TheMealDB URL sends the
// operator's api_key plus the household's inventory keywords to a third party on
// every request. The URL is unambiguously wrong and no substitute can be
// guessed, so startup must refuse rather than warn-and-disable.
func TestValidateRecipeAPIConfigurationRejectsPublicThemealDBURL(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		url      string
		apiKey   string
		wantErr  error
	}{
		{
			name:     "Mealie with an API key on the public host is rejected",
			provider: util.RecipeProviderMealie,
			url:      "https://www.themealdb.com/api/json/v1/1",
			apiKey:   "secret",
			wantErr:  proviantErrors.ErrRecipeAPIProviderURLMismatch,
		},
		{
			name:     "Tandoor with an API key on the public host is rejected",
			provider: util.RecipeProviderTandoor,
			url:      "https://www.themealdb.com/api/json/v1/1",
			apiKey:   "secret",
			wantErr:  proviantErrors.ErrRecipeAPIProviderURLMismatch,
		},
		{
			name:     "case-variant host is rejected",
			provider: util.RecipeProviderMealie,
			url:      "HTTPS://WWW.THEMEALDB.COM/api/json/v1/1",
			apiKey:   "secret",
			wantErr:  proviantErrors.ErrRecipeAPIProviderURLMismatch,
		},
		{
			name:     "bare apex domain is rejected",
			provider: util.RecipeProviderTandoor,
			url:      "https://themealdb.com/api",
			apiKey:   "secret",
			wantErr:  proviantErrors.ErrRecipeAPIProviderURLMismatch,
		},
		{
			name:     "a subdomain of the public host is rejected",
			provider: util.RecipeProviderMealie,
			url:      "https://api.themealdb.com/api/json/v1/1",
			apiKey:   "secret",
			wantErr:  proviantErrors.ErrRecipeAPIProviderURLMismatch,
		},
		{
			// A fully qualified name carrying the DNS root dot resolves to the
			// identical host, so leaving it in place let "www.themealdb.com."
			// past the guard and sent the key to the third party it exists
			// to protect.
			name:     "trailing-dot FQDN is rejected",
			provider: util.RecipeProviderMealie,
			url:      "https://www.themealdb.com./api/json/v1/1",
			apiKey:   "secret",
			wantErr:  proviantErrors.ErrRecipeAPIProviderURLMismatch,
		},
		{
			name:     "trailing-dot apex domain is rejected",
			provider: util.RecipeProviderTandoor,
			url:      "https://themealdb.com./api",
			apiKey:   "secret",
			wantErr:  proviantErrors.ErrRecipeAPIProviderURLMismatch,
		},
		{
			name:     "trailing dot on an unrelated host is accepted",
			provider: util.RecipeProviderMealie,
			url:      "https://mealie.example.com./",
			apiKey:   "secret",
			wantErr:  nil,
		},
		{
			name:     "a proxy host merely containing the domain is accepted",
			provider: util.RecipeProviderMealie,
			url:      "https://themealdb-proxy.example.com",
			apiKey:   "secret",
			wantErr:  nil,
		},
		{
			name:     "the operator's own host is accepted",
			provider: util.RecipeProviderTandoor,
			url:      "https://tandoor.example.com",
			apiKey:   "secret",
			wantErr:  nil,
		},
		{
			// Pins the ordering: with no key there is nothing to leak, so the
			// missing key is the actionable problem and stays recoverable.
			name:     "missing key wins over the URL mismatch",
			provider: util.RecipeProviderMealie,
			url:      "https://www.themealdb.com/api/json/v1/1",
			apiKey:   "",
			wantErr:  proviantErrors.ErrRecipeAPIMissingAPIKey,
		},
		{
			// Pins the gate: the public URL is the correct one for TheMealDB.
			name:     "TheMealDB keeps the public URL",
			provider: util.RecipeProviderThemealDB,
			url:      "https://www.themealdb.com/api/json/v1/1",
			apiKey:   "",
			wantErr:  nil,
		},
		{
			// An unparseable URL can never reach a host at all, so it fails
			// closed in the provider request instead of being a leak path.
			name:     "an unparseable URL is not treated as the public host",
			provider: util.RecipeProviderMealie,
			url:      "://not a url",
			apiKey:   "secret",
			wantErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &ProviantConfiguration{
				RecipeAPI: RecipeAPIConfiguration{
					Provider: tt.provider,
					URL:      tt.url,
					APIKey:   tt.apiKey,
					Timeout:  10,
					CacheTTL: 24,
				},
			}
			err := config.ValidateRecipeAPIConfiguration()
			assertValidationError(t, err, tt.wantErr)
		})
	}
}

// TestRecipeAPIConfigurationUsesInsecureTransport covers the plaintext-HTTP
// warning. A self-hosted provider sends its API key as a bearer/token header
// and the household's inventory-derived keywords in the query string on every
// request, so an http:// instance URL transmits both in cleartext. It is
// reported rather than rejected because a LAN-only instance is legitimate.
func TestRecipeAPIConfigurationUsesInsecureTransport(t *testing.T) {
	tests := []struct {
		name       string
		config     RecipeAPIConfiguration
		selfHosted bool
		insecure   bool
	}{
		{
			name:       "http instance URL is insecure",
			config:     RecipeAPIConfiguration{Provider: util.RecipeProviderMealie, URL: "http://mealie.internal:8080"},
			selfHosted: true,
			insecure:   true,
		},
		{
			name:       "uppercase HTTP scheme is insecure",
			config:     RecipeAPIConfiguration{Provider: util.RecipeProviderTandoor, URL: "HTTP://tandoor.local"},
			selfHosted: true,
			insecure:   true,
		},
		{
			name:       "https instance URL is secure",
			config:     RecipeAPIConfiguration{Provider: util.RecipeProviderMealie, URL: "https://mealie.example.com"},
			selfHosted: true,
			insecure:   false,
		},
		{
			name:       "trailing whitespace does not hide the scheme",
			config:     RecipeAPIConfiguration{Provider: util.RecipeProviderMealie, URL: "  http://mealie.local  "},
			selfHosted: true,
			insecure:   true,
		},
		{
			name:       "unparseable URL is not reported as insecure",
			config:     RecipeAPIConfiguration{Provider: util.RecipeProviderMealie, URL: "://not a url"},
			selfHosted: true,
			insecure:   false,
		},
		{
			name:     "the public TheMealDB API is secure",
			config:   RecipeAPIConfiguration{Provider: util.RecipeProviderThemealDB, URL: util.DefaultTheMealDBRecipeAPIURL},
			insecure: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.UsesInsecureTransport(); got != tt.insecure {
				t.Errorf("UsesInsecureTransport() = %v, want %v", got, tt.insecure)
			}
			if got := tt.config.IsSelfHosted(); got != tt.selfHosted {
				t.Errorf("IsSelfHosted() = %v, want %v", got, tt.selfHosted)
			}
		})
	}
}

// The two self-hosted misconfigurations must stay distinct: the missing key is
// the actionable problem when both are wrong, and only the mismatch case means
// a configured URL that would leak the key.
func TestValidateRecipeAPIConfigurationSelfHostedFailuresAreDistinct(t *testing.T) {
	t.Run("missing key wins over the URL mismatch", func(t *testing.T) {
		config := &ProviantConfiguration{
			RecipeAPI: RecipeAPIConfiguration{
				Provider: util.RecipeProviderMealie,
				URL:      util.DefaultTheMealDBRecipeAPIURL,
				Timeout:  10,
			},
		}
		assertValidationError(t, config.ValidateRecipeAPIConfiguration(), proviantErrors.ErrRecipeAPIMissingAPIKey)
	})

	t.Run("URL mismatch is reported once a key is present", func(t *testing.T) {
		config := &ProviantConfiguration{
			RecipeAPI: RecipeAPIConfiguration{
				Provider: util.RecipeProviderMealie,
				URL:      util.DefaultTheMealDBRecipeAPIURL,
				APIKey:   "secret",
				Timeout:  10,
			},
		}
		assertValidationError(t, config.ValidateRecipeAPIConfiguration(), proviantErrors.ErrRecipeAPIProviderURLMismatch)
	})

	t.Run("a padded API key is trimmed into the field", func(t *testing.T) {
		config := &ProviantConfiguration{
			RecipeAPI: RecipeAPIConfiguration{
				Provider: util.RecipeProviderMealie,
				URL:      "https://mealie.example.com",
				APIKey:   "  token123\n",
				Timeout:  10,
			},
		}
		if err := config.ValidateRecipeAPIConfiguration(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.RecipeAPI.APIKey != "token123" {
			t.Errorf("APIKey = %q, want %q", config.RecipeAPI.APIKey, "token123")
		}
	})
}

func TestValidateOCRReceiptConfiguration(t *testing.T) {
	tests := []struct {
		name     string
		config   *ProviantConfiguration
		wantErr  error
		afterChk func(t *testing.T, config *ProviantConfiguration)
	}{
		{
			name:    "disabled feature is always valid",
			config:  &ProviantConfiguration{},
			wantErr: nil,
		},
		{
			name: "valid enabled configuration",
			config: &ProviantConfiguration{
				OCR: OCRConfiguration{Receipt: ReceiptOCRConfiguration{
					Enabled: true,
					Model:   "gpt-4o-mini",
				}},
			},
			wantErr: nil,
			afterChk: func(t *testing.T, config *ProviantConfiguration) {
				if config.OCR.Receipt.Provider != util.ReceiptOCRProviderOpenAI {
					t.Errorf("Provider = %q, want default %q", config.OCR.Receipt.Provider, util.ReceiptOCRProviderOpenAI)
				}
				if config.OCR.Receipt.Timeout != util.ReceiptScanDefaultTimeout {
					t.Errorf("Timeout = %d, want default %d", config.OCR.Receipt.Timeout, util.ReceiptScanDefaultTimeout)
				}
			},
		},
		{
			name: "non-vision provider tesseract is rejected",
			config: &ProviantConfiguration{
				OCR: OCRConfiguration{Receipt: ReceiptOCRConfiguration{
					Enabled:  true,
					Provider: "tesseract",
					Model:    "gpt-4o-mini",
				}},
			},
			wantErr: proviantErrors.ErrReceiptOCRInvalidProvider,
		},
		{
			name: "empty model is rejected",
			config: &ProviantConfiguration{
				OCR: OCRConfiguration{Receipt: ReceiptOCRConfiguration{
					Enabled: true,
				}},
			},
			wantErr: proviantErrors.ErrReceiptOCREmptyModel,
		},
		{
			name: "whitespace-only model is rejected",
			config: &ProviantConfiguration{
				OCR: OCRConfiguration{Receipt: ReceiptOCRConfiguration{
					Enabled: true,
					Model:   "   ",
				}},
			},
			wantErr: proviantErrors.ErrReceiptOCREmptyModel,
		},
		{
			name: "provider is normalized case-insensitively",
			config: &ProviantConfiguration{
				OCR: OCRConfiguration{Receipt: ReceiptOCRConfiguration{
					Enabled:  true,
					Provider: "OpenAI",
					Model:    "gpt-4o-mini",
				}},
			},
			wantErr: nil,
			afterChk: func(t *testing.T, config *ProviantConfiguration) {
				if config.OCR.Receipt.Provider != util.ReceiptOCRProviderOpenAI {
					t.Errorf("Provider = %q, want %q", config.OCR.Receipt.Provider, util.ReceiptOCRProviderOpenAI)
				}
			},
		},
		{
			name: "endpoint and API key whitespace is trimmed",
			config: &ProviantConfiguration{
				OCR: OCRConfiguration{Receipt: ReceiptOCRConfiguration{
					Enabled:  true,
					Model:    "gpt-4o-mini",
					Endpoint: "  https://llm.example.com/v1\n",
					APIKey:   "  token123  ",
				}},
			},
			wantErr: nil,
			afterChk: func(t *testing.T, config *ProviantConfiguration) {
				if config.OCR.Receipt.Endpoint != "https://llm.example.com/v1" {
					t.Errorf("Endpoint = %q, want %q", config.OCR.Receipt.Endpoint, "https://llm.example.com/v1")
				}
				if config.OCR.Receipt.APIKey != "token123" {
					t.Errorf("APIKey = %q, want %q", config.OCR.Receipt.APIKey, "token123")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.ValidateOCRReceiptConfiguration()
			assertValidationError(t, err, tt.wantErr)
			if err == nil && tt.afterChk != nil {
				tt.afterChk(t, tt.config)
			}
		})
	}
}

// TestReceiptOCRConfigurationUsesInsecureTransport covers the plaintext-HTTP
// warning for the receipt scan endpoint. A custom endpoint authenticates with
// a bearer API key and receives the uploaded receipt photo, so an http:// URL
// transmits both in cleartext. Like the recipe API guard, it is reported
// rather than rejected because a LAN-only endpoint is legitimate.
func TestReceiptOCRConfigurationUsesInsecureTransport(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		insecure bool
	}{
		{name: "http endpoint is insecure", endpoint: "http://llm.example.com/v1", insecure: true},
		{name: "https endpoint is secure", endpoint: "https://llm.example.com/v1", insecure: false},
		{name: "empty endpoint resolves to the https OpenAI default", endpoint: "", insecure: false},
		{name: "unparseable endpoint fails closed", endpoint: "://not-a-url", insecure: false},
		{name: "whitespace is trimmed before parsing", endpoint: "  http://127.0.0.1:1234  ", insecure: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ReceiptOCRConfiguration{Endpoint: tt.endpoint}
			if got := config.UsesInsecureTransport(); got != tt.insecure {
				t.Errorf("UsesInsecureTransport() = %v, want %v", got, tt.insecure)
			}
		})
	}
}
