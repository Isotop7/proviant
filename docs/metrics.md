# Metrics

Proviant exposes Prometheus metrics on `GET /metrics`, next to the `/health` and `/health/ready`
probes. The endpoint is registered only when `PROVIANT_SERVER_METRICSENABLED=true` and it is
served without authentication, so network access has to be restricted to trusted scrapers.

Every metric name starts with `proviant_`. The names are a public contract — a Grafana dashboard
built on them keeps working across releases — so they only change with a deliberate migration.

## Exported metrics

| Metric | Type | Labels | Description |
| --- | --- | --- | --- |
| `proviant_http_requests_total` | counter | `method`, `route`, `status` | Handled HTTP requests |
| `proviant_http_request_duration_seconds` | histogram | `method`, `route` | Request duration |
| `proviant_worker_attempts_total` | counter | `worker` | Runs of a background worker that were started |
| `proviant_worker_failures_total` | counter | `worker` | Worker runs that ended in an error |
| `proviant_worker_last_success_timestamp_seconds` | gauge | `worker` | Unix time of the last successful worker run |
| `proviant_notification_attempts_total` | counter | `provider` | Notification sends that were attempted |
| `proviant_notification_failures_total` | counter | `provider` | Notification sends that failed |
| `proviant_notification_last_success_timestamp_seconds` | gauge | `provider` | Unix time of the last successful send |
| `proviant_backup_age_seconds` | gauge | — | Age of the newest successful household backup |
| `proviant_db_connections` | gauge | `state` | Database pool connections (`open`, `in_use`, `idle`) |
| `proviant_db_connections_max` | gauge | — | Maximum number of open connections (zero means unlimited) |
| `proviant_db_wait_count_total` | counter | — | Connection requests that had to wait |
| `proviant_db_wait_duration_seconds_total` | counter | — | Time spent waiting for a connection |
| `proviant_openfoodfacts_cache_lookups_total` | counter | `result` | Open Food Facts database cache lookups (`hit`, `miss`) |
| `proviant_openfoodfacts_offline_lookups_total` | counter | `result` | Open Food Facts offline dump lookups (`hit`, `miss`) |
| `proviant_openfoodfacts_live_requests_total` | counter | — | Live Open Food Facts API requests |
| `proviant_openfoodfacts_live_failures_total` | counter | — | Live Open Food Facts API requests that failed |

## Label sets

The label sets are closed: `worker` is one of `notification_dispatch`, `invitation_retry`,
`mail_digest`, `monthly_waste_report`, `streak_updates`, `telegram_poller`, `backup`,
`offdump_import`; `provider` is one of `email`, `ntfy`, `telegram`, `webpush`; `result` is `hit`
or `miss`; `state` is `open`, `in_use` or `idle`. Nothing is labelled by product, household, user,
e-mail address or chat id, so the endpoint stays free of personal data and the registry cannot grow
without bound.

## Conventions

Three conventions are worth knowing:

- A worker that has never succeeded has **no** `proviant_worker_last_success_timestamp_seconds`
  series, and the same holds for a provider that never delivered and for the backup age before the
  first backup. Absence is the "never ran" signal — a zero would read as the epoch.
- The worker counters measure the loop, the provider counters measure the delivery. The digest
  scheduler ticks every minute, so a healthy digest worker says nothing about whether the e-mails
  went out — that is what `proviant_notification_last_success_timestamp_seconds` is for.
- A failed delivery leaves the product pending and is retried on the next sweep, so
  `proviant_notification_failures_total` grows with the number of pending products while the
  provider is down. A sweep that could not read a household's preferences or record its outcome
  counts as a failed run: `proviant_worker_failures_total{worker="notification_dispatch"}` rises and
  the last-success timestamp stops moving.
- A "run" is one unit of work per worker, so the counters are not comparable across workers: one
  sweep of the expiring products for `notification_dispatch`, one minute-tick for `mail_digest`, one
  `getUpdates` call for `telegram_poller`, one import pass for `offdump_import`, one backup run for
  `backup`.

A cache lookup that does not return an entry counts as a miss, including one whose query failed.
The bulk import path counts one lookup per distinct barcode, so the hit rate covers the scan path
and the import alike.

## Queries

```promql
# No notification was delivered for six hours
(time() - proviant_notification_last_success_timestamp_seconds) > 21600

# A provider never delivered a notification at all
absent(proviant_notification_last_success_timestamp_seconds{provider="email"})

# A worker has not completed a successful run for a day
(time() - proviant_worker_last_success_timestamp_seconds{worker="backup"}) > 86400

# The last backup is older than a day
proviant_backup_age_seconds > 86400

# No backup was ever written
absent(proviant_backup_age_seconds)

# Open Food Facts cache hit rate over five minutes
sum(rate(proviant_openfoodfacts_cache_lookups_total{result="hit"}[5m]))
  / sum(rate(proviant_openfoodfacts_cache_lookups_total[5m]))

# The database pool is saturated; the maximum is zero while the pool is unlimited
(proviant_db_connections_max > 0)
  and (proviant_db_connections{state="in_use"} / proviant_db_connections_max > 0.8)
```
