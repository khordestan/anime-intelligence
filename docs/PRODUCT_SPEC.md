# Product Specification — Anime Intelligence

## Vision
A trustworthy anime intelligence product combining personal tracking, rich metadata, taste analytics, explainable recommendations, discovery, and release notifications.

## Product areas
1. Tracker — lists, statuses, progress, ratings, notes, and season-level tracking.
2. Catalog — canonical anime, seasons, franchises, relations, studios, staff, characters, genres, themes, and nuanced features.
3. Taste intelligence — declared and inferred preferences with confidence, evidence, and history.
4. Recommendations — Personal Match, Global Essentials, and Discovery Picks.
5. AI analyst — natural-language interface over constrained application tools and evidence.
6. Discovery — feedback loop to learn preferences.
7. Notifications — season announcements, release dates, episodes, delays, and movies.

## Non-functional requirements
- Multi-tenant isolation from the first production migration.
- Field-level provenance for imported and inferred catalog data.
- Idempotent ingestion and notification processing.
- Provider-specific rate limits, retries, and circuit breakers.
- No silent overwriting of conflicting observations.
- Explainable recommendations with algorithm versioning.
- Spoiler-sensitive data marked and hidden by default.
- User data export and deletion before public launch.

## Recommendation categories
- `PERSONAL_MATCH`: compatibility with demonstrated and declared taste.
- `GLOBAL_ESSENTIAL`: influential or acclaimed works, independent of personal fit.
- `DISCOVERY`: deliberate exploration outside established comfort zones.

## AI boundaries
The LLM cannot query arbitrary SQL or directly mutate canonical data. It uses typed application tools. Anime facts must be grounded in catalog records or cited evidence. Model-generated features remain labeled as model-derived until reviewed or corroborated.
