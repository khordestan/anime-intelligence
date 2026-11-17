# Opt-in Wikidata factual synchronization

The worker schedules durable per-title jobs for explicitly configured Wikidata Q IDs. It remains idle without network/database access when CATALOG_SYNC_IDS is empty. No host service, source or schedule is activated by the repository.

## Upgrade and configuration

1. Apply migration **000016** and reapply deployments/catalog-writer-role.sql before updating the worker. The API still requires its existing migration 000015/security contract; no new API/runtime grants are added.
2. Provision the dedicated writer role with its password out of band. CheckWriter rejects elevated/table-owner roles, role memberships and private-data access. Do not use API or administrator credentials.
3. Independently review the CC0 factual scope in PROVIDERS.md and register provider_key=wikidata with a nonempty reviewed policy_url and enabled=true. Only the administrator may approve a source; the queue cannot enable it.
4. Supply CATALOG_DATABASE_URL (DATABASE_URL is never a fallback), CATALOG_SYNC_IDS (up to twenty distinct supported Q IDs) and optionally CATALOG_SYNC_INTERVAL (default 24h, minimum 1h, maximum 7d).
5. Run go run ./cmd/worker in a private environment. It polls once per minute, serially, with a separate connection and a two-minute context per cycle. The interval is the per-title refresh target, not the polling cadence.

The writer has public catalog insert/read/update permissions, without DELETE, ownership or private-table rights. Narrow UPDATE(display_name) on data_sources and UPDATE(external_url) on external IDs support row locks without granting source approval or identity remapping. Provisioning does not remove existing excessive grants; review them separately.

## Durable per-title lifecycle

Each configured ID has one row in catalog_fetch_jobs. Scheduling inserts a missing row and updates its refresh interval without overriding its due time, retry budget, lease or terminal failure. Removing an ID from configuration makes its retained job dormant; it is not deleted or processed. Re-adding it resumes the retained state. Changing the interval affects future completions and does not bring an already scheduled job forward.

At most one due title is claimed per cycle in due-time/ID order. The shared session advisory lock coordinates legacy and queued collectors on this database. Source approval is checked at admission and again before import. No source/job row transaction remains open during HTTP. A job receives a three-minute lease with a random digest and consumes one of five claims, including claims lost to crashes. Provider admission is committed before HTTP and prevents another request for at least that lease window after a crash.

Success atomically commits the existing catalog import (canonical data, provenance, schedule/event rules and source high-water mark), a fenced job acknowledgement, reset retry budget and next per-title due time. Import/acknowledgement failure rolls the entire transaction back. Expired or replaced claims cannot import facts or acknowledge a newer claim. SQL statements have a two-second timeout and cleanup is bounded.

After success the provider is paced for one minute; every cycle still waits at least one minute. Ordinary title failures persist retry delays of 15/30/60/120 minutes, without imposing the entire title delay on other due titles. The admission's existing three-minute source floor remains conservative after failures. Provider Retry-After extends the durable source cooldown and the failed title's retry time; new IDs and restarted workers cannot bypass it. Existing catalog_fetch_state cooldowns, including legacy cooldowns, remain honored on upgrade.

Five failed or crashed claims mark a job FAILED. That job stays dormant while other titles proceed. An operator must investigate before explicitly resetting its state to PENDING, attempts to zero, lease fields to NULL and next_attempt_at to a reviewed retry time using administrator credentials. Configuration changes never silently reset exhausted jobs.

Signal cancellation interrupts waits/fetches. A bounded independent cleanup context attempts to persist a failure even after cancellation; if the process or connection dies, the committed lease/admission permits recovery after expiry. Failure to record an HTTP Retry-After during database loss remains an operational limitation: only pre-fetch admission and the process-local client cooldown are guaranteed. Respect provider limits across different databases; there is no cross-database quota.

## Scope and operational limits

Rows retain explicit public catalog IDs, times, counters, states and lease digests, never provider payloads, response/error strings, private watch lists or credentials. Logs contain generic outcomes, refresh interval and item count, never IDs, labels or connection strings. API credentials receive no queue access. Catalog diagnostics still report provider attempts and metadata freshness; a dedicated queue-inspection CLI/metrics view and operational pruning remain future work.

The worker remains bounded to twenty explicitly reviewed IDs and the existing Wikidata adapter. This is a durable queue for that approved adapter, not comprehensive discovery, a licensed availability adapter or a complete/current catalog. Persistent HTTP caching and broader provider adapters remain pending.

Supported explicit TV-series/TV-season/movie/OVA/ONA classes retain their own identities and sparse facts. No season graph, release status or confirmed SUB/DUB availability is inferred. Existing notification baseline/confirmation rules remain unchanged. Live collection and public deployment are not activated by these changes.
