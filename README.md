![Proviant](./res/icons/proviant_hero.png)

![Golang version](https://img.shields.io/badge/Go-1.26.10-green)
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
- 📅 **OCR scanning** - extract expiry dates from camera photos with the local Tesseract binary, and bulk-import a grocery receipt via an OpenAI-compatible vision model.
- 🗄️ **Archive, consume, or waste products** - soft-delete (archive/consume) to retain history and restore later; hard-delete (waste) to record disposal with webhook events.
- 🔍 **Advanced filtering** - filter by status (All, Expired, Critical, Expiring soon, Fresh, No date, Archived), storage location, or search by product name/barcode.
- 📊 **Dashboard** - live metric tiles (active products, waste rate, waste-free streak, monthly waste goal, savings, archived counts, last added product, products expiring within 7 days), charts (waste rate donut, category breakdown pie, 12-month expiry trend line) and a recent-activity feed.
- 📈 **Waste analytics** - consumed vs wasted breakdown, monthly trends and the cost of waste.
- 🛒 **Shopping list** - shared household list that can import low-stock items.
- 🍽️ **Recipe suggestions** - recipe ideas based on the products you have (TheMealDB, or a self-hosted [Mealie](https://mealie.io/) / [Tandoor](https://tandoor.dev/) instance).
- ⏰ **Notifications** - expiration reminders, monthly waste reports and streak updates via email (SMTP), web push, [Ntfy](https://ntfy.sh/) or a [Telegram](https://telegram.org/) bot, with per-user preferences and an in-app notification centre.
- 📆 **Calendar feed** - subscribe to household expiry dates from any calendar app via an iCal (`.ics`) feed.
- 👥 **Combined Household** - multi-user households with invitations, roles and household-scoped data.
- 🔐 **Two-factor authentication** - enrol an authenticator app (TOTP) per account, with single-use recovery codes for a lost device. A household can require a second factor from every member; the deployment switch keeps the feature off until an admin sets an encryption key.
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
- **Design:** the design system in [`design-system/`](./design-system/) — crème and sage, Jost and DMMono, one source of truth for every token
- **Charts:** [Chart.js](https://www.chartjs.org/)
- **Database:** MariaDB or SQLite
- **Notifications:** SMTP, Web Push, [Ntfy](https://ntfy.sh/), [Telegram](https://telegram.org/)
- **Scanning:** Barcodes via [gozxing](https://github.com/makiuchi-d/gozxing); expiry dates via the local [Tesseract](https://tesseract-ocr.github.io/) binary; receipts via any OpenAI-compatible vision model
- **Recipes:** [TheMealDB](https://www.themealdb.com/) or a self-hosted [Mealie](https://mealie.io/) / [Tandoor](https://tandoor.dev/) instance

## Requirements

- **Go:** 1.26+
- **Node.js:** 24.15+ — the 24 series (for asset build). `npm` enforces the range, `.nvmrc` pins the major for nvm/fnm/asdf
- **Database:** SQLite 3 or MariaDB 10.5+

## Installation and Usage

### Backend

#### Executable

1. Clone the repository: `git clone https://github.com/Isotop7/proviant.git`
2. Setup node_modules and assets (needs [Taskfile](https://taskfile.dev/)): `task init`
3. Navigate to the project directory: `cd proviant/src`
4. Build the application: `go build -o proviant`
5. Copy the desired config file `config.yaml.[mariadb|sqlite].tmpl`, rename it to `config.yaml` and adjust it — `tokenPassword` ships empty and must be set (`openssl rand -hex 32`), the server refuses to start without it or with the placeholder the retired templates shipped
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

The MariaDB compose file falls back to dev-only database credentials. Set both variables for any
deployment others can reach — the defaults are `localdev-root` for the root account and `localdev`
for the `proviant` user, and they are published in this repository: anyone who reaches the compose
network, the host and every container of the project included, can use them. The MariaDB image
applies `MARIADB_*` to an empty data directory only, so the root password below only ever applies
to a fresh volume; the app user's password is handed to the app on every start:

```bash
echo "PROVIANT_MARIADB_ROOT_PASSWORD=$(openssl rand -hex 16)" >> .env
echo "PROVIANT_MARIADB_PASSWORD=$(openssl rand -hex 16)" >> .env
```

An emptied variable is not treated as unset: the database then refuses to initialize (root
password) or starts without that user (app password), and the app fails to authenticate.

`PROVIANT_MARIADB_ROOT_PASSWORD` is the MariaDB root account, `PROVIANT_MARIADB_PASSWORD`
the `proviant` user the app connects with — both are compose-level names, the app itself reads
`PROVIANT_DATABASE_MARIADB_PASSWORD`. The server warns at startup when the password it
authenticates with is a published placeholder, but it never sees the root password: that one only
you can check.

For the MariaDB file, start the stack:

```bash
docker compose -f docker-compose.mariadb.yaml up -d
```

The compose file mounts `src/config.yaml.mariadb.tmpl` as the container's config: the image ships
the SQLite template, which has no MariaDB block, so the mounted file is what puts the instance on
MariaDB.

`PROVIANT_SERVER_BASEURL` is the public URL of the deployment — it decides the `Secure` flag of the
session and CSRF cookies. The compose file defaults it to `http://localhost:5114`, which drops that
flag — right for the plain-HTTP local deployment, wrong for one behind a proxy. Set it to the `https`
URL of your reverse proxy when one terminates TLS in front of the app.

The root password is needed for the manual
`scripts/mariadb-init.sql` run described under [Database](#database) — the app user
itself may not grant privileges. An existing volume keeps the password it was created
with, so the root password in `.env` only ever applies to a fresh volume. To change the
app user's password, write the new value to `PROVIANT_MARIADB_PASSWORD` in `.env` (the compose
file's name for it — the app-level `PROVIANT_DATABASE_MARIADB_PASSWORD` is overridden by the
compose file), set the same value in the database
(as root, like the init run below: `ALTER USER 'proviant'@'%' IDENTIFIED BY '…'`), then
recreate the app container so it picks the new value up (`docker compose -f
docker-compose.mariadb.yaml up -d proviant` — `restart` keeps the old environment).
Recreating the volume (`docker compose -f docker-compose.mariadb.yaml down -v`) also
works: it removes the containers, the project network and the `proviantdb`,
`proviantlogs` and `smtpdb` volumes, so the stack has to come back up with `up -d`.

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

Proviant reads `config.yaml` from the working directory of the process and takes `PROVIANT_*`
environment variables over it. The variable of a key is its path with the dots replaced by
underscores, upper-cased and prefixed: `server.authentication.tokenPassword` is
`PROVIANT_SERVER_AUTHENTICATION_TOKENPASSWORD`.

**[The configuration reference](./docs/configuration.md)** lists every key with its type, the value
it resolves to when the file leaves it out, the constraint the validator enforces and the value the
shipped templates set. It is generated from the configuration itself, so it cannot drift.

The shipped templates are `src/config.yaml.sqlite.tmpl` and `src/config.yaml.mariadb.tmpl`. Copy the
one that matches your database to `config.yaml` next to the binary and set
`server.authentication.tokenPassword` — the server refuses to start without it, and
`openssl rand -hex 32` generates one. A configuration that still carries the placeholder the
retired templates shipped is refused as well, so an install that never replaced it has to rotate.

Gin's debug mode and the log level are both controlled by `server.debug` (`GIN_MODE` is overridden
at startup).

#### Database

MariaDB schemas are provisioned outside the app. `scripts/mariadb-init.sql` creates the main `proviant` schema and, when the [Open Food Facts bulk dump](./docs/configuration.md) is enabled, its separate dump schema (default `off_dump`) and grants. The Docker compose file mounts it for fresh volumes; on an existing volume run it manually, because the MariaDB entrypoint only executes init scripts on an empty data directory:

```bash
mysql -h <host> -u root -p < scripts/mariadb-init.sql
```

The compose database publishes no port, so the manual run happens inside the container. The
script is already mounted there, and the root password is in the container's own environment:

```bash
docker compose -f docker-compose.mariadb.yaml exec -T database sh -c 'MYSQL_PWD="$MARIADB_ROOT_PASSWORD" mariadb -u root < /docker-entrypoint-initdb.d/01-init.sql'
```

That password is the one the volume was created with: on an existing volume the image keeps the
root password of its initialization, so the environment value only matches when the volume is
fresh. A lost root password means a new volume (`docker compose -f docker-compose.mariadb.yaml
down -v`) — the image applies `MARIADB_ROOT_PASSWORD` at initialization only.

The app never creates schemas. SQLite needs none: the dump store is a plain file beside the main database. With the dump enabled on MariaDB it fails to start if the configured schema (default `off_dump`) is missing or unreachable, rather than running half-configured.

#### Notifications

Notifications go out over SMTP, Ntfy, a Telegram bot and web push. The web push subscriptions need no
configuration: the VAPID keys are generated on the first run and stored in the database. The Telegram
bot token belongs to the account that sets it in the app — it is not a configuration key.

The receipt scan reads a receipt photo with a vision-capable, OpenAI-compatible chat-completions
endpoint. It is off by default and needs a model before it is turned on.

#### Recipe suggestions

For a self-hosted provider, set `recipe_api.provider`, `recipe_api.url` and `recipe_api.api_key`
together, and make sure the URL points at your instance — pointing a self-hosted provider at the
public TheMealDB URL would send your API key and product names to a third party. A self-hosted
provider without a key disables recipe suggestions at startup instead of falling back.

#### Scheduled backups

The scheduled backup job writes one full household JSON backup per household to a local directory.
Backup files contain household data in cleartext; point the directory at storage you control and
restrict who can read it.

#### Restore drill

The backup job writes documents; nothing proves that they can be read back. The drill restores the
newest document of every household into a throwaway database and compares the restored rows with the
rows the document says it carries. It never opens the live database.

Run it on a schedule — a cron entry that fails the job when the drill fails:

```cron
17 4 * * * docker exec proviant /app/proviant backup-drill
```

The commands, the exit codes, what the drill compares and what a failure means are documented in
[docs/backup-drill.md](./docs/backup-drill.md).

#### Restore uploads

A restore upload is a full household backup and is therefore larger than an ordinary request body,
so it has its own cap (`restore.maxUploadSizeMB`). The restore drill proves that the newest
scheduled backup of every household still restores.

## Upgrade / Migration

**[The upgrade and rollback guide](./docs/upgrade.md)** covers the backup to take before an upgrade,
what every release changes and when a rollback is safe. The short version:

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
still set to the placeholder the retired templates shipped aborts startup until you generate a
new one (`openssl rand -hex 32`).

### Manual / binary

Replace the `proviant` binary with the new version and restart the service. Migrations are applied automatically on startup. Check the [CHANGELOG](./CHANGELOG.md) for any breaking changes or additional steps.

## AI disclaimer

I've been building `proviant` since 2024 with just basic knowledge of `Go` and I am **not a professional software developer**. Starting this year, I used AI more intensively to speed up development and help with the UI design. While I carefully checked all changes, you might want to **not use** Proviant, if this is a problem for you. I won't judge you, if you don't judge me ✌🏻

## Commit Conventions

This project uses [Conventional Commits](https://www.conventionalcommits.org/). The changelog is auto-generated by [git-cliff](https://git-cliff.org/).

## Documentation

Documentation is generated with `gomarkdoc` and `swagger`:

- [Package documentation](./src/docs/README.md)
- [Swagger definition](./src/docs/swagger.yaml)

The design system has its own documentation:

- [design-system/DESIGN.md](./design-system/DESIGN.md) — the binding rules
- [design-system/deviations.md](./design-system/deviations.md) — every place the app departs from them, with a reason
- [design-system/screens/](./design-system/screens/) — the spec sheets, one per screen

Operator references:

- [docs/configuration.md](./docs/configuration.md) — every configuration key with its type, its default, its constraint and its effect
- [docs/upgrade.md](./docs/upgrade.md) — how to upgrade, what every release changes and when a rollback is safe
- [docs/metrics.md](./docs/metrics.md) — the exported Prometheus metrics, their labels and ready-to-use queries
- [docs/backup-drill.md](./docs/backup-drill.md) — proving that a scheduled backup can be restored

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

The CI pipeline runs on every push, and on pull requests targeting `main` or `develop`. Dependency scanning gates every run; container-image scanning gates pushes to `main`/`develop` and same-repo pull requests, except the ones from Dependabot (their CI token is read-only):

| Scan | Tool | Scope |
| --- | --- | --- |
| Go dependencies | `govulncheck` | `task vuln-go` / CI `check` job |
| npm dependencies | `npm audit` (production gate) | `task vuln-npm` / CI `frontend-check` job |
| Container image | `trivy` (prod image) | `task vuln-image` / CI `image-scan` job + release gate |

Image scanning blocks on fixable `HIGH`/`CRITICAL` findings. Scan flags and the digest-pinned trivy image live in `scripts/image-scan.sh` — the single source of truth shared by CI and `task vuln-image`. The task uses the pinned container image when no local `trivy` binary is found.

The image scan publishes the scanned image to a `ci-scan-<sha>` tag and the release step promotes that exact digest to the published tags, so the report always covers the image that ships. A scan that fails to complete (unreachable vulnerability database, registry error) is reported as an error rather than as a clean result — the gate never passes because the scanner broke. Pull requests from forks and from Dependabot do not run the image scan (their CI token is read-only); the gate is enforced through branch protection on `main` and `develop`.

### Dependency Updates

Dependabot proposes the Go module, the npm packages, the container base images and the pinned GitHub Actions every Monday. The proposal opens a pull request against `main` that runs the CI — the Go lint and test jobs, the coverage gate, the frontend checks and the dependency-pinning check — so an update is testable before it lands. The image scan is the exception: a Dependabot pull request gets a read-only token, so that job runs after the merge instead. Nothing is merged automatically: a maintainer reviews the proposal and merges it once the run is green.

The actions stay pinned to commit SHAs, the trivy image stays pinned to a digest, and Bootstrap 6 (an alpha) is bumped by hand. The triage policy — how a proposal is reviewed, how a dev advisory is closed, how the alpha is handled, who merges and what the merge gate is — lives in [`AGENTS.md`](AGENTS.md) under *Dependency Updates*. `task check-dependabot` verifies the configuration and the pinning invariants. It also holds the toolchains together: the Go version and the Node major have to be the same in the workflows, the Dockerfiles and the manifest, so an image is never built with a different toolchain than the one CI tests with.

## Monitoring

The server exposes two unauthenticated probes and an optional Prometheus endpoint:

| Endpoint | Description |
| --- | --- |
| `GET /health` | Liveness — `200` while the process is running |
| `GET /health/ready` | Readiness — `200` when the database answers a ping, `503` otherwise |
| `GET /metrics` | Prometheus metrics; registered only when `PROVIANT_SERVER_METRICSENABLED=true` |

The `docker-compose.sqlite.yaml` health check targets `/health/ready`.

The container's own health check probes `/health` over plain HTTP, because the app terminates no TLS
— a reverse proxy in front of it does the encryption. The probe falls back to the loopback
interface, so no clear-text traffic crosses a network boundary unless `PROVIANT_SERVER_HOST` names
an address that resolves to another host. The app binds `server.host` from its config, so it has to
listen on the loopback or on every interface for the probe to reach it.

Besides the HTTP counters, the endpoint exports the state of the background workers — notification
delivery per provider, the database pool, the backup age and the Open Food Facts cache hit rate.
The full catalogue, the closed label sets and ready-to-use PromQL queries live in
[docs/metrics.md](./docs/metrics.md).

## Contributing

Contributions are welcome! Fork the repository, make your changes, and submit a pull request.

## License

This project is licensed under the [MIT License](LICENSE).

## Author

- GitHub: [@Isotop7](https://github.com/Isotop7)
- LinkedIn: [Hendrik Röder](https://www.linkedin.com/in/hendrik-r%C3%B6der-9b8483198/)

---
