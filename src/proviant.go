// proviant is a simple and intuitive application to track your bought products and their expiration date to prevent waste of food
package main

import (
	"context"
	"flag"
	"fmt"
	stdlog "log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Isotop7/proviant/controllers"
	dbController "github.com/Isotop7/proviant/controllers/database"
	"github.com/Isotop7/proviant/errors"
	"github.com/Isotop7/proviant/logging"
	"github.com/Isotop7/proviant/migrations"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/configuration"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/router"
	"github.com/Isotop7/proviant/templates"
	"github.com/Isotop7/proviant/util"

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

// setupOffDumpDatabase opens the dedicated handle for the OpenFoodFacts bulk
// dump: a separate SQLite file, or a separate MariaDB schema. It never creates a
// schema; when the schema is missing it fails with a pointer to the init script,
// matching how the main database is provisioned outside the app.
func setupOffDumpDatabase(logger *zerolog.Logger, databaseConfiguration *configuration.DatabaseConfiguration, offConfig *configuration.OpenFoodFactsConfiguration) (*gorm.DB, error) {
	gormConfig := gorm.Config{}
	gormConfig.Logger = logging.ZerologAdapter{LoggingSink: logger}

	switch databaseConfiguration.SelectedEngine {
	case dbController.SQLite:
		dumpFile := util.OffDumpSQLiteFilename(offConfig.DumpDatabase)
		if err := os.MkdirAll(filepath.Dir(dumpFile), 0o750); err != nil {
			return nil, fmt.Errorf("%w: %s", errors.ErrOpenFoodFactsDumpDatabaseUnavailable, err)
		}
		// The dump is re-downloadable, so durability is traded for import speed.
		dsn := dumpFile + "?_busy_timeout=5000&_txlock=immediate&_journal_mode=WAL&_synchronous=OFF"
		dbHandle, err := gorm.Open(sqlite.Open(dsn), &gormConfig)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", errors.ErrOpenFoodFactsDumpDatabaseUnavailable, err)
		}
		return dbHandle, nil
	case dbController.MariaDB:
		databaseURI := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			databaseConfiguration.MariaDB.User,
			databaseConfiguration.MariaDB.Password,
			databaseConfiguration.MariaDB.Host,
			databaseConfiguration.MariaDB.Port,
			offConfig.DumpDatabase)
		dbHandle, err := gorm.Open(mysql.Open(databaseURI), &gormConfig)
		if err != nil {
			return nil, fmt.Errorf("%w: schema %q. Create it with scripts/mariadb-init.sql: %s",
				errors.ErrOpenFoodFactsDumpSchemaUnavailable, offConfig.DumpDatabase, err)
		}
		var schemaName string
		if err := dbHandle.Raw("SELECT SCHEMA_NAME FROM information_schema.SCHEMATA WHERE SCHEMA_NAME = ?", offConfig.DumpDatabase).Scan(&schemaName).Error; err != nil {
			return nil, fmt.Errorf("%w: %s", errors.ErrOpenFoodFactsDumpDatabaseUnavailable, err)
		}
		if schemaName == "" {
			return nil, fmt.Errorf("%w: %q. Create it with scripts/mariadb-init.sql",
				errors.ErrOpenFoodFactsDumpSchemaUnavailable, offConfig.DumpDatabase)
		}
		return dbHandle, nil
	default:
		return nil, errors.ErrDatabaseInvalidEngine
	}
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
	notificationController.SetActivityLogRepo(dbController.NewActivityLogRepository(dbHandle))
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
	return loadConfig(".")
}

// loadConfig reads config.yaml from configDir and unmarshals it. The directory is
// a parameter so tests can point it at a fixture rather than at a developer's
// local config file, which is gitignored and therefore absent in CI.
func loadConfig(configDir string) *configuration.ProviantConfiguration {
	config, err := readConfigFrom(viper.GetViper(), configDir)
	if err != nil {
		panic(err.Error())
	}
	return config
}

// readConfig reads config.yaml from configDir into a configuration of its own.
// Unlike loadConfig it neither touches the global viper nor terminates the
// process: the backup drill runs as a command and has to answer with an exit
// code, and it must not disturb the configuration a running instance reads.
func readConfig(configDir string) (*configuration.ProviantConfiguration, error) {
	return readConfigFrom(viper.New(), configDir)
}

