// configuration defines structs and methods for proviants configuration and specific parts of it
package configuration

import (
	"html/template"
	"net/url"
	"strings"

	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/util"
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
	SkipEmailVerification    bool   `mapstructure:"skip_email_verification"`
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
	LoginPerMinute    int `mapstructure:"login_per_minute"`
	SignupPerMinute   int `mapstructure:"signup_per_minute"`
	ExportPerMinute   int `mapstructure:"export_per_minute"`
	PasswordPerMinute int `mapstructure:"password_per_minute"`
	ScanPerMinute     int `mapstructure:"scan_per_minute"`
	RecipesPerMinute  int `mapstructure:"recipes_per_minute"`
}

// ServerConfiguration contains all properties regarding the proviant server
type ServerConfiguration struct {
	Port int
	// Host is the bind address. Empty means every interface (wildcard), which
	// is the default and keeps container deployments reachable. Set it to
	// 127.0.0.1 for a loopback-only listener.
	Host            string
	Authentication  AuthenticationConfiguration
	CORS            CorsConfiguration
	BaseURL         string
	SecurityHeaders SecurityHeadersConfiguration
	RateLimit       RateLimitConfiguration `mapstructure:"rateLimit"`
	TrustedProxies  []string               `mapstructure:"trustedProxies"`
	MaxUploadSizeMB int                    `mapstructure:"maxUploadSizeMB"`
	Debug           bool                   `mapstructure:"debug"`
	DemoMode        bool                   `mapstructure:"demoMode"`
}

