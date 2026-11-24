# Global Essentials offline baseline

Global Essentials ranks titles by documented public recognition. V1 accepts two kinds of title-level evidence:

- `JURIED_AWARD_WIN` records a win for the specific work in a professionally adjudicated award category. Use the organizer's official published results, such as the Academy's Animated Feature Film results. [Academy Awards results](https://www.oscars.org/oscars/ceremonies/2003)
- `INSTITUTIONAL_SELECTION` records formal selection of the specific work for a curated preservation registry. Use the institution's official selection record. [Library of Congress National Film Registry](https://www.loc.gov/programs/national-film-preservation-board/film-registry/)

Use one stable publisher ID per independent awarding or selecting body, including when it publishes records on multiple sites. Review the exact catalog mapping and the source's use terms under the [data source policy](DATA_SOURCE_POLICY.md). The schema validates the shape of a fixture. Reviewers verify each assertion and publisher identity. Store only permitted factual assertions and citations. The CLI makes no requests to the cited URLs.

Titles with at least one qualifying item rank by distinct evidence kinds descending, then distinct publishers descending, then canonical ID ascending. Repeated citations cannot increase either count. Each result includes the counts and the reviewed evidence that produced them. No account data, popularity counts, genre preferences, aggregate ratings or Discovery coverage scores enter the ranking.

The first baseline favors titles represented in published award and preservation records. It will underrepresent some eras, regions and releases, especially series. A representative evaluation needs independently assigned judgments across these categories. Include relevant titles with no qualifying evidence to measure how much the evidence gate costs in recall.

## Reproduce a synthetic evaluation

```sh
go run ./cmd/evaluate-global-essentials -k 1 <<'JSON'
{
  "version": 1,
  "candidates": [
    { "animeId": "a", "title": "A Title", "evidence": [] },
    {
      "animeId": "b",
      "title": "B Title",
      "evidence": [
        {
          "kind": "JURIED_AWARD_WIN",
          "publisherId": "synthetic-awards",
          "sourceUrl": "https://awards.example/results"
        }
      ]
    }
  ],
  "relevant": ["b"]
}
JSON
```

The report includes the ranked results plus nDCG, fill rate, recall and reciprocal rank at k. The title-ordered baseline ranks the full candidate pool, including unsupported titles. This example gives Global Essentials recall of 1 and title-order recall of 0 at k=1. Bounds are 8 MiB per fixture, 2,000 candidates, 20 evidence records per title and k from 1 to 100.

## Enable the dashboard lane

Set `GLOBAL_ESSENTIALS_FILE` to a local JSON file with `version: 1` and the same `candidates` array as the evaluator. Omit `relevant` from this runtime manifest. Each of its 1–100 candidates must carry at least one reviewed evidence record. The manifest is capped at 1 MiB.

On PostgreSQL API startup, each candidate's ID and title must match the canonical catalog. Invalid manifests fail startup without logging their contents. The API serves the resulting snapshot until restart through `GET /api/v1/recommendations/essentials`. The dashboard shows the lane to signed-out and signed-in users when runtime metadata advertises it. Restart after changing the manifest or reconciling catalog references. An unset file leaves the lane disabled.

The repository ships no live corpus. A representative corpus and held-out judgments remain open.

## Measure evidence coverage

The evaluator reports how many candidates and held-out relevant candidates have qualifying evidence. The recall ceiling at k is the smaller of k and the number of eligible relevant titles, divided by all relevant titles. Compare the ceiling with measured recall to distinguish limited evidence coverage from ordering mistakes.

A five-film pilot used Academy Animated Feature Film wins as the only input and membership in the published 2022 Sight and Sound critics' results as the held-out signal. The cohort deliberately included one film with an Academy win and poll coverage, two additional films in the poll, and two other films released before the poll. At k=3, the ranker filled one slot and retrieved one of three relevant titles, meeting its 0.333 recall ceiling. Alphabetical ordering retrieved two of three. This small, film-only pilot measured the effect of the single source chosen for the trial. It leaves the quality of the ranking across eras, regions, studios and TV series open. [Academy ceremony results](https://www.oscars.org/oscars/ceremonies/2003) · [Sight and Sound critics' poll](https://www.bfi.org.uk/sight-and-sound/greatest-films-all-time)
