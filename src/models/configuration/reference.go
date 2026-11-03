// reference.go exposes the configuration as an operator-facing reference: the
// keys, their types, the value each resolves to when the file omits it and
// the constraints the validator enforces.
//
// The key set comes from the configuration struct itself, so a field that is
// not documented fails the test suite instead of quietly disappearing from the
// documentation. The defaults live here as well, so a default can never be
// registered under a key the struct does not read.
package configuration

import (
	"maps"
	"reflect"
	"strings"

	"github.com/Isotop7/proviant/controllers/database"
	"github.com/Isotop7/proviant/util"
)

// DefaultServerPort is the port the server listens on when the configuration
// names no usable one.
const DefaultServerPort = 5114

// DefaultContentSecurityPolicy is the Content-Security-Policy the server sends
// when the configuration names none.
const DefaultContentSecurityPolicy = "default-src 'self'; script-src 'self' $NONCE; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:;"

// DefaultCSRFTokenMaxAge is the lifetime of the CSRF cookie, in seconds, when
// the configuration names none.
const DefaultCSRFTokenMaxAge = 86400

// Defaults returns the built-in default of every configuration key, keyed by
// the key path the configuration file uses.
//
// The keys are the paths of the configuration struct. A camel-cased key would
// be lower-cased by viper and then miss the snake-cased mapstructure tag of
// its field, and the default would be dropped without a word: that is how the
// password policy ended up with a minimum length of zero.
//
// Every key is in the map. A key viper has never heard of is ignored by the
// environment, so an operator who sets PROVIANT_SERVER_DEMOMODE against a
// configuration file that does not mention the key would watch nothing
// happen. A key without a default of its own therefore carries the zero value
// of its type, which is what the code resolved to anyway.
func Defaults() map[string]any {
	defaults := map[string]any{
		"notification.monthlyWasteReport.day":              1,
		"notification.monthlyWasteReport.hour":             8,
		"notification.mailDigest.enabled":                  true,
		"notification.mailDigest.defaultTime":              "08:00",
		"notification.telegram.pollerWorkers":              10,
		"recipe_api.provider":                              util.RecipeProviderThemealDB,
		"recipe_api.url":                                   util.DefaultTheMealDBRecipeAPIURL,
		"recipe_api.timeout":                               10,
		"recipe_api.cache_enabled":                         true,
		"recipe_api.cache_ttl":                             24,
		"server.authentication.password_min_length":        12,
		"server.authentication.password_require_uppercase": true,
		"server.authentication.password_require_digit":     true,
		"server.authentication.password_require_special":   false,
		"server.authentication.password_check_breached":    true,
		"server.twoFactor.enabled":                         false,
		"server.twoFactor.issuer":                          util.DefaultTwoFactorIssuer,
		"server.twoFactor.recoveryCodeCount":               util.DefaultTwoFactorRecoveryCodeCount,
		"server.maxUploadSizeMB":                           util.DefaultMaxUploadSizeMB,
		"server.rateLimit.login_per_minute":                5,
		"server.rateLimit.signup_per_minute":               3,
		"server.rateLimit.export_per_minute":               1,
		"server.rateLimit.recipes_per_minute":              6,
		"server.demoMode":                                  false,
		"server.metricsEnabled":                            false,
		"ocr.receipt.provider":                             util.ReceiptOCRProviderOpenAI,
		"ocr.receipt.timeout":                              util.ReceiptScanDefaultTimeout,
		"shopping_list_sync.enabled":                       false,
		"shopping_list_sync.interval_minutes":              util.DefaultShoppingListSyncIntervalMinutes,
		"shopping_list_sync.bring_enabled":                 false,
		"shopping_list_sync.todoist_enabled":               false,
		"backup.enabled":                                   false,
		"backup.intervalHours":                             util.DefaultBackupIntervalHours,
		"backup.directory":                                 util.DefaultBackupDirectory,
		"backup.retention":                                 util.DefaultBackupRetention,
		"restore.maxUploadSizeMB":                          util.DefaultRestoreMaxUploadSizeMB,
		"offlineSync.maxBatchBytes":                        util.ReceiptBulkMaxBodyBytes,
		"openfoodfacts.dumpImportEnabled":                  false,
		"openfoodfacts.dumpImportExpiryDays":               util.DefaultOpenFoodFactsDumpExpiryDays,
		"openfoodfacts.dumpUrl":                            util.OpenFoodFactsDumpURL,
		"openfoodfacts.dumpDeltaWindowDays":                util.DefaultOpenFoodFactsDeltaWindowDays,
		"openfoodfacts.dumpStagingPath":                    util.OffDumpStagingDir,
		"openfoodfacts.dumpDatabase":                       util.OffDumpDefaultDatabase,
	}
	maps.Copy(defaults, configFallbacks)
	for _, field := range configKeyFields() {
		if _, ok := defaults[field.Path]; !ok {
			defaults[field.Path] = zeroConfigValue(field.Type)
		}
	}
	return defaults
}

