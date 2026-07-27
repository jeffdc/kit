# Forage SQLite Backup Plan

**Status:** Planned; not implemented

## Goal

Create simple, periodic, application-consistent backups of the production SQLite database at `/home/sprite/forage/forage.db`, stored outside the `forage` Sprite so they survive accidental data loss or deletion of the Sprite.

The current production database is approximately 320 KB, so this design favors simplicity and recoverability over streaming replication or incremental backups.

## Current State

- The application runs as a service on the `forage` Sprite.
- `cmd/forage-server/main.go` defaults `FORAGE_DIR` to `/home/sprite/forage`.
- `internal/storage/storage.go` opens `/home/sprite/forage/forage.db` with SQLite.
- The Sprite has `/usr/bin/sqlite3` installed.
- Sprite filesystems are continuously synchronized to durable object storage, and the platform creates automatic checkpoints. These checkpoints are useful for quick rollback, but older automatic checkpoints are pruned and they are not an independent backup if the Sprite is deleted.
- Fly Volumes are not required for this design.

## Decision

Use a scheduled GitHub Actions workflow as an external cron job. The workflow will wake the Sprite, create a consistent SQLite backup, copy it to the Actions runner, validate it, compress it, and upload it to an S3-compatible object-storage bucket.

Do not use Litestream, add a backup daemon, or change the application.

An external scheduler is intentional. Sprites pause while idle, so cron running inside the Sprite cannot reliably execute at a particular time. GitHub Actions can wake the Sprite by executing the backup command through the Sprite CLI.

For independence from Fly.io, prefer a bucket hosted by another provider, such as Cloudflare R2 or Backblaze B2. Tigris is also compatible if protection from Sprite deletion is sufficient and provider independence is not required.

## Architecture

```text
GitHub Actions schedule or manual dispatch
                 |
                 v
       Authenticate Sprite CLI
                 |
                 v
  Wake forage and run SQLite .backup
                 |
                 v
     Copy snapshot to Actions runner
                 |
                 v
       Run PRAGMA integrity_check
                 |
                 v
         Compress and upload
                 |
                 v
      S3-compatible object bucket
```

All object-storage credentials remain in GitHub Actions secrets. They are not stored on the Sprite.

## Backup Workflow

Create `.github/workflows/backup.yml` with both:

- A daily `schedule` trigger at a non-peak UTC minute.
- A `workflow_dispatch` trigger for backups before deployments or risky maintenance.

Configure a workflow concurrency group so two backup runs cannot operate on the same temporary file simultaneously.

Each run will:

1. Install the Sprite CLI using the official installer.
2. Authenticate non-interactively with `sprite auth setup --token "$SPRITES_TOKEN"`.
3. Execute SQLite's online backup operation on the Sprite:

   ```bash
   sqlite3 /home/sprite/forage/forage.db \
     ".timeout 10000" \
     ".backup '/tmp/forage-backup.db'"
   ```

   The busy timeout permits a short concurrent application write to finish. SQLite's `.backup` operation produces a transactionally consistent snapshot while the web service remains online. The workflow must not copy the live `forage.db` file directly.

4. Stream `/tmp/forage-backup.db` from the Sprite to a file on the Actions runner.
5. Run the following against the downloaded file and require the exact result `ok`:

   ```bash
   sqlite3 forage-backup.db "PRAGMA integrity_check;"
   ```

6. Compress the validated database with gzip.
7. Upload it using a unique UTC timestamped object key, for example:

   ```text
   forage/daily/2026/07/forage-2026-07-27T041700Z.db.gz
   ```

8. Allow the workflow to fail visibly if authentication, backup creation, transfer, validation, compression, or upload fails. A failed run must not replace or remove any earlier backup.

The workflow does not need to compare backups or skip unchanged databases. At the current database size, unconditional daily uploads are simpler and effectively free.

## Secrets and Bucket Permissions

Configure these GitHub Actions secrets:

