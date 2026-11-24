# Recommendation Engine — Personal Match v1

The first deterministic ranking primitive lives in `internal/modules/intelligence`. It is deliberately independent of HTTP and persistence so the scoring behavior can be unit-tested before being connected to user data.

## Profile construction

Ratings are converted into a bounded signal:

`ratingSignal = (rating - 3) / 2`

Thus 5 is positive, 3 is neutral, and 1 is negative. Each anime's feature observation contributes only in proportion to its feature confidence. Aggregated weights are normalized and kept in the range [-1, 1].

Explicit preferences and inferred preferences are stored separately. If both exist for a feature, explicit preference receives 70% weight and inferred preference receives 30% by default. If only one source exists, it is used without dilution.

## Candidate score

For each candidate, only observed features with non-zero confidence are considered:

`score = 50 + 50 * sum(preferenceWeight * featureValue * reliability) / sum(abs(preferenceWeight) * reliability)`

where `reliability = featureConfidence * preferenceConfidence`. The final score is clamped to [0, 100].

A score is a relative compatibility signal, not a probability that the user will like an anime. Candidates without usable evidence are omitted rather than assigned fabricated scores.

## Explainability and safety

- Each result includes the algorithm version.
- Positive and negative feature contributions are returned as evidence.
- Watched/excluded anime IDs are filtered before ranking.
- Duplicate candidates are removed.
- Spoiler-sensitive features must not be sent to default recommendation surfaces.
- The engine does not invent catalog facts; it consumes feature observations supplied by the application.

## Runtime scope

The verified PostgreSQL API loads curated observations, derives profile signals, retrieves candidates and runs the ranking core. The dashboard exposes Personal Match, dismissals and Discovery. Discovery has its own evidence-coverage order and supports feature filtering and pagination. See [runtime recommendations](RECOMMENDATIONS.md) for eligibility, limits and contracts.

The offline evaluator compares ranking and retrieval against explicit held-out judgments. The [Global Essentials offline baseline](GLOBAL_ESSENTIALS.md) evaluates public recognition independently of personal fit. A runtime Essentials lane, richer exploration and feedback learning remain on the [roadmap](../ROADMAP.md).
