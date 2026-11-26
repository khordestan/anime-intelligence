# Evidence-backed Personal Match API

PostgreSQL mode exposes GET /api/v1/preferences, PUT/DELETE /api/v1/preferences/{featureKey}, and GET /api/v1/recommendations?limit=20. Every route requires a verified authenticated account. The API never accepts a target user ID. Reapply deployments/runtime-role.sql for feature-evidence reads and owner-scoped preference writes; startup checks the new evidence read grants.

Preference lists include active non-spoiler-sensitive features, ordered by key and bounded to 256. Weight=null means no declared preference. PUT accepts a finite weight from -1 (avoid) to +1 (prefer), including explicit neutral zero, with declared confidence one. DELETE removes only DECLARED origin; ratings remain intact. Existing export and account-deletion boundaries cover these preferences.

Ranking reads one consistent snapshot of the owner's declared preferences, newest 1000 series ratings and at most 500 untracked candidates with eligible evidence overlapping the owner's profile feature keys. Candidate retrieval prioritizes the number of distinct overlapping features, with canonical ID as the tie-breaker. The final ranking still evaluates compatibility and confidence. Only CURATED observations with a nonblank reference, SERIES scope, NONE spoiler level, positive confidence, nonfuture timestamp and active non-spoiler-sensitive taxonomy qualify. Latest eligible observation per title/feature wins. Unreviewed provider tags, community/model assertions and missing references do not become features automatically.

Own ratings derive signals in memory; no inferred profile is persisted. The response profile contains up to 256 key-ordered active non-spoiler features, with separate optional declared and inferred weight/confidence signals. Inferred signals include the number of the caller's series ratings that contributed eligible evidence for that feature. The inferred signal may include a short list of the caller’s contributing title ratings. It omits private database record identifiers. These algorithmic strengths are not probabilities. Responses include bounded feature-key match/mismatch evidence, not raw references, free prose, credential fields or another account's records. Scores are deterministic compatibility scores, not calibrated probabilities. Ties use canonical IDs and feature keys.

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

The evaluator also reports binary nDCG@k and fill rate for both rankings. nDCG gives more credit to relevant titles near the top and normalizes by the ideal order at the same cutoff. Fill rate is the number of returned titles divided by k, so a result set with insufficient eligible evidence can be distinguished from a full list with poor relevance.

Set `candidateLimit` on the v1 suite to a value from 1 to 500 to evaluate the retrieval cap as well. The evaluator accepts up to 2,000 fixture candidates per case in this mode. It orders candidates by distinct features that can contribute to the profile score, then by canonical ID, before applying the cap. Fixtures supply already-eligible, current evidence. The report adds the retrieved candidate count and the share of held-out relevant titles retrieved per case, plus mean retrieval recall. The title baseline still uses the full eligible fixture. Omitting the limit preserves ranking-only evaluation.

## Hide a Personal Match

PUT /api/v1/recommendations/dismissals/{animeId} hides a canonical catalog title from the verified caller's Personal Match. DELETE on the same resource restores eligibility. Both operations are idempotent. The dashboard can undo its latest hide during the current session. Migration 000023 and refreshed runtime grants are required.

GET /api/v1/recommendations/dismissals lists the verified caller's hidden titles in catalog UUID order, up to 50 per page. Pass `nextCursor` as `after` to continue. The dashboard's Manage hidden matches control lists and restores these saved titles.

## Evaluate Discovery ranking offline

Run `go run ./cmd/evaluate-discovery -k=10 < suite.json` with a version 1 suite. A case has an `id`, `knownFeatures` from its fixture library, candidate IDs and current eligible positive `features`, and independently judged `relevant` candidate IDs. Set `excluded` on tracked or hidden candidates and supply an optional `feature` to match a Discovery filter.

