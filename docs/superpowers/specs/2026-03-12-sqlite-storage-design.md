# SQLite Storage Layer Design

**Date:** 2026-03-12
**Status:** Approved
**Replaces:** PostgreSQL (CloudNativePG) + Valkey from workplan v2

## Decision

Use SQLite (via `modernc.org/sqlite`, pure Go, no CGO) instead of PostgreSQL + Valkey for the skillctl backend storage layer.

## Why

The original workplan specified PostgreSQL via CloudNativePG with Valkey pub/sub for cache invalidation across replicas. This adds two infrastructure dependencies (an operator + a cache server) for a registry that will hold hundreds to low thousands of skills with low write volume.

SQLite on a PVC gives us full SQL (migrations, foreign keys, indexes, FTS5 for search) with zero additional infrastructure. WAL mode provides concurrent readers with single-writer blocking, which is sufficient for this workload.

## What changes

| Concern | Before | After |
|---|---|---|
| Primary store | PostgreSQL (CloudNativePG) | SQLite on PVC (`modernc.org/sqlite`) |
| Cache invalidation | Valkey pub/sub | Not needed - process-local invalidation after own writes, periodic reload for cross-replica |
| Driver | pgx | `modernc.org/sqlite` via `database/sql` |
| Migrations | goose with postgres dialect | goose with sqlite3 dialect |
| Container image | Still scratch, CGO_ENABLED=0 | Same - `modernc.org/sqlite` is pure Go |
| Helm chart | CloudNativePG Cluster CR + Valkey Deployment | Single PVC |
| Federation leader election | PostgreSQL advisory locks | SQLite file lock (single writer) |
| Tags column | `TEXT[]` with GIN index | JSON column or join table |
| Audit log details | `JSONB` | `JSON` (SQLite JSON1 extension) |

## What stays the same

- Goose for migrations (embedded in binary, run on startup)
- Same table structure (skills, skill_versions, marketplaces, federated_skills, audit_log)
- Same `SkillStore` interface - the SQLite implementation replaces the memory store for production
- In-memory cache for fast reads (invalidated after local writes, periodic reload from disk)
- OCI registry for skill archives (oras-go)
- All auth design (OIDC, push tokens, admin groups)

## SQLite configuration

```
PRAGMA journal_mode=WAL;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;
PRAGMA synchronous=NORMAL;
```

WAL mode allows concurrent readers while one writer holds the lock. Other writers block up to `busy_timeout` (5 seconds). With the expected write volume (skill publishes, federation syncs, admin ops), contention will be negligible.

## Schema adaptations

Since SQLite lacks `TEXT[]` arrays and `JSONB`:

- `tags` columns become `TEXT` storing JSON arrays (e.g., `'["cli","go"]'`), queryable via `json_each()`
- `details` in audit_log becomes `TEXT` storing JSON, queryable via `json_extract()`
- `TIMESTAMPTZ` becomes `TEXT` in ISO 8601 format (SQLite has no native datetime type)
- `BIGSERIAL` becomes `INTEGER PRIMARY KEY` (SQLite autoincrement)

## Multi-replica behavior

Multiple replicas can mount the same PVC (ReadWriteMany) and SQLite's file locking handles concurrency. Each replica:
- Reads freely (WAL mode, non-blocking)
- Writes block if another writer holds the lock (up to busy_timeout)
- Invalidates its own in-memory cache after writes
- Periodically reloads cache from disk to pick up other replicas' writes

If ReadWriteMany PVC is not available, a single-replica deployment works fine for the expected scale.

## Backup strategy

Litestream or a CronJob that copies the SQLite file to object storage. This replaces CloudNativePG's built-in backup/restore.
