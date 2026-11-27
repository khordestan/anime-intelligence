# Catalog evidence tool

In catalog details, **Ask about catalog facts** selects evidence for a question about the displayed title. Open **Compare with another title** to search the catalog, select a second title and view the evidence side by side. It recognizes English terms related to format, release year or quarter, episode count, status, native title, genres, studios, demographics, reviewed staff credits, confirmed releases, upcoming schedules or direct prequel and sequel links, or sourced installment numbering. When the topic is outside this list, it shows the supported facts with an explicit fallback message.

`POST /api/v1/analysis/catalog` also accepts two distinct canonical IDs for side-by-side evidence. The request takes `question` and `animeIds`. Evidence uses resolved catalog fields and their reviewed field provenance when present. Blank evidence represents an unknown value. A known episode total of zero is returned as zero. Some starter catalog values have no external source. Long genre and studio lists are capped and marked truncated. When both titles have a known episode count or release year, the API includes a numeric difference for that field. The dashboard describes the difference beside the underlying facts. Unknown values never participate in a comparison.

The route reads public catalog data through the same repository used by catalog details. It exposes a fixed field list and omits prose, spoiler tags and all account records. The default engine uses deterministic word matching and does not contact a model provider.

## Evaluate topic selection offline

Run `go run ./cmd/evaluate-catalog-selector < suite.json` with a version 1 suite of cases. Each case supplies a synthetic or approved `id`, `question` and `expectedFields`. An empty expected list means abstain. Optional `modelFields` records a prior model selection for the same question. The report counts exact matches, abstentions and invalid model selections and calculates mean precision and recall. Empty predictions match empty judgments. Unknown and duplicate model keys count as invalid and score zero. The command reports case IDs and scores without echoing questions or contacting a provider.

## Model-assisted field selection

A PostgreSQL operator can set `ANALYST_OPENAI_MODEL` and `ANALYST_OPENAI_API_KEY` together to enable model-assisted topic selection with a model that supports Structured Outputs in the Responses API. Both values are unset by default. The model receives the submitted question, instructions and the allowed field names. The application sends no title IDs, catalog records, account identifiers or library data. The request uses `store: false`. The checkbox in catalog details is off by default and names the destination before submission. [OpenAI Structured Outputs](https://developers.openai.com/api/docs/guides/structured-outputs)

The assisted endpoint uses a verified session. It returns only evidence projected by the application from the selected titles and reports the model ID. Unknown and duplicate model-selected fields fail the request. The deterministic route remains available for supported topics and provider outages. Each process admits at most two simultaneous provider calls. Replicas sharing the same PostgreSQL database reserve one of twelve calls for the deployment and one of three calls for the owner in the same transaction, within fixed one-minute windows. A denied owner consumes no deployment allowance. Calls count once reserved, including provider errors. Provider requests time out after eight seconds.

Apply migration 000025 and refresh the restricted runtime grants before upgrade. Deployments retain one bounded counter per owner and one deployment counter, with no question content. Owner counters are included in account export and removed on account deletion. Retire prior versions with process-local limits before relying on shared limits.

When both returned evidence lists contain the same genre or studio label, the comparison includes that value in `sharedEvidence`. Matching ignores case and keeps the first title's spelling. The dashboard shows these shared values above the title evidence and its sources. A missing shared value may reflect unknown or truncated evidence.

Release evidence lists up to 20 of the latest confirmed observations at or before the response time, with attribution per episode and track. Upcoming schedules show up to 20 future predictions, soonest first. They are labelled unconfirmed. Both use the latest catalog revision per episode and track.

Continuity evidence includes up to 20 direct prequel or sequel links and their recorded source labels. A limited result can reflect the evidence cap or the catalog graph cap. Link direction describes the recorded relationship and does not suggest a viewing order.

Open **Narrow to release evidence** to choose an episode or a release track for the selected titles. The API accepts the optional `releaseScope` object with `episodeNumber`, `track`, or both. It applies the scope before limiting the evidence and uses only confirmed-release and upcoming-schedule topics. The assisted provider receives the question and allowed field names as usual. The application applies the episode and track selections locally.

Staff evidence shows up to 20 reviewed name and role pairs from the selected title's retained primary observation, with the source and observation time where recorded. A director or writer question selects the staff topic. The response displays stored roles and leaves missing credits unknown. Character credits use the separate spoiler disclosure in catalog details.

Installment evidence shows the primary source's season number and optional split-part number. A question about the season can show both the installment number and the separately sourced airing quarter. A missing season number leaves installment evidence unknown.

Choose an **Evidence topic** in the dashboard to retrieve those facts directly without writing a question. API clients can instead send one to four distinct `topics` in a deterministic request with `animeIds`. The release scope is available for the confirmed-release and upcoming-schedule topics. Model-assisted topic selection uses a question.

The OpenAPI contract describes the same typed evidence response on both analysis routes, including staff credits, installment numbering, episode timestamps and continuity links. [OpenAPI](../api/openapi.yaml)
