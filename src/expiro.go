// expiro is a simple and intuitive application to track your bought products and their expiration date to prevent waste of food
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gitlab.com/Isotop7/expiro/controllers"
	"gitlab.com/Isotop7/expiro/logging"
	"gitlab.com/Isotop7/expiro/models/authentication"
	"gitlab.com/Isotop7/expiro/models/configuration"
	"gitlab.com/Isotop7/expiro/models/database"
	"gitlab.com/Isotop7/expiro/router"
	"gitlab.com/Isotop7/expiro/templates"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// main is the main function used on start of expiro
func main() {
	// Setup config path
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	// Read environment
	viper.SetEnvPrefix("EXPIRO")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Read config
	configErr := viper.ReadInConfig()
	if configErr != nil {
		panic(configErr.Error())
	}

	// Unmarshal yaml to configuration struct
	configuration := configuration.ExpiroConfiguration{}
	err := viper.Unmarshal(&configuration)
	if err != nil {
		panic(err)
	}

	// Setup logging
	var cLogger zerolog.Logger
	// Check if logging to file was enabled
	if configuration.Logging.Enabled {
		// Create multi writer for file and terminal logging
		logFile, logFileOpenErr := os.OpenFile(
			configuration.Logging.File,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0664,
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
	dbValidErr := configuration.ValidateDatabaseConfiguration()
	if dbValidErr != nil {
		panic(dbValidErr)
	} else {
		cLogger.Info().Msg("Database configuration is valid")
	}

	// Generate database URI
	databaseURI := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		configuration.Database.User,
		configuration.Database.Password,
		configuration.Database.Host,
		configuration.Database.Port,
		configuration.Database.Name)

	// Generate gorm config
	var dbErr error
	var dbHandle *gorm.DB
	gormConfig := gorm.Config{}
	// Create Zerolog adapter and pass it to gorm config
	gormConfig.Logger = logging.ZerologAdapter{LoggingSink: &cLogger}
	// Open database handle
	dbHandle, dbErr = gorm.Open(mysql.Open(databaseURI), &gormConfig)

	// Check if external database can be accessed
	if dbErr != nil {
		cLogger.Warn().Msgf("Database '%s' with on server '%s' could not be reached", configuration.Database.Name, configuration.Database.Host)
		panic(dbErr)
	}

	// Run migrations for database and check for errors
	migrationError := dbHandle.AutoMigrate(
		&authentication.User{},
		&database.Product{},
	)
	if migrationError != nil {
		panic(migrationError)
	}

	// Check API controller config and create instance
	validateErr := configuration.ValidateOpenFoodFactsConfiguration()
	if validateErr != nil {
		panic("URL for OpenFoodFactsAPI not set")
	} else {
		cLogger.Info().Msg("OpenFoodFacts configuration is valid")
	}
	offacntrl := controllers.OpenFoodFactsAPIController{
		Configuration: configuration.OpenFoodFacts,
		Logger:        &cLogger,
	}

	// Setup NotificationController if notifications are enabled
	if !configuration.Notification.Enabled {
		cLogger.Info().Msg("Notifications are disabled")
	} else {
		notificationController := controllers.NotificationController{
			Logger:             &cLogger,
			Configuration:      configuration.Notification,
			DatabaseController: &controllers.DatabaseController{DBHandle: dbHandle},
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
	configuration.TemplateCache = templateCache

	// Call function to setup router and pass references
	expiroEngine := router.SetupRouter(&cLogger, &configuration, dbHandle, offacntrl)

	// Get server port or instead set default value
	serverPort := configuration.Server.Port
	if serverPort <= 0 {
		serverPort = 5114
	}

	// Start server
	runErr := expiroEngine.Run(fmt.Sprintf(":%d", serverPort))
	if runErr != nil {
		panic(runErr)
	}
}
