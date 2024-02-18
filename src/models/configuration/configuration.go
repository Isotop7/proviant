// configuration defines structs and methods for expiros configuration and specific parts of it
package configuration

import (
	"errors"
	"html/template"

	"gitlab.com/Isotop7/expiro/controllers/database"
)

// DatabaseConfiguration contains all properties regarding the database connection
type DatabaseConfiguration struct {
	Engine   string
	Host     string
	Port     int
	Name     string
	User     string
	Password string
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
		return errors.New("empty API URL for OpenFoodFacts specified")
	}
	if ec.OpenFoodFacts.Timeout <= 0 {
		return errors.New("invalid timeout for OpenFoodFacts API specified")
	}
	return nil
}

// ValidateDatabaseConfiguration checks the current database configuration for common errors
func (ec ExpiroConfiguration) ValidateDatabaseConfiguration() error {
	if dbEngine := database.SupportedEnginesFromString(ec.Database.Engine); dbEngine == database.InvalidEngine {
		return errors.New("no valid database engine selected")
	}
	if ec.Database.Host == "" {
		return errors.New("no database host specified")
	}
	if ec.Database.User == "" {
		return errors.New("no database user specified")
	}
	if ec.Database.Password == "" {
		return errors.New("no database password specified")
	}
	if ec.Database.Name == "" {
		return errors.New("no database name specified")
	}
	if ec.Database.Port <= 0 {
		return errors.New("no valid database port specified")
	}
	return nil
}
