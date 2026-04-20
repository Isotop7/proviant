// configuration defines structs and methods for proviants configuration and specific parts of it
package configuration

import (
	"html/template"
	"strings"

	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
)

type DatabaseMariaDBConfiguration struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
}

type DatabaseSQLiteConfiguration struct {
	Filepath string
}

// DatabaseConfiguration contains all properties regarding the database connection
type DatabaseConfiguration struct {
	Engine  string
	MariaDB DatabaseMariaDBConfiguration
	SQLite  DatabaseSQLiteConfiguration
	// Additional, non parsed vars
	SelectedEngine database.SupportedEngines
}

// AuthenticationConfiguration contains all properties regarding the JSON Web Tokens
type AuthenticationConfiguration struct {
	TokenPassword            string
	TokenLifetime            int
	MaxLoginAttempts         int
	LockoutDurationMins      int
	PasswordMinLength        int
	PasswordRequireUppercase bool
	PasswordRequireDigit     bool
	PasswordRequireSpecial   bool
	PasswordCheckBreached    bool
}

// CorsConfiguration contains all properties for the CORS configuration of the proviant server
type CorsConfiguration struct {
	AllowAllOrigins bool
	AllowedOrigins  []string
}

// SecurityHeadersConfiguration contains all properties for HTTP security headers
type SecurityHeadersConfiguration struct {
	ContentSecurityPolicy string
}

// ServerConfiguration contains all properties regarding the proviant server
type ServerConfiguration struct {
	Port            int
	Authentication  AuthenticationConfiguration
	CORS            CorsConfiguration
	BaseURL         string
	SecurityHeaders SecurityHeadersConfiguration
}

// LoggingConfiguration contains all properties regarding the log configuration for zerolog
type LoggingConfiguration struct {
	Enabled bool
	File    string
}

// SMTPConfiguration contains all properties regarding the notification handler target
type SMTPConfiguration struct {
	Host        string
	Port        int
	SSL         bool
	User        string
	FromAddress string
	Password    string
}

// NtfyConfiguration contains all properties regarding the ntfy.sh notification provider
type NtfyConfiguration struct {
	URL     string
	Topic   string
	Timeout int
}

// MonthlyWasteReportConfiguration controls when the monthly waste report email is sent.
type MonthlyWasteReportConfiguration struct {
	Day  int `mapstructure:"day"`  // day-of-month (1–28)
	Hour int `mapstructure:"hour"` // hour of day, UTC (0–23)
}

// TelegramConfiguration holds per-instance Telegram settings (no global bot token).
type TelegramConfiguration struct {
	Timeout int // HTTP client timeout in seconds (default: 15)
}

// NotificationConfiguration contains all properties regarding the notification handler
type NotificationConfiguration struct {
	Enabled            bool
	Interval           int
	SMTP               SMTPConfiguration
	Ntfy               NtfyConfiguration
	MonthlyWasteReport MonthlyWasteReportConfiguration `mapstructure:"monthlyWasteReport"`
	Telegram           TelegramConfiguration           `mapstructure:"telegram"`
}

// OpenFoodFactsConfiguration contains all properties regarding the OpenFoodFacts API controller
type OpenFoodFactsConfiguration struct {
	URL          string
	Timeout      int
	CacheEnabled bool
}

// ProviantConfiguration is the configuration wrapper struct
type ProviantConfiguration struct {
	Database      DatabaseConfiguration
	Server        ServerConfiguration
	Logging       LoggingConfiguration
	Notification  NotificationConfiguration
	OpenFoodFacts OpenFoodFactsConfiguration
	TemplateCache map[string]*template.Template
}

// ValidateOpenFoodFactsConfiguration validates the current configuration to connect to the OpenFoodFact API
func (ec *ProviantConfiguration) ValidateOpenFoodFactsConfiguration() error {
	if ec.OpenFoodFacts.URL == "" {
		return errors.ErrOpenFoodFactsAPIEmptyURL
	}
	if ec.OpenFoodFacts.Timeout <= 0 {
		return errors.ErrOpenFoodFactsAPIInvalidTimeout
	}
	return nil
}

// ValidateNotificationConfiguration validates the notification configuration
func (ec *ProviantConfiguration) ValidateNotificationConfiguration() error {
	if !ec.Notification.Enabled {
		return nil // Notifications disabled, no validation needed
	}

	if ec.Notification.Interval <= 0 {
		return errors.ErrNotificationInvalidInterval
	}

	// Validate SMTP configuration if SMTP host is provided
	if ec.Notification.SMTP.Host != "" {
		if ec.Notification.SMTP.Port <= 0 {
			return errors.ErrNotificationInvalidSMTPPort
		}
		if ec.Notification.SMTP.FromAddress == "" {
			return errors.ErrNotificationEmptyFromAddress
		}
	}

	// Validate monthly waste report schedule
	day := ec.Notification.MonthlyWasteReport.Day
	if day < 1 || day > 28 {
		return errors.ErrNotificationInvalidWasteReportDay
	}
	hour := ec.Notification.MonthlyWasteReport.Hour
	if hour < 0 || hour > 23 {
		return errors.ErrNotificationInvalidWasteReportHour
	}

	// Validate ntfy configuration if ntfy URL is provided
	if ec.Notification.Ntfy.URL != "" {
		if ec.Notification.Ntfy.Topic == "" {
			return errors.ErrNotificationEmptyNtfyTopic
		}
		// Basic URL validation
		if !strings.HasPrefix(ec.Notification.Ntfy.URL, "http://") &&
			!strings.HasPrefix(ec.Notification.Ntfy.URL, "https://") {
			return errors.ErrNotificationInvalidNtfyURL
		}
	}

	return nil
}

// ValidateDatabaseConfiguration checks the current database configuration for common errors
func (ec *ProviantConfiguration) ValidateDatabaseConfiguration() error {
	ec.Database.SelectedEngine = database.SupportedEnginesFromString(ec.Database.Engine)
	if ec.Database.SelectedEngine == database.InvalidEngine {
		return errors.ErrDatabaseInvalidEngine
	}

	switch ec.Database.SelectedEngine {
	case database.MariaDB:
		if ec.Database.MariaDB.Host == "" {
			return errors.ErrDatabaseMariaDBEmptyHost
		}
		if ec.Database.MariaDB.User == "" {
			return errors.ErrDatabaseMariaDBEmptyUser
		}
		if ec.Database.MariaDB.Password == "" {
			return errors.ErrDatabaseMariaDBEmptyPassword
		}
		if ec.Database.MariaDB.Name == "" {
			return errors.ErrDatabaseMariaDBEmptyName
		}
		if ec.Database.MariaDB.Port <= 0 {
			return errors.ErrDatabaseMariaDBInvalidPort
		}
	case database.SQLite:
		if ec.Database.SQLite.Filepath == "" {
			return errors.ErrDatabaseSQLiteInvalidPath
		}
	}
	return nil
}
