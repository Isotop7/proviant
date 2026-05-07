// configuration defines structs and methods for proviants configuration and specific parts of it
package configuration

import (
	"html/template"
	"strings"

	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
)

type DatabaseMariaDBConfiguration struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
}

type DatabaseSQLiteConfiguration struct {
	Filepath string
}

// DatabaseConfiguration contains all properties regarding the database connection
type DatabaseConfiguration struct {
	Engine  string
	MariaDB DatabaseMariaDBConfiguration
	SQLite  DatabaseSQLiteConfiguration
	// Additional, non parsed vars
	SelectedEngine database.SupportedEngines
}

// AuthenticationConfiguration contains all properties regarding the JSON Web Tokens
type AuthenticationConfiguration struct {
	TokenPassword            string `mapstructure:"tokenPassword"`
	TokenLifetime            int    `mapstructure:"tokenLifetime"`
	MaxLoginAttempts         int    `mapstructure:"max_login_attempts"`
	LockoutDurationMins      int    `mapstructure:"lockout_duration_mins"`
	PasswordMinLength        int    `mapstructure:"password_min_length"`
	PasswordRequireUppercase bool   `mapstructure:"password_require_uppercase"`
	PasswordRequireDigit     bool   `mapstructure:"password_require_digit"`
	PasswordRequireSpecial   bool   `mapstructure:"password_require_special"`
	PasswordCheckBreached    bool   `mapstructure:"password_check_breached"`
}

// CorsConfiguration contains all properties for the CORS configuration of the proviant server
type CorsConfiguration struct {
	AllowAllOrigins bool
	AllowedOrigins  []string
}

// SecurityHeadersConfiguration contains all properties for HTTP security headers
type SecurityHeadersConfiguration struct {
	ContentSecurityPolicy string
	CSRFTokenMaxAge       int `mapstructure:"csrf_token_max_age"` // seconds; default 86400 (24h)
}

// RateLimitConfiguration holds per-endpoint rate limits in requests per minute.
type RateLimitConfiguration struct {
	LoginPerMinute  int `mapstructure:"login_per_minute"`
	SignupPerMinute int `mapstructure:"signup_per_minute"`
	ExportPerMinute int `mapstructure:"export_per_minute"`
}

// ServerConfiguration contains all properties regarding the proviant server
type ServerConfiguration struct {
	Port            int
	Authentication  AuthenticationConfiguration
	CORS            CorsConfiguration
	BaseURL         string
	SecurityHeaders SecurityHeadersConfiguration
	RateLimit       RateLimitConfiguration `mapstructure:"rateLimit"`
	TrustedProxies  []string               `mapstructure:"trustedProxies"`
	MaxUploadSizeMB int                    `mapstructure:"maxUploadSizeMB"`
	Debug           bool                   `mapstructure:"debug"`
}

// LoggingConfiguration contains all properties regarding the log configuration for zerolog
type LoggingConfiguration struct {
	Enabled bool
	File    string
}

// SMTPConfiguration contains all properties regarding the notification handler target
type SMTPConfiguration struct {
	Host        string
	Port        int
	SSL         bool
	User        string
	FromAddress string
	Password    string
}

// NtfyConfiguration contains all properties regarding the ntfy.sh notification provider
type NtfyConfiguration struct {
	URL     string
	Topic   string
	Timeout int
}

// MonthlyWasteReportConfiguration controls when the monthly waste report email is sent.
type MonthlyWasteReportConfiguration struct {
	Day  int `mapstructure:"day"`  // day-of-month (1–28)
	Hour int `mapstructure:"hour"` // hour of day, UTC (0–23)
}

// TelegramConfiguration holds per-instance Telegram settings (no global bot token).
type TelegramConfiguration struct {
	Timeout       int // HTTP client timeout in seconds (default: 15)
	PollerWorkers int // Number of worker goroutines for polling all users (default: 10)
}

