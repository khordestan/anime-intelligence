# Anime Intelligence development

GitHub is the single source of truth. Start by reading `docs/PROJECT_CONTEXT.md`, then inspect the current code, roadmap, recent commits and GitHub Actions results. Treat historical session reports as hints until checked against the repository. Read scoped instructions before editing their directories.

Prioritize implementation, meaningful tests and fixing failures. Keep `docs/PROJECT_CONTEXT.md` current alongside substantive changes: architecture, implementation status, decisions, security constraints, known issues and next priorities. Record validation in `docs/IMPLEMENTATION_LOG.md`; distinguish local tests, skipped integration tests, pending CI and verified CI. Do not claim a check passed without inspecting its result for the relevant commit.

Keep the repository public. Do not commit secrets, credentials, real account details, personal watch histories or database dumps. Use synthetic fixtures. Maintain authenticated owner-scoped queries, transaction-local forced RLS, separate migration/runtime roles, and account-before-session/proof lock ordering. Keep raw bearer/proof tokens out of logs, persistent browser storage and database plaintext.

Do not deploy publicly until authentication, multi-user isolation and production security are ready and deployment is explicitly authorized. JSON mode is local, single-user and unauthenticated. PostgreSQL tests require a disposable administrator database via `TEST_DATABASE_URL`; without it they skip. Follow the verification commands and launch blockers in the project context rather than assuming a green unit suite covers integration/browser behavior.
