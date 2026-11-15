# Evidence-backed Personal Match API

PostgreSQL mode exposes GET /api/v1/preferences, PUT/DELETE /api/v1/preferences/{featureKey}, and GET /api/v1/recommendations?limit=20. Every route requires a verified authenticated account. The API never accepts a target user ID. Reapply deployments/runtime-role.sql for feature-evidence reads and owner-scoped preference writes; startup checks the new evidence read grants.

Preference lists include active non-spoiler-sensitive features, ordered by key and bounded to 256. Weight=null means no declared preference. PUT accepts a finite weight from -1 (avoid) to +1 (prefer), including explicit neutral zero, with declared confidence one. DELETE removes only DECLARED origin; ratings remain intact. Existing export and account-deletion boundaries cover these preferences.

Ranking reads one consistent snapshot of the owner's declared preferences, newest 1000 series ratings and at most 500 title-ordered candidates outside every owned tracker entry. These are deliberate computation caps, not a global top-500 relevance retrieval strategy. Only CURATED observations with a nonblank reference, SERIES scope, NONE spoiler level, positive confidence, nonfuture timestamp and active non-spoiler-sensitive taxonomy qualify. Latest eligible observation per title/feature wins. Unreviewed provider tags, community/model assertions and missing references do not become features automatically.

Own ratings derive signals in memory; no inferred profile is persisted. Responses include bounded feature-key match/mismatch evidence, not raw references, free prose, credential fields or another account's records. Scores are deterministic compatibility scores, not calibrated probabilities. Ties use canonical IDs and feature keys.

Without supported evidence the result is empty. No real user history or fabricated taxonomy observations are seeded. Curated evidence ingestion/review, candidate retrieval/evaluation, global/discovery lanes, feedback learning and hybrid AI remain pending. No external AI call or public deployment is enabled.

## Dashboard

Verified PostgreSQL accounts can open Personal Match settings to edit/reset declared feature weights and inspect supported matches with feature-key evidence. Empty evidence is stated explicitly. Inputs and results stay in memory; keyed remounts, unmount guards and session/request revisions prevent late results crossing accounts. The panel explains that scores are not probabilities and shows computation limits. It never calls an LLM.