// configFallbacks are the values the code falls back to when a key is absent
// or holds a value it cannot use.
var configFallbacks = map[string]any{
	"server.port": DefaultServerPort,
	"server.host": "",
	"server.securityHeaders.contentSecurityPolicy": DefaultContentSecurityPolicy,
	"server.securityHeaders.csrf_token_max_age":    DefaultCSRFTokenMaxAge,
	"server.authentication.max_login_attempts":     database.DefaultMaxLoginAttempts,
	"server.authentication.lockout_duration_mins":  database.DefaultLockoutDurationMins,
	"calendar.tokenExpiryDays":                     365,
	"calendar.expiringSoonDays":                    30,
	"notification.ntfy.timeout":                    15,
	"notification.telegram.timeout":                15,
	"notification.telegram.pollerWorkers":          10,
	"expiry.critical_threshold_days":               3,
	"expiry.soon_threshold_days":                   7,
	"shopping_list_sync.interval_minutes":          util.DefaultShoppingListSyncIntervalMinutes,
}

// ConfigKey describes one configuration key for the operator reference.
type ConfigKey struct {
	// Path is the key as the configuration file spells it, for example
	// "server.authentication.tokenPassword".
	Path string
	// Env is the environment variable that overrides the key.
	Env string
	// Type is the value type: "string", "int", "bool" or "list".
	Type string
	// Default is the value the key resolves to when the configuration file
	// omits it.
	Default string
	// Constraint states what the validator rejects.
	Constraint string
	// Effect states what the key does.
	Effect string
	// Secret marks a key whose value must never appear in the documentation.
	Secret bool
}

// configKeyDoc carries the operator-facing facts about a key that the struct
// itself cannot state: what the validator rejects, what the key does and
// whether its value is a secret.
type configKeyDoc struct {
	Constraint string
	Effect     string
	Secret     bool
	// Default replaces the rendered default for the keys whose default is
	// prose rather than a value.
	Default string
}