// NotificationConfiguration contains all properties regarding the notification handler
type NotificationConfiguration struct {
	Enabled            bool
	Interval           int
	SMTP               SMTPConfiguration
	Ntfy               NtfyConfiguration
	MonthlyWasteReport MonthlyWasteReportConfiguration `mapstructure:"monthlyWasteReport"`
	Telegram           TelegramConfiguration           `mapstructure:"telegram"`
}

// OpenFoodFactsConfiguration contains all properties regarding the OpenFoodFacts API controller
type OpenFoodFactsConfiguration struct {
	URL               string
	Timeout           int
	CacheEnabled      bool
	ImageCacheEnabled bool
	ImageCachePath    string
}

// OCRConfiguration contains settings for OCR expiry date detection
type OCRConfiguration struct {
	Enabled       bool   `mapstructure:"enabled"`       // master switch
	Provider      string `mapstructure:"provider"`      // "tesseract" (local), "google", "openai"
	APIKey        string `mapstructure:"apiKey"`        // for cloud providers
	Endpoint      string `mapstructure:"endpoint"`      // custom endpoint (e.g., Tesseract HTTP server)
	Timeout       int    `mapstructure:"timeout"`       // seconds per request
	Languages     string `mapstructure:"languages"`     // Tesseract language codes, e.g. "deu+eng"
	TesseractPath string `mapstructure:"tesseractPath"` // absolute path to tesseract binary
}

// RecipeAPIConfiguration contains settings for the recipe suggestions feature
type RecipeAPIConfiguration struct {
	Provider     string `mapstructure:"provider"` // "themealdb" or "spoonacular"
	URL          string `mapstructure:"url"`
	APIKey       string `mapstructure:"api_key"` // optional, for Spoonacular
	Timeout      int    `mapstructure:"timeout"` // seconds
	CacheEnabled bool   `mapstructure:"cache_enabled"`
	CacheTTL     int    `mapstructure:"cache_ttl"` // hours, default 24
}

// ExpiryConfiguration controls the visual expiry-status thresholds.
type ExpiryConfiguration struct {
	CriticalThresholdDays int `mapstructure:"critical_threshold_days"` // days before expiry to mark as critical (default: 3)
	SoonThresholdDays     int `mapstructure:"soon_threshold_days"`     // days before expiry to mark as expiring soon (default: 7)
}

// ProviantConfiguration is the configuration wrapper struct
type ProviantConfiguration struct {
	Database      DatabaseConfiguration
	Server        ServerConfiguration
	Logging       LoggingConfiguration
	Notification  NotificationConfiguration
	OpenFoodFacts OpenFoodFactsConfiguration
	OCR           OCRConfiguration       `mapstructure:"ocr"`
	RecipeAPI     RecipeAPIConfiguration `mapstructure:"recipe_api"`
	Expiry        ExpiryConfiguration    `mapstructure:"expiry"`
	TemplateCache map[string]*template.Template
}

// ValidateOpenFoodFactsConfiguration validates the current configuration to connect to the OpenFoodFact API
func (ec *ProviantConfiguration) ValidateOpenFoodFactsConfiguration() error {
	if ec.OpenFoodFacts.URL == "" {
		return errors.ErrOpenFoodFactsAPIEmptyURL
	}
	if ec.OpenFoodFacts.Timeout <= 0 {
		return errors.ErrOpenFoodFactsAPIInvalidTimeout
	}
	if ec.OpenFoodFacts.ImageCacheEnabled && ec.OpenFoodFacts.ImageCachePath == "" {
		return errors.ErrOpenFoodFactsAPIInvalidImageCachePath
	}
	return nil
}

func validateSMTPConfig(smtp *SMTPConfiguration) error {
	if smtp.Host == "" {
		return nil
	}
	if smtp.Port <= 0 {
		return errors.ErrNotificationInvalidSMTPPort
	}
	if smtp.FromAddress == "" {
		return errors.ErrNotificationEmptyFromAddress
	}
	return nil
}

