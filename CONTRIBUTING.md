# Contributing

Thanks for helping improve Anime Intelligence. The project is public and evolving; its current development status and limitations are documented in [README.md](README.md).

## Before opening a pull request

1. Open an issue first for substantial changes so the product and data model stay coherent.
2. Keep changes focused and explain the problem they solve.
3. Add or update tests for behavior changes.
4. Update the relevant documentation and OpenAPI contract when API behavior changes.
5. Run the checks relevant to your change:
   - `go test -race ./...`
   - `go vet ./...`
   - `go build ./...`
   - In `web/`: `npm install`, `npm run typecheck`, and `npm run build`
   - For migration changes, apply all up migrations and verify rollback order against PostgreSQL 17.

## Public-repository data rules

This repository must remain safe to publish.

- Use synthetic, non-personal fixtures only.
- Never add an individual's watch history, ratings, notes, preferences, email, or account data.
- Never commit credentials, API keys, tokens, private keys, production dumps, or real environment files.
- Do not add provider payloads or scraped catalog data until the provider's current terms explicitly permit the intended storage and use.
- Keep source attribution, retrieval timestamps, and confidence/provenance metadata for catalog facts.
- Do not copy assets, images, descriptions, or datasets unless their license permits the intended use.

## Engineering principles

- Preserve user-data isolation in every user-scoped query and mutation.
- Do not treat an untrusted client-supplied user ID as authentication.
- Keep LLM output separate from verified catalog facts; recommendations and explanations must be evidence-backed.
- Mark spoiler-sensitive observations and avoid exposing them by default.
- Prefer backward-compatible API changes and explicit database migrations.

## Pull request checklist

- [ ] Tests and relevant checks pass.
- [ ] No secrets or personal data are included.
- [ ] Data and assets have a documented permitted source/license.
- [ ] API documentation and migrations are updated where applicable.
- [ ] Security and multi-user implications have been considered.