// configKeyDocs documents every configuration key. A key that is missing here
// fails the test suite: the reference has to say what a key does.
var configKeyDocs = map[string]configKeyDoc{
	"server.port": {
		Constraint: "a value of zero or less falls back to 5114",
		Effect:     "The port the HTTP server listens on.",
	},
	"server.host": {
		Effect: "The address the server binds to. Empty binds every interface, which is what a " +
			"container deployment needs; 127.0.0.1 serves the loopback only.",
	},
	"server.baseURL": {
		Effect: "The public URL of this deployment, with the scheme. The scheme decides whether the " +
			"session and the CSRF cookie carry the Secure attribute, and the URL is the base of every " +
			"link in an email.",
	},
	"server.maxUploadSizeMB": {
		Effect: "The cap for a receipt photo, a CSV import or any other uploaded file, in megabytes.",
	},
	"server.debug": {
		Effect: "Runs Gin in debug mode and raises the log level to debug.",
	},
	"server.demoMode": {
		Effect: "Shows the demo banner on the home page. Nothing else consults the flag — the instance " +
			"is not read-only. The demo image's own PROVIANT_DEMO_MODE is a different variable: it seeds " +
			"the database and generates an ephemeral token password.",
	},
	"server.metricsEnabled": {
		Effect: "Serves the Prometheus /metrics endpoint. It is served without authentication, so only " +
			"trusted scrapers may reach it.",
	},
	"server.authentication.tokenPassword": {
		Constraint: "must not be empty and must not be the placeholder the retired configuration " +
			"templates shipped",
		Effect: "The secret the JSON Web Tokens are signed with. Anyone who knows it can forge a " +
			"session for any account. Generate one with openssl rand -hex 32.",
		Secret:  true,
		Default: "required",
	},
	"server.authentication.tokenLifetime": {
		Constraint: "must be greater than zero",
		Effect:     "How long a session token stays valid, in hours.",
		Default:    "required",
	},
	"server.authentication.max_login_attempts": {
		Constraint: "a value of zero or less falls back to 10",
		Effect:     "How many failed logins lock an account.",
	},
	"server.authentication.lockout_duration_mins": {
		Constraint: "a value of zero or less falls back to 15",
		Effect:     "How long a locked account stays locked, in minutes.",
	},
	"server.authentication.password_min_length": {
		Effect: "The shortest password an account may have. Zero lifts the length rule, and with it " +
			"the breach check.",
	},
	"server.authentication.password_require_uppercase": {
		Effect: "Requires at least one uppercase letter in a new password.",
	},
	"server.authentication.password_require_digit": {
		Effect: "Requires at least one digit in a new password.",
	},
	"server.authentication.password_require_special": {
		Effect: "Requires at least one special character in a new password.",
	},
	"server.authentication.password_check_breached": {
		Effect: "Checks a new password against the Have I Been Pwned API. The check only runs while " +
			"password_min_length is greater than zero.",
	},
	"server.authentication.skip_email_verification": {
		Effect: "Lets a new account sign in without confirming its email address.",
	},
	"server.twoFactor.enabled": {
		Effect: "The master switch for the TOTP second factor. While it is off the server exposes no " +
			"enrolment endpoint and stores no secret.",
	},
	"server.twoFactor.encryptionKey": {
		Constraint: "32 random bytes, base64 encoded; must not be empty, must not be the placeholder " +
			"the templates ship and must differ from server.authentication.tokenPassword",
		Effect: "Encrypts the TOTP secret of every enrolled account at rest. Rotating it locks every " +
			"enrolled device out and the enrolment has to be redone. Generate one with " +
			"openssl rand -base64 32.",
		Secret:  true,
		Default: "required while server.twoFactor.enabled is true",
	},
	"server.twoFactor.issuer": {
		Effect: "The account name the authenticator app shows.",
	},
	"server.twoFactor.recoveryCodeCount": {
		Constraint: "1 to 50",
		Effect:     "How many single-use recovery codes one enrolment issues.",
	},
	"server.cors.allowAllOrigins": {
		Effect: "Allows every origin instead of the listed ones.",
	},
	"server.cors.allowedOrigins": {
		Effect: "The origins a browser may call the API from.",
	},
	"server.trustedProxies": {
		Effect: "The CIDRs of the reverse proxies in front of the server. Their X-Forwarded-For and " +
			"X-Real-IP headers are trusted for the rate limits; without the entry the headers are " +
			"ignored and the connection address is used instead.",
	},
	"server.securityHeaders.contentSecurityPolicy": {
		Effect: "The Content-Security-Policy the server sends.",
	},
	"server.securityHeaders.csrf_token_max_age": {
		Constraint: "a value of zero or less falls back to 86400",
		Effect:     "The lifetime of the CSRF cookie, in seconds.",
	},
	"server.rateLimit.login_per_minute": {
		Constraint: "must be greater than zero",
		Effect:     "Login attempts per minute and client.",
	},
	"server.rateLimit.signup_per_minute": {
		Constraint: "must be greater than zero",
		Effect:     "Sign-up attempts per minute and client.",
	},
	"server.rateLimit.export_per_minute": {
		Constraint: "must be greater than zero",
		Effect:     "Export requests per minute and client.",
	},
	"server.rateLimit.password_per_minute": {
		Constraint: "must be greater than zero",
		Effect:     "Password change and reset requests per minute and client.",
		Default:    "required",
	},
	"server.rateLimit.scan_per_minute": {
		Constraint: "must be greater than zero",
		Effect:     "Barcode and OCR scans per minute and client.",
		Default:    "required",
	},
	"server.rateLimit.recipes_per_minute": {
		Constraint: "must be greater than zero",
		Effect:     "Recipe suggestion requests per minute and client.",
	},
	"calendar.tokenExpiryDays": {
		Constraint: "a value of zero or less falls back to 365",
		Effect:     "How long a calendar subscription stays valid, in days.",
	},
	"calendar.expiringSoonDays": {
		Constraint: "a value of zero or less falls back to 30",
		Effect: "How many days before the expiry date an item counts as expiring soon in the " +
			"calendar feed.",
	},
	"database.engine": {
		Constraint: "must be sqlite or mariadb",
		Effect:     "The database the server keeps its data in.",
		Default:    "required",
	},
	"database.sqlite.filepath": {
		Constraint: "must not be empty; the directory has to exist",
		Effect:     "The SQLite database file.",
		Default:    "required",
	},
	"database.mariadb.host": {
		Constraint: "must not be empty",
		Effect:     "The MariaDB server.",
		Default:    "required",
	},
	"database.mariadb.port": {
		Constraint: "must be greater than zero",
		Effect:     "The port of the MariaDB server.",
		Default:    "required",
	},
	"database.mariadb.name": {
		Constraint: "must not be empty",
		Effect:     "The schema the server stores its data in.",
		Default:    "required",
	},
	"database.mariadb.user": {
		Constraint: "must not be empty",
		Effect:     "The database account.",
		Default:    "required",
	},
	"database.mariadb.password": {
		Constraint: "must not be empty",
		Effect:     "The password of the database account.",
		Secret:     true,
		Default:    "required",
	},
	"logging.enabled": {
		Effect: "Writes the log to a file. While it is off the log goes to the standard output only.",
	},
	"logging.file": {
		Constraint: "must not be empty while logging.enabled is true; the server cannot open the file " +
			"otherwise",
		Effect: "The path of the log file.",
	},
	"notification.enabled": {
		Effect: "The master switch for the expiry notifications.",
	},
	"notification.interval": {
		Constraint: "must be greater than zero while notification.enabled is true; the server " +
			"refuses to start otherwise",
		Effect: "How often the notification sweep runs, in hours, and how long a product waits before " +
			"it is notified again. The number times 24 is the window in days in which the expiring-soon " +
			"webhook fires.",
		Default: "required",
	},
	"notification.smtp.host": {
		Constraint: "a port and a sender address are required once a host is set",
		Effect:     "The SMTP server the notification mail is sent through.",
	},
	"notification.smtp.port": {
		Constraint: "must be greater than zero once a host is set",
		Effect:     "The port of the SMTP server.",
	},
	"notification.smtp.ssl": {
		Effect: "Connects to the SMTP server over TLS.",
	},
	"notification.smtp.user": {
		Effect: "The SMTP account.",
	},
	"notification.smtp.fromAddress": {
		Constraint: "must not be empty once a host is set",
		Effect:     "The sender address of the notification mail.",
	},
	"notification.smtp.password": {
		Effect: "The password of the SMTP account.",
		Secret: true,
	},
	"notification.ntfy.url": {
		Constraint: "must start with http:// or https://",
		Effect:     "The ntfy server the notifications are published to.",
	},
	"notification.ntfy.topic": {
		Constraint: "must not be empty once a URL is set",
		Effect: "The ntfy topic the notifications are published to. Anyone who knows the topic can " +
			"read them.",
		Secret: true,
	},
	"notification.ntfy.timeout": {
		Constraint: "a value of zero or less falls back to 15",
		Effect:     "The ntfy request timeout, in seconds.",
	},
	"notification.monthlyWasteReport.day": {
		Constraint: "1 to 28",
		Effect:     "The day of the month the monthly waste report is sent.",
	},
	"notification.monthlyWasteReport.hour": {
		Constraint: "0 to 23",
		Effect:     "The hour of the day the monthly waste report is sent, in the container's time zone.",
	},
	"notification.telegram.timeout": {
		Constraint: "a value of zero or less falls back to 15",
		Effect:     "The Telegram API request timeout, in seconds.",
	},
	"notification.telegram.pollerWorkers": {
		Constraint: "a value of zero or less falls back to 10",
		Effect:     "How many goroutines poll Telegram for the users' updates.",
	},
	"notification.mailDigest.enabled": {
		Effect: "The master switch for the per-household expiry digest. A household still has to opt in.",
	},
	"notification.mailDigest.defaultTime": {
		Constraint: "an empty value falls back to 08:00",
		Effect:     "The local time the digest goes out for the households that set no time of their own.",
	},
	"openfoodfacts.url": {
		Constraint: "must not be empty; the server refuses to start otherwise",
		Effect:     "The Open Food Facts API the barcode lookups go to.",
		Default:    "required",
	},
	"openfoodfacts.timeout": {
		Constraint: "must be greater than zero; the server refuses to start otherwise",
		Effect:     "The request timeout of the barcode lookup, in seconds.",
		Default:    "required",
	},
	"openfoodfacts.cacheEnabled": {
		Effect: "Caches the barcode lookups in the database, so a scan works without a live request.",
	},
	"openfoodfacts.imageCacheEnabled": {
		Constraint: "a cache path is required once it is on",
		Effect:     "Caches the product images on disk.",
	},
	"openfoodfacts.imageCachePath": {
		Effect: "The directory the product images are cached in.",
	},
	"openfoodfacts.dumpImportEnabled": {
		Effect: "Imports the Open Food Facts bulk dump for an offline barcode lookup. The dump is a " +
			"multi-gigabyte download.",
	},
	"openfoodfacts.dumpImportExpiryDays": {
		Constraint: "must be greater than zero while the import is on",
		Effect:     "How often the full dump is downloaded again, in days.",
	},
	"openfoodfacts.dumpUrl": {
		Constraint: "must not be empty while the import is on",
		Effect:     "The URL of the full Open Food Facts dump.",
	},
	"openfoodfacts.dumpDeltaWindowDays": {
		Constraint: "must not be negative",
		Effect:     "How many days of delta files are considered between two full downloads.",
	},
	"openfoodfacts.dumpStagingPath": {
		Effect: "The directory the full dump is staged in. It makes an interrupted download or import " +
			"resumable and needs room for the multi-gigabyte download.",
	},
	"openfoodfacts.dumpDatabase": {
		Constraint: "must not be empty on MariaDB while the import is on; the server refuses to start " +
			"otherwise",
		Effect: "The dump store: the MariaDB schema, or data/<name>.db on SQLite. A value that names a " +
			"path is used verbatim. The schema has to exist before the import is enabled — " +
			"scripts/mariadb-init.sql creates it.",
	},
	"ocr.enabled": {
		Effect: "Not read by the server. The expiry date scan always runs the local tesseract binary; " +
			"the key is reserved for the cloud providers, which are not implemented.",
	},
	"ocr.provider": {
		Effect: "Not read by the server. Only the local tesseract binary runs — the cloud providers " +
			"(google, openai) are not implemented.",
	},
	"ocr.apiKey": {
		Effect: "Not read by the server. It belongs to the cloud OCR providers, which are not " +
			"implemented.",
		Secret: true,
	},
	"ocr.endpoint": {
		Effect: "Not read by the server. It belongs to a self-hosted OCR server, which is not " +
			"implemented.",
	},
	"ocr.timeout": {
		Effect: "The request timeout of the OCR call, in seconds. Without a value the request " +
			"expires at once.",
		Default: "none",
	},
	"ocr.languages": {
		Effect: "The Tesseract language codes, for example deu+eng.",
	},
	"ocr.tesseractPath": {
		Effect: "The path of the tesseract binary.",
	},
	"ocr.receipt.enabled": {
		Effect: "Enables the receipt scan.",
	},
	"ocr.receipt.provider": {
		Constraint: "must be openai - any OpenAI-compatible vision endpoint - once the receipt scan " +
			"is on",
		Effect: "The vision backend that reads the receipt.",
	},
	"ocr.receipt.apiKey": {
		Effect: "The bearer token of the vision endpoint.",
		Secret: true,
	},
	"ocr.receipt.endpoint": {
		Effect: "The chat-completions URL. Empty uses the OpenAI default; a base URL gets " +
			"/v1/chat/completions appended.",
	},
	"ocr.receipt.model": {
		Constraint: "must not be empty once the receipt scan is on",
		Effect:     "The vision model that reads the receipt.",
		Default:    "required while ocr.receipt.enabled is true",
	},
	"ocr.receipt.timeout": {
		Constraint: "a value of zero or less falls back to 120",
		Effect:     "The request timeout of the receipt scan, in seconds.",
	},
	"expiry.critical_threshold_days": {
		Constraint: "a value of zero or less falls back to 3",
		Effect:     "How many days before the expiry date a product counts as critical.",
	},
	"expiry.soon_threshold_days": {
		Constraint: "a value of zero or less falls back to 7",
		Effect:     "How many days before the expiry date a product counts as expiring soon.",
	},
	"recipe_api.provider": {
		Constraint: "must not be empty; the server refuses to start otherwise. An unknown name falls " +
			"back to themealdb.",
		Effect: "The recipe source: themealdb, mealie or tandoor.",
	},
	"recipe_api.url": {
		Constraint: "must not be empty; the server refuses to start otherwise. A self-hosted provider " +
			"must not point at the public TheMealDB host.",
		Effect: "The base URL of the recipe API.",
	},
	"recipe_api.api_key": {
		Constraint: "required for mealie and tandoor",
		Effect:     "The API key of the self-hosted recipe instance.",
		Secret:     true,
	},
	"recipe_api.timeout": {
		Constraint: "must be greater than zero; the server refuses to start otherwise",
		Effect:     "The request timeout of the recipe API, in seconds.",
	},
	"recipe_api.cache_enabled": {
		Effect: "Caches the recipe responses.",
	},
	"recipe_api.cache_ttl": {
		Constraint: "a value of zero or less falls back to 24",
		Effect:     "How long a cached recipe response stays valid, in hours.",
	},
	"shopping_list_sync.enabled": {
		Effect: "Synchronises the shopping list with the connected services.",
	},
	"shopping_list_sync.interval_minutes": {
		Constraint: "a value of zero or less falls back to 15",
		Effect:     "How often the list is synchronised, in minutes.",
	},
	"shopping_list_sync.bring_enabled": {
		Effect: "Enables the Bring! integration.",
	},
	"shopping_list_sync.todoist_enabled": {
		Effect: "Enables the Todoist integration.",
	},
	"backup.enabled": {
		Effect: "Runs the scheduled household backup.",
	},
	"backup.intervalHours": {
		Constraint: "must be greater than zero once the backup is on",
		Effect:     "How often the backup runs, in hours.",
	},
	"backup.directory": {
		Constraint: "must not be empty once the backup is on",
		Effect: "The directory the backup documents are written to. They carry household data in " +
			"cleartext, so it has to be storage the operator controls.",
	},
	"backup.retention": {
		Constraint: "must not be negative",
		Effect:     "How many backup files are kept per household. Zero keeps every file.",
	},
	"restore.maxUploadSizeMB": {
		Constraint: "a value of zero or less falls back to 50",
		Effect: "The cap for a restore upload, in megabytes. It is separate from " +
			"server.maxUploadSizeMB because a full household backup is larger than an ordinary " +
			"request body.",
	},
	"offlineSync.maxBatchBytes": {
		Constraint: "a value of zero or less falls back to the built-in cap",
		Effect: "The cap for one queued-sync request body. The outbox client reads the value from " +
			"the page and splits its backlog at it.",
		Default: "413696 bytes (about 404 KiB, 100 items with headroom)",
	},
}