func validateNtfyConfig(ntfy NtfyConfiguration) error {
	if ntfy.URL == "" {
		return nil
	}
	if ntfy.Topic == "" {
		return errors.ErrNotificationEmptyNtfyTopic
	}
	if !strings.HasPrefix(ntfy.URL, "http://") && !strings.HasPrefix(ntfy.URL, "https://") {
		return errors.ErrNotificationInvalidNtfyURL
	}
	return nil
}

// ValidateNotificationConfiguration validates the notification configuration
func (ec *ProviantConfiguration) ValidateNotificationConfiguration() error {
	if !ec.Notification.Enabled {
		return nil
	}
	if ec.Notification.Interval <= 0 {
		return errors.ErrNotificationInvalidInterval
	}
	if err := validateSMTPConfig(&ec.Notification.SMTP); err != nil {
		return err
	}
	day := ec.Notification.MonthlyWasteReport.Day
	if day < 1 || day > 28 {
		return errors.ErrNotificationInvalidWasteReportDay
	}
	hour := ec.Notification.MonthlyWasteReport.Hour
	if hour < 0 || hour > 23 {
		return errors.ErrNotificationInvalidWasteReportHour
	}
	return validateNtfyConfig(ec.Notification.Ntfy)
}

// ValidateDatabaseConfiguration checks the current database configuration for common errors
func (ec *ProviantConfiguration) ValidateDatabaseConfiguration() error {
	ec.Database.SelectedEngine = database.SupportedEnginesFromString(ec.Database.Engine)
	if ec.Database.SelectedEngine == database.InvalidEngine {
		return errors.ErrDatabaseInvalidEngine
	}

	switch ec.Database.SelectedEngine {
	case database.MariaDB:
		if ec.Database.MariaDB.Host == "" {
			return errors.ErrDatabaseMariaDBEmptyHost
		}
		if ec.Database.MariaDB.User == "" {
			return errors.ErrDatabaseMariaDBEmptyUser
		}
		if ec.Database.MariaDB.Password == "" {
			return errors.ErrDatabaseMariaDBEmptyPassword
		}
		if ec.Database.MariaDB.Name == "" {
			return errors.ErrDatabaseMariaDBEmptyName
		}
		if ec.Database.MariaDB.Port <= 0 {
			return errors.ErrDatabaseMariaDBInvalidPort
		}
	case database.SQLite:
		if ec.Database.SQLite.Filepath == "" {
			return errors.ErrDatabaseSQLiteInvalidPath
		}
	}
	return nil
}

// ValidateServerConfiguration validates the server authentication configuration
func (ec *ProviantConfiguration) ValidateServerConfiguration() error {
	if ec.Server.Authentication.TokenPassword == "" {
		return errors.ErrServerEmptyTokenPassword
	}
	if ec.Server.Authentication.TokenLifetime <= 0 {
		return errors.ErrServerInvalidTokenLifetime
	}
	rl := ec.Server.RateLimit
	if rl.LoginPerMinute <= 0 || rl.SignupPerMinute <= 0 || rl.ExportPerMinute <= 0 {
		return errors.ErrRateLimitInvalidValue
	}
	return nil
}

// ValidateRecipeAPIConfiguration validates the recipe API configuration
func (ec *ProviantConfiguration) ValidateRecipeAPIConfiguration() error {
	if ec.RecipeAPI.Provider == "" {
		return errors.ErrRecipeInvalidProvider
	}
	if ec.RecipeAPI.URL == "" {
		return errors.ErrRecipeAPIEmptyURL
	}
	if ec.RecipeAPI.Timeout <= 0 {
		return errors.ErrRecipeAPIInvalidTimeout
	}
	if ec.RecipeAPI.CacheTTL <= 0 {
		ec.RecipeAPI.CacheTTL = 24 // default to 24 hours
	}
	return nil
}
