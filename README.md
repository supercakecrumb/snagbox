# snagbox

Self-hosted issue-intake service. A Telegram bot and HTTP API collect issues
from anywhere they surface, drop them into per-project queues, and let
coding agents (or humans) drain those queues in order. Designed to pair well
with Obsidian-agent workflows where an agent needs a durable, structured
inbox of things to fix.

Single Go binary, backed by Postgres and MinIO (S3) for attachments, with a
small admin web UI for triage.

## Quickstart

```bash
cp .env.example .env
# edit .env with real values
docker compose up -d
```

The service listens on `PORT` (default `8080`).

## API surface

| Endpoint                        | Description                                  |
|----------------------------------|-----------------------------------------------|
| `POST /api/v1/issues`            | Submit a new issue                            |
| `GET /api/v1/queue`              | Pull the next unacknowledged issue for a project |
| `POST /api/v1/issues/{id}/ack`   | Acknowledge an issue as handled               |
| `GET /api/v1/issues`             | List issues, with filtering                   |
| `GET /api/v1/attachments/{id}`   | Fetch an issue attachment                     |
| `GET /healthz`                   | Liveness/readiness check                      |

## Reporting issues from your projects

Wiring a bot or web app to file issues into snagbox is a two-value setup
(`SNAGBOX_URL` + a project-scoped ingest token) plus one call. See
**[INTEGRATION.md](INTEGRATION.md)** for the full copy-paste guide.

- Go client: [`client/`](client/README.md) — `go get github.com/supercakecrumb/snagbox/client`
- TypeScript client: [`clients/js/`](clients/js/README.md) — `@supercakecrumb/snagbox`

Both are report-only and dependency-free. Draining the queue (the agent side)
is plain HTTP.

## Deployment

See [DEPLOY.md](DEPLOY.md).

## License

MIT, see [LICENSE](LICENSE).
