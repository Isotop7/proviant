# Configuration reference

Every key the server understands: what it does, what it resolves to when the
configuration file leaves it out, and what the validator rejects.

The document is generated from the configuration itself. A key that the code
reads but this document does not mention fails the test suite, so the two
cannot drift apart.

## How a value is resolved

1. **The environment variable.** The name is the key path with the dots
   replaced by underscores, upper-cased and prefixed with `PROVIANT_`,
   so `server.authentication.tokenPassword` is
   `PROVIANT_SERVER_AUTHENTICATION_TOKENPASSWORD`.
2. **The configuration file.** `config.yaml` in the working directory of the
   process. The shipped templates are `src/config.yaml.sqlite.tmpl` and
   `src/config.yaml.mariadb.tmpl`.
3. **The built-in default.** The **Default** column names it.

A value in the environment wins over the same key in the file, and a key in
the file wins over the default. An environment variable that is set to an
empty string counts as unset.

## The columns

| Column | What it holds |
|---|---|
| Key | The path of the key in the configuration file. |
| Environment variable | The variable that overrides it. |
| Type | The value type: `string`, `int`, `bool` or `list`. |
| Default | The value the key resolves to when the configuration file omits it. |
| Config template | The value the shipped templates set. Where the templates differ, both are named. |
| Constraint | What the validator rejects. A key the server cannot start without says so. |
| Effect | What the key does. |

## Secrets

A key whose value is a credential carries a placeholder here and never its
value. The credentials of this repository — the shipped placeholders the
server refuses or warns about — are not part of this document either.

The Telegram bot token and the VAPID keys are not configuration keys: the
bot token belongs to the account that sets it in the app, and the VAPID keys
are generated into the database on the first run.


## Server

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `server.port` | `PROVIANT_SERVER_PORT` | int | 5114 | 5114 | a value of zero or less falls back to 5114 | The port the HTTP server listens on. |
| `server.host` | `PROVIANT_SERVER_HOST` | string | empty | empty | — | The address the server binds to. Empty binds every interface, which is what a container deployment needs; 127.0.0.1 serves the loopback only. |
| `server.baseURL` | `PROVIANT_SERVER_BASEURL` | string | empty | https://proviant.local.de | — | The public URL of this deployment, with the scheme. The scheme decides whether the session and the CSRF cookie carry the Secure attribute, and the URL is the base of every link in an email. |
| `server.trustedProxies` | `PROVIANT_SERVER_TRUSTEDPROXIES` | list | empty | not set | — | The CIDRs of the reverse proxies in front of the server. Their X-Forwarded-For and X-Real-IP headers are trusted for the rate limits; without the entry the headers are ignored and the connection address is used instead. |
| `server.maxUploadSizeMB` | `PROVIANT_SERVER_MAXUPLOADSIZEMB` | int | 5 | 5 | — | The cap for a receipt photo, a CSV import or any other uploaded file, in megabytes. |
| `server.debug` | `PROVIANT_SERVER_DEBUG` | bool | false | false | — | Runs Gin in debug mode and raises the log level to debug. |
| `server.demoMode` | `PROVIANT_SERVER_DEMOMODE` | bool | false | not set | — | Shows the demo banner on the home page. Nothing else consults the flag — the instance is not read-only. The demo image's own PROVIANT_DEMO_MODE is a different variable: it seeds the database and generates an ephemeral token password. |
| `server.metricsEnabled` | `PROVIANT_SERVER_METRICSENABLED` | bool | false | false | — | Serves the Prometheus /metrics endpoint. It is served without authentication, so only trusted scrapers may reach it. |

