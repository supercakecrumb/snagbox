# Deploying snagbox

snagbox is a single Go binary plus Postgres and an S3-compatible object store.
It ships as a multi-arch image at `ghcr.io/supercakecrumb/snagbox` (built by CI on
every `v*` tag) and is meant to run under a self-hosted PaaS like Coolify.

## What it needs

- **Postgres 17** (any managed or self-hosted instance). snagbox applies its own
  migrations on startup.
- **S3-compatible storage** for photos — MinIO, or any provider. snagbox creates
  its bucket on startup if missing.
- A **Telegram bot token** from @BotFather.
- A public HTTPS URL (for bot-issued admin login links).

## Environment

See [.env.example](.env.example) for the full list. The required ones:

| Var | Notes |
|-----|-------|
| `TELEGRAM_BOT_TOKEN` | From @BotFather. |
| `ADMIN_TG_IDS` | Comma-separated Telegram user ids seeded as admins. |
| `DATABASE_URL` | `postgres://user:pass@host:5432/snagbox?sslmode=...` |
| `S3_ENDPOINT` / `S3_ACCESS_KEY` / `S3_SECRET_KEY` | Object store. `S3_BUCKET` defaults to `snagbox`. |
| `PUBLIC_BASE_URL` | Public https URL, no trailing slash. |
| `SESSION_ENC_KEY` | Base64 32-byte key (`openssl rand -base64 32`). Required for admin login; without it the API and bot still run but the dashboard login is disabled. |
| `DIGEST_HOUR` | Optional 0–23; daily inbox digest to admins. Unset = off. |

## Coolify

1. **New Resource → Docker Image**, image `ghcr.io/supercakecrumb/snagbox:latest`
   (or pin a version tag).
2. Add a **Postgres** database resource; copy its connection string into `DATABASE_URL`.
3. Add a **MinIO** resource (or point at external S3); set the `S3_*` vars.
4. Set the remaining env vars above. Generate `SESSION_ENC_KEY` with
   `openssl rand -base64 32`.
5. Expose port **8080**; set the domain and let Coolify terminate TLS. Put that
   domain (with `https://`) in `PUBLIC_BASE_URL`.
6. Healthcheck: the container already defines one (`/snagbox healthcheck` → `/healthz`);
   Coolify will honor it.
7. Deploy. On first boot, message the bot `/login` (as an admin id) to open the
   dashboard, then create your projects, agents, and ingest tokens.

## Local development

`docker compose up -d` brings up snagbox + Postgres + MinIO using the same
[.env](.env.example) file (copy it to `.env` first). The app is then on
`http://localhost:8080`.
