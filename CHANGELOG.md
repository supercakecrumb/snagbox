# Changelog

All notable changes to snagbox are documented here.


## v0.1.1 - 2026-07-20
### Added
* Integration guide (INTEGRATION.md) and a Go client README showing how to wire snagbox issue reporting into a bot or web project, with a server-side-only token note; refreshed the root README clients section.
* Admin login page shows a "Login with Telegram" button that deep-links to the bot (t.me/<bot>?start=login); the /start login deep link issues a dashboard login link just like /login. The bot's @username is resolved via getMe at startup.
### Fixed
* Bot slash commands (/start, /last, /projects, /login) were registered with a leading slash, which MatchTypeCommand never matches, so every command fell through and got filed as an issue. Commands are now registered slash-less and route correctly, so /login issues a dashboard login link again.

## v0.1.0 - 2026-07-19
### Added
* Server-rendered admin dashboard with msgr-authkit bot-login — CRUD for projects, agents (with per-project permission checkboxes and one-time tokens), ingest tokens, and the Telegram user allowlist, plus an issue browser with photo thumbnails. Adds a /login command to the bot.
* Telegram intake bot — allowlisted users file issues from text and photos (albums grouped into one issue), tag them to a project via inline buttons, and browse with /last and /projects. Photos are downloaded to S3; unknown admins are seeded on first message.
* Core HTTP API — ingest-token issue creation with photo upload to S3, per-agent fan-out queue with independent acks, bulk ack, issue browse, and authenticated attachment streaming. Backed by pgx store repos, a MinIO/S3 blob layer, and a token hashing package.
* Deployment guide (DEPLOY.md) covering Coolify setup, required environment, and local development.
* Optional daily inbox digest (DIGEST_HOUR) that messages admins how many issues are waiting unsorted.
* Dependency-free Go client SDK (client/) for reporting issues, and an integration test suite covering the ingest → fan-out queue → ack round-trip against real Postgres and MinIO.
* TypeScript client SDK (clients/js, @supercakecrumb/snagbox) — a dependency-free, fetch-based reportIssue mirroring the Go client, with JSON and multipart paths and a typed SnagboxError.
* Project skeleton — config, Postgres store with embedded goose migration, health endpoint, Docker/compose (Postgres + MinIO), CI, and the single pre-commit gate.
### Changed
* A failed Telegram bot startup (bad or unreachable token) no longer crashes the service — the API and admin UI keep running and the bot is disabled with an error log.
### Fixed
* CI lint step now uses golangci-lint-action v8, required for the golangci-lint v2 config (v6 rejected it and blocked the release build).
