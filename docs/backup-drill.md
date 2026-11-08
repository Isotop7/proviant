# Backup restore drill

The scheduled backup job writes one versioned household document per household. The drill proves
that every one of them can be restored — before the day it is needed.

A backup that is truncated, written against a schema that has since moved, or missing a household's
rows is indistinguishable from a good one until the day it matters. The drill turns that into a
scheduled job that fails loudly.

## What the drill does

1. Finds the newest document of every household in the backup directory. The name of a document
   carries its household and the moment it was written, so the newest one is read out of the name
   rather than guessed. Only names that follow that convention are picked up: a backup document the
   operator renamed is ignored, and one the operator names with `--file` is drilled on its own.
2. Reads and decodes each document. A document that cannot be read is a broken backup.
3. Checks the schema version. A document this release does not write is reported separately: the
   schema moved, the backup did not break.
4. Creates a scratch database in a temporary directory, migrates it, and seeds it with the
   household and the members each document names. Every household gets its own household in that one
   database, so one restore cannot touch the rows of another.
5. Restores each document with the same code path the user-facing restore uses.
6. Compares the restored rows with the rows the document says it carries.
7. Deletes the scratch database — on success and on failure.

The drill never opens the live database. The restore lands in the scratch file, and the scratch path
is asserted to be a different file than the live one.

One broken household does not stop the drill: every household is restored and compared, and the
failures are reported together. A household that cannot be restored must not hide the ones that can.

## What it compares

| Entity | Compared |
|---|---|
| products | count, barcode, name, categories, quantity, pack size, expiry, private flag, owner, storage location |
| archived products | the same, plus the archived state |
| storage locations | count, name, icon and sort order |
| household | name, description, monthly waste goal type, count and percent |
| recipes, meal-plan entries, shopping-list items | not carried by the backup document — reported, not compared |

The counts say how many rows came back; the field comparison says whether they are the right ones.
A restore that keeps the count and writes the wrong values is the failure a count cannot see.

The comparison is against the document, not against the live household. A backup is as old as the
backup interval, and the household buys products in between, so live counts would disagree with a
perfectly good backup.

## Running it

| Command | Use when |
|---|---|
| `task backup-drill` | A local installation. Drills the newest document of every household in `backup.directory`. |
| `docker exec proviant /app/proviant backup-drill` | A container deployment. |
| `proviant backup-drill --file <path>` | Re-checks one document, e.g. after an incident. |
| `proviant backup-drill --directory <path>` | Drills a directory other than the configured one. |
| `proviant backup-drill --max-age <duration>` | Fails when a backup is older than the duration, e.g. `48h`. `0` switches the check off. |

The output is one block per household:

```
household: Drill Household (id 1)
backup:  /app/data/backups/proviant-backup-household-1-20261103T120000.000Z.json
age:     2s
schema:  3
products           1 expected, 1 restored
archivedProducts   1 expected, 1 restored
storageLocations   2 expected, 2 restored
household: Second Household (id 10)
backup:  /app/data/backups/proviant-backup-household-10-20261103T120000.000Z.json
age:     2s
schema:  3
products           4 expected, 4 restored
archivedProducts   0 expected, 0 restored
storageLocations   1 expected, 1 restored
recipes            not carried by the backup document
mealPlanEntries    not carried by the backup document
shoppingListItems  not carried by the backup document
```

The household is named the way the household knows itself; the id in brackets is the one the file
name carries. A household whose document is broken is reported with the error that broke it, and the
households behind it are still drilled.

## Exit codes

| Exit code | Meaning |
|---|---|
| `0` | Every household's newest backup restores and its counts match — or the scheduled backup is disabled and there is nothing to drill. |
| `1` | A backup is broken: unreadable, unrestorable, or the restored rows differ. |
| `2` | A document carries a schema version this release does not write. |
| `3` | There is nothing to drill: no document, an unusable backup directory, or a usage error. A document named with `--file` that is not there is nothing to drill, not a broken backup. |
| `4` | The backup restores but is older than the bound — the job that writes the backups stopped. |
| `5` | The drill ran out of time before it could restore a household. The backup was not shown to be broken. |

A broken backup outranks everything else: it is a defect that needs attention now, whatever else the
run found.

The exit code comes from the binary. `task backup-drill` prints it (`Backup drill exit code: 2`) and
fails the task, but Task itself reports every failure as exit code 201 — a scheduled job should
call the binary directly.

## The staleness bound

A backup that is never written again restores perfectly well, so a restore check alone cannot see
that the backup job stopped. The drill therefore also asks how old the newest document is.

Without `--max-age` the bound follows the backup interval: twice `backup.intervalHours`, so one
missed run is tolerated and two is a failure. With the default interval of 24 hours the bound is 48
hours. `--max-age` overrides it, and `--max-age 0` switches the check off.

The bound applies to the documents the drill picks out of the backup directory. A document named
with `--file` is exempt: it is a document the operator picked, not the backup job's newest output,
and its age says nothing about the job.

## Scheduling it

```cron
17 4 * * * docker exec proviant /app/proviant backup-drill
```

A restore of one household that takes longer than ten minutes fails, and the drill says it ran out
of time rather than that the backup is broken. The bound is per household, not per run: an instance
with many households needs the time. The whole run is bounded by the sum of its per-household bounds
plus one, so the drill of an instance with many households cannot outgrow the time it asked for. A
deployment that does not run the scheduled backup (`PROVIANT_BACKUP_ENABLED=false`) exits 0 with "the
scheduled backup is disabled" — there is nothing to drill. A deployment whose configuration cannot be
read falls back to the default backup directory and does not check the age of the backups.

## In CI

`.github/workflows/weekly-backup-drill.yml` runs the drill every Sunday against the documents in
`src/testdata/backup-drill/` and asserts the exit codes. The Go tests in
`src/controllers/backup_drill_test.go` cover the same path in-process, so a regression fails the
ordinary CI run, not only the weekly one.

The fixture directory holds one healthy document and two broken ones — a document written against a
newer schema and a truncated file. They are the drill's test data, not household data: nothing in
them identifies a real household. `src/testdata/backup-drill/directory/` holds one document per
household, named the way the scheduler names them, so the drill's selection of the newest document
of every household is exercised too.

Only the fixtures are tracked: a backup document dropped into the directory is ignored by git, because
backup documents carry household data in cleartext. A new fixture needs its own exception line in
`.gitignore`.

## When it fails

| Message | What it means |
|---|---|
| `the document is version N, this release writes version 3` | The backup was written by a newer release. Upgrade before the next backup runs, or the schema needs a migration. |
| `file is not a valid household backup` | The document is truncated or corrupt. The backup job writes through a temporary file, renames it into place and removes that file when the write fails, so the job cannot leave a truncated document — something else did: a manual edit, a full disk during another write, or filesystem damage. The drill ignores the `*.tmp` file a killed backup job leaves behind. |
| `the restored backup does not match the document: …` | The restore dropped a row or wrote a different value. The message names the entity, the row and the field. |
| `the backup is older than the drill allows: …` | The newest backup of a household is older than the bound. The job that writes the backups stopped. |
| `the drill ran out of time: …` | The drill did not finish restoring a household in time. The backup was not shown to be broken — the instance is too big for the bound, or a restore hangs. |
| `no backup document found: the backup directory … does not exist` | The configured directory does not exist. |
| `no backup document found: … holds no backup document` | The backup job has not written anything yet, or the retention pruned everything. |
