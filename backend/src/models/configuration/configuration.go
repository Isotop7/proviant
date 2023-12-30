// configuration defines structs and methods for expiros configuration and specific parts of it
package configuration

import "errors"

// DatabaseConfiguration contains all properties regarding the database connection
type DatabaseConfiguration struct {
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

// ServerConfiguration contains all properties regarding the expiro server
type ServerConfiguration struct {
	Port           int
	Authentication AuthenticationConfiguration
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
	CCAddresses []string
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
}

// ValidateDatabaseConfiguration checks the current database configuration for common errors
func (ec ExpiroConfiguration) ValidateDatabaseConfiguration() error {
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