Coverage ranks all eligible titles by positive feature count. Unfamiliar mode ranks titles by the number of features absent from the fixture library and omits titles with zero unfamiliar features. Both break ties by candidate ID and use the same judged relevant set to compute recall, rank and fill metrics. Use synthetic or approved fixtures. Representative evaluation still needs independent held-out judgments and a catalog with enough reviewed coverage.

## Discovery

GET /api/v1/recommendations/discovery returns untracked, unhidden titles for the verified caller. It works before any preferences or ratings are available. Titles are ordered by the number of distinct positive features supported by their latest eligible curated observations, with canonical ID as a stable tie-breaker. Evidence counts describe catalog coverage. The endpoint defaults to 20 titles and accepts limits from 1 to 50. Hiding or restoring a title affects its eligibility in both Personal Match and Discovery.

Discovery includes `nextCursor` when more titles are available. Pass its `animeId` as `after` and its `evidenceCount` as `afterCount`. The dashboard appends further pages and deduplicates titles if catalog evidence changes between requests.

Discovery shows up to four feature names per title, ordered by the product of the curated feature value and confidence, with feature key breaking ties. Names come from the same eligible positive observations used for the evidence count. The response also includes paired keys and names in `featuredEvidence`, which the dashboard uses to apply that exact feature filter from a result.

Use `featureKey` to require positive evidence for a feature from the preferences taxonomy. Other eligible features still contribute to the title's evidence count and ranking. For filtered pagination, send the same `featureKey` and the cursor's `featureKey` as `afterFeature`. Unknown keys have no results.


Select **Explore features missing from my library** or pass `unfamiliar=true` to rank by the number of supported features with no positive eligible evidence on titles you have saved. A title needs at least one such feature. The mode uses only the verified caller's library and does not interpret absence as a dislike. The existing feature filter still applies. Featured labels show unfamiliar evidence first and each row reports `unfamiliarCount` as well as the full `evidenceCount`.

A cursor in this mode carries `unfamiliar: true` and its `evidenceCount` refers to the unfamiliar ranking count. Send that count as `afterCount` and send `afterUnfamiliar=true` together with `unfamiliar=true`. Switching modes starts from the first page. With an empty or unevidenced library, every eligible positive feature is unfamiliar.

Choose a feature in Discovery to prefer, avoid or clear it for Personal Match. Prefer and Avoid set the declared weight to +1 or −1. Clear removes the declared weight. The preference editor still accepts intermediate weights. The same verified-owner preference record drives Personal Match and appears in account exports.

The **Personal Match results limit** selector requests up to 10, 20, 50 or 100 matches. The current result stays visible after a failed change. The selection is retained while the account dashboard is mounted.

Discovery’s **More like this** and **Less like this** save a private reaction to a title. Select an active reaction again to clear it. **Manage title feedback** includes reactions to titles you later tracked or hid. Personal Match uses the 100 most recent reactions to build fallback signals from eligible positive features. Declared preferences and rating-derived signals take precedence for each feature. The dashboard shows the number of reactions contributing to a fallback signal. Reactions are included in account export and deleted with the account.

### Evaluate Discovery reactions

A v1 Personal Match evaluation case can include up to 100 earlier title reactions under `feedback`. Each reaction supplies a unique `animeId`, a `direction` of 1 for a positive reaction or -1 for a negative reaction, and its eligible `features`. Reaction titles must be outside the case's candidate set, keeping the held-out judgments separate from training.

The existing Personal Match metrics run with ratings and declared preferences. When a case supplies reactions, the evaluator runs the same candidate selection and ranking again with title feedback applied. It reports the paired means across cases with reactions and, when a retrieval cap is set, retrieval metrics for both runs. Use synthetic or approved evidence and judgments. A representative longitudinal dataset remains open.

In **Your feature preferences**, expand **Recent ratings behind** a feature to see up to three of your own series ratings behind its inferred signal. The sample is ordered by rating recency and drawn from the same eligible observations as the displayed supporting count. A neutral series rating does not contribute. The list represents examples from the inference, and a declared preference can take precedence for ranking.
