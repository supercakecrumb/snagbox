# Integrating snagbox into a project

This is the copy-paste guide for wiring **issue/feedback reporting** into another
project (a Telegram bot, a web app, a service). Once wired, anything that
surfaces in that project — a user bug report, a "remember this later" note, a
caught error — becomes an issue in snagbox that your agents drain into Obsidian
(or wherever) on their own schedule.

> **For agents/maintainers:** treat snagbox reporting as a standard feature of
> every bot/web project, the same way you'd add logging or a healthcheck. If a
> project can collect feedback or notes from a human, it should file them into
> snagbox. See the checklist at the bottom.

## How it fits

```
your bot / web app  --(ingest token)-->  POST /api/v1/issues  -->  snagbox queue  -->  your agent
```

The only thing your project needs is a **snagbox base URL** and an **ingest
token** scoped to that project.

## 1. One-time setup (in the snagbox admin UI)

1. Open the admin dashboard, go to **Projects → create** a project for this app
   (e.g. `geodrill`).
2. Go to **Ingest tokens → create**, label it (e.g. `geodrill-bot`), and pick
   that project as the target. Copy the token — it's shown once.
3. Give your project two config values (12-factor, same names everywhere):

   ```
   SNAGBOX_URL=https://snagbox.example.com
   SNAGBOX_INGEST_TOKEN=snb_...      # write-only, scoped to this one project
   ```

The ingest token is **write-only** and can only file issues into its one
project — it cannot read the queue, list issues, or touch any other project. Even
so, treat it like any secret: keep it server-side (see the web note below).

## 2. Go projects

Install the client (it's part of the snagbox module, zero non-stdlib deps):

```bash
go get github.com/supercakecrumb/snagbox/client
```

Construct it once at startup and reuse it:

```go
import "github.com/supercakecrumb/snagbox/client"

sb := client.New(os.Getenv("SNAGBOX_URL"), os.Getenv("SNAGBOX_INGEST_TOKEN"))
```

### Telegram bot pattern

Add a `/feedback` command (or an inline button) that files whatever the user
sends. Text-only:

```go
_, err := sb.ReportIssue(ctx, client.ReportRequest{
    Text: update.Message.Text,
    Meta: map[string]any{"tg_user_id": update.Message.From.ID},
})
```

With a photo (download it from Telegram first, then attach the bytes):

```go
_, err := sb.ReportIssue(ctx, client.ReportRequest{
    Text:   caption,
    Photos: []client.Photo{{Filename: "shot.jpg", ContentType: "image/jpeg", Data: photoBytes}},
})
```

### Web / service pattern

Report from a feedback form handler, or automatically on a caught error:

```go
func handleFeedback(w http.ResponseWriter, r *http.Request) {
    _, err := sb.ReportIssue(r.Context(), client.ReportRequest{
        Text: r.FormValue("message"),
        Meta: map[string]any{"path": r.FormValue("path"), "ua": r.UserAgent()},
    })
    if err != nil { /* log it; don't fail the user's request over a report */ }
}
```

`ReportIssue` returns the created `*client.Issue`; a non-2xx response comes back
as a `*client.APIError` you can inspect for `.StatusCode`.

## 3. JavaScript / TypeScript projects

```bash
npm install @supercakecrumb/snagbox
```

```ts
import { SnagboxClient } from "@supercakecrumb/snagbox";

const sb = new SnagboxClient(process.env.SNAGBOX_URL!, process.env.SNAGBOX_INGEST_TOKEN!);

await sb.reportIssue({
  text: "Checkout button broken on mobile",
  meta: { page: "/checkout" },
  // photos optional: [{ filename, contentType, data: Uint8Array | Blob }]
});
```

> **Web security note:** the ingest token is a bearer credential. **Do not put
> it in front-end / browser code.** Use the JS SDK from your Node backend (API
> route, server action, edge function), and have the browser POST feedback to
> *your* backend, which forwards it to snagbox. Same rule for Go: the token
> lives on the server, never in a client bundle.

## 4. Per-project checklist

- [ ] Project + ingest token created in the snagbox admin UI
- [ ] `SNAGBOX_URL` and `SNAGBOX_INGEST_TOKEN` in `.env.example` (placeholders) and wired via config
- [ ] A user-facing way to file an issue (bot `/feedback` command/button, or a web feedback form)
- [ ] Reporting is server-side only — token never reaches the browser
- [ ] Report failures are logged, not fatal to the user's action

That's it. The consume side (draining the queue) is not your project's concern —
that's a snagbox agent (e.g. the `snagbox-sort` Obsidian skill) with its own
agent token.
