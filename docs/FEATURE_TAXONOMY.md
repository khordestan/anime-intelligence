# Product Feature Taxonomy

This is an internal product taxonomy, not a copy of any provider's tags. The canonical keys are seeded by migration `000003_feature_taxonomy`.

## Categories

- **PROTAGONIST**: overpowered, underdog, antihero, genius, hidden identity, morally ambiguous.
- **POWER_SYSTEM**: progression, leveling system, skill tree, magic, supernatural abilities.
- **COMBAT**: strategic combat, escalating conflict.
- **NARRATIVE**: mystery, plot twists, conspiracy, psychological drama, political intrigue.
- **THEME**: survival, revenge, war.
- **SETTING**: dark fantasy, dystopia, medieval-inspired, post-apocalyptic.
- **PACING**: fast-paced, slow burn, episodic.
- **TONE**: dark, tragic, hopeful, comedy-forward.
- **STAKES**: high stakes.
- **CONTENT**: graphic violence, gore, fan service.
- **ENDING**: shocking revelation, bittersweet, open-ended, conclusive.

## Observation model

A taxonomy definition is not evidence that a particular anime has that feature. Anime-specific observations must be stored separately with:
- feature key and anime identifier
- value/strength and confidence
- source kind (provider, curated, model, or community)
- source/evidence and observation time
- scope (series, season, or arc)
- spoiler level

Ending and plot-revelation observations are spoiler-sensitive and must not appear in default discovery cards or explanations.

## Recommendation use

Feature values should be normalized to a consistent range and weighted by confidence. Explicit user preferences must remain distinct from inferred preferences. The recommender should explain positive matches and important mismatches rather than returning an opaque score.
