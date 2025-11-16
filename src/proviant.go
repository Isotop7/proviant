// proviant is a simple and intuitive application to track your bought products and their expiration date to prevent waste of food
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gitlab.com/Isotop7/proviant/controllers"
	dbController "gitlab.com/Isotop7/proviant/controllers/database"
	"gitlab.com/Isotop7/proviant/errors"
	"gitlab.com/Isotop7/proviant/logging"
	"gitlab.com/Isotop7/proviant/migrations"
	"gitlab.com/Isotop7/proviant/models/authentication"
	"gitlab.com/Isotop7/proviant/models/configuration"
	dbModel "gitlab.com/Isotop7/proviant/models/database"
	"gitlab.com/Isotop7/proviant/router"
	"gitlab.com/Isotop7/proviant/templates"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// main is the main function used on start of proviant
func main() {
	// Setup config path
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	// Read environment
	viper.SetEnvPrefix("PROVIANT")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config
	configErr := viper.ReadInConfig()
	if configErr != nil {
		panic(configErr.Error())
	}

	// Unmarshal yaml to configuration struct
	proviantConfiguration := configuration.ProviantConfiguration{}
	err := viper.Unmarshal(&proviantConfiguration)
	if err != nil {
		panic(err)
	}

	// Setup logging
	var cLogger zerolog.Logger
	// Check if logging to file was enabled
	if proviantConfiguration.Logging.Enabled {
		// Create multi writer for file and terminal logging
		logFile, logFileOpenErr := os.OpenFile(
			proviantConfiguration.Logging.File,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0o664,
		)
		// Check of logfile could be opened
		if logFileOpenErr != nil {
			panic(logFileOpenErr.Error())
		}
		// Add logfile to logging writers
		multi := zerolog.MultiLevelWriter(logFile, zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime})
		cLogger = zerolog.New(multi).Level(zerolog.DebugLevel).With().Timestamp().Caller().Logger()
	} else {
		// Create writer to terminal
		cLogger = zerolog.New(
			zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime},
		).Level(zerolog.DebugLevel).With().Timestamp().Caller().Logger()
	}
	cLogger.Info().Msg("Logging initialized")

	// Validate database parameters
	dbValidErr := proviantConfiguration.ValidateDatabaseConfiguration()
	if dbValidErr != nil {
		panic(dbValidErr)
	} else {
		cLogger.Info().Msg("Database configuration is valid")
	}

	// Setup database connection handle
	dbHandle, setupErr := SetupDatabase(cLogger, proviantConfiguration.Database)
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
	breakingMigrationsError := migrations.RunBreakingDatabaseMigrations(cLogger, dbHandle)
	if breakingMigrationsError != nil {
		panic(breakingMigrationsError)
	}

	// Check API controller config and create instance
	validateErr := proviantConfiguration.ValidateOpenFoodFactsConfiguration()
	if validateErr != nil {
		panic("URL for OpenFoodFactsAPI not set")
	} else {
		cLogger.Info().Msg("OpenFoodFacts configuration is valid")
	}
	offacntrl := controllers.OpenFoodFactsAPIController{
		Configuration: proviantConfiguration.OpenFoodFacts,
		Logger:        &cLogger,
	}

	// Setup NotificationController if notifications are enabled
	if !proviantConfiguration.Notification.Enabled {
		cLogger.Info().Msg("Notifications are disabled")
	} else {
		notificationController := controllers.NotificationController{
			Logger:             &cLogger,
			Configuration:      proviantConfiguration.Notification,
			DatabaseController: &dbController.DatabaseController{DBHandle: dbHandle},
		}
		// Dispatch notification handler goroutine
		notificationController.Dispatch()
	}

	// Setup template cache
	templateCache, err := templates.NewTemplateCache()
	if err != nil {
		cLogger.Error().Msg(err.Error())
		panic(err)
	}
	proviantConfiguration.TemplateCache = templateCache

	// Call function to setup router and pass references
	proviantEngine := router.SetupRouter(&cLogger, &proviantConfiguration, dbHandle, offacntrl)

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

func SetupDatabase(logger zerolog.Logger, configuration configuration.DatabaseConfiguration) (*gorm.DB, error) {
	// Generate gorm config
	var dbErr error
	var dbHandle *gorm.DB
	gormConfig := gorm.Config{}
	// Create Zerolog adapter and pass it to gorm config
	gormConfig.Logger = logging.ZerologAdapter{LoggingSink: &logger}

	switch configuration.SelectedEngine {
	case dbController.MariaDB:
		// Generate database URI
		databaseURI := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			configuration.MariaDB.User,
			configuration.MariaDB.Password,
			configuration.MariaDB.Host,
			configuration.MariaDB.Port,
			configuration.MariaDB.Name)
		// Open database handle
		dbHandle, dbErr = gorm.Open(mysql.Open(databaseURI), &gormConfig)

		// Check if database can be accessed
		if dbErr != nil {
			logger.Warn().Msgf("Database '%s' on server '%s' could not be reached", configuration.MariaDB.Name, configuration.MariaDB.Host)
			return nil, dbErr
		}
	case dbController.SQLite:
		// Create file and handle
		dbHandle, dbErr = gorm.Open(sqlite.Open(configuration.SQLite.Filepath), &gormConfig)

		// Check if database can be accessed
		if dbErr != nil {
			logger.Warn().Msgf("Database on path '%s' could not be opened", configuration.SQLite.Filepath)
			return nil, dbErr
		}
	case dbController.InvalidEngine:
		return nil, errors.ErrDatabaseInvalidEngine
	}

	return dbHandle, nil
}
