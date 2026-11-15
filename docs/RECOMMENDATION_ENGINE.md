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

## Current scope and next integration work

Implemented: profile inference, explicit/inferred preference separation, personal-match ranking, evidence output, limits, exclusion and unit tests.

Not yet implemented: loading observations from PostgreSQL, user-specific HTTP endpoints, global-essential ranking, discovery ranking, feedback learning, and offline evaluation. Those should be added only after source quality and provenance are available.