// CalendarConfiguration contains settings for calendar token expiry and warnings.
type CalendarConfiguration struct {
	TokenExpiryDays  int `mapstructure:"tokenExpiryDays"`  // days until calendar token expires, default 365
	ExpiringSoonDays int `mapstructure:"expiringSoonDays"` // days before expiry to show warning, default 30
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

// MailDigestConfiguration controls when the expiry digest email is sent.
type MailDigestConfiguration struct {
	Enabled     bool
	DefaultTime string `mapstructure:"defaultTime"` // "08:00"
}

// NotificationConfiguration contains all properties regarding the notification handler
type NotificationConfiguration struct {
	Enabled            bool
	Interval           int
	SMTP               SMTPConfiguration
	Ntfy               NtfyConfiguration
	MonthlyWasteReport MonthlyWasteReportConfiguration `mapstructure:"monthlyWasteReport"`
	Telegram           TelegramConfiguration           `mapstructure:"telegram"`
	MailDigest         MailDigestConfiguration         `mapstructure:"mailDigest"`
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
	Enabled       bool                    `mapstructure:"enabled"`       // master switch
	Provider      string                  `mapstructure:"provider"`      // "tesseract" (local), "google", "openai"
	APIKey        string                  `mapstructure:"apiKey"`        // for cloud providers
	Endpoint      string                  `mapstructure:"endpoint"`      // custom endpoint (e.g., Tesseract HTTP server)
	Timeout       int                     `mapstructure:"timeout"`       // seconds per request
	Languages     string                  `mapstructure:"languages"`     // Tesseract language codes, e.g. "deu+eng"
	TesseractPath string                  `mapstructure:"tesseractPath"` // absolute path to tesseract binary
	Receipt       ReceiptOCRConfiguration `mapstructure:"receipt"`       // receipt photo scanning via vision LLM
}

// ReceiptOCRConfiguration contains settings for receipt photo scanning
// (issue #61). Provider is restricted to vision-capable OpenAI-compatible
// endpoints; the whole feature is off unless Enabled is true.
type ReceiptOCRConfiguration struct {
	Enabled  bool   `mapstructure:"enabled"`  // master switch
	Provider string `mapstructure:"provider"` // "openai" — any OpenAI-compatible vision endpoint
	APIKey   string `mapstructure:"apiKey"`   // bearer token for the endpoint
	Endpoint string `mapstructure:"endpoint"` // chat completions URL; empty = OpenAI default
	Model    string `mapstructure:"model"`    // vision model name, required
	Timeout  int    `mapstructure:"timeout"`  // seconds per request, default 120
}

// RecipeAPIConfiguration contains settings for the recipe suggestions feature
type RecipeAPIConfiguration struct {
	Provider     string `mapstructure:"provider"` // "themealdb", "mealie" or "tandoor"
	URL          string `mapstructure:"url"`
	APIKey       string `mapstructure:"api_key"` // required for mealie and tandoor
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
	Calendar      CalendarConfiguration `mapstructure:"calendar"`
	Logging       LoggingConfiguration
	Notification  NotificationConfiguration
	OpenFoodFacts OpenFoodFactsConfiguration
	OCR           OCRConfiguration       `mapstructure:"ocr"`
	RecipeAPI     RecipeAPIConfiguration `mapstructure:"recipe_api"`
	Expiry        ExpiryConfiguration    `mapstructure:"expiry"`
	TemplateCache map[string]*template.Template

	// RecipeAPIDisabled is set at startup when the recipe feature cannot work
	// at all — currently only a self-hosted provider without its API key. It
	// is runtime state derived during validation, never read from the config
	// file. Its purpose is to make the feature degrade to "no suggestions"
	// instead of pairing an unusable provider with a URL it cannot use, which
	// would either 404 on every request or send the household's inventory
	// keywords to a public third-party API.
	RecipeAPIDisabled bool
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

// ValidateOCRReceiptConfiguration validates the receipt scan configuration.
// A disabled feature is always valid: nothing is parsed, no endpoint is
// contacted, so there is nothing to reject. An enabled configuration must
// name a vision-capable provider ("openai" — any OpenAI-compatible endpoint)
// and a model; a non-vision provider such as "tesseract" cannot return
// structured line items at all, so it is a hard validation error rather than
// a runtime fallback.
func (ec *ProviantConfiguration) ValidateOCRReceiptConfiguration() error {
	if !ec.OCR.Receipt.Enabled {
		return nil
	}
	provider := strings.ToLower(strings.TrimSpace(ec.OCR.Receipt.Provider))
	if provider == "" {
		provider = util.ReceiptOCRProviderOpenAI
	}
	ec.OCR.Receipt.Provider = provider
	if provider != util.ReceiptOCRProviderOpenAI {
		return errors.ErrReceiptOCRInvalidProvider
	}
	ec.OCR.Receipt.Model = strings.TrimSpace(ec.OCR.Receipt.Model)
	if ec.OCR.Receipt.Model == "" {
		return errors.ErrReceiptOCREmptyModel
	}
	if ec.OCR.Receipt.Timeout <= 0 {
		ec.OCR.Receipt.Timeout = util.ReceiptScanDefaultTimeout
	}
	// Trimmed like the recipe API key above: mapstructure hands over quoted
	// whitespace verbatim, so trailing whitespace would end up inside the
	// Authorization header or the request URL.
	ec.OCR.Receipt.Endpoint = strings.TrimSpace(ec.OCR.Receipt.Endpoint)
	ec.OCR.Receipt.APIKey = strings.TrimSpace(ec.OCR.Receipt.APIKey)
	return nil
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
	if rl.LoginPerMinute <= 0 || rl.SignupPerMinute <= 0 || rl.ExportPerMinute <= 0 ||
		rl.PasswordPerMinute <= 0 || rl.ScanPerMinute <= 0 || rl.RecipesPerMinute <= 0 {
		return errors.ErrRateLimitInvalidValue
	}
	return nil
}

// IsSelfHosted reports whether the configured provider is a self-hosted
// instance that authenticates with an API key, as opposed to the free public
// TheMealDB API.
//
// It delegates to isSelfHostedRecipeProvider so the provider list that gates
// the api_key check and the public-TheMealDB-host leak guard stays a single
// list. Two hand-maintained copies could drift, and the failure mode is silent:
// a provider present in one but not the other skips the guard whose whole
// purpose is keeping the operator's key and the household's product names off a
// third party.
func (rc RecipeAPIConfiguration) IsSelfHosted() bool {
	return isSelfHostedRecipeProvider(rc.Provider)
}

// themealDBHost is the apex host of the public TheMealDB API.
const themealDBHost = "themealdb.com"

// recipeAPIHost parses rawURL and returns its lowercased hostname with the DNS
// root dot removed. The trailing dot MUST be stripped: a fully qualified name
// that carries it resolves to the identical host, so leaving it in place lets
// "www.themealdb.com." slip past every host comparison built on this function.
// The second return is false when rawURL cannot be parsed.
func recipeAPIHost(rawURL string) (string, bool) {
	parsed, parseErr := url.Parse(strings.TrimSpace(rawURL))
	if parseErr != nil {
		return "", false
	}
	return strings.TrimSuffix(strings.ToLower(parsed.Hostname()), "."), true
}

// isPublicThemealDBHost reports whether rawURL addresses the public TheMealDB
// API. A self-hosted provider paired with it would send the operator's API key
// and the household's inventory-derived search keywords to a third party on
// every request. A URL that fails to parse returns false: it can never reach a
// host at all, so it fails closed rather than open.
func isPublicThemealDBHost(rawURL string) bool {
	host, parsed := recipeAPIHost(rawURL)
	if !parsed {
		return false
	}
	return host == themealDBHost || strings.HasSuffix(host, "."+themealDBHost)
}

// UsesInsecureTransport reports whether the configured provider URL is served
// over plaintext HTTP. A self-hosted provider authenticates every request with
// an API key and puts the household's inventory-derived search keywords in the
// query string, so an http:// instance URL transmits both in cleartext. This is
// surfaced as a startup warning rather than a rejection because a Mealie or
// Tandoor instance reachable only over a trusted LAN is a legitimate
// deployment, and refusing to start would be a worse outcome than the warning.
func (rc RecipeAPIConfiguration) UsesInsecureTransport() bool {
	parsed, parseErr := url.Parse(strings.TrimSpace(rc.URL))
	if parseErr != nil {
		return false
	}
	return strings.EqualFold(parsed.Scheme, "http")
}

// ValidateRecipeAPIConfiguration validates the recipe API configuration.
//
// Validation runs in two stages on purpose: the provider-independent checks
// (URL, timeout, cache TTL) always run first, so the provider-specific checks
// can never return early and skip them. The provider failure modes also need
// deliberately different handling, which is why they are separate errors rather
// than one "unsupported provider" case:
//
//   - An unknown provider name falls back to the TheMealDB dialect and KEEPS
//     the configured URL, because that URL may be a working TheMealDB proxy
//     and rewriting it would silently redirect inventory-derived search
//     keywords to the public API.
//   - A self-hosted provider without its API key keeps NEITHER the dialect
//     nor the URL, because that URL belongs to the instance we can no longer
//     authenticate against. There is no correct fallback URL, so the caller
//     disables the feature instead of guessing one.
//   - A self-hosted provider whose URL is the public TheMealDB endpoint is
//     rejected after the key check: without a key there is nothing to leak, so
//     the missing key is the actionable problem first. With a key present,
//     that URL is unambiguously wrong and no substitute URL can be guessed.
//     Both self-hosted failures are recoverable configuration errors, not
//     programming errors, so the caller degrades the feature instead of
//     refusing to boot.
func (ec *ProviantConfiguration) ValidateRecipeAPIConfiguration() error {
	provider := strings.ToLower(strings.TrimSpace(ec.RecipeAPI.Provider))
	if provider == "" {
		return errors.ErrRecipeInvalidProvider
	}
	if provider != util.RecipeProviderThemealDB && !isSelfHostedRecipeProvider(provider) {
		provider = util.RecipeProviderThemealDB
	}
	ec.RecipeAPI.Provider = provider

	if ec.RecipeAPI.URL == "" {
		return errors.ErrRecipeAPIEmptyURL
	}
	if ec.RecipeAPI.Timeout <= 0 {
		return errors.ErrRecipeAPIInvalidTimeout
	}
	if ec.RecipeAPI.CacheTTL <= 0 {
		ec.RecipeAPI.CacheTTL = 24 // default to 24 hours
	}

	// Trimmed like every other string here: mapstructure hands over quoted
	// whitespace verbatim, so a bare == "" check would let `api_key: " "` enable
	// a provider that then 401s on every request — silently defeating the
	// disable path this error drives.
	ec.RecipeAPI.APIKey = strings.TrimSpace(ec.RecipeAPI.APIKey)
	if ec.RecipeAPI.IsSelfHosted() && ec.RecipeAPI.APIKey == "" {
		return errors.ErrRecipeAPIMissingAPIKey
	}
	if ec.RecipeAPI.IsSelfHosted() && isPublicThemealDBHost(ec.RecipeAPI.URL) {
		return errors.ErrRecipeAPIProviderURLMismatch
	}
	return nil
}

// UsesInsecureTransport reports whether the configured receipt endpoint is
// served over plaintext HTTP. A custom endpoint authenticates every request
// with a bearer API key and receives the uploaded receipt photo, so an
// http:// URL transmits both in cleartext. Mirrors RecipeAPIConfiguration.
// UsesInsecureTransport: surfaced as a startup warning rather than a
// rejection because a LAN-only vision endpoint is a legitimate deployment.
// An empty Endpoint resolves to the https:// OpenAI default, so it is never
// insecure.
func (rc *ReceiptOCRConfiguration) UsesInsecureTransport() bool {
	parsed, parseErr := url.Parse(strings.TrimSpace(rc.Endpoint))
	if parseErr != nil {
		return false
	}
	return strings.EqualFold(parsed.Scheme, "http")
}

// isSelfHostedRecipeProvider reports whether name identifies a self-hosted
// provider that requires an API key. This is the single source of truth for
// that list: RecipeAPIConfiguration.IsSelfHosted delegates here, and
// ValidateRecipeAPIConfiguration uses it to decide which provider names survive
// normalization. Adding a self-hosted provider must touch only this function.
func isSelfHostedRecipeProvider(name string) bool {
	return name == util.RecipeProviderMealie || name == util.RecipeProviderTandoor
}
