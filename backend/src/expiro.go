// expiro is a simple and intuitive application to track your bought products and their expiration date to prevent waste of food
package main

import (
	"expiro/controllers"
	"expiro/models/authentication"
	"expiro/models/configuration"
	"expiro/models/database"
	"expiro/router"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var db *gorm.DB

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
	var logger zerolog.Logger
	if configuration.Logging.Enabled {
		fileLogger, _ := os.OpenFile(
			configuration.Logging.File,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY,
			0664,
		)
		multi := zerolog.MultiLevelWriter(fileLogger, zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime})
		logger = zerolog.New(multi).Level(zerolog.DebugLevel).With().Timestamp().Caller().Logger()
	} else {
		logger = zerolog.New(
			zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime},
		).Level(zerolog.DebugLevel).With().Timestamp().Caller().Logger()
	}

	// Validate database parametes
	dbValidErr := configuration.ValidDatabaseConfiguration()
	if dbValidErr != nil {
		panic(dbValidErr)
	}

	// Generate database URI
	databaseURI := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		configuration.Database.User,
		configuration.Database.Password,
		configuration.Database.Host,
		configuration.Database.Port,
		configuration.Database.Name)

	var dbErr error
	db, dbErr = gorm.Open(mysql.Open(databaseURI), &gorm.Config{})

	// Check if database can be accessed
	if dbErr != nil {
		panic(dbErr.Error())
	}

	// Run migrations for database
	db.AutoMigrate(
		&database.Product{},
		&authentication.User{},
	)

	// Check API controller config and generate instance
	if configuration.OpenFoodFacts.Timeout <= 0 {
		configuration.OpenFoodFacts.Timeout = 5
	}
	if configuration.OpenFoodFacts.URL == "" {
		panic("URL for OpenFoodFactsAPI not set")
	}
	cntrl := controllers.OpenFoodFactsAPIController{
		Configuration: configuration.OpenFoodFacts,
	}

	// Setup NotificationController
	if !configuration.Notification.Enabled {
		logger.Info().Msg("Notifications are disabled")
	} else {
		notificationController := controllers.NotificationController{
			Logger:        &logger,
			Configuration: configuration.Notification,
			DB:            db,
		}
		notificationController.Dispatch()
	}

	// Call function to setup router and pass database interface
	r := router.SetupRouter(&logger, &configuration, db, cntrl)

	// Get server port or instead set default value
	serverPort := configuration.Server.Port
	if serverPort <= 0 {
		serverPort = 5050
	}

	// Start server
	r.Run(fmt.Sprintf(":%d", serverPort))
}
