# Data source and licensing policy

API availability does not grant rights to persist, redistribute, commercially exploit, or use data for model training. Every provider integration must be reviewed against its current terms before implementation.

## Integration checklist
- Confirm API terms, attribution, rate limits, caching and retention rules.
- Confirm commercial use and redistribution/display rights.
- Confirm whether AI/ML processing or training is allowed; do not assume it is.
- Record source, retrieval time, provider identifier, and field-level provenance.
- Keep provider payloads only when permitted.
- Resolve conflicts explicitly; never silently overwrite a verified field.
- Separate provider-confirmed facts from inferred or LLM-enriched observations.

## Pipeline
Provider adapters -> permitted source records -> normalization -> entity resolution -> validation/conflict handling -> canonical catalog -> product features.

## Quality
Track freshness, coverage, confidence, source and validation state separately. An absent record or provider outage must never be interpreted as proof that an anime or airing event does not exist.
