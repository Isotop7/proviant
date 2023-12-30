// expiro is a simple and intuitive application to track your bought products and their expiration date to prevent waste of food
package main

import (
	"fmt"
	"os"
	"time"

	"gitlab.com/Isotop7/expiro/controllers"
	"gitlab.com/Isotop7/expiro/logging"
	"gitlab.com/Isotop7/expiro/models/authentication"
	"gitlab.com/Isotop7/expiro/models/configuration"
	"gitlab.com/Isotop7/expiro/models/database"
	"gitlab.com/Isotop7/expiro/router"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	// Setup config path
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	// Read environment
	viper.AutomaticEnv()

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
	if configuration.Logging.Enabled {
		fileLogger, _ := os.OpenFile(
			configuration.Logging.File,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0664,
		)
		multi := zerolog.MultiLevelWriter(fileLogger, zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime})
		cLogger = zerolog.New(multi).Level(zerolog.DebugLevel).With().Timestamp().Caller().Logger()
	} else {
		cLogger = zerolog.New(
			zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime},
		).Level(zerolog.DebugLevel).With().Timestamp().Caller().Logger()
	}
	cLogger.Info().Msg("Logging initialized")

	// Validate database parametes
	dbValidErr := configuration.ValidDatabaseConfiguration()
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

	var dbErr error
	var db *gorm.DB
	gormConfig := gorm.Config{}
	gormConfig.Logger = logging.ZerologAdapter{LoggingSink: &cLogger}
	db, dbErr = gorm.Open(mysql.Open(databaseURI), &gormConfig)

	// Check if external database can be accessed
	if dbErr != nil {
		cLogger.Warn().Msgf("Database '%s' with on server '%s' could not be reached, falling back to SQLite", configuration.Database.Name, configuration.Database.Host)
		// If not, try to open embedded database
		db, dbErr = gorm.Open(sqlite.Open("expiro.db"), &gormConfig)
		if err != nil {
			panic(dbErr)
		}
	}

	// Run migrations for database
	migrationError := db.AutoMigrate(
		&database.Product{},
		&authentication.User{},
	)
	if migrationError != nil {
		panic(migrationError)
	}

	// Check API controller config and generate instance
	if configuration.OpenFoodFacts.Timeout <= 0 {
		configuration.OpenFoodFacts.Timeout = 5
	}
	if configuration.OpenFoodFacts.URL == "" {
		panic("URL for OpenFoodFactsAPI not set")
	}
	offacntrl := controllers.OpenFoodFactsAPIController{
		Configuration: configuration.OpenFoodFacts,
	}

	// Setup NotificationController
	if !configuration.Notification.Enabled {
		cLogger.Info().Msg("Notifications are disabled")
	} else {
		notificationController := controllers.NotificationController{
			Logger:        &cLogger,
			Configuration: configuration.Notification,
			DB:            db,
		}
		notificationController.Dispatch()
	}

	// Call function to setup router and pass database interface
	r := router.SetupRouter(&cLogger, &configuration, db, offacntrl)

	// Get server port or instead set default value
	serverPort := configuration.Server.Port
	if serverPort <= 0 {
		serverPort = 5050
	}

	// Start server
	runErr := r.Run(fmt.Sprintf(":%d", serverPort))
	if runErr != nil {
		panic(runErr)
	}
}