// nonConfigFields names the struct fields that carry no configuration key:
// the parsed templates, the derived engine and the runtime flag.
var nonConfigFields = map[string]bool{
	"TemplateCache":     true,
	"RecipeAPIDisabled": true,
	"SelectedEngine":    true,
}

// configKeyField is one leaf of the configuration struct.
type configKeyField struct {
	Path string
	Type string
}

// configKeyFields returns every configuration key, in the order the struct
// declares them.
func configKeyFields() []configKeyField {
	var fields []configKeyField
	collectConfigKeyFields(reflect.TypeFor[ProviantConfiguration](), "", &fields)
	return fields
}

// collectConfigKeyFields walks a configuration struct and appends its leaves.
func collectConfigKeyFields(structType reflect.Type, prefix string, fields *[]configKeyField) {
	for field := range structType.Fields() {
		if nonConfigFields[field.Name] {
			continue
		}
		tag, _, _ := strings.Cut(field.Tag.Get("mapstructure"), ",")
		path := prefix + configFieldKey(tag, field.Name)
		if field.Type.Kind() == reflect.Struct {
			collectConfigKeyFields(field.Type, path+".", fields)
			continue
		}
		*fields = append(*fields, configKeyField{Path: path, Type: configFieldType(field.Type)})
	}
}

