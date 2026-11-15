# Security Policy

## Supported versions

The default branch is the only actively maintained development line. There are no production releases yet.

## Reporting a vulnerability

Please do not disclose vulnerabilities in public issues or discussions.

Use GitHub's **Report a vulnerability** flow in the repository's Security tab to send a private report to the maintainers. If that option is unavailable, contact the maintainer through their GitHub profile and ask for a private reporting channel; do not post exploit details publicly. Include:
- A clear description and potential impact.
- Reproduction steps or a proof of concept that does not access other users' data.
- Affected commit, component, and environment.
- Any suggested mitigation, if known.

Please allow maintainers reasonable time to investigate and coordinate a fix before public disclosure. Do not include real credentials, personal data, or third-party data in a report.

## Security boundaries

This project is under active development and is not production-ready. JSON mode uses an unauthenticated local development store. Optional PostgreSQL mode implements bearer authentication and transaction-scoped multi-user isolation, but public-release security hardening and review are incomplete. Do not expose it to the public internet or use it to store sensitive information.

Never commit:
- API keys, passwords, access tokens, private keys, session cookies, or real `.env` files.
- Personal watch histories, ratings, notes, email addresses, or other private user data.
- Database dumps, production logs, or private provider payloads.

Use synthetic fixtures and `.env.example` placeholders only. If a secret is committed accidentally, revoke or rotate it immediately; deleting it from the latest commit does not make it safe.