## Authentication

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `server.authentication.tokenPassword` | `PROVIANT_SERVER_AUTHENTICATION_TOKENPASSWORD` | string | required | empty | must not be empty and must not be the placeholder the retired configuration templates shipped | The secret the JSON Web Tokens are signed with. Anyone who knows it can forge a session for any account. Generate one with openssl rand -hex 32. |
| `server.authentication.tokenLifetime` | `PROVIANT_SERVER_AUTHENTICATION_TOKENLIFETIME` | int | required | 8 | must be greater than zero | How long a session token stays valid, in hours. |
| `server.authentication.max_login_attempts` | `PROVIANT_SERVER_AUTHENTICATION_MAX_LOGIN_ATTEMPTS` | int | 10 | 3 | a value of zero or less falls back to 10 | How many failed logins lock an account. |
| `server.authentication.lockout_duration_mins` | `PROVIANT_SERVER_AUTHENTICATION_LOCKOUT_DURATION_MINS` | int | 15 | 10 | a value of zero or less falls back to 15 | How long a locked account stays locked, in minutes. |
| `server.authentication.password_min_length` | `PROVIANT_SERVER_AUTHENTICATION_PASSWORD_MIN_LENGTH` | int | 12 | 12 | — | The shortest password an account may have. Zero lifts the length rule, and with it the breach check. |
| `server.authentication.password_require_uppercase` | `PROVIANT_SERVER_AUTHENTICATION_PASSWORD_REQUIRE_UPPERCASE` | bool | true | false | — | Requires at least one uppercase letter in a new password. |
| `server.authentication.password_require_digit` | `PROVIANT_SERVER_AUTHENTICATION_PASSWORD_REQUIRE_DIGIT` | bool | true | false | — | Requires at least one digit in a new password. |
| `server.authentication.password_require_special` | `PROVIANT_SERVER_AUTHENTICATION_PASSWORD_REQUIRE_SPECIAL` | bool | false | false | — | Requires at least one special character in a new password. |
| `server.authentication.password_check_breached` | `PROVIANT_SERVER_AUTHENTICATION_PASSWORD_CHECK_BREACHED` | bool | true | true | — | Checks a new password against the Have I Been Pwned API. The check only runs while password_min_length is greater than zero. |
| `server.authentication.skip_email_verification` | `PROVIANT_SERVER_AUTHENTICATION_SKIP_EMAIL_VERIFICATION` | bool | false | not set | — | Lets a new account sign in without confirming its email address. |

## Two-factor authentication

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `server.twoFactor.enabled` | `PROVIANT_SERVER_TWOFACTOR_ENABLED` | bool | false | false | — | The master switch for the TOTP second factor. While it is off the server exposes no enrolment endpoint and stores no secret. |
| `server.twoFactor.encryptionKey` | `PROVIANT_SERVER_TWOFACTOR_ENCRYPTIONKEY` | string | required while server.twoFactor.enabled is true | a placeholder — replace it | 32 random bytes, base64 encoded; must not be empty, must not be the placeholder the templates ship and must differ from server.authentication.tokenPassword | Encrypts the TOTP secret of every enrolled account at rest. Rotating it locks every enrolled device out and the enrolment has to be redone. Generate one with openssl rand -base64 32. |
| `server.twoFactor.issuer` | `PROVIANT_SERVER_TWOFACTOR_ISSUER` | string | Proviant | Proviant | — | The account name the authenticator app shows. |
| `server.twoFactor.recoveryCodeCount` | `PROVIANT_SERVER_TWOFACTOR_RECOVERYCODECOUNT` | int | 10 | 10 | 1 to 50 | How many single-use recovery codes one enrolment issues. |

## CORS

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `server.cors.allowAllOrigins` | `PROVIANT_SERVER_CORS_ALLOWALLORIGINS` | bool | false | false | — | Allows every origin instead of the listed ones. |
| `server.cors.allowedOrigins` | `PROVIANT_SERVER_CORS_ALLOWEDORIGINS` | list | empty | http://localhost, http://myproviant.instance | — | The origins a browser may call the API from. |

## Security headers

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `server.securityHeaders.contentSecurityPolicy` | `PROVIANT_SERVER_SECURITYHEADERS_CONTENTSECURITYPOLICY` | string | default-src 'self'; script-src 'self' $NONCE; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; | default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; | — | The Content-Security-Policy the server sends. |
| `server.securityHeaders.csrf_token_max_age` | `PROVIANT_SERVER_SECURITYHEADERS_CSRF_TOKEN_MAX_AGE` | int | 86400 | 86400 | a value of zero or less falls back to 86400 | The lifetime of the CSRF cookie, in seconds. |