// readConfigFrom reads config.yaml from configDir with the given viper
// instance.
func readConfigFrom(config *viper.Viper, configDir string) (*configuration.ProviantConfiguration, error) {
	// Set configuration file path
	config.SetConfigName("config")
	config.SetConfigType("yaml")
	config.AddConfigPath(configDir)
	// Set env prefix
	config.SetEnvPrefix("PROVIANT")
	config.AutomaticEnv()
	config.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	applyConfigDefaults(config)

	// Read configuration file
	if err := config.ReadInConfig(); err != nil {
		return nil, err
	}

	// Unmarshal yaml to configuration struct
	var proviantConfig configuration.ProviantConfiguration
	if err := config.Unmarshal(&proviantConfig); err != nil {
		return nil, err
	}
	return &proviantConfig, nil
}

// applyConfigDefaults sets the built-in default of every configuration key that
// has one.
func applyConfigDefaults(config *viper.Viper) {
	// Set defaults for monthly waste report schedule
	config.SetDefault("notification.monthlyWasteReport.day", 1)
	config.SetDefault("notification.monthlyWasteReport.hour", 8)

	// Set defaults for digest scheduler
	config.SetDefault("notification.digest.enabled", true)
	config.SetDefault("notification.digest.defaultTime", "08:00")

	// Set defaults for Telegram poller pool
	config.SetDefault("notification.telegram.pollerWorkers", 10)

	// Set defaults for recipe API
	config.SetDefault("recipe_api.provider", "themealdb")
	config.SetDefault("recipe_api.url", util.DefaultTheMealDBRecipeAPIURL)
	config.SetDefault("recipe_api.timeout", 10)
	config.SetDefault("recipe_api.cache_enabled", true)
	config.SetDefault("recipe_api.cache_ttl", 24)

	// Set default password policy
	config.SetDefault("server.authentication.passwordMinLength", 12)
	config.SetDefault("server.authentication.passwordRequireUppercase", true)
	config.SetDefault("server.authentication.passwordRequireDigit", true)
	config.SetDefault("server.authentication.passwordRequireSpecial", false)
	config.SetDefault("server.authentication.passwordCheckBreached", true)
	config.SetDefault("server.twoFactor.enabled", false)
	config.SetDefault("server.twoFactor.issuer", util.DefaultTwoFactorIssuer)
	config.SetDefault("server.twoFactor.recoveryCodeCount", util.DefaultTwoFactorRecoveryCodeCount)
	config.SetDefault("server.maxUploadSizeMB", 5)
	config.SetDefault("server.rateLimit.login_per_minute", 5)
	config.SetDefault("server.rateLimit.signup_per_minute", 3)
	config.SetDefault("server.rateLimit.export_per_minute", 1)
	config.SetDefault("server.rateLimit.recipes_per_minute", 6)
	config.SetDefault("server.demoMode", false)
	config.SetDefault("server.metricsEnabled", false)

	// Receipt scan defaults (issue #61): the OpenAI-compatible endpoint is
	// assumed unless another is configured; timeout follows the vision-LLM
	// reality of multi-second responses.
	config.SetDefault("ocr.receipt.provider", util.ReceiptOCRProviderOpenAI)
	config.SetDefault("ocr.receipt.timeout", util.ReceiptScanDefaultTimeout)

	config.SetDefault("shopping_list_sync.enabled", false)
	config.SetDefault("shopping_list_sync.interval_minutes", util.DefaultShoppingListSyncIntervalMinutes)
	config.SetDefault("shopping_list_sync.bring_enabled", false)
	config.SetDefault("shopping_list_sync.todoist_enabled", false)

	// Scheduled household backup defaults
	config.SetDefault("backup.enabled", false)
	config.SetDefault("backup.intervalHours", util.DefaultBackupIntervalHours)
	config.SetDefault("backup.directory", util.DefaultBackupDirectory)
	config.SetDefault("backup.retention", util.DefaultBackupRetention)

	// Household backup restore defaults
	config.SetDefault("restore.maxUploadSizeMB", util.DefaultRestoreMaxUploadSizeMB)

	// Queued offline sync defaults
	config.SetDefault("offlineSync.maxBatchBytes", util.ReceiptBulkMaxBodyBytes)

	// OpenFoodFacts bulk dump import defaults. The feature is opt-in: the full
	// dump is a multi-gigabyte download, so it never starts on its own.
	config.SetDefault("openfoodfacts.dumpImportEnabled", false)
	config.SetDefault("openfoodfacts.dumpImportExpiryDays", util.DefaultOpenFoodFactsDumpExpiryDays)
	config.SetDefault("openfoodfacts.dumpUrl", util.OpenFoodFactsDumpURL)
	config.SetDefault("openfoodfacts.dumpDeltaWindowDays", util.DefaultOpenFoodFactsDeltaWindowDays)
	config.SetDefault("openfoodfacts.dumpStagingPath", util.OffDumpStagingDir)
	config.SetDefault("openfoodfacts.dumpDatabase", util.OffDumpDefaultDatabase)
}