// configFieldKey returns the key a struct field is stored under: its
// mapstructure tag when it carries one, its lower-cased name otherwise.
func configFieldKey(tag string, fieldName string) string {
	if tag != "" {
		return tag
	}
	return strings.ToLower(fieldName[:1]) + fieldName[1:]
}

// configFieldType returns the operator-facing name of a field type.
func configFieldType(fieldType reflect.Type) string {
	switch fieldType.Kind() {
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "int"
	case reflect.Bool:
		return "bool"
	case reflect.Slice:
		return "list"
	default:
		return fieldType.Kind().String()
	}
}

// configEnvName returns the environment variable that overrides a key: the
// key path with the dots replaced by underscores, upper-cased and prefixed
// with the viper prefix.
func configEnvName(path string) string {
	return "PROVIANT_" + strings.ToUpper(strings.ReplaceAll(path, ".", "_"))
}

// ConfigKeys returns every configuration key with its type, its default, its
// constraint and its effect.
func ConfigKeys() []ConfigKey {
	fields := configKeyFields()
	keys := make([]ConfigKey, 0, len(fields))
	for _, field := range fields {
		doc := configKeyDocs[field.Path]
		keys = append(keys, ConfigKey{
			Path:       field.Path,
			Env:        configEnvName(field.Path),
			Type:       field.Type,
			Default:    configKeyDefault(field.Path, doc),
			Constraint: doc.Constraint,
			Effect:     doc.Effect,
			Secret:     doc.Secret,
		})
	}
	return keys
}

// configKeyDefault returns the value a key resolves to when the configuration
// file omits it.
func configKeyDefault(path string, doc configKeyDoc) string {
	if doc.Default != "" {
		return doc.Default
	}
	value, ok := Defaults()[path]
	if !ok {
		return ""
	}
	if doc.Secret {
		if isEmptyConfigValue(value) {
			return "empty"
		}
		return "set by the operator"
	}
	return renderConfigValue(value)
}

// zeroConfigValue returns the value a key holds when nothing sets it.
func zeroConfigValue(keyType string) any {
	switch keyType {
	case "int":
		return 0
	case "bool":
		return false
	case "list":
		return []string{}
	default:
		return ""
	}
}

// isEmptyConfigValue reports whether a configuration value carries nothing.
func isEmptyConfigValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case []string:
		return len(typed) == 0
	case []any:
		return len(typed) == 0
	default:
		return false
	}
}