## Rate limits

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `server.rateLimit.login_per_minute` | `PROVIANT_SERVER_RATELIMIT_LOGIN_PER_MINUTE` | int | 5 | 5 | must be greater than zero | Login attempts per minute and client. |
| `server.rateLimit.signup_per_minute` | `PROVIANT_SERVER_RATELIMIT_SIGNUP_PER_MINUTE` | int | 3 | 3 | must be greater than zero | Sign-up attempts per minute and client. |
| `server.rateLimit.export_per_minute` | `PROVIANT_SERVER_RATELIMIT_EXPORT_PER_MINUTE` | int | 1 | 1 | must be greater than zero | Export requests per minute and client. |
| `server.rateLimit.password_per_minute` | `PROVIANT_SERVER_RATELIMIT_PASSWORD_PER_MINUTE` | int | required | 3 | must be greater than zero | Password change and reset requests per minute and client. |
| `server.rateLimit.scan_per_minute` | `PROVIANT_SERVER_RATELIMIT_SCAN_PER_MINUTE` | int | required | 10 | must be greater than zero | Barcode and OCR scans per minute and client. |
| `server.rateLimit.recipes_per_minute` | `PROVIANT_SERVER_RATELIMIT_RECIPES_PER_MINUTE` | int | 6 | 6 | must be greater than zero | Recipe suggestion requests per minute and client. |

## Calendar feed

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `calendar.tokenExpiryDays` | `PROVIANT_CALENDAR_TOKENEXPIRYDAYS` | int | 365 | 365 | a value of zero or less falls back to 365 | How long a calendar subscription stays valid, in days. |
| `calendar.expiringSoonDays` | `PROVIANT_CALENDAR_EXPIRINGSOONDAYS` | int | 30 | 30 | a value of zero or less falls back to 30 | How many days before the expiry date an item counts as expiring soon in the calendar feed. |

## Database

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `database.engine` | `PROVIANT_DATABASE_ENGINE` | string | required | sqlite (SQLite) / mariadb (MariaDB) | must be sqlite or mariadb | The database the server keeps its data in. |

## SQLite

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `database.sqlite.filepath` | `PROVIANT_DATABASE_SQLITE_FILEPATH` | string | required | data/proviant.db | must not be empty; the directory has to exist | The SQLite database file. |

## MariaDB

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `database.mariadb.host` | `PROVIANT_DATABASE_MARIADB_HOST` | string | required | 127.0.0.1 | must not be empty | The MariaDB server. |
| `database.mariadb.port` | `PROVIANT_DATABASE_MARIADB_PORT` | int | required | 3306 | must be greater than zero | The port of the MariaDB server. |
| `database.mariadb.name` | `PROVIANT_DATABASE_MARIADB_NAME` | string | required | proviant | must not be empty | The schema the server stores its data in. |
| `database.mariadb.user` | `PROVIANT_DATABASE_MARIADB_USER` | string | required | root | must not be empty | The database account. |
| `database.mariadb.password` | `PROVIANT_DATABASE_MARIADB_PASSWORD` | string | required | a placeholder — replace it | must not be empty | The password of the database account. |

## Logging

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `logging.enabled` | `PROVIANT_LOGGING_ENABLED` | bool | false | true | — | Writes the log to a file. While it is off the log goes to the standard output only. |
| `logging.file` | `PROVIANT_LOGGING_FILE` | string | empty | proviant.log | must not be empty while logging.enabled is true; the server cannot open the file otherwise | The path of the log file. |

## Notifications

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `notification.enabled` | `PROVIANT_NOTIFICATION_ENABLED` | bool | false | true | — | The master switch for the expiry notifications. |
| `notification.interval` | `PROVIANT_NOTIFICATION_INTERVAL` | int | required | 12 | must be greater than zero while notification.enabled is true; the server refuses to start otherwise | How often the notification sweep runs, in hours, and how long a product waits before it is notified again. The number times 24 is the window in days in which the expiring-soon webhook fires. |