// setupLogging initializes the logging and returns a zerolog.Logger instance.
func setupLogging(config *configuration.ProviantConfiguration) *zerolog.Logger {
	logLevel := zerolog.InfoLevel
	if config.Server.Debug {
		logLevel = zerolog.DebugLevel
	}
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
		logFileLogger := zerolog.New(multi).Level(logLevel).With().Timestamp().Caller().Logger()
		return &logFileLogger
	} else {
		// Create terminal logger
		terminalLogger := zerolog.New(
			zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime},
		).Level(logLevel).With().Timestamp().Caller().Logger()
		return &terminalLogger
	}
}

// validateAPIs validates the API configurations.
func validateAPIs(config *configuration.ProviantConfiguration, logger *zerolog.Logger) {
	if err := config.ValidateOpenFoodFactsConfiguration(); err != nil {
		panic("Invalid OpenFoodFacts configuration: " + err.Error())
	}
	rawProvider := config.RecipeAPI.Provider
	if err := config.ValidateRecipeAPIConfiguration(); err != nil {
		// Both self-hosted misconfigurations degrade to RecipeAPIDisabled
		// instead of panicking, because neither has a correct substitute URL
		// to guess. Falling back to TheMealDB is never an option: keeping the
		// configured URL would either issue doomed requests to an instance we
		// cannot authenticate against (missing key) or send the operator's API
		// key plus the household's inventory keywords to the public TheMealDB
		// host (URL mismatch). Disabling keeps startup alive and the recipes
		// page quiet until the config is fixed, and never leaks the key.
		if err == errors.ErrRecipeAPIMissingAPIKey {
			logger.Warn().Msgf(
				"recipe API provider %q requires an api_key; recipe suggestions are disabled until one is configured",
				rawProvider)
			config.RecipeAPIDisabled = true
			return
		}
		if err == errors.ErrRecipeAPIProviderURLMismatch {
			logger.Warn().Msgf(
				"recipe API provider %q is pointed at the public TheMealDB host (%q); recipe suggestions are disabled "+
					"because every request would send your api_key and household product names to a third party. "+
					"Set recipe_api.url to your own %s instance URL",
				rawProvider, config.RecipeAPI.URL, rawProvider)
			config.RecipeAPIDisabled = true
			return
		}
		// Every other error (an empty provider name, an empty URL, an invalid
		// timeout) is a malformed config file with no safe degraded reading, so
		// startup still refuses rather than serving a half-configured feature.
		panic("Invalid recipe API configuration: " + err.Error())
	}
	// A self-hosted provider authenticates with an API key and sends
	// inventory-derived search keywords in the query string, so an http:// URL
	// puts both on the wire in cleartext. Allowed, because a LAN-only instance
	// is a legitimate deployment, but never silently.
	if config.RecipeAPI.IsSelfHosted() && config.RecipeAPI.UsesInsecureTransport() {
		logger.Warn().Msgf(
			"recipe API provider %q uses http:// (%s); its api_key and product search keywords are sent unencrypted. "+
				"Use https:// unless the instance is reachable only over a trusted network",
			config.RecipeAPI.Provider, config.RecipeAPI.URL)
	}
	if !strings.EqualFold(strings.TrimSpace(rawProvider), config.RecipeAPI.Provider) {
		logger.Warn().Msgf("recipe API provider %q is not supported, using %q instead", rawProvider, config.RecipeAPI.Provider)
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

	// An empty host binds every interface, which is what a container deployment
	// needs. Anything else (e.g. 127.0.0.1) narrows the listener to that address.
	bindHost := proviantConfiguration.Server.Host
	runErr := proviantEngine.Run(net.JoinHostPort(bindHost, strconv.Itoa(serverPort)))
	if runErr != nil {
		panic(runErr)
	}
}

// publishedPasswordWarning returns the warning for a database password this
// repository publishes, or an empty string when the password is the
// operator's own. The compose file and the config template publish database
// passwords, so a deployment that never overrides them authenticates with a
// credential everyone who read the repository knows. Allowed — a laptop
// running the shipped compose file is a legitimate deployment — but never
// silent.
func publishedPasswordWarning(config *configuration.ProviantConfiguration) string {
	if !config.Database.UsesPublishedPassword() {
		return ""
	}
	source := util.PublishedMariaDBPasswordSource(config.Database.MariaDB.Password)
	advice := "set PROVIANT_DATABASE_MARIADB_PASSWORD to your own value"
	if source == util.MariaDBComposeFile {
		// The compose file builds the app's value from its own variable, so
		// only a compose deployment can act on the second name.
		advice += " (a compose deployment sets PROVIANT_MARIADB_PASSWORD, which the compose file reads)"
	}
	message := fmt.Sprintf("The database password is the placeholder published in %s — the app "+
		"authenticates as %q with it, and anyone who reaches the database can read and write the household "+
		"data: %s.",
		source, config.Database.MariaDB.User, advice)
	if source == util.MariaDBComposeFile {
		message += fmt.Sprintf(" The compose file also falls back to the published root password %q unless "+
			"PROVIANT_MARIADB_ROOT_PASSWORD is set — the server never sees that one, so check it yourself.",
			util.PublishedMariaDBRootPassword)
	}
	return message
}

// runBackupDrill restores the newest backup into a throwaway database and
// reports whether the restored rows still match the document. It never opens
// the live database: the drill restores into a scratch file of its own.
//
// The exit code tells a scheduled job what happened: zero means the backup
// restores, one means it is broken, two means the schema moved under it and
// three means there was nothing to drill.
func runBackupDrill(args []string) int {
	config, configErr := readConfig(".")
	return runBackupDrillWith(args, config, configErr)
}

// runBackupDrillWith runs the drill against a configuration that is already
// read. The configuration is a parameter so the drill's decisions can be
// tested without a configuration file on disk.
func runBackupDrillWith(
	args []string,
	config *configuration.ProviantConfiguration,
	configErr error,
) int {
	if config == nil && configErr == nil {
		configErr = fmt.Errorf("no configuration was read")
	}

	flags := flag.NewFlagSet(util.BackupDrillCommand, flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	document := flags.String("file", "", "drill this backup document instead of the newest one")
	directory := flags.String("directory", "", "directory the backup documents live in")
	if err := flags.Parse(args); err != nil {
		// A usage error is neither a broken backup nor a broken schema: there
		// is nothing to drill.
		return controllers.BackupDrillExitUnavailable
	}

	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.DateTime}).With().Timestamp().Caller().Logger()

	if configErr != nil {
		logger.Warn().Msgf("Backup drill: the configuration could not be read (%s), the drill falls back to %s",
			configErr, util.DefaultBackupDirectory)
	}
	backupDirectory := strings.TrimSpace(*directory)
	if backupDirectory == "" && configErr == nil {
		backupDirectory = strings.TrimSpace(config.Backup.Directory)
	}
	if backupDirectory == "" {
		backupDirectory = util.DefaultBackupDirectory
	}
	liveDatabasePath := ""
	if configErr == nil {
		liveDatabasePath = config.Database.SQLite.Filepath
	}

	// A deployment that does not run the scheduled backup has nothing to
	// drill. Reporting that as a failure would fail the scheduled job of
	// every installation that never enabled the backup. A document or a
	// directory the operator named is drilled either way.
	if configErr == nil && !config.Backup.Enabled &&
		strings.TrimSpace(*document) == "" && strings.TrimSpace(*directory) == "" {
		logger.Info().Msg("Backup drill: the scheduled backup is disabled, there is nothing to drill")
		return controllers.BackupDrillExitOK
	}

	drill := &controllers.BackupDrill{
		Logger:           &logger,
		BackupDirectory:  backupDirectory,
		BackupFile:       strings.TrimSpace(*document),
		LiveDatabasePath: liveDatabasePath,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(util.DefaultBackupDrillTimeoutMinutes)*time.Minute)
	defer cancel()

	result, err := drill.Run(ctx)
	if result.BackupPath != "" {
		printBackupDrillResult(&result)
	}
	if err != nil {
		logger.Error().Msgf("Backup drill failed: %s", err)
		return controllers.BackupDrillExitCode(err)
	}
	logger.Info().Msg("Backup drill: the newest backup restores and its counts match")
	return controllers.BackupDrillExitOK
}

