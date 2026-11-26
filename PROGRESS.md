# Anime Intelligence implementation status

Updated 2026-11-26 through [main at d4399e5](https://github.com/khordestan/anime-intelligence/commit/d4399e5). Use [ROADMAP.md](ROADMAP.md) for the full milestone checklist.

## What works

| Area | Implementation |
| --- | --- |
| Runtime | Go REST API, Next.js dashboard, loopback JSON dev mode and PostgreSQL multi-user runtime. Explicit migration runner and restricted runtime grants. |
| Identity | Registration, login, session refresh/revocation, password changes, export/deletion, verified-email product gates and persisted per-account failure budgets. Optional TLS SMTP verification/recovery through encrypted leased PostgreSQL queues. Forced transaction-local RLS and explicit owner predicates. |
| Catalog | Sourced details, category and provider facets, confirmed-release filtering and keyset browsing, exact provider lookup, linked continuity and per-track schedules. Reviewed offline ingestion, Jikan conversion, optional observation archive, field arbitration, unused-ID and reviewed continuity retractions, and canonical merge impact reports. Wikidata and TVmaze jobs are opt-in and idle without explicit IDs and approval. |
| Library | Status, ratings, count/exact progress, notes, viewing passes and rewatches, atomic bulk status, rating and count-mode progress actions and add-only watch-list import with preview and export. PostgreSQL library features require a verified owner. |
| Releases | Confirmed episode/movie, eligible linked-season and opted-in schedule moves, verified-source delay reports, and reviewed title cancellations and corrections, verified-owner calendar of unconfirmed schedules, track and scope settings, notification history, unread filter and bulk read. Latest-page polling creates bounded batches and reports the oldest event left pending. The dashboard uses the paged follows API and offers opt-in background email when operators configure release SMTP. |
| Intelligence | Deterministic Personal Match with explicit and rating-derived signals, grounded evidence, owner dismissals and offline evaluation. Discovery lists untracked and unhidden evidence-backed titles, with keyset paging, feature labels/filtering explicit feature preference feedback, an add-to-Planning action and an offline evaluator for both ranking modes. |
| Operations | Liveness and readiness probes, redacted request logging, queue and catalog diagnostics, opt-in scheduler alert rules and collector examples, checksummed image bundles and same-origin smoke checks, a concurrent HTTP workload, and synthetic logical and physical database recovery drills. |

## Known issues and technical debt

- The open issue list at the audit checkpoint contained dependency update PRs and no reported runtime defects. That list is not evidence that the product is defect-free. Review dependency upgrades separately against the pinned Go and Node runtimes.
- The dashboard polls alerts only while visible on the latest page. Background release email requires explicit SMTP configuration and owner consent. Push delivery remains open. Identity SMTP serves account verification and recovery.
- Starter catalog, source coverage and curated feature coverage are limited. Discovery supports evidence coverage and unfamiliar-feature ordering. Global Essentials has an offline ranker, evaluator and opt-in PostgreSQL dashboard lane. The lane requires a reviewed canonical-ID manifest. A deterministic catalog evidence tool is available in title details. Model-assisted topic selection is available with explicit operator configuration and verified-owner opt-in. Learned feedback and broader analyst tools remain open.
- Some historical runbook entries describe their original migration level. Apply all embedded migrations and current grants on new databases.
- Canonical merges, dependent corrections and generalized event retraction are pending. Operator approval and source provenance are prerequisites for wider catalog activation.
- Recovery and restricted role behavior are exercised on synthetic databases. Public ingress controls, an independent security review, production monitoring, real backup/key distribution and recovery objectives remain open. See [security](SECURITY.md), [identity security](docs/IDENTITY_SECURITY.md) and [backup recovery](docs/BACKUP_RECOVERY.md).

## Next development priorities

1. Build a reviewed public evidence corpus and representative held-out judgments for the [offline Global Essentials baseline](docs/GLOBAL_ESSENTIALS.md), then use the results to assess coverage and the ranking served by the opt-in runtime lane. Improve Discovery exploration using the offline ranking/retrieval evaluator.
2. Extend release alerts with a separately authorized channel and a bounded durable delivery design. Preserve consent, owner isolation, event deduplication and retry semantics. Start with synthetic end-to-end tests.
3. Expand catalog coverage through explicitly reviewed source capabilities and permissions, using the offline feed field-presence report and assessing field accuracy separately. Treat the current adapters and operator tools as the integration path.
4. Continue production hardening and independently verify authorization, abuse controls and recovery. Public deployment and live source credentials require an authorized environment. Continue synthetic/local work while those dependencies are pending.

## Verification checkpoint

- The release bundle passed image reload, identity, non-root startup and same-origin ingress checks in its [hosted workflow](https://github.com/khordestan/anime-intelligence/actions/runs/45623784315).
- Reviewed continuity withdrawal, stale-review guards and logical restore with an exact migration ledger passed [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45621832418).
- The catalog provider filter and additional analyst topics passed their full hosted checks before merging. [Provider filter CI](https://github.com/khordestan/anime-intelligence/actions/runs/45625482233) · [Analyst topics CI](https://github.com/khordestan/anime-intelligence/actions/runs/45625810430).

- Reviewed title cancellation publication passed [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45617693989) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45617694054).
- The committed container build passed [Container smoke](https://github.com/khordestan/anime-intelligence/actions/runs/45618401345), [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45618401237) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45618401232). The local dependency build used the Cloud certificate through temporary BuildKit secrets.
- The confirmed-release filter passed focused local PostgreSQL and browser checks, plus [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45619023571) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45619023618).
- Three local runs of the mixed HTTP workload passed with the race detector. The same PostgreSQL test passed in [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45619025346), and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45619025331) passed.
- Release-email alert rules and atomic collector output passed local Prometheus verification. Hosted checks verify the [alert rules](https://github.com/khordestan/anime-intelligence/actions/runs/45618970795) and [collector unit syntax](https://github.com/khordestan/anime-intelligence/actions/runs/45619529062).
- Discovery feature filtering [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45384609599) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45384609592) passed on the PR #38 head.
- Release backlog timestamp [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45385322503) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45385322497) passed on the PR #39 head.
- Follow pagination [CI](https://github.com/khordestan/anime-intelligence/actions/runs/45385684810) and [Security](https://github.com/khordestan/anime-intelligence/actions/runs/45385684474) passed on the PR #40 head `da89cbf`. Those links verify the PR workflow, while the implementation snapshot above names its squash merge.
