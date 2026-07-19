# @supercakecrumb/snagbox

Dependency-free, fetch-based TypeScript SDK for reporting issues to a
[snagbox](https://github.com/supercakecrumb/snagbox) server.

This client covers the **report (write) path only** — `POST /api/v1/issues`.
Consuming reported issues is plain HTTP and does not require a client
library.

## Install

```bash
npm install @supercakecrumb/snagbox
```

## Usage

```ts
import { SnagboxClient } from "@supercakecrumb/snagbox";

const client = new SnagboxClient("https://snagbox.example.com", "<ingestToken>");

const issue = await client.reportIssue({
  text: "The checkout button is broken on mobile",
  meta: { page: "/checkout", userAgent: "..." },
  photos: [
    {
      filename: "screenshot.png",
      contentType: "image/png",
      data: screenshotBytes, // Uint8Array | ArrayBuffer | Blob
    },
  ],
});

console.log(issue.id, issue.attachments);
```

- `text` and/or `photos` must be provided — an empty issue throws before any
  network call.
- When `photos` is non-empty, the request is sent as `multipart/form-data`.
  Otherwise it is sent as `application/json`.
- Non-2xx responses reject with a `SnagboxError` carrying the HTTP `status`
  and the server's error message.

## Requirements

Node.js >= 18 (relies on the global `fetch`, `FormData`, and `Blob`). No
runtime dependencies.
