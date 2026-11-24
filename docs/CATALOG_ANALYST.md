# Catalog evidence tool

In catalog details, **Ask about catalog facts** selects evidence for a question about the displayed title. It recognizes English terms related to format, release year, episode count, status, genres or studios. When the topic is outside this list, it shows the supported facts with an explicit fallback message.

`POST /api/v1/analysis/catalog` also accepts two distinct canonical IDs for side-by-side evidence. The request takes `question` and `animeIds`. Evidence uses resolved catalog fields and their reviewed field provenance when present. Blank evidence represents an unknown value. A known episode total of zero is returned as zero. Some starter catalog values have no external source. Long genre and studio lists are capped and marked truncated.

The route reads public catalog data through the same repository used by catalog details. It exposes a fixed field list and omits prose, spoiler tags, schedules and all account records. The current engine uses deterministic word matching and does not contact a model provider. A later provider can choose from typed evidence tools while keeping catalog reads, permissions and answer provenance in the application.
