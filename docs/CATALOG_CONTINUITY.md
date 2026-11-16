# Bounded linked-title browsing

GET /api/v1/anime/{id}/continuity is a public read in both storage modes. It returns rootId, nodes, edges, limit (50), and truncated. Invalid UUIDs return 400, missing titles 404, and failed reads a fixed 500 response without query/provider internals. Existing no-store/security response headers apply. There are no account-data queries or catalog writes.

Only explicit PREQUEL/SEQUEL relations establish membership, traversed in either direction. No title-name, year, part/season number, side-story, alternative or parent inference is used. Nodes include the selected title and up to 49 reachable titles; each reports its shortest unweighted distance from the root, canonical name, media/status, known-or-unknown episode total, available year/quarter and selected metadata source/observation time. No synopsis, tags, raw source records or watch history is returned.

Edges normalize PREQUEL/SEQUEL into prequel-to-sequel direction and combine reciprocal claims for the same directed pair. Distinct non-null provider keys are retained as sorted source labels; an empty source list means no source is attached, not approval or independent corroboration. A source observation time describes the selected title metadata, not the age of every edge.

## Limits and consistency

PostgreSQL reads one repeatable-read read-only snapshot. Breadth-first expansion requests only the remaining node capacity plus one candidate at each frontier. Results order nodes by distance then canonical UUID, so equally distant choices have deterministic limits. When an additional reachable title is found beyond capacity, truncated is true. Exactly 50 titles can still be a complete retained component. Only edges wholly inside the returned node set are included; reciprocal claims are deduplicated. At most 2,450 directed non-self node pairs can be returned (normal importer-validated acyclic graphs have at most 1,225).

Each statement has a two-second timeout and the complete repository read a three-second context deadline. Large/dense source data may time out instead of producing a partial response; the cap bounds returned/decoded records rather than guaranteeing constant database work. The visited set terminates even with manually inserted cycles; normal imports still reject invalid continuity. No schema or privilege expansion is required.

Local JSON mode returns its selected starter title and empty edges; no relationships are invented.

## Dashboard

Open catalog details, then Browse linked titles. Loading/errors/retry/close are explicit. The panel displays known counts separately from unknown totals, metadata and edge source labels, distance, and a partial-view notice when capped. Node buttons open that title's details; browsing from it starts a new bounded component view. Reported links render twenty at a time with a Show more control. Closing/navigation discards late requests; account/client revision fencing remains in place.

The view may omit franchise titles and is not a recommended watch order. Distance is relative to the selected title, not a season number. This endpoint does not change follow expansion, tracker progress, confirmation rules or notification materialization. The existing FRANCHISE follow still expands its complete explicit continuity component independently of this display cap. See [release alerts](RELEASE_ALERTS.md).

Reviewed franchise membership, graph corrections/retractions, finer season conventions and complete licensed catalog coverage remain separate work. No live source or deployment is activated.
