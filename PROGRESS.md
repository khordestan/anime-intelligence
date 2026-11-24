# Anime Intelligence implementation status

Audited 2026-11-24 against [main at c788693](https://github.com/khordestan/anime-intelligence/commit/c788693a0edc2243785acd2a1fed4fd19d4d2fc8), the feature code, migrations through `000023`, role scripts, API contract, dashboard and tests. Use [ROADMAP.md](ROADMAP.md) for requirements and the complete milestone checklist.

## What works

| Area | Implementation |
| --- | --- |
| Runtime | Go REST API, Next.js dashboard, loopback JSON dev mode and PostgreSQL multi-user runtime. Explicit migration runner and restricted runtime grants. |
| Identity | Registration, login, session refresh/revocation, password changes, export/deletion, verified-email product gates and persisted per-account failure budgets. Optional TLS SMTP verification/recovery through encrypted leased PostgreSQL queues. Forced transaction-local RLS and explicit owner predicates. |
| Catalog | Sourced details, category facets and keyset browsing, exact provider lookup, linked continuity and per-track schedules. Reviewed offline ingestion, Jikan conversion, optional observation archive, field arbitration, unused-ID retraction and provenance. Wikidata and TVmaze jobs are opt-in and idle without explicit IDs and approval. |
| Library | Status, ratings, count/exact progress, notes, viewing passes and rewatches, atomic bulk status/rating actions and add-only watch-list import with preview and export. PostgreSQL library features require a verified owner. |
| Releases | Confirmed episode and eligible linked-season notices, verified-owner calendar of unconfirmed schedules, track and scope settings, notification history, unread filter and bulk read. Latest-page polling creates bounded batches and reports the oldest event left pending. The dashboard uses the paged follows API and can look up a library title’s subscription off-page. |
| Intelligence | Deterministic Personal Match with explicit and rating-derived signals, grounded evidence, owner dismissals and offline evaluation. Discovery lists untracked and unhidden evidence-backed titles, with keyset paging, feature labels/filtering and an add-to-Planning action. |
| Operations | Liveness and readiness probes, redacted request logging, queue/catalog diagnostics, monitoring examples and synthetic database restore and credential-fencing tests. |

## Known issues and technical debt

- The open issue list at the audit checkpoint contained dependency update PRs and no reported runtime defects. That list is not evidence that the product is defect-free. Review dependency upgrades separately against the pinned Go and Node runtimes.
- The dashboard polls alerts only while visible on the latest page. There is no release mail or push path. Identity SMTP serves account verification and recovery.
- Starter catalog, source coverage and curated feature coverage are limited. Discovery orders by evidence coverage. Global Essentials has an offline ranker, evaluator and opt-in PostgreSQL dashboard lane. The lane requires a reviewed canonical-ID manifest. Learned feedback and the LLM analyst have no runtime path.
- Some historical runbook entries describe their original migration level. Apply all embedded migrations and current grants on new databases. The Personal Match OpenAPI description still describes title-ordered retrieval although the repository prioritizes feature overlap.
- Canonical merges, dependent corrections and generalized event retraction are pending. Operator approval and source provenance are prerequisites for wider catalog activation.
- Recovery and restricted role behavior are exercised on synthetic databases. Public ingress controls, an independent security review, production monitoring, real backup/key distribution and recovery objectives remain open. See [security](SECURITY.md), [identity security](docs/IDENTITY_SECURITY.md) and [backup recovery](docs/BACKUP_RECOVERY.md).

## Next development priorities

1. Build a reviewed public evidence corpus and representative held-out judgments for the [offline Global Essentials baseline](docs/GLOBAL_ESSENTIALS.md), then use the results to assess coverage and the ranking served by the opt-in runtime lane. Improve Discovery exploration using the offline ranking/retrieval evaluator.
2. Extend release alerts with a separately authorized channel and a bounded durable delivery design. Preserve consent, owner isolation, event deduplication and retry semantics. Start with synthetic end-to-end tests.
3. Expand catalog coverage through explicitly reviewed source capabilities and permissions, using the offline feed field-presence report and assessing field accuracy separately. Treat the current adapters and operator tools as the integration path.
4. Continue production hardening and independently verify authorization, abuse controls and recovery. Public deployment and live source credentials require an authorized environment. Continue synthetic/local work while those dependencies are pending.

## Verification checkpoint

- Discovery feature filtering [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45384609599) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45384609592) passed on the PR #38 head.
- Release backlog timestamp [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45385322503) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45385322497) passed on the PR #39 head.
- Follow pagination [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45385684810) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45385684474) passed on the PR #40 head `da89cbf`. Those links verify the PR workflow, while the implementation snapshot above names its squash merge.
- Handoff changes are documentation only. No local Go, Node or PostgreSQL suite was run during this audit. See the handoff commit and its Actions runs for validation after the documentation changes.
