package models

import "errors"

type DatabaseConfiguration struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
}

type ServerConfiguration struct {
	Port int
}

type SMTPConfiguration struct {
	Host     string
	Port     int
	SSL      bool
	User     string
	Password string
}

type NotificationConfiguration struct {
	Enabled     bool
	Interval    int
	FromAddress string
	ToAddress   []string
	SMTP        SMTPConfiguration
}

type OpenFoodFactsConfiguration struct {
	URL     string
	Timeout int
}

type ExpiroConfiguration struct {
	Database      DatabaseConfiguration
	Server        ServerConfiguration
	Notification  NotificationConfiguration
	OpenFoodFacts OpenFoodFactsConfiguration
}

func (ec ExpiroConfiguration) ValidDatabaseConfiguration() error {
	if ec.Database.Host == "" {
		return errors.New("no database host specified")
	} else if ec.Database.User == "" {
		return errors.New("no database user specified")
	} else if ec.Database.Password == "" {
		return errors.New("no database password specified")
	}
	return nil
}
