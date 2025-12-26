// proviant is a simple and intuitive application to track your bought products and their expiration date to prevent waste of food
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/controllers"
	dbController "codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/logging"
	"codeberg.org/isotop7/proviant/migrations"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/router"
	"codeberg.org/isotop7/proviant/templates"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// SetupDatabase initializes the database connection and returns a gorm.DB instance.
func setupDatabase(logger *zerolog.Logger, databaseConfiguration *configuration.DatabaseConfiguration) (*gorm.DB, error) {
	// Generate gorm config
	var dbErr error
	var dbHandle *gorm.DB
	gormConfig := gorm.Config{}
	// Create Zerolog adapter and pass it to gorm config
	gormConfig.Logger = logging.ZerologAdapter{LoggingSink: logger}

	// Get database handle based on selected engine
	switch databaseConfiguration.SelectedEngine {
	case dbController.MariaDB:
		// Generate database URI
		databaseURI := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			databaseConfiguration.MariaDB.User,
			databaseConfiguration.MariaDB.Password,
			databaseConfiguration.MariaDB.Host,
			databaseConfiguration.MariaDB.Port,
			databaseConfiguration.MariaDB.Name)
		// Open database handle
		dbHandle, dbErr = gorm.Open(mysql.Open(databaseURI), &gormConfig)

		// Check if database can be accessed
		if dbErr != nil {
			logger.Warn().Msgf("Database '%s' on server '%s' could not be reached", databaseConfiguration.MariaDB.Name, databaseConfiguration.MariaDB.Host)
			return nil, dbErr
		}
	case dbController.SQLite:
		// Create file and handle
		dbHandle, dbErr = gorm.Open(sqlite.Open(databaseConfiguration.SQLite.Filepath), &gormConfig)

		// Check if database can be accessed
		if dbErr != nil {
			logger.Warn().Msgf("Database on path '%s' could not be opened", databaseConfiguration.SQLite.Filepath)
			return nil, dbErr
		}
	case dbController.InvalidEngine:
		return nil, errors.ErrDatabaseInvalidEngine
	}

	return dbHandle, nil
}

// setupNotificationController initializes the notification controller and starts the notification handler goroutine.
func setupNotificationController(logger *zerolog.Logger, proviantConfiguration *configuration.ProviantConfiguration, dbHandle *gorm.DB) {
	// Check if notifications are enabled
	if !proviantConfiguration.Notification.Enabled {
		logger.Info().Msg("Notifications are disabled")
	} else {
		// Generate notification controller
		notificationController := controllers.NotificationController{
			Logger:             logger,
			Configuration:      proviantConfiguration.Notification,
			DatabaseController: &dbController.DatabaseController{DBHandle: dbHandle},
		}
		// Dispatch notification handler goroutine
		notificationController.Dispatch()
	}
}

// setupConfig initializes the configuration and returns a ProviantConfiguration instance.
func setupConfig() *configuration.ProviantConfiguration {
	// Set configuration file path
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	// Set env prefix
	viper.SetEnvPrefix("PROVIANT")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read configuration file
	if err := viper.ReadInConfig(); err != nil {
		panic(err.Error())
	}

	// Unmarshal yaml to configuration struct
	var config configuration.ProviantConfiguration
	if err := viper.Unmarshal(&config); err != nil {
		panic(err)
	}
	return &config
}

// setupLogging initializes the logging and returns a zerolog.Logger instance.
func setupLogging(config *configuration.ProviantConfiguration) *zerolog.Logger {
	if config.Logging.Enabled {
		// Create multi writer for file and terminal logger
		logFile, logFileOpenErr := os.OpenFile(
			config.Logging.File,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0o664,
		)
		// Check if logfile could be opened
		if logFileOpenErr != nil {
			panic(logFileOpenErr.Error())
		}
		// Add logfile to logging writers
		multi := zerolog.MultiLevelWriter(logFile, zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime})
		logFileLogger := zerolog.New(multi).Level(zerolog.DebugLevel).With().Timestamp().Caller().Logger()
		return &logFileLogger
	} else {
		// Create terminal logger
		terminalLogger := zerolog.New(
			zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime},
		).Level(zerolog.DebugLevel).With().Timestamp().Caller().Logger()
		return &terminalLogger
	}
}

// validateAPIs validates the API configurations.
func validateAPIs(config *configuration.ProviantConfiguration) {
	if err := config.ValidateOpenFoodFactsConfiguration(); err != nil {
		panic("URL for OpenFoodFactsAPI not set")
	}
}

// startProviantServer starts the Proviant server.
func startProviantServer(logger *zerolog.Logger, proviantConfiguration *configuration.ProviantConfiguration, dbHandle *gorm.DB, offacntrl *controllers.OpenFoodFactsAPIController) {
	// Call function to setup router and pass references
	proviantEngine := router.SetupRouter(logger, proviantConfiguration, dbHandle, offacntrl)

	// Get server port or instead set default value
	serverPort := proviantConfiguration.Server.Port
	if serverPort <= 0 {
		serverPort = 5114
	}

	// Start server
	runErr := proviantEngine.Run(fmt.Sprintf(":%d", serverPort))
	if runErr != nil {
		panic(runErr)
	}
}

// main is the main function used on start of proviant
func main() {
	// Setup config
	proviantConfiguration := setupConfig()
	// Setup logging
	logger := setupLogging(proviantConfiguration)
	logger.Info().Msg("Logging initialized")

	// Validate database parameters
	dbValidErr := proviantConfiguration.ValidateDatabaseConfiguration()
	if dbValidErr != nil {
		panic(dbValidErr)
	} else {
		logger.Info().Msg("Database configuration is valid")
	}

	// Setup database connection handle
	dbHandle, setupErr := setupDatabase(logger, &proviantConfiguration.Database)
	if setupErr != nil {
		panic(setupErr)
	} else if dbHandle == nil {
		panic("Error getting database handle")
	}

	// Run migrations for database and check for errors
	migrationError := dbHandle.AutoMigrate(
		&dbModel.Household{},
		&authentication.User{},
		&dbModel.Product{},
	)
	if migrationError != nil {
		panic(migrationError)
	}

	// Run migrations for breaking changes
	breakingMigrationsError := migrations.RunBreakingDatabaseMigrations(logger, dbHandle)
	if breakingMigrationsError != nil {
		panic(breakingMigrationsError)
	}

	// Check API controller config and create instance
	validateAPIs(proviantConfiguration)
	offacntrl := &controllers.OpenFoodFactsAPIController{
		Configuration: proviantConfiguration.OpenFoodFacts,
		Logger:        logger,
	}

	// Setup NotificationController if notifications are enabled
	setupNotificationController(logger, proviantConfiguration, dbHandle)

	// Setup template cache
	templateCache, err := templates.NewTemplateCache()
	if err != nil {
		logger.Error().Msg(err.Error())
		panic(err)
	}
	proviantConfiguration.TemplateCache = templateCache

	startProviantServer(logger, proviantConfiguration, dbHandle, offacntrl)
}