// printBackupDrillResult prints the comparison the drill produced.
func printBackupDrillResult(result *controllers.BackupDrillResult) {
	fmt.Printf("backup:  %s\n", result.BackupPath)
	fmt.Printf("age:     %s\n", result.BackupAge.Round(time.Second))
	fmt.Printf("schema:  %d\n", result.SchemaVersion)
	for _, count := range result.Counts {
		if !count.Covered {
			fmt.Printf("%-18s not carried by the backup document\n", string(count.Entity))
			continue
		}
		fmt.Printf("%-18s %d expected, %d restored\n", string(count.Entity), count.Expected, count.Restored)
	}
	for _, mismatch := range result.Mismatches {
		fmt.Printf("%-18s %s: %s is %q in the document and %q in the restore\n",
			mismatch.Entity, mismatch.Key, mismatch.Field, mismatch.Document, mismatch.Restored)
	}
}

// main is the main function used on start of proviant
func main() {
	// The backup drill runs before anything else: it restores a backup into a
	// throwaway database, so the live database is never opened by it.
	if len(os.Args) > 1 && os.Args[1] == util.BackupDrillCommand {
		os.Exit(runBackupDrill(os.Args[2:]))
	}

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

	if warning := publishedPasswordWarning(proviantConfiguration); warning != "" {
		logger.Warn().Msg(warning)
	}

	// Validate server configuration
	if err := proviantConfiguration.ValidateServerConfiguration(); err != nil {
		logger.Error().Msg(err.Error())
		panic(err)
	}

	// Validate two-factor authentication configuration
	if err := proviantConfiguration.ValidateTwoFactorConfiguration(); err != nil {
		logger.Error().Msg(err.Error())
		panic(err)
	}

	// Validate receipt scan configuration
	if err := proviantConfiguration.ValidateOCRReceiptConfiguration(); err != nil {
		logger.Error().Msg(err.Error())
		panic(err)
	}
	// A custom receipt endpoint authenticates with a bearer API key and
	// receives the uploaded receipt photo, so an http:// URL puts both on the
	// wire in cleartext. Allowed, because a LAN-only vision endpoint is a
	// legitimate deployment, but never silently.
	if proviantConfiguration.OCR.Receipt.Enabled && proviantConfiguration.OCR.Receipt.UsesInsecureTransport() {
		logger.Warn().Msgf(
			"receipt OCR endpoint %q uses http://; its api_key and the receipt photos are sent unencrypted. "+
				"Use https:// unless the endpoint is reachable only over a trusted network",
			proviantConfiguration.OCR.Receipt.Endpoint)
	}

	// Validate scheduled backup configuration
	if err := proviantConfiguration.ValidateBackupConfiguration(); err != nil {
		logger.Error().Msg(err.Error())
		panic(err)
	}
	proviantConfiguration.ValidateRestoreConfiguration()
	proviantConfiguration.ValidateOfflineSyncConfiguration()

	// Setup database connection handle
	dbHandle, setupErr := setupDatabase(logger, &proviantConfiguration.Database)
	if setupErr != nil {
		panic(setupErr)
	} else if dbHandle == nil {
		panic("Error getting database handle")
	}

	// Run migrations for database and check for errors
	migrationError := migrations.AutoMigrateAll(dbHandle)
	if migrationError != nil {
		panic(migrationError)
	}

	// Run migrations for breaking changes
	breakingMigrationsError := migrations.RunBreakingDatabaseMigrations(logger, dbHandle)
	if breakingMigrationsError != nil {
		panic(breakingMigrationsError)
	}

	// Backfill default quantity for existing products
	if amountMigrationError := migrations.SetDefaultProductQuantities(logger, dbHandle); amountMigrationError != nil {
		panic(amountMigrationError)
	}

	// Seed ProductCategoryPrice reference data for savings calculator
	if seedErr := migrations.SeedProductCategoryPrices(logger, dbHandle); seedErr != nil {
		panic(seedErr)
	}

	// Initialize webhook service
	controllers.InitWebhookService(dbHandle, logger)

	// Check API controller config and create instance
	validateAPIs(proviantConfiguration, logger)
	offacntrl := &controllers.OpenFoodFactsAPIController{
		Configuration: proviantConfiguration.OpenFoodFacts,
		Logger:        logger,
	}

	// OpenFoodFacts bulk dump import (opt-in). Opens a dedicated store and
	// fails fast if it is misconfigured rather than running without offline data.
	if proviantConfiguration.OpenFoodFacts.DumpImportEnabled {
		offDumpDB, offDumpErr := setupOffDumpDatabase(logger, &proviantConfiguration.Database, &proviantConfiguration.OpenFoodFacts)
		if offDumpErr != nil {
			logger.Error().Msg(offDumpErr.Error())
			panic(offDumpErr)
		}
		offDumpRepo := dbController.NewOffDumpRepository(offDumpDB)
		if offDumpMigrateErr := offDumpRepo.Migrate(); offDumpMigrateErr != nil {
			logger.Error().Msg(offDumpMigrateErr.Error())
			panic(offDumpMigrateErr)
		}
		offDumpController := controllers.NewOffDumpController(logger, &proviantConfiguration.OpenFoodFacts, offDumpRepo)
		offacntrl.OfflineLookup = offDumpController
		offDumpController.Dispatch()
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

	shoppingListSyncController := controllers.NewShoppingListSyncController(logger, &proviantConfiguration.ShoppingListSync, dbHandle)
	shoppingListSyncController.Dispatch()

	backupController := controllers.NewBackupController(logger, &proviantConfiguration.Backup, dbHandle)
	backupController.Dispatch()

	packSizeController := controllers.NewPackSizeController(dbHandle, offacntrl, logger)
	packSizeController.Dispatch()

	// Start background cleanup of expired revoked tokens
	go startRevokedTokenCleanup(logger, dbHandle)

	// Start background cleanup of expired recipe caches
	go startRecipeCacheCleanup(logger, dbHandle)

	// Start background cleanup of expired password-reset rows
	go startPasswordResetCleanup(logger, dbHandle)

	// Start background cleanup of expired product mutation claims
	go startMutationClaimCleanup(logger, dbHandle)

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

// startPasswordResetCleanup runs a goroutine that periodically deletes
// expired password-reset rows so the table does not grow unbounded.
func startPasswordResetCleanup(logger *zerolog.Logger, dbHandle *gorm.DB) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		result := dbHandle.Where("expires_at < ?", time.Now()).Delete(&dbModel.PasswordReset{})
		if result.Error != nil {
			logger.Warn().Msgf("Failed to cleanup expired password resets: %s", result.Error.Error())
			continue
		}
		if result.RowsAffected > 0 {
			logger.Info().Msgf("Cleaned up %d expired password resets", result.RowsAffected)
		}
	}
}

// startMutationClaimCleanup drops idempotency claims past the point where their
// delta can still be replayed, which is what bounds product_mutation_claims: a
// claim is written by every amount change, so nothing else ever collects them.
// The claim model carries no DeletedAt, so this is a hard delete and the
// unique index never spans a tombstoned row.
func startMutationClaimCleanup(logger *zerolog.Logger, dbHandle *gorm.DB) {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-util.ProductMutationClaimTTL)
		result := dbHandle.Where("created_at < ?", cutoff).Delete(&dbModel.ProductMutationClaim{})
		if result.Error != nil {
			logger.Warn().Msgf("Failed to cleanup expired product mutation claims: %s", result.Error.Error())
			continue
		}
		if result.RowsAffected > 0 {
			logger.Info().Msgf("Cleaned up %d expired product mutation claims", result.RowsAffected)
		}
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
	recipeCacheRepository := dbController.NewRecipeCacheRepository(dbHandle)
	ticker := time.NewTicker(24 * time.Hour) // Clean up once per day
	defer ticker.Stop()

	for range ticker.C {
		if err := recipeCacheRepository.CleanupExpiredCaches(); err != nil {
			logger.Warn().Msgf("Failed to cleanup expired recipe caches: %s", err.Error())
		}
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