## SMTP

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `notification.smtp.host` | `PROVIANT_NOTIFICATION_SMTP_HOST` | string | empty | 127.0.0.1 | a port and a sender address are required once a host is set | The SMTP server the notification mail is sent through. |
| `notification.smtp.port` | `PROVIANT_NOTIFICATION_SMTP_PORT` | int | 0 | 25 | must be greater than zero once a host is set | The port of the SMTP server. |
| `notification.smtp.ssl` | `PROVIANT_NOTIFICATION_SMTP_SSL` | bool | false | false | — | Connects to the SMTP server over TLS. |
| `notification.smtp.user` | `PROVIANT_NOTIFICATION_SMTP_USER` | string | empty | user | — | The SMTP account. |
| `notification.smtp.fromAddress` | `PROVIANT_NOTIFICATION_SMTP_FROMADDRESS` | string | empty | sender@local.net | must not be empty once a host is set | The sender address of the notification mail. |
| `notification.smtp.password` | `PROVIANT_NOTIFICATION_SMTP_PASSWORD` | string | empty | a placeholder — replace it | — | The password of the SMTP account. |

## Ntfy

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `notification.ntfy.url` | `PROVIANT_NOTIFICATION_NTFY_URL` | string | empty | https://ntfy.sh | must start with http:// or https:// | The ntfy server the notifications are published to. |
| `notification.ntfy.topic` | `PROVIANT_NOTIFICATION_NTFY_TOPIC` | string | empty | a placeholder — replace it | must not be empty once a URL is set | The ntfy topic the notifications are published to. Anyone who knows the topic can read them. |
| `notification.ntfy.timeout` | `PROVIANT_NOTIFICATION_NTFY_TIMEOUT` | int | 15 | 60 | a value of zero or less falls back to 15 | The ntfy request timeout, in seconds. |

## Monthly waste report

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `notification.monthlyWasteReport.day` | `PROVIANT_NOTIFICATION_MONTHLYWASTEREPORT_DAY` | int | 1 | 1 | 1 to 28 | The day of the month the monthly waste report is sent. |
| `notification.monthlyWasteReport.hour` | `PROVIANT_NOTIFICATION_MONTHLYWASTEREPORT_HOUR` | int | 8 | 8 | 0 to 23 | The hour of the day the monthly waste report is sent, in the container's time zone. |

## Telegram

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `notification.telegram.timeout` | `PROVIANT_NOTIFICATION_TELEGRAM_TIMEOUT` | int | 15 | 10 | a value of zero or less falls back to 15 | The Telegram API request timeout, in seconds. |
| `notification.telegram.pollerWorkers` | `PROVIANT_NOTIFICATION_TELEGRAM_POLLERWORKERS` | int | 10 | not set | a value of zero or less falls back to 10 | How many goroutines poll Telegram for the users' updates. |

## Mail digest

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `notification.mailDigest.enabled` | `PROVIANT_NOTIFICATION_MAILDIGEST_ENABLED` | bool | true | false | — | The master switch for the per-household expiry digest. A household still has to opt in. |
| `notification.mailDigest.defaultTime` | `PROVIANT_NOTIFICATION_MAILDIGEST_DEFAULTTIME` | string | 08:00 | 08:00 | an empty value falls back to 08:00 | The local time the digest goes out for the households that set no time of their own. |

