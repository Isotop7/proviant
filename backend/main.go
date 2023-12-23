package main

import (
	"expiro/backend/controllers"
	"expiro/backend/models"
	"expiro/backend/router"
	"fmt"

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

	configuration := models.ExpiroConfiguration{}
	err := viper.Unmarshal(&configuration)
	if err != nil {
		panic(err)
	}

	// Validate database parametes
	dbValidErr := configuration.ValidDatabaseConfiguration()
	if dbValidErr != nil {
		panic(dbValidErr)
	}

	// Set default values if correctable invalid values were specified
	if configuration.Database.Name == "" {
		configuration.Database.Name = "expiro"
	}
	if configuration.Database.Port <= 0 {
		configuration.Database.Port = 3306
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
	db.AutoMigrate(&models.Product{})

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
		fmt.Println("Notifications are disabled")
	} else {
		notificationController := controllers.NotificationController{
			Configuration: configuration.Notification,
			DB:            db,
		}
		notificationController.Dispatch()
	}

	// Call function to setup router and pass database interface
	r := router.SetupRouter(db, cntrl)

	// Get server port or instead set default value
	serverPort := configuration.Server.Port
	if serverPort <= 0 {
		serverPort = 5050
	}

	// Start server
	r.Run(fmt.Sprintf(":%d", serverPort))
}
