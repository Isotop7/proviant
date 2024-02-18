// configuration defines structs and methods for expiros configuration and specific parts of it
package configuration

import (
	"html/template"

	"gitlab.com/Isotop7/expiro/controllers/database"
	"gitlab.com/Isotop7/expiro/errors"
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
	TokenPassword string
	TokenLifetime int
}

// CorsConfiguration contains all properties for the CORS configuration of the expiro server
type CorsConfiguration struct {
	AllowAllOrigins bool
	AllowedOrigins  []string
}

// ServerConfiguration contains all properties regarding the expiro server
type ServerConfiguration struct {
	Port           int
	Authentication AuthenticationConfiguration
	CORS           CorsConfiguration
}

// LoggingConfiguration contains all properties regarding the log configuration for zerolog
type LoggingConfiguration struct {
	Enabled bool
	File    string
}

// SMTPConfiguration contains all properties regarding the notification handler target
type SMTPConfiguration struct {
	Host     string
	Port     int
	SSL      bool
	User     string
	Password string
}

// NotificationConfiguration contains all properties regarding the notification handler
type NotificationConfiguration struct {
	Enabled     bool
	Interval    int
	FromAddress string
	SMTP        SMTPConfiguration
}

// OpenFoodFactsConfiguration contains all properties regarding the OpenFoodFacts API controller
type OpenFoodFactsConfiguration struct {
	URL     string
	Timeout int
}

// ExpiroConfiguration is the configuration wrapper struct
type ExpiroConfiguration struct {
	Database      DatabaseConfiguration
	Server        ServerConfiguration
	Logging       LoggingConfiguration
	Notification  NotificationConfiguration
	OpenFoodFacts OpenFoodFactsConfiguration
	TemplateCache map[string]*template.Template
}

// ValidateOpenFoodFactsConfiguration validates the current configuration to connect to the OpenFoodFact API
func (ec ExpiroConfiguration) ValidateOpenFoodFactsConfiguration() error {
	if ec.OpenFoodFacts.URL == "" {
		return errors.ErrOpenFoodFactsAPIEmptyUrl
	}
	if ec.OpenFoodFacts.Timeout <= 0 {
		return errors.ErrOpenFoodFactsAPIInvalidTimeout
	}
	return nil
}

// ValidateDatabaseConfiguration checks the current database configuration for common errors
func (ec *ExpiroConfiguration) ValidateDatabaseConfiguration() error {
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
