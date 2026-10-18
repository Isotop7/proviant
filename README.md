![Proviant](./res/icons/proviant_hero.png)

![Golang version](https://img.shields.io/badge/Go-1.26.9-green)
![CI status](https://github.com/Isotop7/proviant/actions/workflows/ci.yml/badge.svg?branch=main)
![Release state](https://img.shields.io/github/v/release/Isotop7/proviant)
![Open issues](https://img.shields.io/github/issues/Isotop7/proviant)
![Open pull requests](https://img.shields.io/github/issues-pr/Isotop7/proviant)
![SQLite](https://img.shields.io/badge/SQLite-Supported-green)
![MariaDB](https://img.shields.io/badge/MariaDB-Supported-blue)
![Docker](https://img.shields.io/badge/Docker-Supported-blue)
![Changelog](https://img.shields.io/badge/Changelog-CHANGELOG.md-blue)
![License](https://img.shields.io/badge/License-MIT-green)

[![Vulnerabilities](https://sonarcloud.io/api/project_badges/measure?project=Isotop7_proviant2&metric=vulnerabilities)](https://sonarcloud.io/summary/new_code?id=Isotop7_proviant2)
[![Bugs](https://sonarcloud.io/api/project_badges/measure?project=Isotop7_proviant2&metric=bugs)](https://sonarcloud.io/summary/new_code?id=Isotop7_proviant2)
[![Quality Gate Status](https://sonarcloud.io/api/project_badges/measure?project=Isotop7_proviant2&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=Isotop7_proviant2)

⁉️ Ever wondered if this oat milk is still fresh or when you opened that hummus? In the past you may have thrown it away. Now there is **proviant**

**proviant** is a simple and intuitive application to track your bought products and their expiration date to prevent food waste 🥗

## Why Proviant?

- **Reduce waste** — know exactly when your food expires before it's too late
- **Save time** — barcode scanning and auto-fill from OpenFoodFacts eliminates manual entry
- **Stay informed** — get notified via email, web push, Ntfy, or Telegram before items expire
- **Multi-household** — manage products for multiple households with family and friends

## Preview

<!-- ui-capture-preview:begin — regenerate with `node .agents/skills/ui-capture/scripts/uicapture.mjs --publish` -->

**Dashboard — metric tiles, waste rate donut, category breakdown and expiry trend charts**

![Dashboard — metric tiles, waste rate donut, category breakdown and expiry trend charts](./screenshots/portal.png)

**Products — search, filter and manage your products**

![Products — search, filter and manage your products](./screenshots/search.png)

<!-- ui-capture-preview:end -->

## Features

- 📝 **Product tracking** - track and log products with expiration dates, storage location and quantity, plus optional opened-product shelf life.
- 📷 **Code scan** - scan barcodes via camera to auto-fill data from [OpenFoodFacts](https://world.openfoodfacts.org/), with optional local caching for offline use; batch mode scans several products in a row.
- 📅 **OCR scanning** - extract expiry dates from camera photos (Tesseract/Google/OpenAI) and bulk-import a grocery receipt via an OpenAI-compatible vision model.
- 🗄️ **Archive, consume, or waste products** - soft-delete (archive/consume) to retain history and restore later; hard-delete (waste) to record disposal with webhook events.
- 🔍 **Advanced filtering** - filter by status (All, Expired, Critical, Expiring soon, Fresh, No date, Archived), storage location, or search by product name/barcode.
- 📊 **Dashboard** - live metric tiles (active products, waste rate, waste-free streak, monthly waste goal, savings, archived counts, last added product, products expiring within 7 days), charts (waste rate donut, category breakdown pie, 12-month expiry trend line) and a recent-activity feed.
- 📈 **Waste analytics** - consumed vs wasted breakdown, monthly trends and the cost of waste.
- 🛒 **Shopping list** - shared household list that can import low-stock items.
- 🍽️ **Recipe suggestions** - recipe ideas based on the products you have (TheMealDB, or a self-hosted [Mealie](https://mealie.io/) / [Tandoor](https://tandoor.dev/) instance).
- ⏰ **Notifications** - expiration reminders, monthly waste reports and streak updates via email (SMTP), web push, [Ntfy](https://ntfy.sh/) or a [Telegram](https://telegram.org/) bot, with per-user preferences and an in-app notification centre.
- 📆 **Calendar feed** - subscribe to household expiry dates from any calendar app via an iCal (`.ics`) feed.
- 👥 **Combined Household** - multi-user households with invitations, roles and household-scoped data.
- 📥 **Export & import** - download products and archive data as CSV or JSON, import products from a CSV template, and restore a full household backup from JSON.
- 🌙 **Theming** - light and dark mode.

## Quick Start

```bash
curl -o docker-compose.yml https://raw.githubusercontent.com/Isotop7/proviant/main/docker-compose.sqlite.yaml
echo "PROVIANT_JWT_SECRET=$(openssl rand -hex 32)" > .env
docker compose up -d
```

Then open http://localhost:5114

## Demo

[Access the demo](https://proviant-demo.hr94.de)

| Username | Password | Email Verified |
|----------|----------|----------------|
| `demo` | `demo` | Yes |
| `alice` | `AliceDemo123!` | Yes |
| `bob` | `BobDemo123!` | Yes |

## Technologies and Tools

- **Web:** [Go](https://go.dev/), [Gin](https://gin-gonic.com/), [Gorm](https://gorm.io/index.html), [Bootstrap](https://getbootstrap.com/)
- **Charts:** [Chart.js](https://www.chartjs.org/)
- **Database:** MariaDB or SQLite
- **Notifications:** SMTP, Web Push, [Ntfy](https://ntfy.sh/), [Telegram](https://telegram.org/)
- **Scanning:** Barcodes via [gozxing](https://github.com/makiuchi-d/gozxing); expiry dates via [Tesseract](https://tesseract-ocr.github.io/)/Google/OpenAI; receipts via any OpenAI-compatible vision model
- **Recipes:** [TheMealDB](https://www.themealdb.com/) or a self-hosted [Mealie](https://mealie.io/) / [Tandoor](https://tandoor.dev/) instance

## Requirements

- **Go:** 1.26+
- **Node.js:** 24+ (for asset build)
- **Database:** SQLite 3 or MariaDB 10.5+

## Installation and Usage

### Backend

#### Executable

1. Clone the repository: `git clone https://github.com/Isotop7/proviant.git`
2. Setup node_modules and assets (needs [Taskfile](https://taskfile.dev/)): `task init`
3. Navigate to the project directory: `cd proviant/src`
4. Build the application: `go build -o proviant`
5. Copy the desired config file `config.yaml.[mariadb|sqlite].tmpl`, rename it to `config.yaml` and adjust it — `tokenPassword` ships empty and must be set (`openssl rand -hex 32`), the server refuses to start without it or with the old public placeholder `secret key`
6. Run the server: `./proviant`

#### Docker

`proviant` can be started with `docker compose`

- [External MariaDB database](./docker-compose.mariadb.yaml)
- [Internal SQLite database](./docker-compose.sqlite.yaml)

Both compose files require `PROVIANT_JWT_SECRET` in a `.env` next to them — compose
refuses to start without it:

```bash
echo "PROVIANT_JWT_SECRET=$(openssl rand -hex 32)" > .env
```

**IMPORTANT:** The mounted directories need to be chowned by the proviant app user.

```bash
chown -R 100:101 <path>
```

##### Prebuilt images

Official images are published to GitHub Container Registry on every release. Running one needs no
repository checkout — unlike the compose files shipped in this repo, which always build from source
(`build: ./`) and never reference the published image:

```bash
docker pull ghcr.io/isotop7/proviant:latest
docker run --rm -p 5114:5114 \
  -e PROVIANT_SERVER_AUTHENTICATION_TOKENPASSWORD="$(openssl rand -hex 32)" \
  ghcr.io/isotop7/proviant:latest
```

| Tag | Description |
|---|---|
| `vX.Y.Z` | Exact release version |
| `X.Y.Z` | Same image as `vX.Y.Z`, without the `v` prefix |
| `latest` | Newest final release (never a pre-release) |
| `vX.Y.Z-demo` / `X.Y.Z-demo` | Matching release with demo data |
| `main` / `develop` | Unreleased builds from the respective branch, may be broken |
| `demo` | Demo image, rebuilt on every `main`/`develop` merge |

**IMPORTANT:**

- Images are built for `linux/amd64` only. On native ARM hosts (e.g. Raspberry Pi) the pull fails
  with `no matching manifest for linux/arm64/v8` — build from source there. Runtimes with emulation
  (Docker Desktop on Apple Silicon) can pull it with `docker run --platform linux/amd64 …`.
- A version tag is never reused for a different release, but re-running a release workflow
  re-pushes it. For a bit-for-bit reproducible deployment, pin the digest printed in that release's
  notes (`ghcr.io/isotop7/proviant@sha256:…`) rather than the tag.

### Configuration

Configuration is set via environment variables. The complete list of available options and their default values is below. Template configuration files are available in `src/config.yaml.sqlite.tmpl` and `src/config.yaml.mariadb.tmpl`.

#### Server

| Environment Variable | Default | Description |
|---|---|---|
| `PROVIANT_SERVER_PORT` | `5114` | Listening port |
| `PROVIANT_SERVER_HOST` | *(empty)* | Bind address; empty binds every interface, set `127.0.0.1` for a loopback-only listener |
| `PROVIANT_SERVER_BASEURL` | `https://proviant.local.de` | URL of Proviant with protocol (used in emails, notifications) |
| `PROVIANT_SERVER_MAXUPLOADSIZEMB` | `5` | Maximum upload size in MB (receipt photos, CSV imports) |
| `PROVIANT_SERVER_DEBUG` | `false` | Run Gin in debug mode |
| `PROVIANT_SERVER_DEMOMODE` | `false` | Enable demo mode (read-only demo behaviour) |
| `PROVIANT_SERVER_METRICSENABLED` | `false` | Expose the unauthenticated Prometheus `/metrics` endpoint |
| `PROVIANT_SERVER_TRUSTEDPROXIES` | *(empty)* | Comma-separated list of trusted reverse-proxy CIDRs for `X-Forwarded-For`/`X-Real-IP`; empty ignores those headers and uses the connection address |
| **Authentication** |||
| `PROVIANT_SERVER_AUTHENTICATION_TOKENPASSWORD` | *(required — no default)* | Secret used for JSON Web Tokens. Generate one with `openssl rand -hex 32`. Startup fails if empty or if it is the old public placeholder `secret key` — upgrading installs still carrying that value must rotate. |
| `PROVIANT_SERVER_AUTHENTICATION_TOKENLIFETIME` | `8` | Lifetime of JSON Web Tokens in hours |
| `PROVIANT_SERVER_AUTHENTICATION_MAXLOGINATTEMPTS` | `3` | Maximum failed login attempts before lockout |
| `PROVIANT_SERVER_AUTHENTICATION_LOCKOUTDURATIONMINS` | `10` | Lockout duration in minutes after max failed attempts |
| `PROVIANT_SERVER_AUTHENTICATION_PASSWORDMINLENGTH` | `12` | Minimum password length |
| `PROVIANT_SERVER_AUTHENTICATION_PASSWORDREQUIREUPPERCASE` | `false` | Require at least one uppercase letter |
| `PROVIANT_SERVER_AUTHENTICATION_PASSWORDREQUIREDIGIT` | `false` | Require at least one digit |
| `PROVIANT_SERVER_AUTHENTICATION_PASSWORDREQUIRESPECIAL` | `false` | Require at least one special character |
| `PROVIANT_SERVER_AUTHENTICATION_PASSWORDCHECKBREACHED` | `true` | Check passwords against HaveIBeenPwned API |
| `PROVIANT_SERVER_AUTHENTICATION_SKIPEMAILVERIFICATION` | `false` | Skip the email-verification requirement for new accounts |
| **CORS** |||
| `PROVIANT_SERVER_CORS_ALLOWALLORIGINS` | `false` | Allow all origins (`true`) or use `allowedOrigins` list (`false`) |
| `PROVIANT_SERVER_CORS_ALLOWEDORIGINS` | `http://localhost,http://myproviant.instance` | Comma-separated list of allowed origins when `allowAllOrigins` is `false` |
| **Rate limiting (requests per minute, per client)** |||
| `PROVIANT_SERVER_RATELIMIT_LOGIN_PER_MINUTE` | `5` | Login attempts |
| `PROVIANT_SERVER_RATELIMIT_SIGNUP_PER_MINUTE` | `3` | Sign-ups |
| `PROVIANT_SERVER_RATELIMIT_EXPORT_PER_MINUTE` | `1` | Data exports |
| `PROVIANT_SERVER_RATELIMIT_PASSWORD_PER_MINUTE` | `3` | Password change / reset requests |
| `PROVIANT_SERVER_RATELIMIT_SCAN_PER_MINUTE` | `10` | Barcode and OCR scans |
| `PROVIANT_SERVER_RATELIMIT_RECIPES_PER_MINUTE` | `6` | Recipe suggestion requests |
| **Security headers** |||
| `PROVIANT_SERVER_SECURITYHEADERS_CONTENT_SECURITY_POLICY` | `default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:;` | Content Security Policy header |
| `PROVIANT_SERVER_SECURITYHEADERS_CSRF_TOKEN_MAX_AGE` | `86400` | CSRF token lifetime in seconds |

#### Calendar feed

| Environment Variable | Default | Description |
|---|---|---|
| `PROVIANT_CALENDAR_TOKENEXPIRYDAYS` | `365` | Days until a calendar subscription token expires |
| `PROVIANT_CALENDAR_EXPIRINGSOONDAYS` | `30` | Days before expiry to include in the calendar feed |

#### Database

| Environment Variable | Default | Description |
|---|---|---|
| `PROVIANT_DATABASE_ENGINE` | `sqlite` | Database engine (`sqlite` or `mariadb`) |
| **SQLite** |||
| `PROVIANT_DATABASE_SQLITE_FILEPATH` | `data/proviant.db` | Path to SQLite database file (folder must exist) |
| **MariaDB** |||
| `PROVIANT_DATABASE_MARIADB_HOST` | `127.0.0.1` | MariaDB server IP or hostname |
| `PROVIANT_DATABASE_MARIADB_PORT` | `3306` | MariaDB server port |
| `PROVIANT_DATABASE_MARIADB_NAME` | `proviant` | Database name |
| `PROVIANT_DATABASE_MARIADB_USER` | `root` (SQLite: `proviant`) | Database user |
| `PROVIANT_DATABASE_MARIADB_PASSWORD` | `password` | Database user password |

#### Logging

| Environment Variable | Default | Description |
|---|---|---|
| `PROVIANT_LOGGING_ENABLED` | `true` | Enable file logging |
| `PROVIANT_LOGGING_FILE` | `proviant.log` | Path to log file |

#### Notifications

| Environment Variable | Default | Description |
|---|---|---|
| `PROVIANT_NOTIFICATION_ENABLED` | `true` | Enable notifications |
| `PROVIANT_NOTIFICATION_INTERVAL` | `12` | Interval in hours when notifications should be sent |
| `PROVIANT_NOTIFICATION_SMTP_HOST` | `127.0.0.1` | SMTP server host or IP address |
| `PROVIANT_NOTIFICATION_SMTP_PORT` | `25` | SMTP server port |
| `PROVIANT_NOTIFICATION_SMTP_SSL` | `false` | Enable/disable SSL for SMTP |
| `PROVIANT_NOTIFICATION_SMTP_USER` | `user` | SMTP username |
| `PROVIANT_NOTIFICATION_SMTP_PASSWORD` | `password` | SMTP password |
| `PROVIANT_NOTIFICATION_SMTP_FROMADDRESS` | `sender@local.net` | Sender email address for notifications |
| `PROVIANT_NOTIFICATION_NTFY_URL` | `https://ntfy.sh` | Ntfy server URL |
| `PROVIANT_NOTIFICATION_NTFY_TOPIC` | `default_topic` | Default Ntfy topic/channel |
| `PROVIANT_NOTIFICATION_NTFY_TIMEOUT` | `60` | Ntfy message timeout in seconds |
| `PROVIANT_NOTIFICATION_TELEGRAM_TIMEOUT` | `10` | Telegram API request timeout in seconds (15 when unset) |
| `PROVIANT_NOTIFICATION_TELEGRAM_POLLERWORKERS` | `10` | Worker goroutines used to poll Telegram for all users |
| `PROVIANT_NOTIFICATION_MONTHLYWASTEREPORT_DAY` | `1` | Day of month to send monthly waste report (1–28) |
| `PROVIANT_NOTIFICATION_MONTHLYWASTEREPORT_HOUR` | `8` | UTC hour to send monthly waste report (0–23) |
| `PROVIANT_NOTIFICATION_MAILDIGEST_ENABLED` | `false` | Enable the per-household expiry digest email |
| `PROVIANT_NOTIFICATION_MAILDIGEST_DEFAULTTIME` | `08:00` | Default local send time for households that have not set their own |

Web push subscriptions need no configuration: VAPID keys are generated automatically on first run and stored in the database.

#### OpenFoodFacts

| Environment Variable | Default | Description |
|---|---|---|
| `PROVIANT_OPENFOODFACTS_URL` | `https://world.openfoodfacts.org/api/v2/product` | OpenFoodFacts API URL |
| `PROVIANT_OPENFOODFACTS_TIMEOUT` | `5` | API request timeout in seconds |
| `PROVIANT_OPENFOODFACTS_CACHEENABLED` | `true` | Cache API responses in database for offline use |
| `PROVIANT_OPENFOODFACTS_IMAGECACHEENABLED` | `false` | Cache product images locally |
| `PROVIANT_OPENFOODFACTS_IMAGECACHEPATH` | `data/images` | Path to image cache directory |

#### OCR (expiry date scanning)

| Environment Variable | Default | Description |
|---|---|---|
| `PROVIANT_OCR_ENABLED` | `true` | Enable OCR expiry date scanning |
| `PROVIANT_OCR_PROVIDER` | `tesseract` | OCR provider: `tesseract` (local), `google`, `openai` |
| `PROVIANT_OCR_API_KEY` | *(empty)* | API key for cloud providers (Google/OpenAI) |
| `PROVIANT_OCR_ENDPOINT` | *(empty)* | Custom OCR server endpoint (for local Tesseract HTTP) |
| `PROVIANT_OCR_TIMEOUT` | `10` | OCR request timeout in seconds |
| `PROVIANT_OCR_LANGUAGES` | `deu+eng` | Tesseract language codes (e.g., `deu+eng`) |
| `PROVIANT_OCR_TESSERACTPATH` | `/usr/bin/tesseract` | Absolute path to the Tesseract binary |

#### Receipt scanning

Receipt scanning extracts line items from a receipt photo using a vision-capable, OpenAI-compatible chat-completions endpoint. It is off by default and a `model` is required when enabled.

| Environment Variable | Default | Description |
|---|---|---|
| `PROVIANT_OCR_RECEIPT_ENABLED` | `false` | Enable receipt photo scanning |
| `PROVIANT_OCR_RECEIPT_PROVIDER` | `openai` | Provider (any OpenAI-compatible vision endpoint) |
| `PROVIANT_OCR_RECEIPT_APIKEY` | *(empty)* | Bearer token for the endpoint |
| `PROVIANT_OCR_RECEIPT_ENDPOINT` | *(empty)* | Chat-completions URL; empty uses the OpenAI default |
| `PROVIANT_OCR_RECEIPT_MODEL` | *(empty — required when enabled)* | Vision model name |
| `PROVIANT_OCR_RECEIPT_TIMEOUT` | `120` | Request timeout in seconds |

#### Recipe suggestions

| Environment Variable | Default | Description |
|---|---|---|
| `PROVIANT_RECIPE_API_PROVIDER` | `themealdb` | Recipe provider: `themealdb`, `mealie` or `tandoor` |
| `PROVIANT_RECIPE_API_URL` | `https://www.themealdb.com/api/json/v1/1` | Recipe API endpoint (base URL, `/search.php` is appended automatically) |
| `PROVIANT_RECIPE_API_API_KEY` | *(empty)* | API key (required for the self-hosted `mealie` and `tandoor` providers) |
| `PROVIANT_RECIPE_API_TIMEOUT` | `10` | Recipe API request timeout in seconds |
| `PROVIANT_RECIPE_API_CACHEENABLED` | `true` | Cache recipe responses |
| `PROVIANT_RECIPE_API_CACHETTL` | `24` | Cache TTL in hours |

For a self-hosted provider, set `PROVIANT_RECIPE_API_PROVIDER`, `PROVIANT_RECIPE_API_URL` and `PROVIANT_RECIPE_API_API_KEY` together, and make sure the URL points at your instance — pointing a self-hosted provider at the public TheMealDB URL would send your API key and product names to a third party. A self-hosted provider without a key disables recipe suggestions at startup instead of falling back.

#### Expiry thresholds

| Environment Variable | Default | Description |
|---|---|---|
| `PROVIANT_EXPIRY_CRITICAL_THRESHOLD_DAYS` | `3` | Days before expiry to mark as critical |
| `PROVIANT_EXPIRY_SOON_THRESHOLD_DAYS` | `7` | Days before expiry to mark as expiring soon |

#### Scheduled backups

The scheduled backup job writes one full household JSON backup per household to a local directory. Backup files contain household data in cleartext; point the directory at storage you control and restrict who can read it.

| Environment Variable | Default | Description |
|---|---|---|
| `PROVIANT_BACKUP_ENABLED` | `false` | Enable the scheduled household backup job |
| `PROVIANT_BACKUP_INTERVALHOURS` | `24` | Hours between backup runs |
| `PROVIANT_BACKUP_DIRECTORY` | `data/backups` | Directory the backup documents are written to |
| `PROVIANT_BACKUP_RETENTION` | `7` | Backup files kept per household; `0` keeps every file |

#### Restore uploads

| Environment Variable | Default | Description |
|---|---|---|
| `PROVIANT_RESTORE_MAXUPLOADSIZEMB` | `50` | Maximum restore upload size in MB |

Gin's debug mode is controlled by `PROVIANT_SERVER_DEBUG` (`GIN_MODE` is overridden at startup).

## Upgrade / Migration

### Docker

When running the prebuilt image, replace the container with the version you want to upgrade to:

```bash
docker pull ghcr.io/isotop7/proviant:vX.Y.Z
docker rm -f proviant
docker run -d --name proviant --restart always -p 5114:5114 \
  -e PROVIANT_DATABASE_ENGINE=sqlite \
  -e PROVIANT_LOGGING_FILE=logs/proviant.log \
  -v proviantdb:/app/data \
  -v proviantlogs:/app/logs \
  ghcr.io/isotop7/proviant:vX.Y.Z
```

The container runs as the unprivileged proviant user, so the mounted volumes must be writable by
`100:101` (`docker run --rm -v proviantdb:/data alpine chown -R 100:101 /data`).

`:latest` always points at the newest final release. The compose files in this repository build from
source (`build: ./`), so `docker compose pull` is a no-op for them — upgrade those with
`docker compose build --pull && docker compose up -d`, or switch to the prebuilt image as above.

Database migrations run automatically on startup. See the [CHANGELOG](./CHANGELOG.md) for breaking changes that may require manual intervention.

Upgrades also rotate two things on first start: existing `audit_logs` rows are attributed
to the household their actor belongs to *now* (a member who changed households before the
upgrade will see their older entries under their current household), and a `tokenPassword`
still set to the old public placeholder `secret key` aborts startup until you generate a
new one (`openssl rand -hex 32`).

### Manual / binary

Replace the `proviant` binary with the new version and restart the service. Migrations are applied automatically on startup. Check the [CHANGELOG](./CHANGELOG.md) for any breaking changes or additional steps.

## AI disclaimer

I've been building `proviant` since 2024 with just basic knowledge of `Go` and I am **not a professional software developer**. Starting this year, I used AI more intensively to speed up development and help with the UI design. While I carefully checked all changes, you might want to **not use** Proviant, if this is a problem for you. I won't judge you, if you don't judge me ✌🏻

## Commit Conventions

This project uses [Conventional Commits](https://www.conventionalcommits.org/). See [CONTRIBUTING.md](./CONTRIBUTING.md) for the full prefix reference. The changelog is auto-generated by [git-cliff](https://git-cliff.org/).

## Documentation

Documentation is generated with `gomarkdoc` and `swagger`:

- [Package documentation](./src/docs/README.md)
- [Swagger definition](./src/docs/swagger.yaml)

## Screenshots

<!-- ui-capture:begin — regenerate with `node .agents/skills/ui-capture/scripts/uicapture.mjs --publish` -->

<table>
  <tr>
    <th>Desktop</th>
    <th>Mobile</th>
  </tr>
  <tr>
    <td width="50%"><img src="./screenshots/portal.png" alt="Dashboard — metric tiles, waste rate, category breakdown and expiry trend charts — desktop"></td>
    <td width="25%"><img src="./screenshots/portal_mobile.png" alt="Dashboard — metric tiles, waste rate, category breakdown and expiry trend charts — mobile"></td>
  </tr>
</table>

<table>
  <tr>
    <th>Desktop</th>
    <th>Mobile</th>
  </tr>
  <tr>
    <td width="50%"><img src="./screenshots/search.png" alt="Products — search, filter, adjust and manage what is in your household — desktop"></td>
    <td width="25%"><img src="./screenshots/search_mobile.png" alt="Products — search, filter, adjust and manage what is in your household — mobile"></td>
  </tr>
</table>

<table>
  <tr>
    <th>Desktop</th>
    <th>Mobile</th>
  </tr>
  <tr>
    <td width="50%"><img src="./screenshots/products-expired.png" alt="Products (expired) — filter view of past-expiry items — desktop"></td>
    <td width="25%"><img src="./screenshots/products-expired_mobile.png" alt="Products (expired) — filter view of past-expiry items — mobile"></td>
  </tr>
</table>

<table>
  <tr>
    <th>Desktop</th>
    <th>Mobile</th>
  </tr>
  <tr>
    <td width="50%"><img src="./screenshots/products-consumed.png" alt="Products (consumed) — filter view of archived items — desktop"></td>
    <td width="25%"><img src="./screenshots/products-consumed_mobile.png" alt="Products (consumed) — filter view of archived items — mobile"></td>
  </tr>
</table>

<table>
  <tr>
    <th>Desktop</th>
    <th>Mobile</th>
  </tr>
  <tr>
    <td width="50%"><img src="./screenshots/products-grid.png" alt="Products (grid) — grid view of your inventory — desktop"></td>
    <td width="25%"><img src="./screenshots/products-grid_mobile.png" alt="Products (grid) — grid view of your inventory — mobile"></td>
  </tr>
</table>

<table>
  <tr>
    <th>Desktop</th>
    <th>Mobile</th>
  </tr>
  <tr>
    <td width="50%"><img src="./screenshots/product-edit.png" alt="Product edit — adjust expiry, stock, and details — desktop"></td>
    <td width="25%"><img src="./screenshots/product-edit_mobile.png" alt="Product edit — adjust expiry, stock, and details — mobile"></td>
  </tr>
</table>

<table>
  <tr>
    <th>Desktop</th>
    <th>Mobile</th>
  </tr>
  <tr>
    <td width="50%"><img src="./screenshots/create.png" alt="Add product — barcode scan with auto-fill from OpenFoodFacts — desktop"></td>
    <td width="25%"><img src="./screenshots/create_mobile.png" alt="Add product — barcode scan with auto-fill from OpenFoodFacts — mobile"></td>
  </tr>
</table>

<table>
  <tr>
    <th>Desktop</th>
    <th>Mobile</th>
  </tr>
  <tr>
    <td width="50%"><img src="./screenshots/receipt.png" alt="Receipt scan — OCR bulk entry from a grocery receipt photo — desktop"></td>
    <td width="25%"><img src="./screenshots/receipt_mobile.png" alt="Receipt scan — OCR bulk entry from a grocery receipt photo — mobile"></td>
  </tr>
</table>

<table>
  <tr>
    <th>Desktop</th>
    <th>Mobile</th>
  </tr>
  <tr>
    <td width="50%"><img src="./screenshots/recipe.png" alt="Recipes — suggestions built from the products you already have — desktop"></td>
    <td width="25%"><img src="./screenshots/recipe_mobile.png" alt="Recipes — suggestions built from the products you already have — mobile"></td>
  </tr>
</table>

<table>
  <tr>
    <th>Desktop</th>
    <th>Mobile</th>
  </tr>
  <tr>
    <td width="50%"><img src="./screenshots/mealplan.png" alt="Meal plan — monthly calendar for planning saved recipes — desktop"></td>
    <td width="25%"><img src="./screenshots/mealplan_mobile.png" alt="Meal plan — monthly calendar for planning saved recipes — mobile"></td>
  </tr>
</table>

<table>
  <tr>
    <th>Desktop</th>
    <th>Mobile</th>
  </tr>
  <tr>
    <td width="50%"><img src="./screenshots/waste.png" alt="Waste analytics — consumed vs wasted, monthly breakdown and cost of waste — desktop"></td>
    <td width="25%"><img src="./screenshots/waste_mobile.png" alt="Waste analytics — consumed vs wasted, monthly breakdown and cost of waste — mobile"></td>
  </tr>
</table>

<table>
  <tr>
    <th>Desktop</th>
    <th>Mobile</th>
  </tr>
  <tr>
    <td width="50%"><img src="./screenshots/shopping.png" alt="Shopping list — shared household list with low-stock import — desktop"></td>
    <td width="25%"><img src="./screenshots/shopping_mobile.png" alt="Shopping list — shared household list with low-stock import — mobile"></td>
  </tr>
</table>

<table>
  <tr>
    <th>Desktop</th>
    <th>Mobile</th>
  </tr>
  <tr>
    <td width="50%"><img src="./screenshots/onboarding.png" alt="Onboarding — profile, notifications and household setup — desktop"></td>
    <td width="25%"><img src="./screenshots/onboarding_mobile.png" alt="Onboarding — profile, notifications and household setup — mobile"></td>
  </tr>
</table>

<table>
  <tr>
    <th>Desktop</th>
    <th>Mobile</th>
  </tr>
  <tr>
    <td width="50%"><img src="./screenshots/settings.png" alt="Settings — notifications, household, tokens and security — desktop"></td>
    <td width="25%"><img src="./screenshots/settings_mobile.png" alt="Settings — notifications, household, tokens and security — mobile"></td>
  </tr>
</table>

<table>
  <tr>
    <th>Desktop</th>
    <th>Mobile</th>
  </tr>
  <tr>
    <td width="50%"><img src="./screenshots/user.png" alt="User — profile and household — desktop"></td>
    <td width="25%"><img src="./screenshots/user_mobile.png" alt="User — profile and household — mobile"></td>
  </tr>
</table>

<table>
  <tr>
    <th>Desktop</th>
    <th>Mobile</th>
  </tr>
  <tr>
    <td width="50%"><img src="./screenshots/not-found.png" alt="Not found — error page — desktop"></td>
    <td width="25%"><img src="./screenshots/not-found_mobile.png" alt="Not found — error page — mobile"></td>
  </tr>
</table>

**Sign in**

![Sign in](./screenshots/login.png)

**Forgot password**

![Forgot password](./screenshots/forgot-password.png)

**Reset password**

![Reset password](./screenshots/reset-password.png)

**Accept invite**

![Accept invite](./screenshots/invite-accept.png)

**Verify email**

![Verify email](./screenshots/verify-email.png)

**Unsubscribe**

![Unsubscribe](./screenshots/unsubscribe.png)

<!-- ui-capture:end -->

## Security Scanning

The CI pipeline runs on every push, and on pull requests targeting `main` or `develop`. Dependency scanning gates every run; container-image scanning gates pushes to `main`/`develop` and same-repo pull requests:

| Scan | Tool | Scope |
| --- | --- | --- |
| Go dependencies | `govulncheck` | `task vuln-go` / CI `check` job |
| npm dependencies | `npm audit` (production gate) | `task vuln-npm` / CI `frontend-check` job |
| Container image | `trivy` (prod image) | `task vuln-image` / CI `image-scan` job + release gate |

Image scanning blocks on fixable `HIGH`/`CRITICAL` findings. Scan flags and the digest-pinned trivy image live in `scripts/image-scan.sh` — the single source of truth shared by CI and `task vuln-image`. The task uses the pinned container image when no local `trivy` binary is found.

The image scan publishes the scanned image to a `ci-scan-<sha>` tag and the release step promotes that exact digest to the published tags, so the report always covers the image that ships. A scan that fails to complete (unreachable vulnerability database, registry error) is reported as an error rather than as a clean result — the gate never passes because the scanner broke. Pull requests from forks do not run the image scan (their CI token is read-only); the gate is enforced through branch protection on `main` and `develop`.

## Monitoring

The server exposes two unauthenticated probes and an optional Prometheus endpoint:

| Endpoint | Description |
| --- | --- |
| `GET /health` | Liveness — `200` while the process is running |
| `GET /health/ready` | Readiness — `200` when the database answers a ping, `503` otherwise |
| `GET /metrics` | Prometheus metrics; registered only when `PROVIANT_SERVER_METRICSENABLED=true` |

The `docker-compose.sqlite.yaml` health check targets `/health/ready`.

## Contributing

Contributions are welcome! Fork the repository, make your changes, and submit a pull request.

## License

This project is licensed under the [MIT License](LICENSE).

## Author

- GitHub: [@Isotop7](https://github.com/Isotop7)
- LinkedIn: [Hendrik Röder](https://www.linkedin.com/in/hendrik-r%C3%B6der-9b8483198/)

---
