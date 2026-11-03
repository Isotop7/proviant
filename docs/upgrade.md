# Upgrade and rollback

How to move a Proviant deployment to a new release, what the new release changes, and what it
costs to go back.

The container images are the primary distribution, so the instructions are Docker-first and the
binary comes last.

## Before you upgrade

### Take a database backup

The database migrations run on startup, and one of them rewrites data. A backup taken *before* the
upgrade is the only way back from a release that dropped a column.

**SQLite** — the database is a single file. Stop the server and copy it:

```bash
docker stop proviant
docker cp proviant:/app/data/proviant.db ./proviant-$(date +%F).db
docker start proviant
```

The path inside the container follows `database.sqlite.filepath`; the default is
`data/proviant.db`, which the image mounts at `/app/data`. Copying the file while the server
writes to it can produce a torn copy — stop it first.

**MariaDB** — the app never creates a schema, so the dump comes from the database itself:

```bash
mysqldump -h <host> -P <port> -u <user> -p <schema> > proviant-$(date +%F).sql
```

The compose file publishes no port for the database, so the dump runs inside the container:

```bash
docker compose -f docker-compose.mariadb.yaml exec -T database \
  sh -c 'mariadb-dump -u root -p"$MARIADB_ROOT_PASSWORD" proviant' > proviant-$(date +%F).sql
```

The password in the container's environment is the one the volume was created with — on an
existing volume the image keeps the password of its initialization.

### The scheduled backup is not a database backup

`backup.enabled` runs a scheduled backup that writes one JSON document per household. It carries
household data — products, storage locations, members — and nothing else: no accounts, no
two-factor secrets, no schema. It restores a household, not a database.

[docs/backup-drill.md](./backup-drill.md) describes the drill that proves the newest document still
restores.

## Upgrading

### Docker

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

The container runs as the unprivileged `proviant` user, so the mounted volumes have to be writable
by `100:101`:

```bash
docker run --rm -v proviantdb:/data alpine chown -R 100:101 /data
```

The volume names in the command are the ones a plain `docker run` creates. A deployment that
keeps its data in a compose volume has to keep that volume's name — a container that starts on
an empty volume comes up with an empty database.

`:latest` always points at the newest final release. For a deployment that has to stay put, pin the
digest the release notes print rather than the tag.

### docker compose

The compose files in this repository build from source (`build: ./`), so `docker compose pull` is a
no-op for them. Upgrade those with:

```bash
docker compose build --pull && docker compose up -d
```

Switch to the prebuilt image as above to stop rebuilding.

### Binary

Replace the `proviant` binary and restart the service. The migrations run on startup.

## What the releases change

### v0.26.0 — in development

Nothing in the database is rewritten and no schema change is made, so this release rolls back
cleanly. Three things an operator notices:

- **The password policy defaults now apply.** A configuration file that leaves out
  `server.authentication.password_min_length` used to end up with no minimum length at all — and
  with no breach check either, because the check only runs while a minimum length is set. The
  default is now 12, the breach check runs, and the uppercase and digit requirements default to on.
  The shipped templates set the two requirements to off, so a template-based deployment does not
  change. A hand-written configuration that omits the keys gets the stricter policy.
- **The mail digest is on unless the configuration says otherwise.** The default of
  `notification.mailDigest.enabled` is now `true`; the shipped templates set it to `false`. A
  household still has to opt in for itself, so no mail goes out that did not go out before.
- **Every environment variable reaches the server.** A variable for a key the configuration file
  does not mention used to be ignored, because viper only knows the keys it has seen. Every
  configuration key now carries a built-in default, so `PROVIANT_SERVER_DEMOMODE` works against a
  configuration file that never mentions `server.demoMode`.

The server also warns at startup when the database password is one this repository publishes, and
`proviant backup-drill` proves that the newest scheduled backup still restores.

### v0.25.0

- **The container's time zone decides the dates.** A date-only value — the date printed on a
  product — is read in the container's time zone, the daily notification schedules run in it, and
  the dashboard buckets are anchored to one instant in it. Set `TZ` to the time zone of the
  household. The compose files default it to `UTC`; the image does the same.
- **The products list is paginated, and its response is no longer a bare array.** `GET
  /api/v1/products` takes `limit` (1 to 1000, 100 by default) and `offset` (0 by default) and
  answers with an object: the rows of the page under `products`, the total number of products the
  filter matched under `total`, the window under `limit` and `offset`. The same change applies to
  `GET /api/v1/products/archived` and `GET /api/v1/products/search`. A client that read a bare
  array now reads the first hundred products and has to walk the rest by offset, and a client that
  iterates the response has to read `products` instead. The order is stable, so a client that only
  reads walks every product exactly once — as long as the household's products do not change in
  between.
  One more response a direct client may match on: the admin password reset now answers "Password
  reset prepared, the member is being sent a mail" instead of "Password reset email sent".
- **A new configuration block, `server.twoFactor`.** The templates ship it with
  `server.twoFactor.enabled: false`. To turn the second factor on, set the switch and a
  `server.twoFactor.encryptionKey`: 32 random bytes, base64 encoded, different from
  `server.authentication.tokenPassword`. The key encrypts the TOTP secret of every enrolled
  account at rest — rotating it locks every enrolled device out and the enrolment has to be redone.
  The placeholder the template ships is refused at startup, so generate a key with
  `openssl rand -base64 32` before you enable the feature.
