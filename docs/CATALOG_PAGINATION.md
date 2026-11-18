# Catalog offset navigation

GET /api/v1/anime retains items/count/limit/offset and now includes optional nextOffset and previousOffset. Keep the same q and metadata filters when using a returned offset.

nextOffset is emitted only when the returned page is full and its successor is within the existing 100,000 offset cap. It is a navigation hint: the next page can be empty, because no extra count/lookahead query is made. previousOffset moves back one requested page, clamped to zero. Missing nextOffset indicates a short page or the navigation cap.

Sorting remains canonical title/ID after filters. Independent requests do not share a snapshot: catalog changes may shift boundaries, producing duplicates or omissions across pages. This is not a stable cursor or a complete-result count. Client deduplication/restarting may be appropriate while ingestion runs.

No private data, schema/grants, source activation or deployment changes. Local and PostgreSQL HTTP routes share the same hints. Synthetic HTTP tests cover full/short/filtered-empty pages, prior-page clamping and the upper cap. Local toolchains unavailable; exact CI pending.

## Dashboard navigation

The dashboard uses the [cursor API](CATALOG_FILTERS.md#cursor-pagination). Previous reuses the earlier page's cursor, and Retry repeats the failed destination without discarding the current page. Starting a new search or applying filters clears the cursor trail. The offset API described above remains available to other clients.

Use Browse catalog to start at the first page without a query or filters. Search and applied filters still narrow the displayed results when browsing is enabled.
