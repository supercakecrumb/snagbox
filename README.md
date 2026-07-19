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

## Clients

- Go client: `client/` (coming milestone)
- TypeScript client: `clients/js/` (coming milestone)

## License

MIT, see [LICENSE](LICENSE).
