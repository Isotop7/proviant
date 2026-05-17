// proviant is a simple and intuitive application to track your bought products and their expiration date to prevent waste of food
package main

import (
	"fmt"
	stdlog "log"
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
	var err error
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
		dbHandle, err = gorm.Open(mysql.Open(databaseURI), &gormConfig)

		// Check if database can be accessed
		if err != nil {
			logger.Warn().Msgf("Database '%s' on server '%s' could not be reached", databaseConfiguration.MariaDB.Name, databaseConfiguration.MariaDB.Host)
			return nil, err
		}
	case dbController.SQLite:
		// Create file and handle
		dbHandle, err = gorm.Open(sqlite.Open(databaseConfiguration.SQLite.Filepath), &gormConfig)

		// Check if database can be accessed
		if err != nil {
			logger.Warn().Msgf("Database on path '%s' could not be opened", databaseConfiguration.SQLite.Filepath)
			return nil, err
		}
	case dbController.InvalidEngine:
		return nil, errors.ErrDatabaseInvalidEngine
	}

	return dbHandle, nil
}

// setupNotificationController initializes the notification controller and starts the notification handler goroutine.
func setupNotificationController(logger *zerolog.Logger, proviantConfiguration *configuration.ProviantConfiguration, dbHandle *gorm.DB) *controllers.NotificationController {
	notificationRepo := dbController.NewNotificationRepositoryWithLogger(dbHandle, logger)
	productRepo := dbController.NewProductRepository(dbHandle)
	notificationController := controllers.NewNotificationController(
		logger,
		&proviantConfiguration.Notification,
		notificationRepo,
		productRepo,
	)
	notificationController.StreakRepo = dbController.NewStreakRepository(dbHandle)
	// Dispatch notification handler goroutine
	notificationController.Dispatch()
	// Dispatch invitation email retry goroutine (uses same Interval config)
	notificationController.DispatchInvitations(proviantConfiguration.Server.BaseURL)
	// Dispatch monthly waste report goroutine
	notificationController.DispatchMonthlyWasteReports()
	// Dispatch daily streak update goroutine
	notificationController.DispatchStreakUpdates()
	// Start per-user Telegram long-polling goroutines for all users with a bot token
	notificationController.StartAllUserTelegramPollers()
	// Start per-household Mail Digest scheduler
	notificationController.StartMailDigestScheduler(proviantConfiguration.Server.BaseURL)
	return notificationController
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

	// Set defaults for monthly waste report schedule
	viper.SetDefault("notification.monthlyWasteReport.day", 1)
	viper.SetDefault("notification.monthlyWasteReport.hour", 8)

	// Set defaults for digest scheduler
	viper.SetDefault("notification.digest.enabled", true)
	viper.SetDefault("notification.digest.defaultTime", "08:00")

	// Set defaults for Telegram poller pool
	viper.SetDefault("notification.telegram.pollerWorkers", 10)

	// Set defaults for recipe API
	viper.SetDefault("recipe_api.provider", "themealdb")
	viper.SetDefault("recipe_api.url", "https://www.themealdb.com/api/json/v1/1")
	viper.SetDefault("recipe_api.timeout", 10)
	viper.SetDefault("recipe_api.cache_enabled", true)
	viper.SetDefault("recipe_api.cache_ttl", 24)

	// Set default password policy
	viper.SetDefault("server.authentication.passwordMinLength", 12)
	viper.SetDefault("server.authentication.passwordRequireUppercase", true)
	viper.SetDefault("server.authentication.passwordRequireDigit", true)
	viper.SetDefault("server.authentication.passwordRequireSpecial", false)
	viper.SetDefault("server.authentication.passwordCheckBreached", true)
	viper.SetDefault("server.maxUploadSizeMB", 5)
	viper.SetDefault("server.rateLimit.login_per_minute", 5)
	viper.SetDefault("server.rateLimit.signup_per_minute", 3)
	viper.SetDefault("server.rateLimit.export_per_minute", 1)
	viper.SetDefault("server.demoMode", false)

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
	if err := config.ValidateRecipeAPIConfiguration(); err != nil {
		panic("Invalid recipe API configuration: " + err.Error())
	}
}