- `SPRITES_TOKEN`
- `BACKUP_S3_ACCESS_KEY_ID`
- `BACKUP_S3_SECRET_ACCESS_KEY`
- `BACKUP_S3_BUCKET`
- `BACKUP_S3_ENDPOINT`
- `BACKUP_S3_REGION`, if required by the provider

The object-storage credential should be restricted to the backup bucket and the `forage/` prefix. Prefer an append-only policy that can upload and inspect objects but cannot delete existing backups.

Enable encryption at rest using the bucket provider's standard server-side encryption. Do not make the bucket or its objects public.

## Retention

Initial policy:

- Create one backup per day.
- Retain daily backups for 30 days.
- Optionally retain one monthly backup for 12 months if longer history is useful.

Retention should be enforced by bucket lifecycle rules rather than delete commands in the workflow. If monthly retention complicates the provider configuration, retain all daily backups for one year instead; the database is small enough that the difference is negligible.

## Restore Procedure

Restores are manual and deliberately separate from the automated backup workflow.

1. Select and download the desired timestamped object.
2. Decompress it to a new local path.
3. Validate it before touching production:

   ```bash
   sqlite3 restored-forage.db "PRAGMA integrity_check;"
   ```

4. Inspect important application-level data, such as book and bookseller counts.
5. Stop the Sprite `web` service.
6. Preserve the current production database before replacing it.
7. Copy the validated restored database to `/home/sprite/forage/forage.db`.
8. Start the `web` service.
9. Exercise the application through its public URL and verify reads and a controlled write.

The exact operational commands should be tested and recorded during implementation. Restoration is destructive and must never be automated as part of the scheduled backup workflow.

## Verification Before Relying on the Backup

Implementation is complete only after an end-to-end manual workflow run proves all of the following:

1. The scheduled workflow can authenticate and wake the `forage` Sprite.
2. The uploaded object exists under the expected timestamped key.
3. A fresh runner can download and decompress the object.
4. `PRAGMA integrity_check` returns `ok` on the downloaded database.
5. The restored database contains expected production records.
6. A restore drill to a temporary, non-production path succeeds.
7. The bucket lifecycle policy and access restrictions match the intended retention and prevent public access.

Repeat a restore drill periodically, such as quarterly. A backup is not considered reliable until its restore path has been exercised.

## Alternatives Considered

### Cron inside the Sprite

This has fewer external components, but it cannot reliably run while the Sprite is paused. It is unsuitable for a dependable wall-clock schedule.

### Cron on a Mac or NAS

This can execute the same snapshot-and-upload flow and is reasonable if an always-on machine already exists. GitHub Actions avoids depending on a personal machine's uptime and network connection.

### Native Sprite checkpoints

The platform already creates automatic checkpoints, and manual checkpoints are useful before deployments or schema changes. They remain a secondary, quick-rollback mechanism rather than the independent backup described here.

### Litestream

Continuous replication and point-in-time recovery are unnecessary for this project's database size and recovery requirements. They add a daemon, WAL-specific behavior, configuration, and operational surface without enough benefit.

## Implementation Tasks for Later

### 1. Provision the backup bucket

Choose an S3-compatible provider, create a private bucket, configure restricted upload credentials, and add the lifecycle policy.

### 2. Add the scheduled workflow

Create `.github/workflows/backup.yml` with daily and manual triggers, concurrency control, Sprite authentication, SQLite backup creation, transfer, validation, compression, and object upload.

### 3. Run an end-to-end backup

Trigger the workflow manually, inspect its logs, and confirm that the timestamped object can be downloaded, decompressed, and validated independently.

### 4. Perform a non-production restore drill

Restore the uploaded database to a temporary path, run integrity and application-level checks, and record any provider-specific recovery commands in this document.

## References

- [Sprites lifecycle and persistence](https://docs.sprites.dev/concepts/lifecycle/)
- [Sprites checkpoints](https://docs.sprites.dev/concepts/checkpoints/)
- [Sprites CLI authentication for CI/CD](https://docs.sprites.dev/cli/authentication/)
- [SQLite Online Backup API](https://www.sqlite.org/backup.html)
