# Backup restore drill

The scheduled backup job writes one versioned household document per household. The drill proves
that the newest document can be restored — before the day it is needed.

A backup that is truncated, written against a schema that has since moved, or missing a household's
rows is indistinguishable from a good one until the day it matters. The drill turns that into a
scheduled job that fails loudly.

## What the drill does

1. Finds the newest document in the backup directory — the newest file by the timestamp in its name.
2. Reads and decodes it. A document that cannot be read is a broken backup.
3. Checks the schema version. A document this release does not write is reported separately: the
   schema moved, the backup did not break.
4. Creates a scratch database in a temporary directory, migrates it, and seeds it with the
   household and the members the document names.
5. Restores the document with the same code path the user-facing restore uses.
6. Compares the restored rows with the rows the document says it carries.
7. Deletes the scratch database — on success and on failure.

The drill never opens the live database. The restore lands in the scratch file, and the scratch path
is asserted to be a different file than the live one.

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
| `task backup-drill` | A local installation. Drills the newest document in `backup.directory`. |
| `docker exec proviant /app/proviant backup-drill` | A container deployment. |
| `proviant backup-drill --file <path>` | Re-checks one document, e.g. after an incident. |
| `proviant backup-drill --directory <path>` | Drills a directory other than the configured one. |

The output is the comparison table and the age of the document:

```
backup:  /app/data/backups/proviant-backup-household-1-20261103T120000.000Z.json
age:     2s
schema:  3
products           1 expected, 1 restored
archivedProducts   1 expected, 1 restored
storageLocations   2 expected, 2 restored
recipes            not carried by the backup document
mealPlanEntries    not carried by the backup document
shoppingListItems  not carried by the backup document
```

## Exit codes

| Exit code | Meaning |
|---|---|
| `0` | The newest backup restores and its counts match — or the scheduled backup is disabled and there is nothing to drill. |
| `1` | The backup is broken: unreadable, unrestorable, or the restored rows differ. |
| `2` | The document carries a schema version this release does not write. |
| `3` | There is nothing to drill: no document, an unusable backup directory, or a usage error. |

The exit code comes from the binary. `task backup-drill` prints it (`Backup drill exit code: 2`) and
fails the task, but Task itself reports every failure as exit code 201 — a scheduled job should
call the binary directly.

## Scheduling it

```cron
17 4 * * * docker exec proviant /app/proviant backup-drill
```

A run that takes longer than ten minutes fails. A deployment that does not run the scheduled backup
(`PROVIANT_BACKUP_ENABLED=false`) exits 0 with "the scheduled backup is disabled" — there is nothing
to drill.

## In CI

`.github/workflows/weekly-backup-drill.yml` runs the drill every Sunday against the documents in
`src/testdata/backup-drill/` and asserts the exit codes. The Go tests in
`src/controllers/backup_drill_test.go` cover the same path in-process, so a regression fails the
ordinary CI run, not only the weekly one.

The fixture directory holds one healthy document and two broken ones — a document written against a
newer schema and a truncated file. They are the drill's test data, not household data: nothing in
them identifies a real household.

## When it fails

| Message | What it means |
|---|---|
| `the document is version N, this release writes version 3` | The backup was written by a newer release. Upgrade before the next backup runs, or the schema needs a migration. |
| `file is not a valid household backup` | The document is truncated or corrupt. The backup job writes through a temporary file, renames it into place and removes that file when the write fails, so the job cannot leave a truncated document — something else did: a manual edit, a full disk during another write, or filesystem damage. The drill ignores the `*.tmp` file a killed backup job leaves behind. |
| `the restored backup does not match the document: …` | The restore dropped a row or wrote a different value. The message names the entity, the row and the field. |
| `no backup document found: the backup directory … does not exist` | The configured directory does not exist. |
| `no backup document found: … holds no backup document` | The backup job has not written anything yet, or the retention pruned everything. |