// startProviantServer starts the Proviant server.
func startProviantServer(logger *zerolog.Logger, proviantConfiguration *configuration.ProviantConfiguration, dbHandle *gorm.DB, offacntrl *controllers.OpenFoodFactsAPIController, notificationController *controllers.NotificationController, ocrController *controllers.OCRControllerImpl) {
	// Call function to setup router and pass references
	proviantEngine := router.SetupRouter(logger, proviantConfiguration, dbHandle, offacntrl, notificationController, ocrController)

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
	logger.Info().Msgf("Configuration loaded from: %s", viper.ConfigFileUsed())
	stdlog.SetOutput(logger)
	stdlog.SetFlags(0)

	// Validate database parameters
	dbValidErr := proviantConfiguration.ValidateDatabaseConfiguration()
	if dbValidErr != nil {
		panic(dbValidErr)
	} else {
		logger.Info().Msg("Database configuration is valid")
	}

	// Validate server configuration
	if err := proviantConfiguration.ValidateServerConfiguration(); err != nil {
		logger.Error().Msg(err.Error())
		panic(err)
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
		&dbModel.StorageLocation{},
		&authentication.User{},
		&authentication.RevokedToken{},
		&authentication.PersonalAccessToken{},
		&authentication.CalendarToken{},
		&dbModel.Product{},
		&dbModel.HouseholdApplication{},
		&dbModel.HouseholdInvitation{},
		&dbModel.OnboardingState{},
		&dbModel.OpenFoodFactsCache{},
		&dbModel.RecipeCache{},
		&dbModel.EmailVerification{},
		&dbModel.Webhook{},
		&dbModel.WebhookDeliveryLog{},
		&dbModel.WasteStreak{},
		&dbModel.ProductCategoryPrice{},
		&dbModel.SavingsRecord{},
		&dbModel.ExpiryScan{},
		&dbModel.AuditLog{},
		&dbModel.ActivityLog{},
		&dbModel.WebPushConfig{},
		&dbModel.MailDigestUnsubscribeToken{},
		&dbModel.ShoppingListItem{},
	)
	if migrationError != nil {
		panic(migrationError)
	}

	// Run migrations for breaking changes
	breakingMigrationsError := migrations.RunBreakingDatabaseMigrations(logger, dbHandle)
	if breakingMigrationsError != nil {
		panic(breakingMigrationsError)
	}

	// Backfill default amount for existing products
	if amountMigrationError := migrations.SetDefaultProductAmounts(logger, dbHandle); amountMigrationError != nil {
		panic(amountMigrationError)
	}

	// Seed ProductCategoryPrice reference data for savings calculator
	if seedErr := migrations.SeedProductCategoryPrices(logger, dbHandle); seedErr != nil {
		panic(seedErr)
	}

	// Initialize webhook service
	controllers.InitWebhookService(dbHandle, logger)

	// Check API controller config and create instance
	validateAPIs(proviantConfiguration)
	offacntrl := &controllers.OpenFoodFactsAPIController{
		Configuration: proviantConfiguration.OpenFoodFacts,
		Logger:        logger,
	}

	// Validate notification configuration
	if proviantConfiguration.Notification.Enabled {
		if err := proviantConfiguration.ValidateNotificationConfiguration(); err != nil {
			logger.Error().Msg(err.Error())
			panic(err)
		}
	}

	// Setup template cache
	templates.SetExpiryThresholds(proviantConfiguration.Expiry.CriticalThresholdDays, proviantConfiguration.Expiry.SoonThresholdDays)
	templateCache, err := templates.NewTemplateCache()
	if err != nil {
		logger.Error().Msg(err.Error())
		panic(err)
	}
	proviantConfiguration.TemplateCache = templateCache

	notificationController := setupNotificationController(logger, proviantConfiguration, dbHandle)

	// Initialize OCR controller
	ocrController := controllers.NewOCRController(logger, &proviantConfiguration.OCR)

	// Start background cleanup of expired revoked tokens
	go startRevokedTokenCleanup(logger, dbHandle)

	// Start background cleanup of expired recipe caches
	go startRecipeCacheCleanup(logger, dbHandle)

	// Backfill storage hints and images for existing cache entries
	if proviantConfiguration.OpenFoodFacts.CacheEnabled {
		go backfillOpenFoodFactsCache(logger, dbHandle, offacntrl)
	}

	startProviantServer(logger, proviantConfiguration, dbHandle, offacntrl, notificationController, ocrController)
}

// startRevokedTokenCleanup runs a goroutine that periodically cleans up expired revoked tokens
func startRevokedTokenCleanup(logger *zerolog.Logger, dbHandle *gorm.DB) {
	ticker := time.NewTicker(1 * time.Hour) // Clean up every hour
	defer ticker.Stop()

	for range ticker.C {
		cleanupExpiredRevokedTokens(logger, dbHandle)
	}
}

// cleanupExpiredRevokedTokens deletes revoked tokens that have expired
func cleanupExpiredRevokedTokens(logger *zerolog.Logger, dbHandle *gorm.DB) {
	result := dbHandle.Where("expires_at < ?", time.Now()).Delete(&authentication.RevokedToken{})
	if result.Error != nil {
		logger.Warn().Msgf("Failed to cleanup expired revoked tokens: %s", result.Error.Error())
		return
	}
	if result.RowsAffected > 0 {
		logger.Info().Msgf("Cleaned up %d expired revoked tokens", result.RowsAffected)
	}
}

