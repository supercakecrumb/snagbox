# snagbox Go client

Dependency-free Go SDK for reporting issues to a [snagbox](../README.md) server.
Covers the **report (write) path only** — `POST /api/v1/issues`. Draining the
queue is plain HTTP and needs no client.

## Install

```bash
go get github.com/supercakecrumb/snagbox/client
```

## Usage

```go
import "github.com/supercakecrumb/snagbox/client"

sb := client.New(os.Getenv("SNAGBOX_URL"), os.Getenv("SNAGBOX_INGEST_TOKEN"))

issue, err := sb.ReportIssue(ctx, client.ReportRequest{
    Text: "The checkout button is broken on mobile",
    Meta: map[string]any{"page": "/checkout"},
    Photos: []client.Photo{
        {Filename: "screenshot.png", ContentType: "image/png", Data: pngBytes},
    },
})
```

- `Text` and/or `Photos` must be set — an empty request errors before any network call.
- With photos the request is `multipart/form-data`; otherwise JSON.
- Non-2xx responses come back as a `*client.APIError` (inspect `.StatusCode`).
- Pass `client.WithHTTPClient(hc)` to `New` to supply your own `*http.Client`.

The ingest token is write-only and scoped to one project — keep it server-side.
See [INTEGRATION.md](../INTEGRATION.md) for the full bot/web integration guide.
