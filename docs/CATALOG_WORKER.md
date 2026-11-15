# Opt-in Wikidata factual synchronization

The worker now supports one explicitly configured Wikidata fact-sync job. It stays idle with no network/database access when CATALOG_SYNC_IDS is empty. No host service, source or schedule is activated by the repository.

Apply migration 000014 and reapply catalog writer/reader grants before updating worker or diagnostics. The API's private security boundary is unchanged.

## Configuration

1. Apply migrations and provision deployments/catalog-writer-role.sql as an administrator. Set its password out of band. Use a new dedicated role; CheckWriter rejects elevated/table-owner roles, role memberships and any private-table/column access.
2. Review the CC0 factual source scope in PROVIDERS.md and register provider_key=wikidata with a nonempty reviewed policy_url and enabled=true. Only the administrator may enable sources or change policy URLs.
3. Supply the writer connection as CATALOG_DATABASE_URL. DATABASE_URL is never a fallback.
4. Set CATALOG_SYNC_IDS to up to twenty explicit supported Q IDs and optionally CATALOG_SYNC_INTERVAL (default 24h, minimum 1h, maximum 7d).
5. Run go run ./cmd/worker in a private development environment.

The role has public catalog insert/read/update permissions only, without DELETE, schema ownership or private-table rights. PostgreSQL row locking requires UPDATE privilege on a selected table: narrowly scoped UPDATE(display_name) on data_sources and UPDATE(external_url) on external IDs support existing importer locks without allowing source approval or identity remapping. Existing roles must be reviewed for extra grants; the provisioning script does not revoke pre-existing privileges.

## Lifecycle and boundaries

Each cycle uses a separate bounded connection, a two-minute context and the provider client's request limits. A session advisory lock suppresses simultaneous collectors sharing this database. No database row transaction remains open during network fetches. Source approval is checked before fetching and again during atomic import. The source-wide persisted observation time prevents refetching before the interval after process restarts. A future timestamp fails closed. Updating the configured item list does not override that source-wide due time.

Cycles run serially and wait the full configured interval after success, skip or failure; no tight retries or overlapping tasks occur. Signal cancellation interrupts waiting/fetching and closes the connection, releasing the advisory lock. Logs contain only generic outcomes, interval and item count, never connection strings, item labels, provider response bodies or account data.

The scheduler is not a durable job queue. Migration 000014 persists each admitted attempt, its next allowed time and an allowlisted outcome before external fetching. A crash leaves STARTED with at least the configured interval of backoff. Rate-limit responses persist the greater of the interval and provider Retry-After, including cooldowns longer than one hour. A restarted worker checks that row before fetching. Success/failure outcomes never contain response bodies, connection details or selected item lists. Attempt counts saturate at one billion.

Persistent HTTP content caching, queued/per-title jobs and global quotas across different databases remain pending. If database connectivity is lost after HTTP responds, the longer provider cooldown may not be recorded; the pre-fetch interval remains durable and the in-process client retains its cooldown. Operators must respect source limits across databases/restarts and investigate source failures. No source is automatically enabled.

Wikidata facts remain sparse and retain item granularity, sometimes spanning seasons. They do not supply confirmed SUB/DUB availability, studio review, synopsis or reliable release status, so this job cannot invent episode alerts. Comprehensive availability collection and public deployment remain blocked on suitable data/operational review.