// startRecipeCacheCleanup runs a goroutine that periodically cleans up expired recipe caches
func startRecipeCacheCleanup(logger *zerolog.Logger, dbHandle *gorm.DB) {
	ticker := time.NewTicker(24 * time.Hour) // Clean up once per day
	defer ticker.Stop()

	for range ticker.C {
		cleanupExpiredRecipeCaches(logger, dbHandle)
	}
}

// cleanupExpiredRecipeCaches deletes recipe cache entries that have expired
func cleanupExpiredRecipeCaches(logger *zerolog.Logger, dbHandle *gorm.DB) {
	result := dbHandle.Where("expires_at < ?", time.Now()).Delete(&dbModel.RecipeCache{})
	if result.Error != nil {
		logger.Warn().Msgf("Failed to cleanup expired recipe caches: %s", result.Error.Error())
		return
	}
	if result.RowsAffected > 0 {
		logger.Info().Msgf("Cleaned up %d expired recipe caches", result.RowsAffected)
	}
}

// backfillOpenFoodFactsCache fetches missing storage hints and downloads remote product images
// for cache entries created before these features were added. Runs once at startup.
func backfillOpenFoodFactsCache(logger *zerolog.Logger, dbHandle *gorm.DB, offacntrl *controllers.OpenFoodFactsAPIController) {
	productRepo := dbController.NewProductRepository(dbHandle)

	// Pass 1: entries missing storage hint — requires an OFF API call.
	// Also download the image while we have the entry, if image caching is enabled.
	hintEntries, err := productRepo.GetOpenFoodFactsCacheWithoutStorageHint()
	if err != nil {
		logger.Warn().Msgf("Cache backfill: failed to query entries missing storage hint: %s", err)
	}

	processedBarcodes := make(map[string]bool, len(hintEntries))
	hintsUpdated := 0

	for i := range hintEntries {
		entry := &hintEntries[i]
		processedBarcodes[entry.Barcode] = true

		product, apiErr := offacntrl.GetDataset(entry.Barcode)
		if apiErr != nil {
			logger.Warn().Msgf("Cache backfill: OFF API error for %s: %s", entry.Barcode, apiErr)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		if product.StorageHint != "" {
			if updateErr := productRepo.UpdateOpenFoodFactsCacheStorageHint(entry.Barcode, product.StorageHint); updateErr != nil {
				logger.Warn().Msgf("Cache backfill: failed to update storage hint for %s: %s", entry.Barcode, updateErr)
			} else {
				hintsUpdated++
			}
		}

		// Download image if caching is enabled and image is not yet local
		if offacntrl.Configuration.ImageCacheEnabled {
			imageURL := entry.ImageURL
			if imageURL == "" {
				imageURL = product.ImageURL
			}
			if imageURL != "" && !strings.HasPrefix(imageURL, "/product-images/") {
				localPath, imgErr := offacntrl.DownloadImage(imageURL, entry.Barcode)
				if imgErr != nil {
					logger.Warn().Msgf("Cache backfill: failed to download image for %s: %s", entry.Barcode, imgErr)
				} else if updateErr := productRepo.UpdateOpenFoodFactsCacheImageURL(entry.Barcode, localPath); updateErr != nil {
					logger.Warn().Msgf("Cache backfill: failed to update image URL for %s: %s", entry.Barcode, updateErr)
				}
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	// Pass 2: entries that already have a storage hint but still have a remote image URL.
	// No API call needed — just download the image directly.
	imagesDownloaded := 0
	if offacntrl.Configuration.ImageCacheEnabled {
		imageEntries, imgQueryErr := productRepo.GetOpenFoodFactsCacheWithRemoteImageURL()
		if imgQueryErr != nil {
			logger.Warn().Msgf("Cache backfill: failed to query entries with remote images: %s", imgQueryErr)
		}

		for i := range imageEntries {
			entry := &imageEntries[i]
			if processedBarcodes[entry.Barcode] {
				continue
			}
			localPath, imgErr := offacntrl.DownloadImage(entry.ImageURL, entry.Barcode)
			if imgErr != nil {
				logger.Warn().Msgf("Cache backfill: failed to download image for %s: %s", entry.Barcode, imgErr)
			} else if updateErr := productRepo.UpdateOpenFoodFactsCacheImageURL(entry.Barcode, localPath); updateErr != nil {
				logger.Warn().Msgf("Cache backfill: failed to update image URL for %s: %s", entry.Barcode, updateErr)
			} else {
				imagesDownloaded++
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	logger.Info().Msgf("Cache backfill: complete — storage hints: %d, images downloaded: %d",
		hintsUpdated, imagesDownloaded)
}