## Open Food Facts

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `openfoodfacts.url` | `PROVIANT_OPENFOODFACTS_URL` | string | required | https://world.openfoodfacts.org/api/v2/product | must not be empty; the server refuses to start otherwise | The Open Food Facts API the barcode lookups go to. |
| `openfoodfacts.timeout` | `PROVIANT_OPENFOODFACTS_TIMEOUT` | int | required | 5 | must be greater than zero; the server refuses to start otherwise | The request timeout of the barcode lookup, in seconds. |
| `openfoodfacts.cacheEnabled` | `PROVIANT_OPENFOODFACTS_CACHEENABLED` | bool | false | true | — | Caches the barcode lookups in the database, so a scan works without a live request. |
| `openfoodfacts.imageCacheEnabled` | `PROVIANT_OPENFOODFACTS_IMAGECACHEENABLED` | bool | false | false | a cache path is required once it is on | Caches the product images on disk. |
| `openfoodfacts.imageCachePath` | `PROVIANT_OPENFOODFACTS_IMAGECACHEPATH` | string | empty | data/images | — | The directory the product images are cached in. |
| `openfoodfacts.dumpImportEnabled` | `PROVIANT_OPENFOODFACTS_DUMPIMPORTENABLED` | bool | false | false | — | Imports the Open Food Facts bulk dump for an offline barcode lookup. The dump is a multi-gigabyte download. |
| `openfoodfacts.dumpImportExpiryDays` | `PROVIANT_OPENFOODFACTS_DUMPIMPORTEXPIRYDAYS` | int | 7 | 7 | must be greater than zero while the import is on | How often the full dump is downloaded again, in days. |
| `openfoodfacts.dumpUrl` | `PROVIANT_OPENFOODFACTS_DUMPURL` | string | https://static.openfoodfacts.org/data/openfoodfacts-products.jsonl.gz | https://static.openfoodfacts.org/data/openfoodfacts-products.jsonl.gz | must not be empty while the import is on | The URL of the full Open Food Facts dump. |
| `openfoodfacts.dumpDeltaWindowDays` | `PROVIANT_OPENFOODFACTS_DUMPDELTAWINDOWDAYS` | int | 14 | 14 | must not be negative | How many days of delta files are considered between two full downloads. |
| `openfoodfacts.dumpDatabase` | `PROVIANT_OPENFOODFACTS_DUMPDATABASE` | string | off_dump | off_dump | must not be empty on MariaDB while the import is on; the server refuses to start otherwise | The dump store: the MariaDB schema, or data/<name>.db on SQLite. A value that names a path is used verbatim. The schema has to exist before the import is enabled — scripts/mariadb-init.sql creates it. |
| `openfoodfacts.dumpStagingPath` | `PROVIANT_OPENFOODFACTS_DUMPSTAGINGPATH` | string | data | data | — | The directory the full dump is staged in. It makes an interrupted download or import resumable and needs room for the multi-gigabyte download. |

## OCR (expiry date scanning)

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `ocr.enabled` | `PROVIANT_OCR_ENABLED` | bool | false | true | — | Not read by the server. The expiry date scan always runs the local tesseract binary; the key is reserved for the cloud providers, which are not implemented. |
| `ocr.provider` | `PROVIANT_OCR_PROVIDER` | string | empty | tesseract | — | Not read by the server. Only the local tesseract binary runs — the cloud providers (google, openai) are not implemented. |
| `ocr.apiKey` | `PROVIANT_OCR_APIKEY` | string | empty | empty | — | Not read by the server. It belongs to the cloud OCR providers, which are not implemented. |
| `ocr.endpoint` | `PROVIANT_OCR_ENDPOINT` | string | empty | not set | — | Not read by the server. It belongs to a self-hosted OCR server, which is not implemented. |
| `ocr.timeout` | `PROVIANT_OCR_TIMEOUT` | int | none | 10 | — | The request timeout of the OCR call, in seconds. Without a value the request expires at once. |
| `ocr.languages` | `PROVIANT_OCR_LANGUAGES` | string | empty | deu+eng | — | The Tesseract language codes, for example deu+eng. |
| `ocr.tesseractPath` | `PROVIANT_OCR_TESSERACTPATH` | string | empty | not set | — | The path of the tesseract binary. |

## Receipt scanning

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `ocr.receipt.enabled` | `PROVIANT_OCR_RECEIPT_ENABLED` | bool | false | false | — | Enables the receipt scan. |
| `ocr.receipt.provider` | `PROVIANT_OCR_RECEIPT_PROVIDER` | string | openai | openai | must be openai - any OpenAI-compatible vision endpoint - once the receipt scan is on | The vision backend that reads the receipt. |
| `ocr.receipt.apiKey` | `PROVIANT_OCR_RECEIPT_APIKEY` | string | empty | empty | — | The bearer token of the vision endpoint. |
| `ocr.receipt.endpoint` | `PROVIANT_OCR_RECEIPT_ENDPOINT` | string | empty | empty | — | The chat-completions URL. Empty uses the OpenAI default; a base URL gets /v1/chat/completions appended. |
| `ocr.receipt.model` | `PROVIANT_OCR_RECEIPT_MODEL` | string | required while ocr.receipt.enabled is true | empty | must not be empty once the receipt scan is on | The vision model that reads the receipt. |
| `ocr.receipt.timeout` | `PROVIANT_OCR_RECEIPT_TIMEOUT` | int | 120 | 120 | a value of zero or less falls back to 120 | The request timeout of the receipt scan, in seconds. |

