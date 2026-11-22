# Evidence-backed Personal Match API

PostgreSQL mode exposes GET /api/v1/preferences, PUT/DELETE /api/v1/preferences/{featureKey}, and GET /api/v1/recommendations?limit=20. Every route requires a verified authenticated account. The API never accepts a target user ID. Reapply deployments/runtime-role.sql for feature-evidence reads and owner-scoped preference writes; startup checks the new evidence read grants.

Preference lists include active non-spoiler-sensitive features, ordered by key and bounded to 256. Weight=null means no declared preference. PUT accepts a finite weight from -1 (avoid) to +1 (prefer), including explicit neutral zero, with declared confidence one. DELETE removes only DECLARED origin; ratings remain intact. Existing export and account-deletion boundaries cover these preferences.

Ranking reads one consistent snapshot of the owner's declared preferences, newest 1000 series ratings and at most 500 untracked candidates with eligible evidence overlapping the owner's profile feature keys. Candidate retrieval prioritizes the number of distinct overlapping features, with canonical ID as the tie-breaker. The final ranking still evaluates compatibility and confidence. Only CURATED observations with a nonblank reference, SERIES scope, NONE spoiler level, positive confidence, nonfuture timestamp and active non-spoiler-sensitive taxonomy qualify. Latest eligible observation per title/feature wins. Unreviewed provider tags, community/model assertions and missing references do not become features automatically.

Own ratings derive signals in memory; no inferred profile is persisted. The response profile contains up to 256 key-ordered active non-spoiler features, with separate optional declared and inferred weight/confidence signals. Inferred signals include the number of the caller's series ratings that contributed eligible evidence for that feature. It contains no rated-title list or private record identifiers. These algorithmic strengths are not probabilities. Responses include bounded feature-key match/mismatch evidence, not raw references, free prose, credential fields or another account's records. Scores are deterministic compatibility scores, not calibrated probabilities. Ties use canonical IDs and feature keys.

Without supported evidence the result is empty. No real user history or fabricated taxonomy observations are seeded. The offline reviewed importer is available; real evidence coverage, editorial review, retrieval evaluation, global/discovery lanes, feedback learning and hybrid AI remain pending. No external AI call or public deployment is enabled.

## Dashboard

Verified PostgreSQL accounts can open Personal Match settings to edit/reset declared feature weights and inspect supported matches with feature-key evidence. Empty evidence is stated explicitly. Inputs and results stay in memory; keyed remounts, unmount guards and session/request revisions prevent late results crossing accounts. The panel distinguishes declared input from own-rating-derived direction/strength, explains that scores and strengths are not probabilities, and shows computation limits. It never calls an LLM.

Consistent preference/ranking reads retry at most three times for PostgreSQL serialization/deadlock conflicts, rechecking session/owner/verification on every attempt. Cancellation and other failures are not retried. Mutations are not part of this retry wrapper. This avoids transient failures when a concurrent session-row update invalidates a repeatable-read snapshot.

## Offline ranking evaluation

Run `go run ./cmd/evaluate-personal-match -k 10 < fixture.json` to compare Personal Match against ascending title order with canonical ID as the tie-breaker. Each case supplies its training ratings, preferences, candidates, optional exclusions, and held-out relevant candidate IDs. Already-rated and explicitly excluded titles cannot appear in either ranking or in the relevance labels. The evaluator uses the same ranking engine and accepts a preselected candidate set, so it measures ranking within that set. Catalog retrieval quality requires a separate evaluation.

The output gives recall@k and reciprocal rank@k per case and the mean across cases. Recall divides relevant hits by the number of relevant candidates. Reciprocal rank is the inverse of the first relevant result position within k, or zero if no relevant result appears. Use reviewed or synthetic fixtures without account identifiers or real watch histories.

```json
{"version":1,"cases":[{"id":"example","preferences":{"synthetic.feature":{"weight":1,"confidence":1}},"candidates":[{"animeId":"synthetic-title","title":"Synthetic title","features":[{"key":"synthetic.feature","value":1,"confidence":1}]}],"relevant":["synthetic-title"]}]}
```

Candidate retrieval only uses feature keys that can contribute to the compatibility score under the default blend. Neutral or zero-confidence signals can still appear in the profile when applicable.