- **The schema grew, nothing was rewritten.** The new tables are `household_categories` and
  `two_factor_recovery_codes`; the new columns are `users.two_factor_enabled`,
  `users.two_factor_secret`, `users.two_factor_last_used_step`, `users.two_factor_pending_secret`,
  `users.two_factor_pending_expiry`, `households.require_two_factor`, `products.waste_cause`,
  `products.category_key`, `activity_logs.waste_cause`, `savings_records.waste_cause` and
  `shopping_list_items.category_key`. `BackfillProductCategoryKeys` fills the canonical category of
  the products that carry free-text categories; it adds a value to a column and touches nothing
  else.

### v0.24.0 — the count model

A product is counted in packs now. `products.amount` and `products.unit` are gone; `products.quantity`
holds the number of packs on hand and `products.pack_size` the size of one pack.

- `ConvertProductAmountsToQuantities` rewrites every product. A measured product (500 g) becomes
  one pack of "500 g"; a counted product keeps its count. The minimum stock moves with it.
- `ConvertRecipeIngredientsToPortions` does the same for the amounts and units of the recipe
  ingredients.
- `MarkLegacySavingsRecords` flags the savings records that were written before the cutover and
  writes the `count_model` cutover marker. Their money and their CO2 were scaled by the net
  content, so they are not comparable with the records after the cutover and stay out of the
  aggregates.
- `DropUnitColumns` drops `products.amount`, `products.unit`, `shopping_list_items.unit`,
  `households.unit_system`, `recipe_ingredients.amount` and `recipe_ingredients.unit`.

The CSV import rejects the old format: a file whose header carries `unit` or `amount` is refused
instead of read, because its quantity column held a net content and would land as a pack count.

## Rolling back

### When a rollback is safe

A rollback is safe when the release you are leaving ran no migration that removed or rewrote a
value.

- **v0.26.0 → v0.25.0** — safe. The release rewrites no data and changes no schema.
- **v0.25.0 → v0.24.0** — safe for the schema. Every v0.25.0 migration adds a table or a column,
  and GORM's `AutoMigrate` never drops a column, so the old binary starts on the new schema. The
  data v0.25.0 wrote into the new columns — the waste cause, the canonical category, the household
  categories, the recovery codes, the two-factor secrets — is invisible to v0.24.0: the waste
  analytics lose the cause, the shelf-life defaults lose their category, and a household that
  required a second factor stops requiring it. Roll the image back as a whole, because the v0.25.0
  frontend expects the paginated products API.

### When a rollback is not safe

**v0.24.0 → v0.23.0 is not safe.** `DropUnitColumns` removed the columns the old binary reads,
and `ConvertProductAmountsToQuantities` had already rewritten the net contents into `quantity` and
`pack_size`, which nothing reads back. The old binary queries columns that no longer exist and the
products list fails. The net contents survive only in a database backup taken *before* the
upgrade — the backup the household wrote in the old format is rejected by the importer for the same
reason.

The rule: **a rollback across a release that ran a destructive migration needs a database backup
taken before that upgrade.** Nothing else recovers the data.

## The migrations that change the data

| Migration | Release | What it does | Rolling back across it |
|---|---|---|---|
| `DropLegacyStorageLocationColumn` | v0.17.0 | Drops the free-text `products.storage_location` column; the storage location became a row of its own. | The old binary reads a column that no longer exists. Unsafe without a backup. |
| `RenamePushNotificationColumns` | v0.17.0 | Renames the legacy push columns to the `web_push` prefix. | The subscription stays in the new columns; the old binary finds an empty subscription. |
| `BackfillAuditLogHouseholdID` | v0.21.0 | Attributes the existing audit log entries to the household their actor belongs to now. | The old attribution is not restored. |
| `ConvertProductAmountsToQuantities` | v0.24.0 | Rewrites the net content of every product into a pack count and a pack size. | The net content is not read back. Unsafe without a backup. |
| `ConvertRecipeIngredientsToPortions` | v0.24.0 | Rewrites the amount and the unit of every recipe ingredient into a portion. | The old format is not restored. Unsafe without a backup. |
| `MarkLegacySavingsRecords` | v0.24.0 | Flags the savings records written before the count model and writes the cutover marker. | The savings figures change meaning across the boundary. |
| `DropUnitColumns` | v0.24.0 | Drops `products.amount`, `products.unit`, `shopping_list_items.unit`, `households.unit_system`, `recipe_ingredients.amount` and `recipe_ingredients.unit`. | The old binary reads columns that no longer exist. Unsafe without a backup. |
| `BackfillProductCategoryKeys` | v0.25.0 | Fills the canonical category of the products that carry free-text categories. | Safe: it adds a value to a column and touches nothing else. |

## See also

- [docs/configuration.md](./configuration.md) — every configuration key, its default and its
  constraint.
- [docs/metrics.md](./metrics.md) — the exported Prometheus metrics.
- [docs/backup-drill.md](./backup-drill.md) — the backup restore drill.