## Expiry thresholds

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `expiry.critical_threshold_days` | `PROVIANT_EXPIRY_CRITICAL_THRESHOLD_DAYS` | int | 3 | 3 | a value of zero or less falls back to 3 | How many days before the expiry date a product counts as critical. |
| `expiry.soon_threshold_days` | `PROVIANT_EXPIRY_SOON_THRESHOLD_DAYS` | int | 7 | 7 | a value of zero or less falls back to 7 | How many days before the expiry date a product counts as expiring soon. |

## Recipe suggestions

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `recipe_api.provider` | `PROVIANT_RECIPE_API_PROVIDER` | string | themealdb | themealdb | must not be empty; the server refuses to start otherwise. An unknown name falls back to themealdb. | The recipe source: themealdb, mealie or tandoor. |
| `recipe_api.url` | `PROVIANT_RECIPE_API_URL` | string | https://www.themealdb.com/api/json/v1/1 | https://www.themealdb.com/api/json/v1/1 | must not be empty; the server refuses to start otherwise. A self-hosted provider must not point at the public TheMealDB host. | The base URL of the recipe API. |
| `recipe_api.api_key` | `PROVIANT_RECIPE_API_API_KEY` | string | empty | empty | required for mealie and tandoor | The API key of the self-hosted recipe instance. |
| `recipe_api.timeout` | `PROVIANT_RECIPE_API_TIMEOUT` | int | 10 | 10 | must be greater than zero; the server refuses to start otherwise | The request timeout of the recipe API, in seconds. |
| `recipe_api.cache_enabled` | `PROVIANT_RECIPE_API_CACHE_ENABLED` | bool | true | true | — | Caches the recipe responses. |
| `recipe_api.cache_ttl` | `PROVIANT_RECIPE_API_CACHE_TTL` | int | 24 | 24 | a value of zero or less falls back to 24 | How long a cached recipe response stays valid, in hours. |

## Shopping list sync

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `shopping_list_sync.enabled` | `PROVIANT_SHOPPING_LIST_SYNC_ENABLED` | bool | false | false | — | Synchronises the shopping list with the connected services. |
| `shopping_list_sync.interval_minutes` | `PROVIANT_SHOPPING_LIST_SYNC_INTERVAL_MINUTES` | int | 15 | 15 | a value of zero or less falls back to 15 | How often the list is synchronised, in minutes. |
| `shopping_list_sync.bring_enabled` | `PROVIANT_SHOPPING_LIST_SYNC_BRING_ENABLED` | bool | false | false | — | Enables the Bring! integration. |
| `shopping_list_sync.todoist_enabled` | `PROVIANT_SHOPPING_LIST_SYNC_TODOIST_ENABLED` | bool | false | false | — | Enables the Todoist integration. |

## Scheduled backups

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `backup.enabled` | `PROVIANT_BACKUP_ENABLED` | bool | false | false | — | Runs the scheduled household backup. |
| `backup.intervalHours` | `PROVIANT_BACKUP_INTERVALHOURS` | int | 24 | 24 | must be greater than zero once the backup is on | How often the backup runs, in hours. |
| `backup.directory` | `PROVIANT_BACKUP_DIRECTORY` | string | data/backups | data/backups | must not be empty once the backup is on | The directory the backup documents are written to. They carry household data in cleartext, so it has to be storage the operator controls. |
| `backup.retention` | `PROVIANT_BACKUP_RETENTION` | int | 7 | 7 | must not be negative | How many backup files are kept per household. Zero keeps every file. |

## Restore uploads

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `restore.maxUploadSizeMB` | `PROVIANT_RESTORE_MAXUPLOADSIZEMB` | int | 50 | 50 | a value of zero or less falls back to 50 | The cap for a restore upload, in megabytes. It is separate from server.maxUploadSizeMB because a full household backup is larger than an ordinary request body. |

## Queued offline sync

| Key | Environment variable | Type | Default | Config template | Constraint | Effect |
|---|---|---|---|---|---|---|
| `offlineSync.maxBatchBytes` | `PROVIANT_OFFLINESYNC_MAXBATCHBYTES` | int | 413696 bytes (about 404 KiB, 100 items with headroom) | not set | a value of zero or less falls back to the built-in cap | The cap for one queued-sync request body. The outbox client reads the value from the page and splits its backlog at it. |
