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

A reviewed corpus, representative held-out judgments and a runtime lane remain open.
